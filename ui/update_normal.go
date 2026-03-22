package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
)

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
			if m.serverSelected > 0 {
				m.serverSelected--
			}
		case panelJobs:
			if m.selected > 0 {
				m.selected--
				m.detailScroll = 0
			}
		case panelHistory:
			if m.historySelected > 0 {
				m.historySelected--
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
			if m.serverSelected < m.manager.ServerCount()-1 {
				m.serverSelected++
			}
		case panelJobs:
			if m.selected < len(m.jobs)-1 {
				m.selected++
				m.detailScroll = 0
			}
		case panelHistory:
			if m.historySelected < len(m.history)-1 {
				m.historySelected++
				m.detailScroll = 0
			}
		case panelDetail:
			m.detailScroll++
		}

	case "enter":
		if m.focusPanel == panelServers {
			return m.switchToServer(m.serverSelected)
		}
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			m.mode = modeForm
			m.form = newFormForEdit(m.jobs[m.selected], m.selected, m.activeDirLister())
			m.statusMsg = ""
			return m, m.form.focusActive()
		}

	case "e":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			m.mode = modeForm
			m.form = newFormForEdit(m.jobs[m.selected], m.selected, m.activeDirLister())
			m.statusMsg = ""
			return m, m.form.focusActive()
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
				m.statusMsg = fmt.Sprintf("Disconnected from %s", info.Name)
				m.statusKind = statusInfo
				m.statusID++
				if m.manager.ActiveIndex() == idx {
					m.manager.SwitchTo(0)
					b := m.manager.ActiveBackend()
					return m, tea.Batch(
						loadJobs(b), loadHistory(b),
						clearStatusAfter(m.statusID, 3*time.Second),
					)
				}
				return m, clearStatusAfter(m.statusID, 3*time.Second)
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
			m.statusMsg = ""
		} else if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			m.mode = modeConfirmDelete
			m.statusMsg = ""
		} else if m.focusPanel == panelHistory && len(m.history) > 0 {
			m.mode = modeConfirmDeleteHistory
			m.statusMsg = ""
		}

	case " ":
		if m.focusPanel == panelServers {
			return m.switchToServer(m.serverSelected)
		}
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			m.jobs[m.selected].Enabled = !m.jobs[m.selected].Enabled
			status := "enabled"
			if !m.jobs[m.selected].Enabled {
				status = "disabled"
			}
			m.statusMsg = fmt.Sprintf("Job '%s' %s", m.jobs[m.selected].Name, status)
			m.statusKind = statusSuccess
			m.statusID++
			b := m.manager.ActiveBackend()
			return m, tea.Batch(saveJobs(b, m.jobs), clearStatusAfter(m.statusID, 4*time.Second))
		}

	case "r":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			job := m.jobs[m.selected]
			m.statusMsg = fmt.Sprintf("Running '%s'...", job.Name)
			m.statusKind = statusInfo
			b := m.manager.ActiveBackend()
			return m, runJob(b, job.Name, job.Command)
		}

	case "U":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			job := &m.jobs[m.selected]
			if job.Wrapped {
				m.statusMsg = fmt.Sprintf("Job '%s' is already up to date", job.Name)
				m.statusKind = statusInfo
				m.statusID++
				return m, clearStatusAfter(m.statusID, 3*time.Second)
			}
			job.Wrapped = true
			m.statusMsg = fmt.Sprintf("Updated '%s' to latest format", job.Name)
			m.statusKind = statusSuccess
			m.statusID++
			b := m.manager.ActiveBackend()
			return m, tea.Batch(saveJobs(b, m.jobs), clearStatusAfter(m.statusID, 4*time.Second))
		}

	case "R":
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
	}
	return m, nil
}

func (m Model) switchToServer(index int) (tea.Model, tea.Cmd) {
	if index == m.manager.ActiveIndex() {
		return m, nil
	}

	info := m.manager.ServerAt(index)

	if index == 0 {
		m.manager.SwitchTo(0)
		b := m.manager.ActiveBackend()
		m.statusMsg = "Switched to local"
		m.statusKind = statusSuccess
		m.statusID++
		return m, tea.Batch(loadJobs(b), loadHistory(b), clearStatusAfter(m.statusID, 3*time.Second))
	}

	if info.Status == backend.ConnConnected {
		m.manager.SwitchTo(index)
		if cached := m.manager.GetCache(index); cached != nil {
			m.jobs = cached.Jobs
			m.history = cached.History
			m.selected = 0
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
