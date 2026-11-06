package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

func renderDetail(job *cron.Job, width int, runs runState) string {
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

	// Type
	if job.OneShot {
		typeStr := lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("One-shot")
		if !job.Enabled {
			typeStr += " " + mutedItemStyle.Render("(completed)")
		}
		b.WriteString(renderDetailRow("Type", typeStr))
		b.WriteString("\n")
	}

	// Name (with optional colored tag)
	nameDisplay := detailValueStyle.Render(job.Name)
	if job.Tag != "" {
		tagColor := job.TagColor
		if tagColor == "" {
			tagColor = string(colorRed)
		}
		nameDisplay += " " + lipgloss.NewStyle().
			Foreground(lipgloss.Color(tagColor)).
			Bold(true).
			Render(job.Tag)
	}
	b.WriteString(renderDetailRow("Name", nameDisplay))
	b.WriteString("\n")

	// Project
	if job.Project != "" {
		b.WriteString(renderDetailRow("Project", detailValueStyle.Render(job.Project)))
		b.WriteString("\n")
	}

	// Schedule
	b.WriteString(renderDetailRow("Schedule", detailValueStyle.Render(cron.CronToHuman(job.Schedule))))
	b.WriteString("\n")

	// Cron expression
	b.WriteString(renderDetailRow("Expression", mutedItemStyle.Render(job.Schedule)))
	b.WriteString("\n")

	// Runner, and the run in progress
	b.WriteString(renderDetailRow("Runner", detailValueStyle.Render(job.EffectiveRunner())))
	b.WriteString("\n")
	if a, ok := runs.running(job.ID); ok {
		running := runningBadge() + " " + detailValueStyle.Render("since "+relativeTime(a.StartedAt))
		if a.Terminal != "" {
			running += "  " + mutedItemStyle.Render("press ") + helpKeyStyle.Render("a") + mutedItemStyle.Render(" to attach")
		}
		b.WriteString(renderDetailRow("Running", running))
		b.WriteString("\n")
	}

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
	if job.OneShot {
		if !job.Enabled {
			b.WriteString(detailHeaderStyle.Render("  Completed"))
			b.WriteString("\n")
			b.WriteString("  " + mutedItemStyle.Render("This one-shot job has been executed") + "\n")
		} else {
			b.WriteString(detailHeaderStyle.Render("  Scheduled For"))
			b.WriteString("\n")
			nextRuns := cron.NextRuns(job.Schedule, 1)
			if len(nextRuns) == 0 {
				b.WriteString("  " + mutedItemStyle.Render("Could not calculate") + "\n")
			} else {
				timeStr := nextRuns[0].Format("Mon Jan 02, 2006 at 3:04 PM")
				b.WriteString("  " + detailValueStyle.Render(timeStr) + "\n")
			}
		}
	} else {
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
	}

	return b.String()
}

func renderDetailRow(label, value string) string {
	return fmt.Sprintf("  %s %s", detailLabelStyle.Render(label+":"), value)
}

func renderHistoryDetail(entry *history.Entry, width int, runs runState) string {
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

	renderRunRows(&b, entry, runs)

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

// renderRunRows adds the run-engine fields of a history entry: exit code,
// runner and its herdr terminal (and whether it can still be attached).
func renderRunRows(b *strings.Builder, entry *history.Entry, runs runState) {
	row := func(label, value string) {
		b.WriteString(renderDetailRow(label, value))
		b.WriteString("\n")
	}
	switch {
	case entry.Signal != 0:
		row("Exit", errorStyle.Render(fmt.Sprintf("killed (signal %d)", entry.Signal)))
	case entry.ExitCode != nil:
		row("Exit", detailValueStyle.Render(fmt.Sprint(*entry.ExitCode)))
	}
	if entry.Runner != "" {
		row("Runner", detailValueStyle.Render(entry.Runner))
	}
	if entry.FallbackReason != "" {
		row("Fallback", warnStyle.Render(entry.FallbackReason))
	}
	if entry.Terminal != "" {
		kept, live := runs.terminalKept(entry.Terminal)
		state := mutedItemStyle.Render("(removed)")
		switch {
		case live:
			state = terminalBadgeStyle.Render("open") + "  " + mutedItemStyle.Render("press ") + helpKeyStyle.Render("a") + mutedItemStyle.Render(" to attach")
		case kept:
			state = mutedItemStyle.Render("finished, kept") + "  " + mutedItemStyle.Render("press ") + helpKeyStyle.Render("a") + mutedItemStyle.Render(" to view")
		}
		row("Terminal", detailValueStyle.Render(entry.Terminal)+" "+state)
	}
	if entry.RunID != "" {
		row("Run", mutedItemStyle.Render(entry.RunID))
	}
	if entry.OutputTruncated {
		row("Note", warnStyle.Render("output truncated by herdr-core scrollback"))
	}
}
