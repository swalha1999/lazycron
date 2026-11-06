package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/setup"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that scheduled jobs can run: cron, herdr-core, service, linger, claude",
	Long: "Read-only health check of this machine (or --server). Exits 1 when a check " +
		"failed; warnings do not change the exit code. Asking herdr-core for its status " +
		"starts a core if none is running, as any hc command does.",
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

// doctorReport is the JSON result of doctor.
type doctorReport struct {
	Server string       `json:"server"`
	Runner string       `json:"runner"`
	OK     bool         `json:"ok"`
	Checks []setup.Step `json:"checks"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		host, _, err := setupTarget(b)
		if err != nil {
			return errUnavailable("%v", err)
		}
		opts := setup.DoctorOptions{Runner: cron.DefaultRunner}
		if globalServer == "" {
			if cfg, err := config.Load(); err == nil {
				opts.HCPath = cfg.Herdr.Bin
			}
		}
		report := doctorReport{Server: serverLabel(), Runner: cron.DefaultRunner}
		report.Checks = append(setup.Doctor(host, opts), lazycronChecks(b)...)
		report.OK = true
		failed := 0
		for _, c := range report.Checks {
			if c.Status == setup.StatusFailed {
				report.OK = false
				failed++
			}
		}
		if err := render(cmd, report, func(w io.Writer) error {
			printChecks(w, report)
			return nil
		}); err != nil {
			return err
		}
		if !report.OK {
			return errFailure("doctor found %d problem(s)", failed).withHint("see the hints above; `lazycron setup` fixes most")
		}
		return nil
	})
}

// lazycronChecks inspects lazycron's own state on the backend.
func lazycronChecks(b backend.Backend) []setup.Step {
	var checks []setup.Step
	jobs, err := b.ReadJobs()
	switch {
	case err != nil:
		checks = append(checks, setup.Step{Name: "jobs", Status: setup.StatusFailed, Detail: err.Error()})
	default:
		stale := 0
		for _, j := range jobs {
			if j.NeedsRewrite() {
				stale++
			}
		}
		st := setup.Step{Name: "jobs", Status: setup.StatusOK, Detail: fmt.Sprintf("%d jobs", len(jobs))}
		if stale > 0 {
			st.Status = setup.StatusWarning
			st.Detail = fmt.Sprintf("%d of %d jobs are not in their runner's crontab format", stale, len(jobs))
			st.Hint = "run `lazycron setup` to rewrite them"
		}
		checks = append(checks, st)
	}
	if active, err := b.ActiveRuns(); err == nil {
		lost := 0
		for _, a := range active {
			if a.Stale {
				lost++
			}
		}
		st := setup.Step{Name: "runs", Status: setup.StatusOK, Detail: fmt.Sprintf("%d in progress", len(active)-lost)}
		if lost > 0 {
			st.Status = setup.StatusWarning
			st.Detail = fmt.Sprintf("%d run(s) lost their lazycron process (machine restarted mid-run?)", lost)
			st.Hint = "see `lazycron runs`; their terminals may still be readable with `lazycron logs <run>`"
		}
		checks = append(checks, st)
	}
	return checks
}

func printChecks(w io.Writer, r doctorReport) {
	fmt.Fprintf(w, "Doctor for %s (default runner: %s)\n", r.Server, r.Runner)
	for _, c := range r.Checks {
		fmt.Fprintf(w, "  %-8s %-18s %s\n", c.Status, c.Name, c.Detail)
		if c.Hint != "" {
			fmt.Fprintf(w, "  %-8s %-18s hint: %s\n", "", "", c.Hint)
		}
	}
}
