package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderCompletions(c *completerModel, width int) string {
	if !c.active {
		return ""
	}

	dropWidth := width
	if dropWidth > 60 {
		dropWidth = 60
	}
	if dropWidth < 30 {
		dropWidth = 30
	}
	innerWidth := dropWidth - 4 // border + padding

	crumbStyle := lipgloss.NewStyle().Foreground(colorMuted)
	crumbActiveStyle := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	selectedBg := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	normalEntry := lipgloss.NewStyle().Foreground(colorFg)
	mutedEntry := lipgloss.NewStyle().Foreground(colorMuted)
	matchStyle := lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	dirArrow := lipgloss.NewStyle().Foreground(colorCyan)
	emptyStyle := lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	var b strings.Builder

	// Breadcrumb
	crumb := c.breadcrumb(innerWidth)
	if crumb != "" {
		// Split to highlight last segment
		parts := strings.Split(crumb, " / ")
		if len(parts) > 1 {
			prefix := strings.Join(parts[:len(parts)-1], crumbStyle.Render(" / "))
			last := crumbActiveStyle.Render(parts[len(parts)-1])
			b.WriteString(crumbStyle.Render(prefix) + crumbStyle.Render(" / ") + last)
		} else {
			b.WriteString(crumbActiveStyle.Render(crumb))
		}
		b.WriteString("\n")
	}

	// Handle special states
	if c.permError {
		b.WriteString(emptyStyle.Render("  (permission denied)"))
		return renderCompleterBox(b.String(), dropWidth)
	}
	if c.emptyDir {
		b.WriteString(emptyStyle.Render("  (no subdirectories)"))
		return renderCompleterBox(b.String(), dropWidth)
	}
	if c.noMatches {
		b.WriteString(emptyStyle.Render("  (no matches)"))
		return renderCompleterBox(b.String(), dropWidth)
	}
	if len(c.suggestions) == 0 {
		return ""
	}

	// Calculate visible window around selection
	total := len(c.suggestions)
	startIdx := 0
	endIdx := total
	if total > visibleRows {
		// Center the selection in the window
		startIdx = c.selected - visibleRows/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + visibleRows
		if endIdx > total {
			endIdx = total
			startIdx = endIdx - visibleRows
		}
	}

	// Scroll indicator top
	if startIdx > 0 {
		b.WriteString(mutedEntry.Render("  ↑ more") + "\n")
	}

	// Suggestion rows
	for i := startIdx; i < endIdx; i++ {
		s := c.suggestions[i]
		isSelected := i == c.selected

		// Build the name with match highlighting
		var nameStr string
		if len(s.matchRanges) > 0 && !isSelected {
			nameStr = highlightMatches(s.name, s.matchRanges, matchStyle, normalEntry)
		} else if isSelected {
			nameStr = selectedBg.Render(s.name)
		} else {
			nameStr = normalEntry.Render(s.name)
		}

		// Prefix and suffix
		var prefix string
		if isSelected {
			prefix = selectedBg.Render("▸ ")
		} else {
			prefix = "  "
		}

		suffix := ""
		if s.hasChildren {
			suffix = " " + dirArrow.Render("›")
		} else {
			suffix = " " + mutedEntry.Render("·")
		}

		row := prefix + nameStr + mutedEntry.Render("/") + suffix
		b.WriteString(row)
		if i < endIdx-1 || endIdx < total {
			b.WriteString("\n")
		}
	}

	// Scroll indicator bottom
	if endIdx < total {
		b.WriteString(mutedEntry.Render("  ↓ more"))
	}

	return renderCompleterBox(b.String(), dropWidth)
}

func renderCompleterBox(content string, width int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(width - 2).
		Render(content)
}

// highlightMatches renders a string with matched character ranges highlighted.
func highlightMatches(name string, ranges [][2]int, matchSty, normalSty lipgloss.Style) string {
	if len(ranges) == 0 {
		return normalSty.Render(name)
	}

	// Build a set of matched positions
	matched := make(map[int]bool)
	for _, r := range ranges {
		for i := r[0]; i < r[1] && i < len(name); i++ {
			matched[i] = true
		}
	}

	var b strings.Builder
	inMatch := false
	start := 0

	for i := 0; i <= len(name); i++ {
		isMatch := i < len(name) && matched[i]
		if i == len(name) || isMatch != inMatch {
			// Flush segment
			segment := name[start:i]
			if segment != "" {
				if inMatch {
					b.WriteString(matchSty.Render(segment))
				} else {
					b.WriteString(normalSty.Render(segment))
				}
			}
			start = i
			inMatch = isMatch
		}
	}

	return b.String()
}
