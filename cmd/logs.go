package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/history"
)

var logsCmd = &cobra.Command{
	Use:   "logs <job-or-run>",
	Short: "Show a run's output, or a job's recent runs",
	Long: "With a run id, prints that run's output (live from its herdr terminal while it " +
		"runs). With a job, lists its recent runs; -o prints the newest run's output.",
	Args: cobra.ExactArgs(1),
	RunE: runLogs,
}

var (
	logsCount  int
	logsOutput bool
)

func init() {
	logsCmd.Flags().StringVarP(&globalServer, "server", "s", "", "target server name from config")
	logsCmd.Flags().IntVarP(&logsCount, "n", "n", 10, "number of entries to show")
	logsCmd.Flags().BoolVarP(&logsOutput, "output", "o", false, "show full output for the last run")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	ref := args[0]
	return withBackend(func(b backend.Backend) error {
		run, rec, err := resolveRun(b, ref)
		if err != nil {
			return noRunsYet(cmd, b, ref, err)
		}
		if logsOutput || run.ID == ref {
			return printRunOutput(cmd, b, run, rec)
		}
		return printJobRuns(cmd, rec, run.JobID)
	})
}

// noRunsYet answers `logs <job>` for a job with no runs yet with an
// empty list (as before); anything else is the original error.
func noRunsYet(cmd *cobra.Command, b backend.Backend, ref string, err error) error {
	jobs, rerr := b.ReadJobs()
	if rerr != nil {
		return err
	}
	if _, ferr := findJob(jobs, ref); ferr != nil {
		return err
	}
	return render(cmd, map[string]any{"runs": []runView{}}, func(w io.Writer) error {
		_, werr := fmt.Fprintf(w, "No history found for %q\n", ref)
		return werr
	})
}

// printRunOutput prints one run with its output.
func printRunOutput(cmd *cobra.Command, b backend.Backend, run runView, rec runRecords) error {
	output := rec.entries[run.ID].Output
	if run.State == stateRunning {
		output = liveOutput(b, run)
	}
	run.Output = &output
	return render(cmd, run, func(w io.Writer) error {
		text := strings.TrimSpace(output)
		if text == "" {
			text = "(no output)"
		}
		_, err := fmt.Fprintln(w, text)
		return err
	})
}

// liveOutput reads a running run's terminal; plain runs have no live output.
func liveOutput(b backend.Backend, run runView) string {
	if run.Terminal == "" {
		return ""
	}
	hc, err := b.Herdr()
	if err != nil {
		return ""
	}
	r, err := hc.Read(run.Terminal, 2000)
	if err != nil {
		return ""
	}
	return r.Text
}

// printJobRuns lists a job's recent runs in the classic logs format.
func printJobRuns(cmd *cobra.Command, rec runRecords, jobID string) error {
	var runs []runView
	for _, r := range rec.views {
		if r.JobID == jobID {
			runs = append(runs, r)
		}
	}
	if logsCount > 0 && len(runs) > logsCount {
		runs = runs[:logsCount]
	}
	return render(cmd, map[string]any{"runs": runs}, func(w io.Writer) error {
		for _, r := range runs {
			if r.State == stateRunning {
				fmt.Fprintf(w, "%s  …  running (%s)\n", r.StartedAt, r.ID)
				continue
			}
			e := rec.entries[r.ID]
			ts, parsed := formatTimestamp(e.Timestamp)
			fmt.Fprintf(w, "%s  %s  %s\n", ts, formatStatus(e.Success), formatDetail(e, parsed))
		}
		return nil
	})
}

// formatTimestamp parses the RFC3339 timestamp and returns the formatted date
// and the duration string if two consecutive entries can be compared.
func formatTimestamp(raw string) (formatted string, parsed time.Time) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw, time.Time{}
	}
	return t.Format("2006-01-02 15:04"), t
}

func formatStatus(success *bool) string {
	if success == nil {
		return "?"
	}
	if *success {
		return "\u2713"
	}
	return "\u2717"
}

func formatDetail(e history.Entry, ts time.Time) string {
	if e.Success != nil && !*e.Success {
		// Show first line of output as error summary.
		output := strings.TrimSpace(e.Output)
		if output != "" {
			if idx := strings.IndexByte(output, '\n'); idx > 0 {
				output = output[:idx]
			}
			if len(output) > 80 {
				output = output[:77] + "..."
			}
			return output
		}
		return "(failed)"
	}
	return ""
}
