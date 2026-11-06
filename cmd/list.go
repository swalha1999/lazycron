package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
)

var listCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List all cron jobs",
	Args:    cobra.NoArgs,
	RunE:    runList,
}

var listProject string

func init() {
	listCmd.Flags().StringVarP(&listProject, "project", "p", "", "only jobs in this project group")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		jobs, err := b.ReadJobs()
		if err != nil {
			return errUnavailable("read jobs: %v", err)
		}
		if listProject != "" {
			jobs = filterByProject(jobs, listProject)
		}
		return render(cmd, map[string]any{"server": serverLabel(), "jobs": toJobViews(jobs)}, func(w io.Writer) error {
			return printJobTable(w, jobs)
		})
	})
}

func printJobTable(w io.Writer, jobs []cron.Job) error {
	if len(jobs) == 0 {
		fmt.Fprintln(w, "No cron jobs found.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tNAME\tPROJECT\tRUNNER\tSCHEDULE\tCOMMAND")
	for _, job := range jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		project := job.Project
		if project == "" {
			project = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", job.ID, status, job.Name, project,
			job.EffectiveRunner(), job.Schedule, truncate(job.Command, 60))
	}
	return tw.Flush()
}

// truncate shortens s to at most n bytes, marking the cut with "...".
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
