package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/history"
)

func renderHistoryList(entries []history.Entry, selected, width, height int, focused bool, matchSet map[int]bool) string {
	if len(entries) == 0 {
		return mutedItemStyle.Render("\n  No history yet")
	}

	// Build list of visible indices
	var visible []int
	for i := range entries {
		if matchSet == nil || matchSet[i] {
			visible = append(visible, i)
		}
	}

	if len(visible) == 0 {
		return mutedItemStyle.Render("\n  No matches")
	}

	// Find selected position within visible list
	selPos := 0
	for j, idx := range visible {
		if idx == selected {
			selPos = j
			break
		}
	}

	var b strings.Builder

	listHeight := height - 2
	if listHeight < 1 {
		listHeight = 1
	}

	startPos := 0
	if selPos >= listHeight {
		startPos = selPos - listHeight + 1
	}
	endPos := startPos + listHeight
	if endPos > len(visible) {
		endPos = len(visible)
	}

	maxNameWidth := width - 16
	if maxNameWidth < 8 {
		maxNameWidth = 8
	}

	for p := startPos; p < endPos; p++ {
		i := visible[p]
		entry := entries[i]

		name := entry.JobName
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-1] + "…"
		}

		ago := relativeTime(entry.Timestamp)

		// Determine success/failure status
		failed := entry.Success != nil && !*entry.Success
		succeeded := entry.Success != nil && *entry.Success
		var prefix string
		if failed {
			prefix = "✗ "
		} else if succeeded {
			prefix = "✓ "
		} else {
			prefix = "  "
		}

		line := fmt.Sprintf("%-*s  %s", maxNameWidth, name, mutedItemStyle.Render(ago))

		if i == selected && focused {
			if failed {
				line = historyFailedSelectedStyle.Render("▶ " + line)
			} else if succeeded {
				line = historySuccessSelectedStyle.Render("▶ " + line)
			} else {
				line = selectedStyle.Render("▶ " + line)
			}
		} else if failed {
			line = historyFailedStyle.Render(prefix + line)
		} else if succeeded {
			line = historySuccessStyle.Render(prefix + line)
		} else {
			line = normalStyle.Render(prefix + line)
		}

		b.WriteString(line)
		if p < endPos-1 {
			b.WriteString("\n")
		}
	}

	if len(visible) > listHeight {
		scrollInfo := fmt.Sprintf(" %d/%d", selPos+1, len(visible))
		b.WriteString("\n")
		b.WriteString(mutedItemStyle.Render(scrollInfo))
	}

	return b.String()
}

func relativeTime(timestamp string) string {
	var t time.Time
	var err error
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05-0700",  // RFC3339 without colon in tz offset
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
	} {
		t, err = time.Parse(layout, timestamp)
		if err == nil {
			break
		}
	}
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
