package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
)

var runCmd = &cobra.Command{
	Use:   "run <job-name>",
	Short: "Run a cron job immediately",
	Args:  cobra.ExactArgs(1),
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	name := args[0]

	if err := cron.CheckCrontabAvailable(); err != nil {
		return err
	}

	raw, err := cron.ReadCrontab()
	if err != nil {
		return fmt.Errorf("failed to read crontab: %w", err)
	}
	jobs := cron.Parse(raw)

	var target *cron.Job
	for i := range jobs {
		if jobs[i].Name == name {
			target = &jobs[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("job %q not found", name)
	}

	fmt.Printf("Running '%s'...\n", target.Name)

	out, err := exec.Command("sh", "-c", target.Command).CombinedOutput()
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	if err != nil {
		return fmt.Errorf("job failed: %w", err)
	}

	fmt.Printf("Job '%s' completed successfully.\n", target.Name)
	return nil
}
