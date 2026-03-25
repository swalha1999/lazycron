package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/monitor"
)

// monitorGroupRow represents a displayable row in the monitor view.
// It can be either a job group header or an individual job instance.
type monitorGroupRow struct {
	isHeader bool
	jobID    string
	jobName  string
	count    int // for headers: number of instances

	// For instance rows
	pid       int
	state     string
	cpu       string
	memory    string
	elapsed   string
	startTime time.Time
}

// monitorRefreshMsg is sent when it's time to refresh the monitor view.
type monitorRefreshMsg struct{}

// monitorTickCmd returns a command that sends a refresh message after a delay.
func monitorTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return monitorRefreshMsg{}
	})
}

// buildMonitorRows converts running jobs into displayable rows (headers + instances).
func (m *Model) buildMonitorRows() {
	b := m.manager.ActiveBackend()
	if b == nil {
		m.monitorGroups = nil
		return
	}

	runningJobs, err := b.GetRunningJobs()
	if err != nil {
		m.monitorGroups = nil
		return
	}

	// Populate job names from m.jobs
	jobNameMap := make(map[string]string)
	for _, job := range m.jobs {
		jobNameMap[job.ID] = job.Name
	}

	// Set job names in running jobs
	for i := range runningJobs {
		if name, ok := jobNameMap[runningJobs[i].JobID]; ok {
			runningJobs[i].JobName = name
		} else {
			runningJobs[i].JobName = runningJobs[i].JobID // fallback
		}
	}

	// Group by job
	groups := monitor.GroupByJob(runningJobs)

	// Build display rows
	rows := []monitorGroupRow{}
	for _, group := range groups {
		// Add header row
		rows = append(rows, monitorGroupRow{
			isHeader: true,
			jobID:    group.JobID,
			jobName:  group.JobName,
			count:    len(group.Instances),
		})

		// Add instance rows
		for _, inst := range group.Instances {
			rows = append(rows, monitorGroupRow{
				isHeader:  false,
				jobID:     inst.JobID,
				jobName:   inst.JobName,
				pid:       inst.PID,
				state:     inst.State,
				cpu:       inst.CPU,
				memory:    inst.Memory,
				elapsed:   monitor.FormatElapsed(inst.Elapsed),
				startTime: inst.StartTime,
			})
		}
	}

	m.monitorGroups = rows

	// Adjust selection if out of bounds
	if m.monitorSelected >= len(m.monitorGroups) {
		m.monitorSelected = len(m.monitorGroups) - 1
	}
	if m.monitorSelected < 0 {
		m.monitorSelected = 0
	}
}

// renderMonitorView renders the full-screen monitor view.
func (m Model) renderMonitorView() string {
	if len(m.monitorGroups) == 0 {
		return m.renderEmptyMonitor()
	}

	// Account for: top bar (1) + bottom bar (1) = 2 lines
	availableHeight := m.height - 2

	// Build the panel content
	title := m.renderMonitorTitle()
	header := m.renderMonitorTableHeader()
	separator := m.renderMonitorSeparator()

	// Calculate visible rows
	// Panel border adds 2 lines (top + bottom)
	// title, header, separator = 3 lines
	// Panel padding = 2 lines
	visibleRows := availableHeight - 7

	if visibleRows < 3 {
		visibleRows = 3
	}

	// Auto-scroll to keep selection visible
	if m.monitorSelected < m.monitorScroll {
		m.monitorScroll = m.monitorSelected
	}
	if m.monitorSelected >= m.monitorScroll+visibleRows {
		m.monitorScroll = m.monitorSelected - visibleRows + 1
	}

	var lines []string
	for i := m.monitorScroll; i < len(m.monitorGroups) && i < m.monitorScroll+visibleRows; i++ {
		row := m.monitorGroups[i]
		line := m.renderMonitorTableRow(row, i == m.monitorSelected, m.width-8)
		lines = append(lines, line)
	}

	// Pad to fill height
	for len(lines) < visibleRows {
		lines = append(lines, "")
	}

	content := strings.Join(lines, "\n")

	// Build the full panel (without footer - it goes in bottom bar)
	panelContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		header,
		separator,
		content,
	)

	// Wrap in panel border
	panel := activePanelStyle.
		Width(m.width - 4).
		Height(availableHeight - 2).
		Render(panelContent)

	return panel
}

// renderMonitorTitle renders the title bar for the monitor view.
func (m Model) renderMonitorTitle() string {
	totalInstances := 0
	uniqueJobs := 0
	for _, row := range m.monitorGroups {
		if row.isHeader {
			uniqueJobs++
			totalInstances += row.count
		}
	}

	serverName := "local"
	if b := m.manager.ActiveBackend(); b != nil {
		serverName = b.Name()
	}

	title := fmt.Sprintf("🔍 Running Jobs Monitor - %s", serverName)
	stats := fmt.Sprintf("%d instances • %d jobs", totalInstances, uniqueJobs)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorHighlight).
		PaddingLeft(1)

	statsStyle := lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingRight(1)

	return titleStyle.Render(title) + "  " + statsStyle.Render(stats)
}

// renderMonitorTableHeader renders the column headers.
func (m Model) renderMonitorTableHeader() string {
	headers := []string{
		padRight("JOB NAME", 30),
		padRight("PID", 8),
		padRight("STATE", 6),
		padRight("CPU%", 6),
		padRight("MEM", 8),
		padRight("ELAPSED", 12),
		padRight("STARTED", 12),
	}

	headerText := "  " + strings.Join(headers, " ")

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorCyan).
		PaddingLeft(1)

	return style.Render(headerText)
}

