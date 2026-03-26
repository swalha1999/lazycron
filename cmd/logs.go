package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/history"
)

var logsCmd = &cobra.Command{
	Use:   "logs <job-name-or-id>",
	Short: "View execution history for a specific job",
	Long:  "Show recent execution history for a cron job, including timestamps, status, and duration or error output.",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

var (
	logsServer string
	logsCount  int
	logsOutput bool
)

func init() {
	logsCmd.Flags().StringVarP(&logsServer, "server", "s", "", "target server name from config")
	logsCmd.Flags().IntVarP(&logsCount, "n", "n", 10, "number of entries to show")
	logsCmd.Flags().BoolVarP(&logsOutput, "output", "o", false, "show full output for the last run")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	name := args[0]

	b, err := resolveBackend(logsServer)
	if err != nil {
		return err
	}
	defer b.Close()

	entries, err := b.LoadHistory()
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	// Filter entries matching job name or ID.
	var matched []history.Entry
	for _, e := range entries {
		if strings.EqualFold(e.JobName, name) || strings.EqualFold(e.JobID, name) {
			matched = append(matched, e)
		}
	}

	if len(matched) == 0 {
		fmt.Printf("No history found for %q\n", name)
		return nil
	}

	// If --output flag, show full output of the most recent run.
	if logsOutput {
		latest := matched[0]
		output := strings.TrimSpace(latest.Output)
		if output == "" {
			fmt.Println("(no output)")
		} else {
			fmt.Println(output)
		}
		return nil
	}

	// Limit to -n entries.
	if logsCount > 0 && len(matched) > logsCount {
		matched = matched[:logsCount]
	}

	for _, e := range matched {
		ts, duration := formatTimestamp(e.Timestamp)
		status := formatStatus(e.Success)
		detail := formatDetail(e, duration)
		fmt.Printf("%s  %s  %s\n", ts, status, detail)
	}

	return nil
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
