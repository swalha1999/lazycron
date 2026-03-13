package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Top bar
	topBar := renderTopBar(m.mode, m.width)

	// Bottom bar
	bottomBar := renderBottomBar(m.mode, m.focusPanel, m.statusMsg, m.statusKind, m.width)

	// Main content area: fill remaining space after top bar (1 line) and bottom bar
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

	case modeHelp:
		fg := renderHelpScreen()
		content = overlay(panels, fg, m.width, contentHeight)

	case modeRunOutput:
		fg := renderRunOutput(m.runJobName, m.runOutput, m.runOutputFailed, m.runOutputScroll, m.width, contentHeight)
		content = overlay(panels, fg, m.width, contentHeight)

	default:
		content = panels
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		content,
		bottomBar,
	)
}

func (m Model) renderPanels(height int) string {
	listWidth := m.width * 2 / 5
	detailWidth := m.width - listWidth - 4 // account for borders

	if listWidth < 20 {
		listWidth = 20
	}

	innerHeight := height - 2 // border (top + bottom)
	if innerHeight < 3 {
		innerHeight = 3
	}

	// Split left panel into jobs (top) and history (bottom)
	jobsHeight := innerHeight / 2
	historyHeight := innerHeight - jobsHeight - 2 // -2 for border between panels

	if jobsHeight < 3 {
		jobsHeight = 3
	}
	if historyHeight < 3 {
		historyHeight = 3
	}

	// Top-left: Jobs panel
	listContent := renderJobList(m.jobs, m.selected, listWidth-4, jobsHeight)
	jobsActive := m.focusPanel == panelJobs
	jobsPanelStyle := panelStyle
	if jobsActive {
		jobsPanelStyle = activePanelStyle
	}
	jobsPanel := jobsPanelStyle.
		Width(listWidth).
		Height(jobsHeight).
		Render(listContent)
	jobsPanel = injectBorderTitle(jobsPanel, "1", "Jobs", jobsActive)

	// Bottom-left: History panel
	historyContent := renderHistoryList(m.history, m.historySelected, listWidth-4, historyHeight, m.focusPanel == panelHistory)
	historyActive := m.focusPanel == panelHistory
	historyPanelStyle := panelStyle
	if historyActive {
		historyPanelStyle = activePanelStyle
	}
	historyPanel := historyPanelStyle.
		Width(listWidth).
		Height(historyHeight).
		Render(historyContent)
	historyPanel = injectBorderTitle(historyPanel, "2", "History", historyActive)

	leftPanel := lipgloss.JoinVertical(lipgloss.Left, jobsPanel, historyPanel)

	// Right panel: show job or history detail based on last left panel focus
	var detailContent string
	showHistory := m.focusPanel == panelHistory || (m.focusPanel == panelDetail && m.lastLeftPanel == panelHistory)
	if showHistory && len(m.history) > 0 {
		var entry *history.Entry
		if m.historySelected >= 0 && m.historySelected < len(m.history) {
			entry = &m.history[m.historySelected]
		}
		detailContent = renderHistoryDetail(entry, detailWidth-4)
	} else {
		var selectedJob *cron.Job
		if m.selected >= 0 && m.selected < len(m.jobs) {
			selectedJob = &m.jobs[m.selected]
		}
		detailContent = renderDetail(selectedJob, detailWidth-4)
	}

	// Apply scroll to detail content only when it overflows
	detailLines := strings.Split(detailContent, "\n")
	visibleHeight := innerHeight - 2 // account for padding
	if len(detailLines) > visibleHeight {
		maxScroll := len(detailLines) - visibleHeight
		if m.detailScroll > maxScroll {
			m.detailScroll = maxScroll
		}
		if m.detailScroll > 0 {
			detailContent = strings.Join(detailLines[m.detailScroll:], "\n")
		}
	} else {
		m.detailScroll = 0
	}

	detailActive := m.focusPanel == panelDetail
	rightPanelStyle := panelStyle
	if detailActive {
		rightPanelStyle = activePanelStyle
	}
	rightPanel := rightPanelStyle.
		Width(detailWidth).
		Height(innerHeight).
		Render(detailContent)
	rightPanel = injectBorderTitle(rightPanel, "3", "Details", detailActive)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
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

	// Build the title segment: ─[N] Title─
	titleSegment := borderStyle.Render("─") +
		numberStyle.Render("["+number+"]") +
		borderStyle.Render("─") +
		titleTextStyle.Render(title) +
		borderStyle.Render("─")

	// Get the visual width of the original top border line
	topLineWidth := ansi.StringWidth(lines[0])
	titleWidth := ansi.StringWidth(titleSegment)

	// Build: ╭ + titleSegment + remaining ─s + ╮
	remaining := topLineWidth - 2 - titleWidth // -2 for ╭ and ╮
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

	// Pad bg to height
	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	fgHeight := len(fgLines)
	fgWidth := lipgloss.Width(fgBox)

	// Center position
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
			// Left portion of bg (ANSI-aware truncate to leftOffset chars)
			left := ansi.Truncate(bgLine, leftOffset, "")
			// Right portion of bg (skip leftOffset+fgWidth chars)
			right := ansi.TruncateLeft(bgLine, rightOffset, "")
			bgLines[row] = left + fgLine + right
		}
	}

	return strings.Join(bgLines[:height], "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
