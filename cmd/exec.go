package cmd

import (
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/runner"
)

var execCmd = &cobra.Command{
	Use:   "exec <job-id>",
	Short: "Run one job and record it (what crontab lines call; prefer `run`)",
	Long: "Runs a job's script through herdr-core (or plainly, as a fallback), writes " +
		"the history entry and applies --once. Crontab lines in herdr mode call this. " +
		"It prints nothing unless --json is given, so cron sends no mail.",
	Args: cobra.ExactArgs(1),
	RunE: runExec,
}

var (
	execName    string
	execRunner  string
	execTrigger string
	execRunID   string
	execOnce    bool
)

func init() {
	execCmd.Flags().StringVar(&execName, "name", "", "job name for the history entry (default: the id)")
	execCmd.Flags().StringVar(&execRunner, "runner", config.RunnerHerdr, "plain or herdr")
	execCmd.Flags().StringVar(&execTrigger, "trigger", runner.TriggerCron, "what started the run: cron or manual")
	execCmd.Flags().StringVar(&execRunID, "run-id", "", "run id to use (default: generated)")
	execCmd.Flags().BoolVar(&execOnce, "once", false, "disable the job after this run")
	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) error {
	if globalServer != "" {
		return errUsage("exec runs on this machine; use `lazycron run --server %s` instead", globalServer)
	}
	if err := validateRunner(execRunner); err != nil {
		return err
	}
	if err := cron.ValidateID(args[0]); err != nil {
		return errUsage("%v", err)
	}
	name := execName
	if name == "" {
		name = args[0]
	}
	job := cron.Job{ID: args[0], Name: name, Runner: execRunner}
	return withBackend(func(b backend.Backend) error {
		entry, err := b.ExecJob(job, backend.ExecRequest{RunID: execRunID, Trigger: execTrigger, Once: execOnce})
		if err != nil {
			return errFailure("%v", err)
		}
		if jsonForced(cmd) {
			return writeJSON(cmd.OutOrStdout(), entry)
		}
		return nil
	})
}
