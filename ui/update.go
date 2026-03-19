package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case jobsLoadedMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		m.jobs = msg.jobs
		if len(m.jobs) > 0 {
			m.statusMsg = fmt.Sprintf("Loaded %d job(s)", len(m.jobs))
			m.statusKind = statusInfo
			m.statusID++
			return m, clearStatusAfter(m.statusID, 2*time.Second)
		}
		return m, nil

	case jobSavedMsg:
		if msg.err != nil {
			m.statusMsg = "Save failed: " + msg.err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		m.manager.InvalidateCache(m.manager.ActiveIndex())
		return m, nil

	case historyTickMsg:
		b := m.manager.ActiveBackend()
		return m, tea.Batch(loadHistory(b), historyTick())

	case historyLoadedMsg:
		if msg.err == nil {
			m.history = msg.entries
		}
		return m, nil

	case historySavedMsg:
		if msg.err == nil {
			b := m.manager.ActiveBackend()
			return m, loadHistory(b)
		}
		return m, nil

	case jobRanMsg:
		m.statusID++
		output := msg.output
		if output == "" && msg.err != nil {
			output = msg.err.Error()
		}
		success := msg.err == nil
		b := m.manager.ActiveBackend()
		saveCmd := saveHistory(b, msg.name, output, success)

		if msg.err != nil {
			m.runOutput = msg.output
			if m.runOutput == "" {
				m.runOutput = msg.err.Error()
			}
			m.runJobName = msg.name
			m.runOutputFailed = true
			m.mode = modeRunOutput
			m.runOutputScroll = 0
			m.statusMsg = ""
			return m, saveCmd
		}
		if msg.output != "" {
			m.runOutput = msg.output
			m.runJobName = msg.name
			m.runOutputFailed = false
			m.mode = modeRunOutput
			m.runOutputScroll = 0
			m.statusMsg = ""
			return m, saveCmd
		}
		m.statusMsg = fmt.Sprintf("Job '%s' ran successfully", msg.name)
		m.statusKind = statusSuccess
		return m, tea.Batch(saveCmd, clearStatusAfter(m.statusID, 4*time.Second))

	case serverConnectedMsg:
		if msg.err != nil {
			m.serverSwitching = false
			// If auth failed, prompt for password
			var authErr *sshclient.AuthError
			if errors.As(msg.err, &authErr) {
				m.mode = modePasswordPrompt
				m.passwordInput = newPasswordInput()
				m.passwordServerIdx = msg.index
				m.statusMsg = "Authentication failed — enter password or configure SSH keys"
				m.statusKind = statusInfo
				return m, m.passwordInput.Focus()
			}
			m.statusMsg = fmt.Sprintf("Connection failed: %s", msg.err)
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		return m, loadServerData(m.manager, msg.index)

	case serverDataLoadedMsg:
		m.serverSwitching = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to load data: %s", msg.err)
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		m.manager.SetCache(msg.index, &backend.CachedData{
			Jobs:      msg.jobs,
			History:   msg.history,
			FetchedAt: time.Now(),
		})
		if msg.index == m.manager.ActiveIndex() {
			m.jobs = msg.jobs
			m.history = msg.history
			m.selected = 0
			m.historySelected = 0
			serverName := m.manager.ServerAt(msg.index).Name
			m.statusMsg = fmt.Sprintf("Switched to %s", serverName)
			m.statusKind = statusSuccess
			m.statusID++
			return m, clearStatusAfter(m.statusID, 3*time.Second)
		}
		return m, nil

	case clearStatusMsg:
		if msg.id == m.statusID {
			m.statusMsg = ""
			m.statusKind = statusNone
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward other messages to form textinput (for cursor blink)
	switch m.mode {
	case modeForm:
		cmd := m.form.updateInput(msg)
		return m, cmd
	case modeAddServer:
		cmd := m.serverForm.updateInput(msg)
		return m, cmd
	case modePasswordPrompt:
		var cmd tea.Cmd
		m.passwordInput, cmd = m.passwordInput.Update(msg)
		return m, cmd
	case modeTemplatePicker:
		if m.templatePicker.phase == phaseVariables && len(m.templatePicker.variableInputs) > 0 {
			idx := m.templatePicker.activeVariable
			var cmd tea.Cmd
			m.templatePicker.variableInputs[idx], cmd = m.templatePicker.variableInputs[idx].Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case modeForm:
		return m.handleFormKey(msg)
	case modeConfirmDelete:
		return m.handleConfirmKey(msg)
	case modeConfirmDeleteServer:
		return m.handleConfirmDeleteServerKey(msg)
	case modeConfirmDeleteHistory:
		return m.handleConfirmDeleteHistoryKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	case modeRunOutput:
		return m.handleRunOutputKey(msg)
	case modeAddServer:
		return m.handleAddServerKey(msg)
	case modePasswordPrompt:
		return m.handlePasswordPromptKey(msg)
	case modeNewJobChoice:
		return m.handleNewJobChoiceKey(msg)
	case modeTemplatePicker:
		return m.handleTemplatePickerKey(msg)
	default:
		return m.handleNormalKey(msg)
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
			m.form = newFormForEdit(m.jobs[m.selected], m.selected)
			m.statusMsg = ""
			return m, m.form.focusActive()
		}

	case "e":
		if m.focusPanel == panelJobs && len(m.jobs) > 0 {
			m.mode = modeForm
			m.form = newFormForEdit(m.jobs[m.selected], m.selected)
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

func (m Model) handleAddServerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.mode = modeNormal
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)

	case "enter":
		srv, err := m.serverForm.buildServerConfig()
		if err != nil {
			m.statusMsg = err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}

		// Save to config file
		if err := config.AddServer(srv); err != nil {
			m.statusMsg = "Failed to save config: " + err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
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
		m.statusMsg = fmt.Sprintf("Added server '%s'", srv.Name)
		m.statusKind = statusSuccess
		m.statusID++
		m.serverSelected = m.manager.ServerCount() - 1
		return m, clearStatusAfter(m.statusID, 4*time.Second)

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
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)

	case "enter":
		password := m.passwordInput.Value()
		if password == "" {
			m.statusMsg = "Password cannot be empty"
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 3*time.Second)
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
	case "y", "Y":
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
		m.statusMsg = fmt.Sprintf("Removed server '%s'", serverName)
		m.statusKind = statusSuccess
		m.statusID++

		if switchedToLocal {
			b := m.manager.ActiveBackend()
			return m, tea.Batch(loadJobs(b), loadHistory(b), clearStatusAfter(m.statusID, 4*time.Second))
		}
		return m, clearStatusAfter(m.statusID, 4*time.Second)

	case "n", "N", "esc":
		m.mode = modeNormal
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)
	}
	return m, nil
}

func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.form.picker.focused {
		syncPickerToInput := func() {
			m.form.inputs[fieldSchedule].SetValue(m.form.picker.Expression())
		}
		switch key {
		case "up", "k":
			m.form.picker.scrollUp()
			syncPickerToInput()
			return m, nil
		case "down", "j":
			m.form.picker.scrollDown()
			syncPickerToInput()
			return m, nil
		case "left", "h":
			m.form.picker.moveLeft()
			return m, nil
		case "right", "l":
			m.form.picker.moveRight()
			return m, nil
		case " ":
			m.form.picker.cycleMode()
			syncPickerToInput()
			return m, nil
		case "esc":
			m.mode = modeNormal
			m.statusMsg = "Cancelled"
			m.statusKind = statusInfo
			m.statusID++
			return m, clearStatusAfter(m.statusID, 3*time.Second)
		case "tab":
			cmd := m.form.nextField()
			return m, cmd
		case "enter":
			m.form.inputs[fieldSchedule].SetValue(m.form.picker.Expression())
			m.form.picker.focused = false
		case "shift+tab":
			cmd := m.form.prevField()
			return m, cmd
		default:
			return m, nil
		}
	}

	if m.form.activeField == fieldWorkDir && m.form.completer.active {
		switch key {
		case "down":
			m.form.completer.selectNext()
			return m, nil
		case "up":
			m.form.completer.selectPrev()
			return m, nil
		case "enter", "right":
			if m.form.completer.selected >= 0 {
				path := m.form.completer.drillIn()
				if path != "" {
					m.form.inputs[fieldWorkDir].SetValue(path)
					m.form.inputs[fieldWorkDir].CursorEnd()
				}
				return m, nil
			}
			if key == "right" {
				return m, nil
			}
		case "left":
			path := m.form.completer.drillOut()
			m.form.inputs[fieldWorkDir].SetValue(path)
			m.form.inputs[fieldWorkDir].CursorEnd()
			return m, nil
		case "esc":
			m.form.completer.reset()
			return m, nil
		}
	}

	switch key {
	case "esc":
		m.mode = modeNormal
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)

	case "enter":
		job, err := m.form.buildJob()
		if err != nil {
			m.statusMsg = err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}

		b := m.manager.ActiveBackend()
		if m.form.editing {
			job.Enabled = m.jobs[m.form.editIndex].Enabled
			m.jobs[m.form.editIndex] = job
			m.statusMsg = fmt.Sprintf("Updated job '%s'", job.Name)
		} else {
			m.jobs = append(m.jobs, job)
			m.selected = len(m.jobs) - 1
			m.statusMsg = fmt.Sprintf("Created job '%s'", job.Name)
		}
		m.statusKind = statusSuccess
		m.mode = modeNormal
		m.statusID++
		return m, tea.Batch(saveJobs(b, m.jobs), clearStatusAfter(m.statusID, 4*time.Second))

	case "tab":
		cmd := m.form.nextField()
		return m, cmd
	case "shift+tab":
		cmd := m.form.prevField()
		return m, cmd
	default:
		cmd := m.form.updateInput(msg)
		if m.form.activeField == fieldWorkDir {
			m.form.completer.update(m.form.inputs[fieldWorkDir].Value())
		}
		return m, cmd
	}
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if len(m.jobs) > 0 {
			name := m.jobs[m.selected].Name
			m.jobs = append(m.jobs[:m.selected], m.jobs[m.selected+1:]...)
			if m.selected >= len(m.jobs) && m.selected > 0 {
				m.selected--
			}
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

func (m Model) handleNewJobChoiceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "1":
		m.mode = modeForm
		m.form = newForm()
		m.statusMsg = ""
		return m, m.form.focusActive()
	case "t", "2":
		m.mode = modeTemplatePicker
		m.templatePicker = newTemplatePicker()
		m.statusMsg = ""
		return m, nil
	case "esc", "q":
		m.mode = modeNormal
		return m, nil
	}
	return m, nil
}

