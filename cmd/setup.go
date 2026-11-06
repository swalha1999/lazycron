package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/runner"
	"github.com/swalha1999/lazycron/setup"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Prepare this machine (or --server) to run jobs through herdr-core",
	Long: "Idempotently: installs herdr-core with its official installer if missing " +
		"(herdr is a separate project; opt out with --no-herdr or LAZYCRON_NO_HERDR=1), " +
		"links ~/.lazycron/bin/lazycron for crontab lines, installs a user service that " +
		"starts the core at boot (systemd on Linux, launchd on macOS), enables linger so it " +
		"runs with nobody logged in, sets the herdr runner as default and rewrites the " +
		"crontab in the herdr format. Never stops, restarts or updates a running core.",
	Args: cobra.NoArgs,
	RunE: runSetup,
}

var setupOpts struct {
	noHerdr   bool
	noService bool
	noLinger  bool
	runner    string
}

func init() {
	f := setupCmd.Flags()
	f.BoolVar(&setupOpts.noHerdr, "no-herdr", false, "do not install herdr-core (also LAZYCRON_NO_HERDR=1)")
	f.BoolVar(&setupOpts.noService, "no-service", false, "do not install the boot service")
	f.BoolVar(&setupOpts.noLinger, "no-linger", false, "do not enable systemd linger")
	f.StringVar(&setupOpts.runner, "runner", "", "default runner to set: herdr (when herdr-core is ready) or plain")
	addDryRunFlag(setupCmd)
	rootCmd.AddCommand(setupCmd)
}

// setupReport is the JSON result of setup.
type setupReport struct {
	setup.Result
	Server string `json:"server"`
	DryRun bool   `json:"dry_run"`
	Runner string `json:"runner"`
}

func runSetup(cmd *cobra.Command, args []string) error {
	if err := validateRunner(setupOpts.runner); err != nil {
		return err
	}
	return withBackend(func(b backend.Backend) error {
		host, lazycronPath, err := setupTarget(b)
		if err != nil {
			return errUnavailable("%v", err)
		}
		res := setup.Run(host, setup.Options{
			DryRun:       dryRun,
			InstallHerdr: !setupOpts.noHerdr && os.Getenv("LAZYCRON_NO_HERDR") != "1",
			Service:      !setupOpts.noService,
			Linger:       !setupOpts.noLinger,
			LazycronPath: lazycronPath,
		})
		report := setupReport{Result: res, Server: serverLabel(), DryRun: dryRun}
		report.Runner = chooseRunner(res.HerdrReady)
		report.Steps = append(report.Steps, saveRunnerSetting(report.Runner))
		report.Steps = append(report.Steps, rewriteCrontab(b, report.Runner))
		if globalServer == "" && report.Runner == config.RunnerHerdr && !dryRun {
			if cfg, err := config.Load(); err == nil {
				runner.NameAppSpace(cfg.Herdr.SpaceID, config.DefaultSpaceName)
			}
		}
		if err := render(cmd, report, func(w io.Writer) error {
			printSetup(w, report)
			return nil
		}); err != nil {
			return err
		}
		if report.Failed() {
			return errFailure("setup did not complete").withHint("fix the failed steps and run `lazycron setup` again")
		}
		return nil
	})
}

// localSetupHost is this machine for setup; tests replace it.
var localSetupHost = func() setup.Host { return setup.LocalHost{} }

// setupTarget returns the host to set up and the lazycron binary to link.
// With --cron-file the host is still this machine; only the crontab step
// targets the file.
func setupTarget(b backend.Backend) (setup.Host, string, error) {
	if rb, ok := b.(*backend.RemoteBackend); ok {
		h, err := newSSHHost(rb.Client())
		return h, "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return localSetupHost(), "", nil
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return localSetupHost(), exe, nil
}

// chooseRunner picks the default runner setup leaves in place.
func chooseRunner(herdrReady bool) string {
	if setupOpts.runner != "" {
		return setupOpts.runner
	}
	if herdrReady {
		return config.RunnerHerdr
	}
	return config.RunnerPlain
}

// saveRunnerSetting stores the runner as the default (locally) or as the
// server's runner (with --server), plus a stable herdr space id.
func saveRunnerSetting(runnerName string) setup.Step {
	step := setup.Step{Name: "config", Detail: "runner: " + runnerName}
	cfg, err := config.Load()
	if err != nil {
		step.Status, step.Detail = setup.StatusFailed, err.Error()
		return step
	}
	changed := cfg.EnsureSpaceID()
	if globalServer == "" {
		changed = changed || cfg.Runner != runnerName
		cfg.Runner = runnerName
	} else {
		srv, err := findServer(cfg, globalServer)
		if err != nil {
			step.Status, step.Detail = setup.StatusFailed, err.Error()
			return step
		}
		changed = changed || srv.Runner != runnerName
		srv.Runner = runnerName
		step.Detail = fmt.Sprintf("servers.%s.runner: %s", globalServer, runnerName)
	}
	switch {
	case !changed:
		step.Status = setup.StatusOK
	case dryRun:
		step.Status = setup.StatusWouldChange
	default:
		if err := config.Save(cfg); err != nil {
			step.Status, step.Detail = setup.StatusFailed, err.Error()
			return step
		}
		step.Status = setup.StatusChanged
	}
	cron.DefaultRunner = runnerName
	return step
}

// rewriteCrontab writes the jobs back when any line is not in the format
// of its runner (herdr exec line or plain record line). Ids, names and
// commands are preserved.
func rewriteCrontab(b backend.Backend, runnerName string) setup.Step {
	step := setup.Step{Name: "crontab"}
	jobs, err := b.ReadJobs()
	if err != nil {
		step.Status, step.Detail = setup.StatusFailed, err.Error()
		return step
	}
	pending := 0
	for _, j := range jobs {
		if j.NeedsRewrite() {
			pending++
		}
	}
	step.Detail = fmt.Sprintf("%d of %d jobs to rewrite (default runner %s)", pending, len(jobs), runnerName)
	switch {
	case pending == 0:
		step.Status = setup.StatusOK
	case dryRun:
		step.Status = setup.StatusWouldChange
	default:
		if err := b.WriteJobs(jobs); err != nil {
			step.Status, step.Detail = setup.StatusFailed, err.Error()
			return step
		}
		step.Status = setup.StatusChanged
	}
	return step
}

func printSetup(w io.Writer, r setupReport) {
	if r.DryRun {
		fmt.Fprintln(w, "[dry-run] nothing was changed")
	}
	fmt.Fprintf(w, "Setup of %s:\n", r.Server)
	for _, s := range r.Steps {
		fmt.Fprintf(w, "  %-13s %-18s %s\n", s.Status, s.Name, s.Detail)
		if s.Hint != "" {
			fmt.Fprintf(w, "  %-13s %-18s hint: %s\n", "", "", s.Hint)
		}
	}
	fmt.Fprintf(w, "Default runner: %s\n", r.Runner)
}
