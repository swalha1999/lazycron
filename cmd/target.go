package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/record"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

// loadGlobalSettings runs before every command: it applies settings from
// ~/.lazycron/config.yml that shape how jobs are written (the default runner).
func loadGlobalSettings(cmd *cobra.Command, args []string) error {
	humanOut = progress(cmd)
	cfg, err := config.Load()
	if err != nil {
		if cmd == execCmd { // a broken config must not stop a scheduled job
			fmt.Fprintf(cmd.ErrOrStderr(), "lazycron exec: ignoring ~/.lazycron/config.yml: %v\n", err)
			return nil
		}
		return errFailure("load ~/.lazycron/config.yml: %v", err).withHint("fix or remove the file")
	}
	if !config.ValidRunner(cfg.Runner) {
		return errFailure("config: unknown runner %q", cfg.Runner).withHint("use %q or %q", config.RunnerPlain, config.RunnerHerdr)
	}
	cron.DefaultRunner = cfg.DefaultRunner()
	return nil
}

// openBackend returns the backend selected by the global --server and
// --cron-file flags. The caller must Close it.
func openBackend() (backend.Backend, error) {
	return resolveBackend(globalServer)
}

// resolveBackend creates the backend for serverName: a remote server from
// config, the --cron-file test backend, or the local crontab.
func resolveBackend(serverName string) (backend.Backend, error) {
	if serverName != "" {
		return openRemoteBackend(serverName)
	}
	if cronFile != "" {
		return backend.NewFileBackend(cronFile, cronFile+".history"), nil
	}
	if err := cron.CheckCrontabAvailable(); err != nil {
		return nil, errUnavailable("%v", err).withHint("install cron (cronie on Arch, cron on Debian/Ubuntu)")
	}
	if err := record.InstallRecord(); err != nil {
		return nil, errFailure("install record script: %v", err)
	}
	record.InstallSelfLink()
	return backend.NewLocalBackend(), nil
}

// findServer returns the configured server called name.
func findServer(cfg *config.Config, name string) (*config.ServerConfig, error) {
	var names []string
	for i := range cfg.Servers {
		if cfg.Servers[i].Name == name {
			return &cfg.Servers[i], nil
		}
		names = append(names, cfg.Servers[i].Name)
	}
	if len(names) == 0 {
		return nil, errNotFound("server %q not found (no servers configured)", name).withHint("add one with `lazycron server add`")
	}
	return nil, errNotFound("server %q not found", name).withHint("available: %s", strings.Join(names, ", "))
}

// newRemoteBackend builds an unconnected backend for a configured server.
func newRemoteBackend(srv config.ServerConfig) *backend.RemoteBackend {
	client := sshclient.NewClient(srv.Host, srv.PortOrDefault(), srv.User, "", config.ExpandHome(srv.KeyPath), srv.UseAgent)
	remote := backend.NewRemoteBackend(srv.Name, client)
	remote.SetDefaultRunner(srv.Runner)
	return remote
}

// openRemoteBackend connects to the configured server called name.
func openRemoteBackend(name string) (backend.Backend, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, errFailure("load config: %v", err)
	}
	srv, err := findServer(cfg, name)
	if err != nil {
		return nil, err
	}
	if srv.Runner != "" {
		cron.DefaultRunner = srv.Runner
	}
	remote := newRemoteBackend(*srv)
	if err := remote.EnsureRecordScript(); err != nil {
		remote.Close()
		return nil, errUnavailable("connect to %s: %v", name, err).withHint("check it with `lazycron server test %s`", name)
	}
	return remote, nil
}

// serverLabel names the target of the current command for output.
func serverLabel() string {
	switch {
	case globalServer != "":
		return globalServer
	case cronFile != "":
		return "file:" + cronFile
	default:
		return "local"
	}
}

// withBackend opens the selected backend, runs fn and closes it.
func withBackend(fn func(b backend.Backend) error) error {
	b, err := openBackend()
	if err != nil {
		return err
	}
	defer b.Close()
	return fn(b)
}