// renderMonitorSeparator renders a separator line.
func (m Model) renderMonitorSeparator() string {
	style := lipgloss.NewStyle().
		Foreground(colorBorder).
		PaddingLeft(1)

	return style.Render(strings.Repeat("─", m.width-6))
}

// renderMonitorTableRow renders a single row in the table.
func (m Model) renderMonitorTableRow(row monitorGroupRow, selected bool, width int) string {
	if row.isHeader {
		return m.renderMonitorHeaderRow(row, selected, width)
	}
	return m.renderMonitorInstanceRow(row, selected, width)
}

// renderMonitorHeaderRow renders a job group header.
func (m Model) renderMonitorHeaderRow(row monitorGroupRow, selected bool, width int) string {
	warning := ""
	if row.count > 3 {
		warning = " ⚠️"
	}

	text := fmt.Sprintf("  ▸ %s (%d running)%s", row.jobName, row.count, warning)

	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorHighlight).
		Width(width)

	if selected {
		style = style.Background(lipgloss.Color("#44475a"))
	}

	return style.Render(text)
}

// renderMonitorInstanceRow renders an individual running job instance.
func (m Model) renderMonitorInstanceRow(row monitorGroupRow, selected bool, width int) string {
	// Truncate job name if too long
	jobName := row.jobName
	if len(jobName) > 28 {
		jobName = jobName[:25] + "..."
	}

	// Format start time
	startTimeStr := row.startTime.Format("15:04:05")

	// Format state with color (but not when selected - background would override)
	stateText := row.state
	if !selected {
		stateColor := colorFg
		switch row.state {
		case "R":
			stateColor = colorGreen // Running - green
		case "S":
			stateColor = colorMuted // Sleeping - gray
		case "D":
			stateColor = colorRed // Disk wait - red
		case "Z":
			stateColor = colorRed // Zombie - red
		}
		stateStyle := lipgloss.NewStyle().Foreground(stateColor)
		stateText = stateStyle.Render(row.state)
	}

	columns := []string{
		padRight("    "+jobName, 30),
		padRight(fmt.Sprintf("%d", row.pid), 8),
		padRight(stateText, 6),
		padRight(row.cpu+"%", 6),
		padRight(row.memory, 8),
		padRight(row.elapsed, 12),
		padRight(startTimeStr, 12),
	}

	text := "  " + strings.Join(columns, " ")

	style := lipgloss.NewStyle().
		Foreground(colorFg).
		Width(width)

	if selected {
		style = style.Background(lipgloss.Color("#44475a")).Bold(true)
	}

	return style.Render(text)
}

// padRight pads a string to the right with spaces.
func padRight(s string, width int) string {
	// Strip ANSI codes for width calculation
	displayWidth := lipgloss.Width(s)
	if displayWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-displayWidth)
}

// renderEmptyMonitor renders the view when no jobs are running.
func (m Model) renderEmptyMonitor() string {
	serverName := "local"
	if b := m.manager.ActiveBackend(); b != nil {
		serverName = b.Name()
	}

	// Account for: top bar (1) + bottom bar (1) = 2 lines
	availableHeight := m.height - 2

	title := fmt.Sprintf("🔍 Running Jobs Monitor - %s", serverName)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorHighlight).
		PaddingLeft(1)

	emptyMsg := "No jobs currently running"
	emptyStyle := lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingTop(2).
		PaddingLeft(2)

	helpMsg := "Jobs will appear here when they are running. Press 'r' to refresh or 'q' to exit."
	helpStyle := lipgloss.NewStyle().
		Foreground(colorMuted).
		PaddingLeft(2)

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(title),
		"",
		emptyStyle.Render(emptyMsg),
		helpStyle.Render(helpMsg),
	)

	panel := activePanelStyle.
		Width(m.width - 4).
		Height(availableHeight - 2).
		Render(content)

	return panel
}

// handleMonitorKey handles keyboard input in monitor mode.
func (m *Model) handleMonitorKey(key string) tea.Cmd {
	switch key {
	case "q", "esc":
		// Exit monitor mode
		m.mode = modeNormal
		return nil

	case "up":
		if len(m.monitorGroups) == 0 {
			return nil
		}
		m.monitorSelected--
		if m.monitorSelected < 0 {
			m.monitorSelected = 0
		}
		return nil

	case "down":
		if len(m.monitorGroups) == 0 {
			return nil
		}
		m.monitorSelected++
		if m.monitorSelected >= len(m.monitorGroups) {
			m.monitorSelected = len(m.monitorGroups) - 1
		}
		return nil

	case "r":
		// Manual refresh
		m.buildMonitorRows()
		return nil

	case "k":
		// Kill selected job
		return m.killSelectedJob()
	}

	return nil
}

// killSelectedJob kills the currently selected job instance.
func (m *Model) killSelectedJob() tea.Cmd {
	if len(m.monitorGroups) == 0 || m.monitorSelected < 0 || m.monitorSelected >= len(m.monitorGroups) {
		return m.setStatus("No job selected", statusError, 2*time.Second)
	}

	row := m.monitorGroups[m.monitorSelected]

	// Can only kill instances, not headers
	if row.isHeader {
		return m.setStatus("Select a job instance to kill (not the header)", statusError, 2*time.Second)
	}

	b := m.manager.ActiveBackend()
	if b == nil {
		return m.setStatus("No backend available", statusError, 2*time.Second)
	}

	// Kill the job
	if err := b.KillJob(row.pid); err != nil {
		return m.setStatus(fmt.Sprintf("Failed to kill PID %d: %v", row.pid, err), statusError, 3*time.Second)
	}

	// Refresh the monitor view
	m.buildMonitorRows()

	return m.setStatus(fmt.Sprintf("Killed PID %d", row.pid), statusSuccess, 2*time.Second)
}
