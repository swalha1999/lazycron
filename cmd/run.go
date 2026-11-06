package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/runner"
)

var runCmd = &cobra.Command{
	Use:   "run <job>",
	Short: "Run a job now (through herdr-core when the job's runner is herdr)",
	Long: "Runs a job immediately and records it in history like a scheduled run. " +
		"By default it waits and prints the result; --no-wait returns the run id at " +
		"once (follow it with `lazycron wait <run>`). Exits 1 when the job fails.",
	Args: cobra.ExactArgs(1),
	RunE: runRun,
}

var runNoWait bool

func init() {
	runCmd.Flags().BoolVar(&runNoWait, "no-wait", false, "start the run in the background and print its id")
	runCmd.Flags().BoolVar(&runNoWait, "bg", false, "alias for --no-wait")
	addDryRunFlag(runCmd)
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		jobs, err := b.ReadJobs()
		if err != nil {
			return errUnavailable("read jobs: %v", err)
		}
		i, err := findJob(jobs, args[0])
		if err != nil {
			return err
		}
		job := jobs[i]
		if dryRun {
			return render(cmd, map[string]any{"dry_run": true, "job": toJobView(job)}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "[dry-run] would run %q (%s) with the %s runner\n", job.Name, job.ID, job.EffectiveRunner())
				return err
			})
		}
		if runNoWait {
			return startInBackground(cmd, b, job)
		}
		return runAndReport(cmd, b, job)
	})
}

// runAndReport runs the job, waits, prints the run and fails if the job did.
func runAndReport(cmd *cobra.Command, b backend.Backend, job cron.Job) error {
	if !wantJSON(cmd) {
		fmt.Fprintf(cmd.OutOrStdout(), "Running '%s' (%s runner)...\n", job.Name, job.EffectiveRunner())
	}
	entry, err := b.ExecJob(job, backend.ExecRequest{Trigger: runner.TriggerManual})
	if err != nil {
		return errFailure("run %s: %v", job.ID, err)
	}
	view := runFromEntry(entry)
	view.Output = &entry.Output
	if err := render(cmd, view, func(w io.Writer) error {
		printRunResult(w, view)
		return nil
	}); err != nil {
		return err
	}
	return runOutcomeError(view)
}

// runOutcomeError turns a failed run into the job_failed error (exit 1).
func runOutcomeError(v runView) error {
	if v.State == stateSucceeded {
		return nil
	}
	return errJobFailed("job %q %s", v.JobName, describeEnd(v)).withHint("see `lazycron logs %s`", v.ID)
}

// describeEnd says how a finished run ended, in words.
func describeEnd(v runView) string {
	switch {
	case v.State == stateSucceeded:
		return "succeeded"
	case v.Signal != 0:
		return fmt.Sprintf("was killed (signal %d)", v.Signal)
	case v.ExitCode != nil:
		return fmt.Sprintf("failed with exit code %d", *v.ExitCode)
	default:
		return "ended without an exit code (" + v.State + ")"
	}
}

func printRunResult(w io.Writer, v runView) {
	if v.Output != nil && *v.Output != "" {
		fmt.Fprintln(w, *v.Output)
	}
	if v.FallbackReason != "" {
		fmt.Fprintf(w, "(ran plainly: %s)\n", v.FallbackReason)
	}
	fmt.Fprintf(w, "Job '%s' %s. Run: %s", v.JobName, describeEnd(v), v.ID)
	if v.Terminal != "" {
		fmt.Fprintf(w, "  Terminal: %s", v.Terminal)
	}
	fmt.Fprintln(w)
}

// startTimeout bounds how long --no-wait waits for the run to register.
var startTimeout = 15 * time.Second

// startInBackground starts a detached run and prints it once it registered.
func startInBackground(cmd *cobra.Command, b backend.Backend, job cron.Job) error {
	runID := runner.NewRunID(job.ID, time.Now())
	if err := b.StartJob(job, backend.ExecRequest{RunID: runID, Trigger: runner.TriggerManual}); err != nil {
		return errFailure("start %s: %v", job.ID, err)
	}
	view, err := awaitRegistered(b, runID)
	if err != nil {
		return err
	}
	return render(cmd, view, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "Started '%s'. Run: %s (follow with `lazycron wait %s`)\n", job.Name, view.ID, view.ID)
		return err
	})
}

// awaitRegistered polls until runID shows up as active or finished.
func awaitRegistered(b backend.Backend, runID string) (runView, error) {
	deadline := time.Now().Add(startTimeout)
	for {
		rec, err := loadRuns(b, "")
		if err != nil {
			return runView{}, err
		}
		for _, r := range rec.views {
			if r.ID == runID {
				return r, nil
			}
		}
		if time.Now().After(deadline) {
			return runView{}, errFailure("run %s did not start within %s", runID, startTimeout).withHint("check `lazycron doctor`")
		}
		time.Sleep(pollInterval(b))
	}
}

// pollInterval is how often to re-read run state: slower over SSH.
func pollInterval(b backend.Backend) time.Duration {
	if _, remote := b.(*backend.RemoteBackend); remote {
		return 3 * time.Second
	}
	return 500 * time.Millisecond
}
