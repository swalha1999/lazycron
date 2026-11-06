package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
)

var runsCmd = &cobra.Command{
	Use:   "runs [job]",
	Short: "List runs (in progress and finished), newest first",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRuns,
}

var runsLimit int

func init() {
	runsCmd.Flags().IntVarP(&runsLimit, "limit", "n", 20, "show at most this many runs (0: all)")
	rootCmd.AddCommand(runsCmd)
}

func runRuns(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		jobID := ""
		if len(args) == 1 {
			jobs, err := b.ReadJobs()
			if err != nil {
				return errUnavailable("read jobs: %v", err)
			}
			i, err := findJob(jobs, args[0])
			if err != nil {
				return err
			}
			jobID = jobs[i].ID
		}
		runs, err := listRuns(b, jobID)
		if err != nil {
			return err
		}
		if runsLimit > 0 && len(runs) > runsLimit {
			runs = runs[:runsLimit]
		}
		if runs == nil {
			runs = []runView{}
		}
		return render(cmd, map[string]any{"server": serverLabel(), "runs": runs}, func(w io.Writer) error {
			return printRunTable(w, runs)
		})
	})
}

func printRunTable(w io.Writer, runs []runView) error {
	if len(runs) == 0 {
		fmt.Fprintln(w, "No runs yet.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "RUN\tJOB\tSTATE\tEXIT\tRUNNER\tTERMINAL\tSTARTED")
	for _, r := range runs {
		exit := "-"
		if r.ExitCode != nil {
			exit = fmt.Sprint(*r.ExitCode)
		} else if r.Signal != 0 {
			exit = fmt.Sprintf("sig %d", r.Signal)
		}
		started := r.StartedAt
		if started == "" {
			started = r.FinishedAt
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.JobName, r.State, exit,
			orDash(r.Runner), orDash(r.Terminal), started)
	}
	return tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
