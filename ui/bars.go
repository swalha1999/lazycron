package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderTopBar(m mode, width int) string {
	title := titleStyle.Render("lazycron")
	var modeStr string
	switch m {
	case modeNormal:
		modeStr = modeStyle.Render("NORMAL")
	case modeForm:
		modeStr = modeStyle.Render("EDIT")
	case modeConfirmDelete:
		modeStr = modeStyle.Render("CONFIRM")
	case modeHelp:
		modeStr = modeStyle.Render("HELP")
	case modeRunOutput:
		modeStr = modeStyle.Render("OUTPUT")
	}

	spacer := strings.Repeat(" ", max(0, width-lipgloss.Width(title)-lipgloss.Width(modeStr)))
	bar := title + spacer + modeStr

	return topBarStyle.Width(width).Render(bar)
}

func renderBottomBar(m mode, focusPanel int, statusMsg string, statusKind statusType, width int) string {
	var help string
	switch m {
	case modeNormal:
		help = helpBinding("↑/↓", "move") + helpSep() +
			helpBinding("←/→", "panel") + helpSep() +
			helpBinding("n", "new") + helpSep()
		if focusPanel == panelJobs {
			help += helpBinding("enter", "edit") + helpSep() +
				helpBinding("d", "delete") + helpSep() +
				helpBinding("space", "toggle") + helpSep() +
				helpBinding("r", "run") + helpSep() +
				helpBinding("U", "update fmt") + helpSep()
		}
		help += helpBinding("R", "refresh") + helpSep() +
			helpBinding("?", "help") + helpSep() +
			helpBinding("q", "quit")
	case modeForm:
		help = helpBinding("tab", "next") + helpSep() +
			helpBinding("shift+tab", "prev") + helpSep() +
			helpBinding("enter", "save") + helpSep() +
			helpBinding("esc", "cancel")
	case modeConfirmDelete:
		help = helpBinding("y", "confirm") + helpSep() +
			helpBinding("n", "cancel")
	case modeHelp:
		help = helpBinding("esc", "back")
	case modeRunOutput:
		help = helpBinding("esc", "close")
	}

	// Status message
	var status string
	if statusMsg != "" {
		switch statusKind {
		case statusError:
			status = errorStyle.Render(" ✗ " + statusMsg)
		case statusSuccess:
			status = successStyle.Render(" ✓ " + statusMsg)
		default:
			status = statusBarStyle.Render(" " + statusMsg)
		}
	}

	helpLine := statusBarStyle.Width(width).Render(help)
	if status != "" {
		statusLine := lipgloss.NewStyle().
			Width(width).
			Render(status)
		return lipgloss.JoinVertical(lipgloss.Left, statusLine, helpLine)
	}
	return helpLine
}

func helpBinding(key, desc string) string {
	return helpKeyStyle.Render(key) + helpDescStyle.Render(" "+desc)
}

func helpSep() string {
	return helpDescStyle.Render("  ")
}

func renderHelpScreen() string {
	bindings := []struct{ key, desc string }{
		{"n", "Create new job"},
		{"enter / e", "Edit selected job"},
		{"d", "Delete selected job"},
		{"space", "Toggle enable/disable"},
		{"r", "Run job now"},
		{"U", "Update job to latest format"},
		{"1/2/3/tab", "Switch panel"},
		{"R", "Refresh from crontab"},
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"?", "Show/hide help"},
		{"q", "Quit"},
	}

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Keybindings") + "\n\n")
	for _, bind := range bindings {
		key := helpKeyStyle.Render(fmt.Sprintf("  %-14s", bind.key))
		desc := detailValueStyle.Render(bind.desc)
		b.WriteString(key + " " + desc + "\n")
	}

	content := b.String()
	return formStyle.Width(44).Render(content)
}

func renderRunOutput(name, output string, failed bool, scroll, width, maxHeight int) string {
	boxWidth := width - 10
	if boxWidth > 80 {
		boxWidth = 80
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	var b strings.Builder

	// Title with status indicator
	if failed {
		b.WriteString(errorStyle.Render("  ✗ Run Failed: " + name))
	} else {
		b.WriteString(successStyle.Render("  ✓ Run Output: " + name))
	}
	b.WriteString("\n\n")

	if output == "" {
		b.WriteString(mutedItemStyle.Render("  (no output)"))
		b.WriteString("\n")
	} else {
		// Word-wrap each line of output
		var allLines []string
		for _, rawLine := range strings.Split(output, "\n") {
			if rawLine == "" {
				allLines = append(allLines, "")
			} else {
				wrapped := wrapText(rawLine, boxWidth-8)
				allLines = append(allLines, wrapped...)
			}
		}

		// Calculate visible window
		visibleLines := maxHeight - 10
		if visibleLines < 3 {
			visibleLines = 3
		}

		startLine := scroll
		if len(allLines) <= visibleLines {
			startLine = 0
		} else if startLine > len(allLines)-visibleLines {
			startLine = len(allLines) - visibleLines
		}

		endLine := startLine + visibleLines
		if endLine > len(allLines) {
			endLine = len(allLines)
		}

		for _, line := range allLines[startLine:endLine] {
			b.WriteString("  " + detailValueStyle.Render(line) + "\n")
		}

		if len(allLines) > visibleLines {
			scrollInfo := fmt.Sprintf("  [lines %d–%d of %d]", startLine+1, endLine, len(allLines))
			b.WriteString(mutedItemStyle.Render(scrollInfo) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("  esc: close"))

	return formStyle.Width(boxWidth).Render(b.String())
}
