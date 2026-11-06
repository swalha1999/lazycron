package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
)

var killCmd = &cobra.Command{
	Use:   "kill <run-or-job>",
	Short: "Stop a running run (or close a finished run's held terminal)",
	Long: "Ends a run in progress: its herdr terminal, or a plain run's process group. " +
		"For a finished herdr run whose terminal is still held open, closes that terminal.",
	Args: cobra.ExactArgs(1),
	RunE: runKill,
}

func init() {
	addDryRunFlag(killCmd)
	rootCmd.AddCommand(killCmd)
}

// killResult is the JSON result of kill.
type killResult struct {
	Action string  `json:"action"` // killed, closed_terminal, unchanged
	DryRun bool    `json:"dry_run"`
	Run    runView `json:"run"`
}

func runKill(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		run, _, err := resolveRun(b, args[0])
		if err != nil {
			return err
		}
		res, err := killRun(b, run)
		if err != nil {
			return err
		}
		return render(cmd, res, func(w io.Writer) error {
			prefix := ""
			if res.DryRun {
				prefix = "[dry-run] "
			}
			_, err := fmt.Fprintf(w, "%s%s: run %s of '%s'\n", prefix, res.Action, run.ID, run.JobName)
			return err
		})
	})
}

// killRun ends a run (or its held terminal), honouring --dry-run.
func killRun(b backend.Backend, run runView) (killResult, error) {
	res := killResult{DryRun: dryRun, Run: run}
	switch {
	case run.Terminal != "":
		hc, err := b.Herdr()
		if err != nil {
			return res, errUnavailable("%v", err)
		}
		term, err := hc.Get(run.Terminal)
		if err != nil {
			return res, errNotFound("terminal %s of run %s: %v", run.Terminal, run.ID, err)
		}
		if !term.Live() {
			res.Action = actionUnchanged
			return res, nil
		}
		res.Action = "killed"
		if run.finished() {
			res.Action = "closed_terminal"
		}
		if !dryRun {
			if err := hc.Kill(run.Terminal); err != nil {
				return res, errFailure("%v", err)
			}
		}
	case run.State == stateRunning:
		pid := runPID(b, run.ID)
		if pid == 0 {
			return res, errConflict("run %s has no process to kill", run.ID)
		}
		res.Action = "killed"
		if !dryRun {
			if err := b.KillProcessGroup(pid); err != nil {
				return res, errFailure("kill run %s: %v", run.ID, err)
			}
		}
	default:
		res.Action = actionUnchanged
	}
	return res, nil
}

// runPID returns a plain run's process group from its marker.
func runPID(b backend.Backend, runID string) int {
	active, err := b.ActiveRuns()
	if err != nil {
		return 0
	}
	for _, a := range active {
		if a.RunID == runID {
			return a.PID
		}
	}
	return 0
}
