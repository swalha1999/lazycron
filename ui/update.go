package ui

import (
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case splashDoneMsg:
		if m.mode == modeSplash {
			m.mode = modeNormal
		}
		return m, nil

	case jobsLoadedMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		m.jobs = msg.jobs
		cmds := []tea.Cmd{}
		if len(m.jobs) > 0 {
			m.statusMsg = fmt.Sprintf("Loaded %d job(s)", len(m.jobs))
			m.statusKind = statusInfo
			m.statusID++
			cmds = append(cmds, clearStatusAfter(m.statusID, 2*time.Second))
		}
		// Auto-disable completed one-shot jobs (backup for record.sh)
		if len(m.jobs) > 0 && len(m.history) > 0 {
			cmds = append(cmds, disableCompletedOneShots(m.manager.ActiveBackend(), m.jobs, m.history))
		}
		return m, tea.Batch(cmds...)

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
			// Auto-disable completed one-shot jobs (backup for record.sh)
			if len(m.jobs) > 0 && len(m.history) > 0 {
				return m, disableCompletedOneShots(m.manager.ActiveBackend(), m.jobs, m.history)
			}
		}
		return m, nil

	case disableCompletedOneShotsMsg:
		if msg.err != nil {
			m.statusMsg = "Auto-disable failed: " + msg.err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		if msg.disabled > 0 {
			// Reload jobs to reflect the changes
			b := m.manager.ActiveBackend()
			return m, loadJobs(b)
		}
		return m, nil

	case historySavedMsg:
		if msg.err == nil {
			b := m.manager.ActiveBackend()
			return m, loadHistory(b)
		}
		return m, nil

	case historyDeletedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Failed to delete history: %v", msg.err)
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 4*time.Second)
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
		saveCmd := saveHistory(b, msg.id, msg.name, output, success)

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
			m.selectedRow = 0
			m.historySelected = 0
			serverName := m.manager.ServerAt(msg.index).Name
			m.statusMsg = fmt.Sprintf("Switched to %s", serverName)
			m.statusKind = statusSuccess
			m.statusID++
			return m, clearStatusAfter(m.statusID, 3*time.Second)
		}
		return m, nil

	case selfUpdateMsg:
		m.statusID++
		if msg.err != nil {
			m.statusMsg = "Update failed: " + msg.err.Error()
			m.statusKind = statusError
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		if msg.newVersion == "" {
			m.statusMsg = "Already on the latest version"
			m.statusKind = statusSuccess
			return m, clearStatusAfter(m.statusID, 3*time.Second)
		}
		if msg.needsSudo {
			m.statusMsg = "Need sudo to install — enter your password..."
			m.statusKind = statusInfo
			return m, sudoInstall(msg.newVersion, msg.tmpBinary, msg.targetPath)
		}
		m.statusMsg = fmt.Sprintf("Updated to %s — restart lazycron to use the new version", msg.newVersion)
		m.statusKind = statusSuccess
		return m, clearStatusAfter(m.statusID, 8*time.Second)

	case selfUpdateSudoMsg:
		m.statusID++
		if msg.err != nil {
			m.statusMsg = "Update failed: " + msg.err.Error()
			m.statusKind = statusError
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		m.statusMsg = fmt.Sprintf("Updated to %s — restart lazycron to use the new version", msg.newVersion)
		m.statusKind = statusSuccess
		return m, clearStatusAfter(m.statusID, 8*time.Second)

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
	case modeProjectPrompt:
		var cmd tea.Cmd
		m.projectInput, cmd = m.projectInput.Update(msg)
		return m, cmd
	case modeSearch:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
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
	if m.mode == modeSplash {
		m.mode = modeNormal
		return m, nil
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
	case modeProjectPrompt:
		return m.handleProjectPromptKey(msg)
	case modeSearch:
		return m.handleSearchKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}
