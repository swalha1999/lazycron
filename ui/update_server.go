package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

func (m Model) handleAddServerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.mode = modeNormal
		return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)

	case "enter":
		srv, err := m.serverForm.buildServerConfig()
		if err != nil {
			return m, m.setStatus(err.Error(), statusError, 5*time.Second)
		}

		// Save to config file
		if err := config.AddServer(srv); err != nil {
			return m, m.setStatus("Failed to save config: "+err.Error(), statusError, 5*time.Second)
		}

		// Add to manager
		info := backend.ServerInfo{
			Name:   srv.Name,
			Host:   srv.Host,
			Port:   srv.Port,
			User:   srv.User,
			Status: backend.ConnDisconnected,
		}
		client := sshclient.NewClient(srv.Host, srv.Port, srv.User, "", config.ExpandHome(srv.KeyPath), srv.UseAgent)
		remote := backend.NewRemoteBackend(srv.Name, client)
		m.manager.AddServer(info, remote)

		m.mode = modeNormal
		m.serverSelected = m.manager.ServerCount() - 1
		return m, m.setStatus(fmt.Sprintf("Added server '%s'", srv.Name), statusSuccess, 4*time.Second)

	case "tab":
		cmd := m.serverForm.nextField()
		return m, cmd
	case "shift+tab":
		cmd := m.serverForm.prevField()
		return m, cmd
	default:
		cmd := m.serverForm.updateInput(msg)
		return m, cmd
	}
}

func (m Model) handlePasswordPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)

	case "enter":
		password := m.passwordInput.Value()
		if password == "" {
			return m, m.setStatus("Password cannot be empty", statusError, 3*time.Second)
		}
		m.mode = modeNormal
		m.serverSwitching = true
		m.statusMsg = "Connecting..."
		m.statusKind = statusInfo
		return m, connectServerWithPassword(m.manager, m.passwordServerIdx, password)

	default:
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return m, cmd
	}
}

func (m Model) handleConfirmDeleteServerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "right", "h", "l":
		m.confirmYes = !m.confirmYes
		return m, nil
	case "y", "Y":
		m.confirmYes = true
		return m.executeConfirmDeleteServer()
	case "enter":
		return m.executeConfirmDeleteServer()
	case "n", "N", "esc":
		m.mode = modeNormal
		return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)
	}
	return m, nil
}

func (m Model) executeConfirmDeleteServer() (tea.Model, tea.Cmd) {
	if !m.confirmYes {
		m.mode = modeNormal
		return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)
	}
	idx := m.serverSelected
	if idx <= 0 || idx >= m.manager.ServerCount() {
		m.mode = modeNormal
		return m, nil
	}
	serverName := m.manager.ServerAt(idx).Name

	// Remove from config file
	config.RemoveServer(serverName)

	// Remove from manager (also closes backend)
	switchedToLocal := m.manager.ActiveIndex() == idx
	m.manager.RemoveServer(idx)

	// Fix selection
	if m.serverSelected >= m.manager.ServerCount() {
		m.serverSelected = m.manager.ServerCount() - 1
	}

	m.mode = modeNormal
	cmd := m.setStatus(fmt.Sprintf("Removed server '%s'", serverName), statusSuccess, 4*time.Second)
	if switchedToLocal {
		b := m.manager.ActiveBackend()
		return m, tea.Batch(loadJobs(b), loadHistory(b), cmd)
	}
	return m, cmd
}
