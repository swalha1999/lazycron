package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleConfirmDialogKey handles key input for any yes/no confirmation dialog.
// On confirm (y/Y or enter when "yes" is selected) it invokes execute, which
// can assume the user has confirmed and only needs to perform the action.
func (m Model) handleConfirmDialogKey(msg tea.KeyMsg, execute func(Model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "right", "h", "l":
		m.confirmYes = !m.confirmYes
		return m, nil
	case "y", "Y":
		m.confirmYes = true
		return execute(m)
	case "enter":
		if !m.confirmYes {
			m.mode = modeNormal
			return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)
		}
		return execute(m)
	case "n", "N", "esc":
		m.mode = modeNormal
		return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)
	}
	return m, nil
}

func (m Model) executeConfirmDelete() (tea.Model, tea.Cmd) {
	jobIdx := m.currentJobIndex()
	if jobIdx >= 0 && jobIdx < len(m.jobs) {
		name := m.jobs[jobIdx].Name
		m.jobs = append(m.jobs[:jobIdx], m.jobs[jobIdx+1:]...)
		m.clampSelectedRow()
		m.mode = modeNormal
		b := m.manager.ActiveBackend()
		return m, tea.Batch(saveJobs(b, m.jobs), m.setStatus(fmt.Sprintf("Deleted job '%s'", name), statusSuccess, 4*time.Second))
	}
	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "?":
		m.mode = modeNormal
	}
	return m, nil
}

func (m Model) executeConfirmDeleteHistory() (tea.Model, tea.Cmd) {
	if m.historySelected >= 0 && m.historySelected < len(m.history) {
		entry := m.history[m.historySelected]
		var cmds []tea.Cmd
		if entry.FilePath != "" {
			b := m.manager.ActiveBackend()
			cmds = append(cmds, deleteHistory(b, entry.FilePath))
		}
		m.history = append(m.history[:m.historySelected], m.history[m.historySelected+1:]...)
		if m.historySelected >= len(m.history) && m.historySelected > 0 {
			m.historySelected--
		}
		m.mode = modeNormal
		cmds = append(cmds, m.setStatus(fmt.Sprintf("Deleted history entry '%s'", entry.JobName), statusSuccess, 4*time.Second))
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

func (m Model) handleProjectPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		jobIdx := m.currentJobIndex()
		if jobIdx >= 0 {
			newProject := strings.TrimSpace(m.projectInput.Value())
			m.jobs[jobIdx].Project = newProject
			m.mode = modeNormal
			var statusText string
			if newProject != "" {
				statusText = fmt.Sprintf("Set project '%s' on '%s'", newProject, m.jobs[jobIdx].Name)
			} else {
				statusText = fmt.Sprintf("Cleared project on '%s'", m.jobs[jobIdx].Name)
			}
			// Rebuild rows and move selection to follow the job
			rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
			m.selectedRow = rowForJobIdx(rows, jobIdx)
			b := m.manager.ActiveBackend()
			return m, tea.Batch(saveJobs(b, m.jobs), m.setStatus(statusText, statusSuccess, 4*time.Second))
		}
		m.mode = modeNormal
	case "esc":
		m.mode = modeNormal
		return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)
	default:
		var cmd tea.Cmd
		m.projectInput, cmd = m.projectInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleRunOutputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.runOutputScroll = 0
	case "j", "down":
		m.runOutputScroll++
	case "k", "up":
		if m.runOutputScroll > 0 {
			m.runOutputScroll--
		}
	}
	return m, nil
}
