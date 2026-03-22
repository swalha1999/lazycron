package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		jobIdx := m.currentJobIndex()
		if jobIdx >= 0 && jobIdx < len(m.jobs) {
			name := m.jobs[jobIdx].Name
			m.jobs = append(m.jobs[:jobIdx], m.jobs[jobIdx+1:]...)
			m.clampSelectedRow()
			m.statusMsg = fmt.Sprintf("Deleted job '%s'", name)
			m.statusKind = statusSuccess
			m.mode = modeNormal
			m.statusID++
			b := m.manager.ActiveBackend()
			return m, tea.Batch(saveJobs(b, m.jobs), clearStatusAfter(m.statusID, 4*time.Second))
		}
	case "n", "N", "esc":
		m.mode = modeNormal
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)
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

func (m Model) handleConfirmDeleteHistoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.historySelected >= 0 && m.historySelected < len(m.history) {
			entry := m.history[m.historySelected]
			if entry.FilePath != "" {
				os.Remove(entry.FilePath)
			}
			m.history = append(m.history[:m.historySelected], m.history[m.historySelected+1:]...)
			if m.historySelected >= len(m.history) && m.historySelected > 0 {
				m.historySelected--
			}
			m.statusMsg = fmt.Sprintf("Deleted history entry '%s'", entry.JobName)
			m.statusKind = statusSuccess
			m.mode = modeNormal
			m.statusID++
			return m, clearStatusAfter(m.statusID, 4*time.Second)
		}
	case "n", "N", "esc":
		m.mode = modeNormal
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)
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
			if newProject != "" {
				m.statusMsg = fmt.Sprintf("Set project '%s' on '%s'", newProject, m.jobs[jobIdx].Name)
			} else {
				m.statusMsg = fmt.Sprintf("Cleared project on '%s'", m.jobs[jobIdx].Name)
			}
			m.statusKind = statusSuccess
			m.statusID++
			// Rebuild rows and move selection to follow the job
			rows := buildRows(m.jobs, m.collapsedProjects)
			m.selectedRow = rowForJobIdx(rows, jobIdx)
			b := m.manager.ActiveBackend()
			return m, tea.Batch(saveJobs(b, m.jobs), clearStatusAfter(m.statusID, 4*time.Second))
		}
		m.mode = modeNormal
	case "esc":
		m.mode = modeNormal
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)
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
