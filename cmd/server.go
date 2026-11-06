package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/setup"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage remote servers (used with --server on any command)",
}

var serverAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a server (idempotent; a different server under the same name is a conflict)",
	Args:  cobra.NoArgs,
	RunE:  runServerAdd,
}

var serverLsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List configured servers",
	Args:    cobra.NoArgs,
	RunE:    runServerLs,
}

var serverRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a server from the config (its crontab is left alone)",
	Args:  cobra.ExactArgs(1),
	RunE:  runServerRm,
}

var serverTestCmd = &cobra.Command{
	Use:   "test <name>",
	Short: "Connect to a server and check lazycron, herdr-core and cron there",
	Args:  cobra.ExactArgs(1),
	RunE:  runServerTest,
}

var newServer config.ServerConfig

func init() {
	f := serverAddCmd.Flags()
	f.StringVar(&newServer.Name, "name", "", "server name used with --server (required)")
	f.StringVar(&newServer.Host, "host", "", "hostname or IP (required)")
	f.StringVar(&newServer.User, "user", "", "SSH user (required)")
	f.IntVar(&newServer.Port, "port", 22, "SSH port")
	f.StringVar(&newServer.KeyPath, "key", "", "SSH private key path")
	f.BoolVar(&newServer.UseAgent, "agent", false, "use the SSH agent")
	for _, name := range []string{"name", "host", "user"} {
		serverAddCmd.MarkFlagRequired(name)
	}
	addDryRunFlag(serverAddCmd)
	addDryRunFlag(serverRmCmd)
	serverCmd.AddCommand(serverAddCmd, serverLsCmd, serverRmCmd, serverTestCmd)
	rootCmd.AddCommand(serverCmd)
}

// serverResult is the JSON result of server add/rm.
type serverResult struct {
	Action string              `json:"action"`
	DryRun bool                `json:"dry_run"`
	Server config.ServerConfig `json:"server"`
}

func runServerAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return errFailure("load config: %v", err)
	}
	srv := newServer
	if srv.Port == 22 {
		srv.Port = 0 // stored as the default
	}
	res := serverResult{Action: actionCreated, DryRun: dryRun, Server: srv}
	if existing, err := findServer(cfg, srv.Name); err == nil {
		normalized := *existing
		normalized.Runner = ""
		if normalized != srv {
			return errConflict("server %q already exists with different settings", srv.Name).withHint("remove it first with `lazycron server rm %s`", srv.Name)
		}
		res.Action, res.Server = actionUnchanged, *existing
	} else if !dryRun {
		cfg.Servers = append(cfg.Servers, srv)
		if err := config.Save(cfg); err != nil {
			return errFailure("save config: %v", err)
		}
	}
	return render(cmd, res, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "%s server %q (%s@%s)\n", res.Action, srv.Name, srv.User, srv.Host)
		return err
	})
}

func runServerLs(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return errFailure("load config: %v", err)
	}
	servers := cfg.Servers
	if servers == nil {
		servers = []config.ServerConfig{}
	}
	return render(cmd, map[string]any{"servers": servers}, func(w io.Writer) error {
		if len(servers) == 0 {
			fmt.Fprintln(w, "No servers configured. Add one with `lazycron server add`.")
			return nil
		}
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tADDRESS\tRUNNER")
		for _, s := range servers {
			fmt.Fprintf(tw, "%s\t%s@%s:%d\t%s\n", s.Name, s.User, s.Host, s.PortOrDefault(), orDash(s.Runner))
		}
		return tw.Flush()
	})
}

func runServerRm(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return errFailure("load config: %v", err)
	}
	srv, err := findServer(cfg, args[0])
	if err != nil {
		return err
	}
	res := serverResult{Action: actionDeleted, DryRun: dryRun, Server: *srv}
	if !dryRun {
		if err := config.RemoveServer(args[0]); err != nil {
			return errFailure("save config: %v", err)
		}
	}
	return render(cmd, res, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "%s server %q\n", res.Action, res.Server.Name)
		return err
	})
}

func runServerTest(cmd *cobra.Command, args []string) error {
	b, err := openRemoteBackend(args[0])
	if err != nil {
		return err
	}
	defer b.Close()
	globalServer = args[0]
	host, _, err := setupTarget(b)
	if err != nil {
		return errUnavailable("%v", err)
	}
	report := doctorReport{Server: args[0], Runner: config.RunnerPlain, OK: true}
	cfg, _ := config.Load()
	if srv, err := findServer(cfg, args[0]); err == nil && srv.Runner != "" {
		report.Runner = srv.Runner
	}
	report.Checks = append([]setup.Step{{Name: "ssh", Status: setup.StatusOK, Detail: "connected"}},
		setup.Doctor(host, setup.DoctorOptions{Runner: report.Runner})...)
	for _, c := range report.Checks {
		if c.Status == setup.StatusFailed {
			report.OK = false
		}
	}
	return render(cmd, report, func(w io.Writer) error {
		printChecks(w, report)
		return nil
	})
}
