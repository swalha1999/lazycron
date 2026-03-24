package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new cron job",
	Long:  "Add a new cron job non-interactively with flags.",
	RunE:  runAdd,
}

var (
	addName     string
	addSchedule string
	addCommand  string
	addWorkDir  string
	addProject  string
	addDisabled bool
	addOnce     bool
)

func init() {
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Job name (required)")
	addCmd.Flags().StringVarP(&addSchedule, "schedule", "s", "", "Cron schedule or human-readable (required)")
	addCmd.Flags().StringVarP(&addCommand, "command", "c", "", "Command to run (required)")
	addCmd.Flags().StringVarP(&addWorkDir, "workdir", "w", "", "Working directory (optional)")
	addCmd.Flags().StringVarP(&addProject, "project", "p", "", "Project group (optional)")
	addCmd.Flags().BoolVar(&addDisabled, "disabled", false, "Create the job in disabled state")
	addCmd.Flags().BoolVar(&addOnce, "once", false, "Run once at the specified datetime and then disable")

	addCmd.MarkFlagRequired("name")
	addCmd.MarkFlagRequired("schedule")
	addCmd.MarkFlagRequired("command")

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	if err := cron.CheckCrontabAvailable(); err != nil {
		return err
	}

	var cronExpr string
	if addOnce {
		expr, _, err := cron.DatetimeToCron(addSchedule)
		if err != nil {
			return fmt.Errorf("invalid datetime %q: %w", addSchedule, err)
		}
		cronExpr = expr
	} else {
		cronExpr = cron.HumanToCron(addSchedule)
		if err := cron.ValidateCron(cronExpr); err != nil {
			return fmt.Errorf("invalid schedule %q: %w", addSchedule, err)
		}
	}

	finalCmd := addCommand
	if addWorkDir != "" {
		finalCmd = fmt.Sprintf("cd %s && %s", addWorkDir, addCommand)
	}

	job := cron.Job{
		ID:       cron.GenerateID(),
		Name:     addName,
		Schedule: cronExpr,
		Command:  finalCmd,
		Enabled:  !addDisabled,
		Wrapped:  true,
		OneShot:  addOnce,
		Project:  addProject,
	}

	raw, err := cron.ReadCrontab()
	if err != nil {
		return fmt.Errorf("failed to read crontab: %w", err)
	}
	jobs := cron.Parse(raw)

	jobs = append(jobs, job)
	if err := cron.WriteCrontab(jobs); err != nil {
		return fmt.Errorf("failed to write crontab: %w", err)
	}

	fmt.Printf("Created job '%s' with schedule: %s\n", job.Name, cronExpr)
	return nil
}
