package ui

import (
	"fmt"
	"strings"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

func renderDetail(job *cron.Job, width int) string {
	if job == nil {
		return mutedItemStyle.Render("\n  Select a job to view details")
	}

	var b strings.Builder

	// Status
	status := enabledDotStyle.Render("● Enabled")
	if !job.Enabled {
		status = disabledDotStyle.Render("○ Disabled")
	}
	b.WriteString(renderDetailRow("Status", status))
	b.WriteString("\n")

	// Name
	b.WriteString(renderDetailRow("Name", detailValueStyle.Render(job.Name)))
	b.WriteString("\n")

	// Schedule
	b.WriteString(renderDetailRow("Schedule", detailValueStyle.Render(cron.CronToHuman(job.Schedule))))
	b.WriteString("\n")

	// Cron expression
	b.WriteString(renderDetailRow("Expression", mutedItemStyle.Render(job.Schedule)))
	b.WriteString("\n")

	// Format warning
	if !job.Wrapped {
		b.WriteString("\n")
		b.WriteString("  " + warnStyle.Render("⚠ Outdated format") + "  " + mutedItemStyle.Render("press ") + helpKeyStyle.Render("U") + mutedItemStyle.Render(" to update"))
		b.WriteString("\n")
	}

	// Command
	b.WriteString("\n")
	b.WriteString(detailHeaderStyle.Render("  Command"))
	b.WriteString("\n")
	cmdLines := wrapText(job.Command, width-6)
	for _, line := range cmdLines {
		b.WriteString("  " + detailValueStyle.Render(line) + "\n")
	}

	// Next runs
	b.WriteString("\n")
	b.WriteString(detailHeaderStyle.Render("  Next Runs"))
	b.WriteString("\n")
	nextRuns := cron.NextRuns(job.Schedule, 3)
	if len(nextRuns) == 0 {
		b.WriteString("  " + mutedItemStyle.Render("Could not calculate") + "\n")
	} else {
		for i, t := range nextRuns {
			timeStr := t.Format("Mon Jan 02, 2006 at 3:04 PM")
			b.WriteString(fmt.Sprintf("  %s %s\n",
				mutedItemStyle.Render(fmt.Sprintf("%d.", i+1)),
				detailValueStyle.Render(timeStr),
			))
		}
	}

	return b.String()
}

func renderDetailRow(label, value string) string {
	return fmt.Sprintf("  %s %s", detailLabelStyle.Render(label+":"), value)
}

func renderHistoryDetail(entry *history.Entry, width int) string {
	if entry == nil {
		return mutedItemStyle.Render("\n  Select a history entry to view details")
	}

	var b strings.Builder

	// Job name
	b.WriteString(renderDetailRow("Job", detailValueStyle.Render(entry.JobName)))
	b.WriteString("\n")

	// Status
	if entry.Success != nil {
		if *entry.Success {
			b.WriteString(renderDetailRow("Status", enabledDotStyle.Render("● Success")))
		} else {
			b.WriteString(renderDetailRow("Status", errorStyle.Render("✗ Failed")))
		}
		b.WriteString("\n")
	}

	// Timestamp
	b.WriteString(renderDetailRow("Ran at", detailValueStyle.Render(entry.Timestamp)))
	b.WriteString("\n")

	// Relative time
	b.WriteString(renderDetailRow("When", mutedItemStyle.Render(relativeTime(entry.Timestamp))))
	b.WriteString("\n")

	// Output
	b.WriteString("\n")
	b.WriteString(detailHeaderStyle.Render("  Output"))
	b.WriteString("\n")

	if entry.Output == "" {
		b.WriteString("  " + mutedItemStyle.Render("(no output)") + "\n")
	} else {
		lines := strings.Split(entry.Output, "\n")
		for _, line := range lines {
			wrapped := wrapText(line, width-6)
			for _, w := range wrapped {
				b.WriteString("  " + detailValueStyle.Render(w) + "\n")
			}
		}
	}

	return b.String()
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 40
	}
	var lines []string
	for len(text) > width {
		// Find last space before width
		idx := strings.LastIndex(text[:width], " ")
		if idx <= 0 {
			idx = width
		}
		lines = append(lines, text[:idx])
		text = text[idx:]
		text = strings.TrimPrefix(text, " ")
	}
	if text != "" {
		lines = append(lines, text)
	}
	return lines
}
