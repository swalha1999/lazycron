package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
)

var waitCmd = &cobra.Command{
	Use:   "wait <run-or-job>",
	Short: "Wait until a run ends and print its result",
	Long: "Waits for a run (by run id, or a job's newest run) to finish. Exits 0 when it " +
		"succeeded, 1 (job_failed) when it failed, 6 (timeout) when --timeout passes first.",
	Args: cobra.ExactArgs(1),
	RunE: runWait,
}

var waitTimeout time.Duration

func init() {
	waitCmd.Flags().DurationVar(&waitTimeout, "timeout", 0, "give up after this long, e.g. 10m (0: wait forever)")
	rootCmd.AddCommand(waitCmd)
}

func runWait(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		run, err := waitForRun(b, args[0], waitTimeout)
		if err != nil {
			return err
		}
		if err := render(cmd, run, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, "Run %s of '%s' %s.\n", run.ID, run.JobName, describeEnd(run))
			return err
		}); err != nil {
			return err
		}
		return runOutcomeError(run)
	})
}

// waitForRun polls until the run ref names has finished.
func waitForRun(b backend.Backend, ref string, timeout time.Duration) (runView, error) {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	run, _, err := resolveRun(b, ref)
	if err != nil {
		return runView{}, err
	}
	for !run.finished() {
		if !deadline.IsZero() && time.Now().After(deadline) {
			return run, errTimeout("run %s still running after %s", run.ID, timeout).withHint("wait again; the run keeps going")
		}
		time.Sleep(pollInterval(b))
		if run, _, err = resolveRun(b, run.ID); err != nil {
			return runView{}, err
		}
	}
	return run, nil
}