func (m Model) handleTemplatePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	tp := &m.templatePicker
	key := msg.String()

	switch tp.phase {
	case phaseChooseCategory:
		switch key {
		case "up", "k":
			if tp.categorySelected > 0 {
				tp.categorySelected--
			}
		case "down", "j":
			if tp.categorySelected < len(tp.categories)-1 {
				tp.categorySelected++
			}
		case "enter":
			tp.selectCategory()
		case "esc":
			m.mode = modeNewJobChoice
		}
		return m, nil

	case phaseChooseTemplate:
		switch key {
		case "up", "k":
			if tp.templateSelected > 0 {
				tp.templateSelected--
			}
		case "down", "j":
			if tp.templateSelected < len(tp.templateList)-1 {
				tp.templateSelected++
			}
		case "enter":
			tp.selectTemplate()
			// Focus the first variable input if we're in variable phase
			if tp.phase == phaseVariables && len(tp.variableInputs) > 0 {
				return m, tp.variableInputs[0].Focus()
			}
		case "esc":
			tp.back()
		}
		return m, nil

	case phaseVariables:
		// Handle completer interactions when active on a path variable
		if tp.activeVarIsPath() && tp.completer.active {
			switch key {
			case "down":
				tp.completer.selectNext()
				return m, nil
			case "up":
				tp.completer.selectPrev()
				return m, nil
			case "enter", "right":
				if tp.completer.selected >= 0 {
					path := tp.completer.drillIn()
					if path != "" {
						tp.variableInputs[tp.activeVariable].SetValue(path)
						tp.variableInputs[tp.activeVariable].CursorEnd()
					}
					return m, nil
				}
				if key == "right" {
					return m, nil
				}
				// enter with no selection falls through to create job
			case "left":
				path := tp.completer.drillOut()
				tp.variableInputs[tp.activeVariable].SetValue(path)
				tp.variableInputs[tp.activeVariable].CursorEnd()
				return m, nil
			case "esc":
				tp.completer.reset()
				return m, nil
			}
		}

		switch key {
		case "enter":
			// Build the job from the template
			values := tp.buildValues()
			resolvedCmd, cronExpr := tp.selectedTmpl.Apply(values)

			// Extract work dir from "cd <path> && <command>" pattern
			workDir := ""
			if strings.HasPrefix(resolvedCmd, "cd ") {
				if idx := strings.Index(resolvedCmd, " && "); idx != -1 {
					workDir = strings.TrimPrefix(resolvedCmd[:idx], "cd ")
					resolvedCmd = strings.TrimSpace(resolvedCmd[idx+4:])
				}
			}

			// Pre-fill the job form with template data
			m.mode = modeForm
			m.form = newForm()
			m.form.inputs[fieldName].SetValue(tp.selectedTmpl.Name)
			m.form.inputs[fieldCommand].SetValue(resolvedCmd)
			m.form.inputs[fieldSchedule].SetValue(cronExpr)
			m.form.inputs[fieldWorkDir].SetValue(workDir)
			m.form.picker.ParseExpression(cronExpr)
			return m, m.form.focusActive()

		case "tab":
			if len(tp.variableInputs) > 0 {
				tp.variableInputs[tp.activeVariable].Blur()
				tp.completer.reset()
				tp.activeVariable = (tp.activeVariable + 1) % len(tp.variableInputs)
				tp.activateCompleterForCurrentVar()
				return m, tp.variableInputs[tp.activeVariable].Focus()
			}
		case "shift+tab":
			if len(tp.variableInputs) > 0 {
				tp.variableInputs[tp.activeVariable].Blur()
				tp.completer.reset()
				tp.activeVariable = (tp.activeVariable - 1 + len(tp.variableInputs)) % len(tp.variableInputs)
				tp.activateCompleterForCurrentVar()
				return m, tp.variableInputs[tp.activeVariable].Focus()
			}
		case "esc":
			tp.completer.reset()
			if !tp.back() {
				m.mode = modeNewJobChoice
			}
		default:
			// Forward to active variable input
			if len(tp.variableInputs) > 0 {
				var cmd tea.Cmd
				tp.variableInputs[tp.activeVariable], cmd = tp.variableInputs[tp.activeVariable].Update(msg)
				// Update completer as user types in a path variable
				if tp.activeVarIsPath() {
					tp.completer.update(tp.variableInputs[tp.activeVariable].Value())
				}
				return m, cmd
			}
		}
		return m, nil
	}

	return m, nil
}
