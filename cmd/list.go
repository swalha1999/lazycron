package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cron jobs",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	if err := cron.CheckCrontabAvailable(); err != nil {
		return err
	}

	raw, err := cron.ReadCrontab()
	if err != nil {
		return fmt.Errorf("failed to read crontab: %w", err)
	}
	jobs := cron.Parse(raw)

	if len(jobs) == 0 {
		fmt.Println("No cron jobs found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tNAME\tSCHEDULE\tCOMMAND")
	fmt.Fprintln(w, "------\t----\t--------\t-------")

	for _, job := range jobs {
		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}
		cmdStr := job.Command
		if len(cmdStr) > 60 {
			cmdStr = cmdStr[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", status, job.Name, job.Schedule, cmdStr)
	}

	return w.Flush()
}
