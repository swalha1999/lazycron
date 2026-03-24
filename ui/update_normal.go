package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
)

// currentJobIndex returns the job index for the current selectedRow, or -1 if on a header.
func (m *Model) currentJobIndex() int {
	rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
	if m.selectedRow < 0 || m.selectedRow >= len(rows) {
		return -1
	}
	if rows[m.selectedRow].kind == rowJob {
		return rows[m.selectedRow].jobIdx
	}
	return -1
}

// isOnHeader returns true if the current selectedRow is a group header.
func (m *Model) isOnHeader() bool {
	rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
	if m.selectedRow < 0 || m.selectedRow >= len(rows) {
		return false
	}
	return rows[m.selectedRow].kind == rowHeader
}

// toggleCurrentHeader toggles the collapse state of the header at selectedRow.
func (m *Model) toggleCurrentHeader() {
	rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
	if m.selectedRow < 0 || m.selectedRow >= len(rows) {
		return
	}
	if rows[m.selectedRow].kind == rowHeader {
		project := rows[m.selectedRow].project
		m.collapsedProjects[project] = !m.collapsedProjects[project]
	}
}

// clampSelectedRow ensures selectedRow is within bounds of current rows.
func (m *Model) clampSelectedRow() {
	rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
	if len(rows) == 0 {
		m.selectedRow = 0
		return
	}
	if m.selectedRow >= len(rows) {
		m.selectedRow = len(rows) - 1
	}
	if m.selectedRow < 0 {
		m.selectedRow = 0
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "tab", "right", "l":
		m.focusPanel = (m.focusPanel + 1) % panelCount
		if m.focusPanel != panelDetail && m.focusPanel != panelServers {
			m.lastLeftPanel = m.focusPanel
		}
		return m, nil
	case "shift+tab", "left", "h":
		m.focusPanel = (m.focusPanel + panelCount - 1) % panelCount
		if m.focusPanel != panelDetail && m.focusPanel != panelServers {
			m.lastLeftPanel = m.focusPanel
		}
		return m, nil
	case "1":
		m.focusPanel = panelServers
		return m, nil
	case "2":
		m.focusPanel = panelJobs
		m.lastLeftPanel = panelJobs
		return m, nil
	case "3":
		m.focusPanel = panelHistory
		m.lastLeftPanel = panelHistory
		return m, nil
	case "4":
		m.focusPanel = panelDetail
		return m, nil

	case "up", "k":
		switch m.focusPanel {
		case panelServers:
			if next := m.nextVisibleServer(m.serverSelected-1, -1); next >= 0 {
				m.serverSelected = next
			}
		case panelJobs:
			rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
			if m.selectedRow > 0 {
				m.selectedRow--
				m.detailScroll = 0
			}
			_ = rows
		case panelHistory:
			if next := m.nextVisibleHistory(m.historySelected-1, -1); next >= 0 {
				m.historySelected = next
				m.detailScroll = 0
			}
		case panelDetail:
			if m.detailScroll > 0 {
				m.detailScroll--
			}
		}

	case "down", "j":
		switch m.focusPanel {
		case panelServers:
			if next := m.nextVisibleServer(m.serverSelected+1, +1); next >= 0 {
				m.serverSelected = next
			}
		case panelJobs:
			rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
			if m.selectedRow < len(rows)-1 {
				m.selectedRow++
				m.detailScroll = 0
			}
		case panelHistory:
			if next := m.nextVisibleHistory(m.historySelected+1, +1); next >= 0 {
				m.historySelected = next
				m.detailScroll = 0
			}
		case panelDetail:
			m.detailScroll++
		}

	case "K":
		if m.focusPanel == panelJobs && len(m.jobs) > 1 && !m.isOnHeader() {
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				swapIdx := findSiblingJob(m.jobs, jobIdx, -1)
				if swapIdx >= 0 {
					m.jobs[jobIdx], m.jobs[swapIdx] = m.jobs[swapIdx], m.jobs[jobIdx]
					rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
					m.selectedRow = rowForJobIdx(rows, swapIdx)
					b := m.manager.ActiveBackend()
					return m, tea.Batch(saveJobs(b, m.jobs), m.setStatus(fmt.Sprintf("Moved '%s' up", m.jobs[swapIdx].Name), statusInfo, 2*time.Second))
				}
			}
		}

	case "J":
		if m.focusPanel == panelJobs && len(m.jobs) > 1 && !m.isOnHeader() {
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				swapIdx := findSiblingJob(m.jobs, jobIdx, +1)
				if swapIdx >= 0 {
					m.jobs[jobIdx], m.jobs[swapIdx] = m.jobs[swapIdx], m.jobs[jobIdx]
					rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
					m.selectedRow = rowForJobIdx(rows, swapIdx)
					b := m.manager.ActiveBackend()
					return m, tea.Batch(saveJobs(b, m.jobs), m.setStatus(fmt.Sprintf("Moved '%s' down", m.jobs[swapIdx].Name), statusInfo, 2*time.Second))
				}
			}
		}

	case "enter":
		if m.focusPanel == panelServers {
			return m.switchToServer(m.serverSelected)
		}
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			if m.isOnHeader() {
				m.toggleCurrentHeader()
				m.clampSelectedRow()
				return m, nil
			}
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				m.mode = modeForm
				m.form = newFormForEdit(m.jobs[jobIdx], jobIdx, m.activeDirLister())
				m.statusMsg = ""
				return m, m.form.focusActive()
			}
		}

	case "e":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 && !m.isOnHeader() {
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				m.mode = modeForm
				m.form = newFormForEdit(m.jobs[jobIdx], jobIdx, m.activeDirLister())
				m.statusMsg = ""
				return m, m.form.focusActive()
			}
		}

	case "a":
		if m.focusPanel == panelServers {
			m.mode = modeAddServer
			m.serverForm = newServerForm()
			m.statusMsg = ""
			return m, m.serverForm.focusActive()
		}

	case "c":
		if m.focusPanel == panelServers {
			idx := m.serverSelected
			if idx == 0 {
				return m, nil // Local is always connected
			}
			info := m.manager.ServerAt(idx)
			if info.Status == backend.ConnDisconnected || info.Status == backend.ConnError {
				m.statusMsg = fmt.Sprintf("Connecting to %s...", info.Name)
				m.statusKind = statusInfo
				return m, connectServer(m.manager, idx)
			}
		}

	case "d":
		if m.focusPanel == panelServers {
			idx := m.serverSelected
			if idx == 0 {
				return m, nil
			}
			info := m.manager.ServerAt(idx)
			if info.Status == backend.ConnConnected || info.Status == backend.ConnConnecting {
				b := m.manager.BackendAt(idx)
				if b != nil {
					b.Close()
				}
				m.manager.SetServerStatus(idx, backend.ConnDisconnected, "")
				m.manager.InvalidateCache(idx)
				cmd := m.setStatus(fmt.Sprintf("Disconnected from %s", info.Name), statusInfo, 3*time.Second)
				if m.manager.ActiveIndex() == idx {
					m.manager.SwitchTo(0)
					b := m.manager.ActiveBackend()
					return m, tea.Batch(loadJobs(b), loadHistory(b), cmd)
				}
				return m, cmd
			}
		}

	case "n":
		m.mode = modeNewJobChoice
		m.statusMsg = ""
		return m, nil

	case "D":
		if m.focusPanel == panelServers {
			if m.serverSelected == 0 {
				return m, nil // Can't remove local
			}
			m.mode = modeConfirmDeleteServer
			m.confirmYes = false
			m.statusMsg = ""
		} else if m.focusPanel == panelJobs && len(m.jobs) > 0 && !m.isOnHeader() {
			m.mode = modeConfirmDelete
			m.confirmYes = false
			m.statusMsg = ""
		} else if m.focusPanel == panelHistory && len(m.history) > 0 {
			m.mode = modeConfirmDeleteHistory
			m.confirmYes = false
			m.statusMsg = ""
		}

	case " ":
		if m.focusPanel == panelServers {
			return m.switchToServer(m.serverSelected)
		}
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			if m.isOnHeader() {
				m.toggleCurrentHeader()
				m.clampSelectedRow()
				return m, nil
			}
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				m.jobs[jobIdx].Enabled = !m.jobs[jobIdx].Enabled
				status := "enabled"
				if !m.jobs[jobIdx].Enabled {
					status = "disabled"
				}
				b := m.manager.ActiveBackend()
				return m, tea.Batch(saveJobs(b, m.jobs), m.setStatus(fmt.Sprintf("Job '%s' %s", m.jobs[jobIdx].Name, status), statusSuccess, 4*time.Second))
			}
		}

	case "p":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 && !m.isOnHeader() {
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				m.projectInput = newProjectInput()
				m.projectInput.SetValue(m.jobs[jobIdx].Project)
				m.mode = modeProjectPrompt
				m.statusMsg = ""
				return m, m.projectInput.Focus()
			}
		}

	case "r":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 && !m.isOnHeader() {
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				job := m.jobs[jobIdx]
				m.statusMsg = fmt.Sprintf("Running '%s'...", job.Name)
				m.statusKind = statusInfo
				b := m.manager.ActiveBackend()
				return m, runJob(b, job.ID, job.Name, job.Command)
			}
		}

	case "U":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 && !m.isOnHeader() {
			jobIdx := m.currentJobIndex()
			if jobIdx >= 0 {
				job := &m.jobs[jobIdx]
				if job.Wrapped {
					return m, m.setStatus(fmt.Sprintf("Job '%s' is already up to date", job.Name), statusInfo, 3*time.Second)
				}
				job.Wrapped = true
				b := m.manager.ActiveBackend()
				return m, tea.Batch(saveJobs(b, m.jobs), m.setStatus(fmt.Sprintf("Updated '%s' to latest format", job.Name), statusSuccess, 4*time.Second))
			}
		}

	case "R":
		m.clearSearch()
		m.statusMsg = "Refreshing..."
		m.statusKind = statusInfo
		b := m.manager.ActiveBackend()
		return m, tea.Batch(loadJobs(b), loadHistory(b))

	case "u":
		m.statusMsg = "Checking for updates..."
		m.statusKind = statusInfo
		return m, selfUpdate(m.version)

	case "?":
		m.mode = modeHelp
		m.statusMsg = ""

	case "/":
		if m.focusPanel != panelDetail {
			return m, m.enterSearch()
		}

	case "esc":
		if m.hasActiveSearch() {
			m.clearSearch()
			return m, nil
		}
	}
	return m, nil
}

func (m Model) switchToServer(index int) (tea.Model, tea.Cmd) {
	if index == m.manager.ActiveIndex() {
		return m, nil
	}
	m.clearSearch()

	info := m.manager.ServerAt(index)

	if index == 0 {
		m.manager.SwitchTo(0)
		b := m.manager.ActiveBackend()
		return m, tea.Batch(loadJobs(b), loadHistory(b), m.setStatus("Switched to local", statusSuccess, 3*time.Second))
	}

	if info.Status == backend.ConnConnected {
		m.manager.SwitchTo(index)
		if cached := m.manager.GetCache(index); cached != nil {
			m.jobs = cached.Jobs
			m.history = cached.History
			m.selected = 0
			m.selectedRow = 0
			m.historySelected = 0
		}
		return m, loadServerData(m.manager, index)
	}

	m.serverSwitching = true
	m.manager.SwitchTo(index)
	m.statusMsg = fmt.Sprintf("Connecting to %s...", info.Name)
	m.statusKind = statusInfo
	return m, connectServer(m.manager, index)
}
