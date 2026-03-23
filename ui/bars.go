package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderTopBar(m mode, serverName string, version string, width int) string {
	title := titleStyle.Render("lazycron")
	versionTag := lipgloss.NewStyle().Foreground(colorMuted).Render(version)
	serverTag := lipgloss.NewStyle().Foreground(colorCyan).Render("[" + serverName + "]")

	var modeStr string
	switch m {
	case modeNormal:
		modeStr = modeStyle.Render("NORMAL")
	case modeForm:
		modeStr = modeStyle.Render("EDIT")
	case modeConfirmDelete, modeConfirmDeleteServer, modeConfirmDeleteHistory:
		modeStr = modeStyle.Render("CONFIRM")
	case modeHelp:
		modeStr = modeStyle.Render("HELP")
	case modeRunOutput:
		modeStr = modeStyle.Render("OUTPUT")
	case modeAddServer:
		modeStr = modeStyle.Render("ADD SERVER")
	case modePasswordPrompt:
		modeStr = modeStyle.Render("PASSWORD")
	case modeNewJobChoice:
		modeStr = modeStyle.Render("NEW JOB")
	case modeTemplatePicker:
		modeStr = modeStyle.Render("TEMPLATE")
	}

	leftPart := title + " " + versionTag + "  " + serverTag
	spacer := strings.Repeat(" ", max(0, width-lipgloss.Width(leftPart)-lipgloss.Width(modeStr)))
	bar := leftPart + spacer + modeStr

	return topBarStyle.Width(width).Render(bar)
}

func renderBottomBar(m mode, focusPanel int, statusMsg string, statusKind statusType, width int) string {
	var help string
	switch m {
	case modeNormal:
		help = helpBinding("↑/↓", "move") + helpSep() +
			helpBinding("←/→", "panel") + helpSep()
		if focusPanel == panelServers {
			help += helpBinding("enter/space", "switch") + helpSep() +
				helpBinding("a", "add") + helpSep() +
				helpBinding("c", "connect") + helpSep() +
				helpBinding("d", "disconnect") + helpSep() +
				helpBinding("D", "remove") + helpSep()
		} else if focusPanel == panelJobs {
			help += helpBinding("n", "new") + helpSep() +
				helpBinding("enter", "edit") + helpSep() +
				helpBinding("D", "delete") + helpSep() +
				helpBinding("space", "toggle") + helpSep() +
				helpBinding("r", "run") + helpSep() +
				helpBinding("ctrl+↑/↓", "reorder") + helpSep() +
				helpBinding("U", "update fmt") + helpSep()
		} else if focusPanel == panelHistory {
			help += helpBinding("n", "new") + helpSep() +
				helpBinding("D", "delete") + helpSep()
		} else {
			help += helpBinding("n", "new") + helpSep()
		}
		help += helpBinding("R", "refresh") + helpSep() +
			helpBinding("u", "update app") + helpSep() +
			helpBinding("?", "help") + helpSep() +
			helpBinding("q", "quit")
	case modeForm, modeAddServer:
		help = helpBinding("tab", "next") + helpSep() +
			helpBinding("shift+tab", "prev") + helpSep() +
			helpBinding("enter", "save") + helpSep() +
			helpBinding("esc", "cancel")
	case modePasswordPrompt:
		help = helpBinding("enter", "connect") + helpSep() +
			helpBinding("esc", "cancel")
	case modeConfirmDelete, modeConfirmDeleteServer, modeConfirmDeleteHistory:
		help = helpBinding("←/→", "select") + helpSep() +
			helpBinding("enter", "confirm") + helpSep() +
			helpBinding("y/n", "yes/no") + helpSep() +
			helpBinding("esc", "cancel")
	case modeHelp:
		help = helpBinding("esc", "back")
	case modeRunOutput:
		help = helpBinding("esc", "close")
	case modeNewJobChoice:
		help = helpBinding("b", "blank") + helpSep() +
			helpBinding("t", "template") + helpSep() +
			helpBinding("esc", "cancel")
	case modeTemplatePicker:
		help = helpBinding("↑/↓", "select") + helpSep() +
			helpBinding("enter", "choose") + helpSep() +
			helpBinding("esc", "back")
	}

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
		{"", "── Server Panel ──"},
		{"enter", "Switch to selected server"},
		{"a", "Add new server"},
		{"c", "Connect to server"},
		{"d", "Disconnect server"},
		{"D", "Remove server"},
		{"", "── Jobs Panel ──"},
		{"n", "Create new job"},
		{"enter / e", "Edit selected job"},
		{"D", "Delete selected job"},
		{"space", "Toggle enable/disable"},
		{"r", "Run job now"},
		{"U", "Update job to latest format"},
		{"p", "Set/change project group"},
		{"ctrl+↑/↓", "Move job up/down (reorder)"},
		{"", "── General ──"},
		{"1/2/3/4/tab", "Switch panel"},
		{"R", "Refresh from crontab"},
		{"u", "Update app to latest version"},
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"?", "Show/hide help"},
		{"q", "Quit"},
	}

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Keybindings") + "\n\n")
	for _, bind := range bindings {
		if bind.key == "" {
			// Section header
			b.WriteString("  " + formLabelStyle.Render(bind.desc) + "\n")
		} else {
			key := helpKeyStyle.Render(fmt.Sprintf("  %-14s", bind.key))
			desc := detailValueStyle.Render(bind.desc)
			b.WriteString(key + " " + desc + "\n")
		}
	}

	content := b.String()
	return formStyle.Width(48).Render(content)
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
		var allLines []string
		for _, rawLine := range strings.Split(output, "\n") {
			if rawLine == "" {
				allLines = append(allLines, "")
			} else {
				wrapped := wrapText(rawLine, boxWidth-8)
				allLines = append(allLines, wrapped...)
			}
		}

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
