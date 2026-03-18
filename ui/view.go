package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	serverName := m.manager.ServerAt(m.manager.ActiveIndex()).Name
	topBar := renderTopBar(m.mode, serverName, m.width)

	bottomBar := renderBottomBar(m.mode, m.focusPanel, m.statusMsg, m.statusKind, m.width)

	contentHeight := m.height - 1 - lipgloss.Height(bottomBar)

	panels := m.renderPanels(contentHeight)

	var content string
	switch m.mode {
	case modeForm:
		fg := renderForm(&m.form, m.width)
		content = overlay(panels, fg, m.width, contentHeight)

	case modeConfirmDelete:
		jobName := ""
		if len(m.jobs) > 0 {
			jobName = m.jobs[m.selected].Name
		}
		fg := renderConfirmDialog(fmt.Sprintf("Delete job '%s'?", jobName))
		content = overlay(panels, fg, m.width, contentHeight)

	case modeConfirmDeleteServer:
		serverName := ""
		if m.serverSelected > 0 && m.serverSelected < m.manager.ServerCount() {
			serverName = m.manager.ServerAt(m.serverSelected).Name
		}
		fg := renderConfirmDialog(fmt.Sprintf("Remove server '%s'?", serverName))
		content = overlay(panels, fg, m.width, contentHeight)

	case modeConfirmDeleteHistory:
		entryName := ""
		if m.historySelected >= 0 && m.historySelected < len(m.history) {
			entryName = m.history[m.historySelected].JobName
		}
		fg := renderConfirmDialog(fmt.Sprintf("Delete history entry '%s'?", entryName))
		content = overlay(panels, fg, m.width, contentHeight)

	case modeHelp:
		fg := renderHelpScreen()
		content = overlay(panels, fg, m.width, contentHeight)

	case modeRunOutput:
		fg := renderRunOutput(m.runJobName, m.runOutput, m.runOutputFailed, m.runOutputScroll, m.width, contentHeight)
		content = overlay(panels, fg, m.width, contentHeight)

	case modeAddServer:
		fg := renderServerForm(&m.serverForm, m.width)
		content = overlay(panels, fg, m.width, contentHeight)

	case modePasswordPrompt:
		info := m.manager.ServerAt(m.passwordServerIdx)
		fg := renderPasswordPrompt(&m.passwordInput, info.Name, info.Host, info.User, m.width)
		content = overlay(panels, fg, m.width, contentHeight)

	case modeNewJobChoice:
		fg := renderNewJobChoice(m.width)
		content = overlay(panels, fg, m.width, contentHeight)

	case modeTemplatePicker:
		fg := renderTemplatePicker(&m.templatePicker, m.width)
		content = overlay(panels, fg, m.width, contentHeight)

	default:
		content = panels
	}

	// Overlay "Connecting..." when switching servers
	if m.serverSwitching {
		connectingBox := formStyle.Width(30).Render(
			connectingDotStyle.Render("● ") + "Connecting...",
		)
		content = overlay(content, connectingBox, m.width, contentHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		content,
		bottomBar,
	)
}

func (m Model) renderPanels(height int) string {
	listWidth := m.width * 2 / 5
	detailWidth := m.width - listWidth - 4

	if listWidth < 20 {
		listWidth = 20
	}

	innerHeight := height - 2
	if innerHeight < 3 {
		innerHeight = 3
	}

	// Server panel height: compact
	serverCount := m.manager.ServerCount()
	serverInnerHeight := serverCount
	if serverInnerHeight > 6 {
		serverInnerHeight = 6
	}
	if serverInnerHeight < 1 {
		serverInnerHeight = 1
	}

	// Remaining height for jobs + history
	remainingHeight := innerHeight - serverInnerHeight - 2
	if remainingHeight < 6 {
		remainingHeight = 6
	}

	jobsHeight := remainingHeight / 2
	historyHeight := remainingHeight - jobsHeight - 2

	if jobsHeight < 3 {
		jobsHeight = 3
	}
	if historyHeight < 3 {
		historyHeight = 3
	}

	// [1] Servers panel
	servers := m.manager.Servers()
	serverContent := renderServerList(servers, m.serverSelected, m.manager.ActiveIndex(), listWidth-4, serverInnerHeight, m.focusPanel == panelServers)
	serversActive := m.focusPanel == panelServers
	serversPanelStyle := panelStyle
	if serversActive {
		serversPanelStyle = activePanelStyle
	}
	serversBox := serversPanelStyle.
		Width(listWidth).
		Height(serverInnerHeight).
		Render(serverContent)
	serversBox = injectBorderTitle(serversBox, "1", "Servers", serversActive)

	// [2] Jobs panel
	listContent := renderJobList(m.jobs, m.selected, listWidth-4, jobsHeight)
	jobsActive := m.focusPanel == panelJobs
	jobsPanelStyle := panelStyle
	if jobsActive {
		jobsPanelStyle = activePanelStyle
	}
	jobsBox := jobsPanelStyle.
		Width(listWidth).
		Height(jobsHeight).
		Render(listContent)
	jobsBox = injectBorderTitle(jobsBox, "2", "Jobs", jobsActive)

	// [3] History panel
	historyContent := renderHistoryList(m.history, m.historySelected, listWidth-4, historyHeight, m.focusPanel == panelHistory)
	historyActive := m.focusPanel == panelHistory
	historyPanelStyle := panelStyle
	if historyActive {
		historyPanelStyle = activePanelStyle
	}
	historyBox := historyPanelStyle.
		Width(listWidth).
		Height(historyHeight).
		Render(historyContent)
	historyBox = injectBorderTitle(historyBox, "3", "History", historyActive)

	leftPanel := lipgloss.JoinVertical(lipgloss.Left, serversBox, jobsBox, historyBox)

	// [4] Details panel
	detailContent := m.buildDetailContent(detailWidth - 4)
	detailContent = m.applyDetailScroll(detailContent, innerHeight)

	detailActive := m.focusPanel == panelDetail
	rightPanelStyle := panelStyle
	if detailActive {
		rightPanelStyle = activePanelStyle
	}
	rightPanel := rightPanelStyle.
		Width(detailWidth).
		Height(innerHeight).
		Render(detailContent)
	rightPanel = injectBorderTitle(rightPanel, "4", "Details", detailActive)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m Model) buildDetailContent(width int) string {
	showHistory := m.focusPanel == panelHistory || (m.focusPanel == panelDetail && m.lastLeftPanel == panelHistory)
	if showHistory && len(m.history) > 0 {
		var entry *history.Entry
		if m.historySelected >= 0 && m.historySelected < len(m.history) {
			entry = &m.history[m.historySelected]
		}
		return renderHistoryDetail(entry, width)
	}
	var selectedJob *cron.Job
	if m.selected >= 0 && m.selected < len(m.jobs) {
		selectedJob = &m.jobs[m.selected]
	}
	return renderDetail(selectedJob, width)
}

