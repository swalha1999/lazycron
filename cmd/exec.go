package cmd

import (
	"fmt"

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
		"It prints nothing unless --json is given, so cron sends no mail. It exits non-zero " +
		"only when it could not start the job; the crontab line then runs the job plainly.",
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
	entry, err := execBackend().ExecJob(job, backend.ExecRequest{RunID: execRunID, Trigger: execTrigger, Once: execOnce})
	switch {
	case err != nil && entry.RunID == "":
		return errFailure("could not start %s: %v", job.ID, err)
	case err != nil:
		// The job ran; only recording it failed. Exiting non-zero here would
		// make the crontab line run the job a second time.
		fmt.Fprintf(cmd.ErrOrStderr(), "lazycron exec: %s ran but %v\n", job.ID, err)
	}
	if jsonForced(cmd) {
		return writeJSON(cmd.OutOrStdout(), entry)
	}
	return nil
}

// execBackend is the backend exec runs on. Unlike other commands it needs
// neither crontab on PATH nor the record script: under cron it must start
// the job whatever else is wrong.
func execBackend() backend.Backend {
	if cronFile != "" {
		return backend.NewFileBackend(cronFile, cronFile+".history")
	}
	return backend.NewLocalBackend()
}
