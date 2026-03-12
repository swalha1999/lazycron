package ui

import (
	"fmt"
	"strings"

	"github.com/swalha1999/lazycron/cron"
)

func renderJobList(jobs []cron.Job, selected int, width, height int) string {
	if len(jobs) == 0 {
		empty := mutedItemStyle.Render("No cron jobs found")
		hint := mutedItemStyle.Render("Press 'n' to create one")
		return fmt.Sprintf("\n  %s\n\n  %s", empty, hint)
	}

	var b strings.Builder
	maxNameWidth := 0
	for _, job := range jobs {
		if len(job.Name) > maxNameWidth {
			maxNameWidth = len(job.Name)
		}
	}
	if maxNameWidth > width-20 {
		maxNameWidth = width - 20
	}
	if maxNameWidth < 8 {
		maxNameWidth = 8
	}

	// Calculate visible area for scrolling
	listHeight := height - 2
	if listHeight < 1 {
		listHeight = 1
	}

	startIdx := 0
	if selected >= listHeight {
		startIdx = selected - listHeight + 1
	}
	endIdx := startIdx + listHeight
	if endIdx > len(jobs) {
		endIdx = len(jobs)
	}

	for i := startIdx; i < endIdx; i++ {
		job := jobs[i]

		// Status dot
		dot := enabledDotStyle.Render("●")
		if !job.Enabled {
			dot = disabledDotStyle.Render("○")
		}

		// Name (truncated)
		name := job.Name
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-1] + "…"
		}

		// Schedule (human readable)
		schedule := cron.CronToHuman(job.Schedule)
		if len(schedule) > 24 {
			schedule = schedule[:23] + "…"
		}

		// Command (truncated)
		cmdWidth := width - maxNameWidth - 30
		if cmdWidth < 10 {
			cmdWidth = 10
		}
		cmd := job.Command
		if len(cmd) > cmdWidth {
			cmd = cmd[:cmdWidth-1] + "…"
		}

		line := fmt.Sprintf(" %s %-*s  %s", dot, maxNameWidth, name, mutedItemStyle.Render(schedule))

		if i == selected {
			line = selectedStyle.Render("▶ " + line)
		} else {
			line = normalStyle.Render("  " + line)
		}

		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(jobs) > listHeight {
		scrollInfo := fmt.Sprintf(" %d/%d", selected+1, len(jobs))
		b.WriteString("\n")
		b.WriteString(mutedItemStyle.Render(scrollInfo))
	}

	return b.String()
}