func (m Model) applyDetailScroll(detailContent string, innerHeight int) string {
	detailLines := strings.Split(detailContent, "\n")
	visibleHeight := innerHeight - 2
	totalLines := len(detailLines)

	if totalLines <= visibleHeight {
		return detailContent
	}

	maxScroll := totalLines - visibleHeight
	scroll := m.detailScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	endIdx := scroll + visibleHeight
	if endIdx > totalLines {
		endIdx = totalLines
	}

	visible := detailLines[scroll:endIdx]

	// Add scroll indicators
	if scroll > 0 {
		visible[0] = mutedItemStyle.Render(fmt.Sprintf("  ↑ scroll up (%d more)", scroll))
	}
	if endIdx < totalLines {
		visible[len(visible)-1] = mutedItemStyle.Render(fmt.Sprintf("  ↓ scroll down (%d more)", totalLines-endIdx))
	}

	return strings.Join(visible, "\n")
}

// injectBorderTitle replaces the top border line to embed "[N] Title" in it.
func injectBorderTitle(rendered, number, title string, active bool) string {
	lines := strings.Split(rendered, "\n")
	if len(lines) == 0 {
		return rendered
	}

	borderColor := colorBorder
	if active {
		borderColor = colorActiveBorder
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	numberStyle := lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	titleTextStyle := panelTitleStyle
	if !active {
		titleTextStyle = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	}

	titleSegment := borderStyle.Render("─") +
		numberStyle.Render("["+number+"]") +
		borderStyle.Render("─") +
		titleTextStyle.Render(title) +
		borderStyle.Render("─")

	topLineWidth := ansi.StringWidth(lines[0])
	titleWidth := ansi.StringWidth(titleSegment)

	remaining := topLineWidth - 2 - titleWidth
	if remaining < 0 {
		remaining = 0
	}
	newTop := borderStyle.Render("╭") + titleSegment + borderStyle.Render(strings.Repeat("─", remaining)+"╮")

	lines[0] = newTop
	return strings.Join(lines, "\n")
}

// overlay centers the fgBox on top of bg, preserving bg on both sides.
func overlay(bg, fgBox string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fgBox, "\n")

	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	fgHeight := len(fgLines)
	fgWidth := lipgloss.Width(fgBox)

	topOffset := (height - fgHeight) / 2
	leftOffset := (width - fgWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	rightOffset := leftOffset + fgWidth

	for i, fgLine := range fgLines {
		row := topOffset + i
		if row >= 0 && row < len(bgLines) {
			bgLine := bgLines[row]
			left := ansi.Truncate(bgLine, leftOffset, "")
			right := ansi.TruncateLeft(bgLine, rightOffset, "")
			bgLines[row] = left + fgLine + right
		}
	}

	return strings.Join(bgLines[:height], "\n")
}

func renderPasswordPrompt(input *textinput.Model, serverName, host, user string, width int) string {
	formWidth := width - 10
	if formWidth > 50 {
		formWidth = 50
	}
	if formWidth < 40 {
		formWidth = 40
	}

	inputWidth := formWidth - 6

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Password Required"))
	b.WriteString("\n\n")
	b.WriteString(mutedItemStyle.Render(fmt.Sprintf("  Server: %s (%s)", serverName, host)))
	b.WriteString("\n\n")

	label := formLabelStyle.Render("  Password: ")
	input.Width = inputWidth
	rendered := lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1).
		Render(input.View())
	b.WriteString(label + rendered)
	b.WriteString("\n\n")

	b.WriteString(mutedItemStyle.Render("  For better security, use SSH keys instead."))
	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render(fmt.Sprintf("  Tip: ssh-copy-id %s@%s", user, host)))
	b.WriteString("\n\n")
	b.WriteString("  " +
		helpBinding("enter", "connect") + helpSep() +
		helpBinding("esc", "cancel"))

	return formStyle.Width(formWidth).Render(b.String())
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
