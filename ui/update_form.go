package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
			return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)
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
		handled := m.form.completer.handleKey(key, func(path string) {
			m.form.inputs[fieldWorkDir].SetValue(path)
			m.form.inputs[fieldWorkDir].CursorEnd()
		})
		if handled {
			return m, nil
		}
	}

	switch key {
	case "ctrl+o":
		m.form.oneShot = !m.form.oneShot
		if m.form.oneShot {
			// Clear schedule field for fresh datetime input
			m.form.inputs[fieldSchedule].SetValue("")
			m.form.picker.focused = false
		} else {
			m.form.inputs[fieldSchedule].SetValue(m.form.picker.Expression())
		}
		return m, nil

	case "esc":
		m.mode = modeNormal
		return m, m.setStatus("Cancelled", statusInfo, 3*time.Second)

	case "enter":
		job, err := m.form.buildJob()
		if err != nil {
			return m, m.setStatus(err.Error(), statusError, 5*time.Second)
		}

		b := m.manager.ActiveBackend()
		var statusText string
		if m.form.editing {
			job.Enabled = m.jobs[m.form.editIndex].Enabled
			m.jobs[m.form.editIndex] = job
			// Re-position selection to follow the job (project may have changed)
			rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
			m.selectedRow = rowForJobIdx(rows, m.form.editIndex)
			statusText = fmt.Sprintf("Updated job '%s'", job.Name)
		} else {
			m.jobs = append(m.jobs, job)
			// Point selectedRow to the new job's visual row
			rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
			m.selectedRow = rowForJobIdx(rows, len(m.jobs)-1)
			statusText = fmt.Sprintf("Created job '%s'", job.Name)
		}
		m.mode = modeNormal
		return m, tea.Batch(saveJobs(b, m.jobs), m.setStatus(statusText, statusSuccess, 4*time.Second))

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

func (m Model) handleNewJobChoiceKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "b", "1":
		m.mode = modeForm
		m.form = newForm(m.activeDirLister())
		m.statusMsg = ""
		return m, m.form.focusActive()
	case "t", "2":
		m.mode = modeTemplatePicker
		m.templatePicker = newTemplatePicker(m.activeDirLister())
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
			handled := tp.completer.handleKey(key, func(path string) {
				tp.variableInputs[tp.activeVariable].SetValue(path)
				tp.variableInputs[tp.activeVariable].CursorEnd()
			})
			if handled {
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
			m.form = newForm(m.activeDirLister())
			m.form.inputs[fieldName].SetValue(tp.selectedTmpl.Name)
			m.form.inputs[fieldCommand].SetValue(resolvedCmd)
			m.form.inputs[fieldSchedule].SetValue(cronExpr)
			m.form.inputs[fieldWorkDir].SetValue(workDir)
			m.form.tag = tp.selectedTmpl.Tag
			m.form.tagColor = tp.selectedTmpl.TagColor
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
