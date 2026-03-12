package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/history"
)

func renderHistoryList(entries []history.Entry, selected, width, height int, focused bool) string {
	if len(entries) == 0 {
		return mutedItemStyle.Render("\n  No history yet")
	}

	var b strings.Builder

	listHeight := height - 2
	if listHeight < 1 {
		listHeight = 1
	}

	startIdx := 0
	if selected >= listHeight {
		startIdx = selected - listHeight + 1
	}
	endIdx := startIdx + listHeight
	if endIdx > len(entries) {
		endIdx = len(entries)
	}

	maxNameWidth := width - 16
	if maxNameWidth < 8 {
		maxNameWidth = 8
	}

	for i := startIdx; i < endIdx; i++ {
		entry := entries[i]

		name := entry.JobName
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-1] + "…"
		}

		ago := relativeTime(entry.Timestamp)

		line := fmt.Sprintf(" %-*s  %s", maxNameWidth, name, mutedItemStyle.Render(ago))

		if i == selected && focused {
			line = selectedStyle.Render("▶ " + line)
		} else {
			line = normalStyle.Render("  " + line)
		}

		b.WriteString(line)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	if len(entries) > listHeight {
		scrollInfo := fmt.Sprintf(" %d/%d", selected+1, len(entries))
		b.WriteString("\n")
		b.WriteString(mutedItemStyle.Render(scrollInfo))
	}

	return b.String()
}

func relativeTime(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}

	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	case d < 48*time.Hour:
		return "yesterday"
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}
