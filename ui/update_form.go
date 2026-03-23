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
			// Re-position selection to follow the job (project may have changed)
			rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
			m.selectedRow = rowForJobIdx(rows, m.form.editIndex)
			m.statusMsg = fmt.Sprintf("Updated job '%s'", job.Name)
		} else {
			m.jobs = append(m.jobs, job)
			// Point selectedRow to the new job's visual row
			rows := buildRows(m.jobs, m.collapsedProjects, m.searchJobMatch)
			m.selectedRow = rowForJobIdx(rows, len(m.jobs)-1)
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
