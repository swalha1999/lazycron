package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
)

var getCmd = &cobra.Command{
	Use:   "get <job>",
	Short: "Show one job (by id or name), with its latest run",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

var rmCmd = &cobra.Command{
	Use:     "rm <job>",
	Aliases: []string{"delete"},
	Short:   "Delete a job",
	Args:    cobra.ExactArgs(1),
	RunE:    runRm,
}

var enableCmd = &cobra.Command{
	Use:   "enable <job>",
	Short: "Enable a job (no-op if already enabled)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return mutateJobs(cmd, setEnabled(args[0], true))
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable <job>",
	Short: "Disable a job without deleting it (no-op if already disabled)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return mutateJobs(cmd, setEnabled(args[0], false))
	},
}

func init() {
	for _, c := range []*cobra.Command{rmCmd, enableCmd, disableCmd} {
		addDryRunFlag(c)
	}
	rootCmd.AddCommand(getCmd, rmCmd, enableCmd, disableCmd)
}

// jobDetail is the JSON shape of `get`: the job plus its latest run.
type jobDetail struct {
	jobView
	Server  string   `json:"server"`
	LastRun *runView `json:"last_run"`
}

func runGet(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		jobs, err := b.ReadJobs()
		if err != nil {
			return errUnavailable("read jobs: %v", err)
		}
		i, err := findJob(jobs, args[0])
		if err != nil {
			return err
		}
		detail := jobDetail{jobView: toJobView(jobs[i]), Server: serverLabel()}
		if runs, err := listRuns(b, jobs[i].ID); err == nil && len(runs) > 0 {
			detail.LastRun = &runs[0]
		}
		return render(cmd, detail, func(w io.Writer) error {
			printJobDetail(w, detail)
			return nil
		})
	})
}

func printJobDetail(w io.Writer, d jobDetail) {
	status := "enabled"
	if !d.Enabled {
		status = "disabled"
	}
	fmt.Fprintf(w, "ID:        %s\n", d.ID)
	fmt.Fprintf(w, "Name:      %s\n", d.Name)
	fmt.Fprintf(w, "Status:    %s\n", status)
	fmt.Fprintf(w, "Schedule:  %s (%s)\n", d.Schedule, d.ScheduleHuman)
	fmt.Fprintf(w, "Runner:    %s\n", d.Runner)
	if d.Project != "" {
		fmt.Fprintf(w, "Project:   %s\n", d.Project)
	}
	if len(d.NextRuns) > 0 {
		fmt.Fprintf(w, "Next run:  %s\n", d.NextRuns[0])
	}
	fmt.Fprintf(w, "Command:\n  %s\n", d.Command)
	if d.LastRun != nil {
		fmt.Fprintf(w, "Last run:  %s %s (%s)\n", d.LastRun.ID, d.LastRun.State, d.LastRun.StartedAt)
	}
}

func runRm(cmd *cobra.Command, args []string) error {
	return mutateJobs(cmd, func(jobs []cron.Job) ([]cron.Job, mutationResult, error) {
		i, err := findJob(jobs, args[0])
		if err != nil {
			return nil, mutationResult{}, err
		}
		res := mutationResult{Action: actionDeleted, Job: toJobView(jobs[i])}
		return append(jobs[:i:i], jobs[i+1:]...), res, nil
	})
}

func setEnabled(ref string, enabled bool) jobMutation {
	return updateJob(ref, func(j *cron.Job) error {
		j.Enabled = enabled
		return nil
	})
}
