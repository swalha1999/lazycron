package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/cron"
)

const (
	fieldName     = 0
	fieldCommand  = 1
	fieldSchedule = 2
	fieldWorkDir  = 3
	fieldCount    = 4
)

var fieldLabels = [fieldCount]string{
	"Name",
	"Command",
	"Schedule",
	"Work Dir",
}

var fieldHints = [fieldCount]string{
	"Identifier for this job",
	"Shell command to execute",
	"Cron expression or 'every day at 9am'",
	"Optional: working directory",
}

type formModel struct {
	inputs      [fieldCount]textinput.Model
	activeField int
	editing     bool
	editIndex   int
	picker      pickerModel
	completer   completerModel
	tag         string // colored tag from template
	tagColor    string // hex color for the tag
	oneShot     bool   // one-shot mode: run once at a specific datetime
}

func newInput(i int) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = fieldHints[i]
	ti.CharLimit = 512
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
	return ti
}

func newForm() formModel {
	f := formModel{}
	for i := 0; i < fieldCount; i++ {
		f.inputs[i] = newInput(i)
	}
	f.picker = newPicker()
	f.inputs[fieldSchedule].SetValue(f.picker.Expression())
	return f
}

func newFormForEdit(job cron.Job, index int) formModel {
	f := formModel{
		editing:   true,
		editIndex: index,
	}
	for i := 0; i < fieldCount; i++ {
		f.inputs[i] = newInput(i)
	}

	// Extract workdir from command
	cmd := job.Command
	workDir := ""
	if strings.HasPrefix(cmd, "cd ") {
		if idx := strings.Index(cmd, " && "); idx != -1 {
			workDir = strings.TrimPrefix(cmd[:idx], "cd ")
			cmd = strings.TrimSpace(cmd[idx+4:])
		}
	}

	f.inputs[fieldName].SetValue(job.Name)
	f.inputs[fieldCommand].SetValue(cmd)
	f.inputs[fieldSchedule].SetValue(job.Schedule)
	f.inputs[fieldWorkDir].SetValue(workDir)
	f.tag = job.Tag
	f.tagColor = job.TagColor
	f.oneShot = job.OneShot

	f.picker = newPicker()
	if !f.oneShot {
		f.picker.ParseExpression(job.Schedule)
	}

	return f
}

// focusActive focuses the current active field and returns the blink cmd.
func (f *formModel) focusActive() tea.Cmd {
	f.picker.focused = false
	if f.activeField == fieldWorkDir {
		f.completer.activate(f.inputs[fieldWorkDir].Value())
	}
	return f.inputs[f.activeField].Focus()
}

func (f *formModel) nextField() tea.Cmd {
	// If on schedule textinput and not one-shot, tab into picker
	if f.activeField == fieldSchedule && !f.picker.focused && !f.oneShot {
		f.inputs[fieldSchedule].Blur()
		f.syncInputToPicker()
		f.picker.focused = true
		return nil
	}
	if f.picker.focused {
		f.picker.focused = false
	} else {
		f.inputs[f.activeField].Blur()
	}
	f.completer.reset()
	f.activeField = (f.activeField + 1) % fieldCount
	return f.focusActive()
}

func (f *formModel) prevField() tea.Cmd {
	if f.picker.focused {
		f.picker.focused = false
		return f.inputs[fieldSchedule].Focus()
	}
	f.inputs[f.activeField].Blur()
	f.completer.reset()
	prev := (f.activeField - 1 + fieldCount) % fieldCount
	if prev == fieldSchedule && !f.oneShot {
		f.activeField = fieldSchedule
		f.syncInputToPicker()
		f.picker.focused = true
		return nil
	}
	f.activeField = prev
	return f.focusActive()
}

// syncInputToPicker parses the schedule textinput value into the picker.
func (f *formModel) syncInputToPicker() {
	expr := strings.TrimSpace(f.inputs[fieldSchedule].Value())
	if expr == "" {
		return
	}
	cronExpr := cron.HumanToCron(expr)
	f.picker.ParseExpression(cronExpr)
}

// updateInput passes a message to the active textinput (for key handling and cursor blink).
func (f *formModel) updateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.activeField], cmd = f.inputs[f.activeField].Update(msg)
	return cmd
}

func (f *formModel) buildJob() (cron.Job, error) {
	name := strings.TrimSpace(f.inputs[fieldName].Value())
	command := strings.TrimSpace(f.inputs[fieldCommand].Value())
	schedule := strings.TrimSpace(f.inputs[fieldSchedule].Value())
	workDir := strings.TrimSpace(f.inputs[fieldWorkDir].Value())

	if name == "" {
		return cron.Job{}, fmt.Errorf("name is required")
	}
	if command == "" {
		return cron.Job{}, fmt.Errorf("command is required")
	}
	if schedule == "" {
		return cron.Job{}, fmt.Errorf("schedule is required")
	}

	var cronExpr string
	if f.oneShot {
		expr, _, err := cron.DatetimeToCron(schedule)
		if err != nil {
			return cron.Job{}, fmt.Errorf("invalid datetime: %w", err)
		}
		cronExpr = expr
	} else {
		cronExpr = cron.HumanToCron(schedule)
		if err := cron.ValidateCron(cronExpr); err != nil {
			return cron.Job{}, fmt.Errorf("invalid schedule: %w", err)
		}
	}

	finalCmd := command
	if workDir != "" {
		finalCmd = fmt.Sprintf("cd %s && %s", workDir, finalCmd)
	}

	return cron.Job{
		Name:     name,
		Schedule: cronExpr,
		Command:  finalCmd,
		Enabled:  true,
		Wrapped:  true,
		Tag:      f.tag,
		TagColor: f.tagColor,
		OneShot:  f.oneShot,
	}, nil
}

func renderForm(f *formModel, width int) string {
	formWidth := width - 10
	if formWidth > 80 {
		formWidth = 80
	}
	if formWidth < 40 {
		formWidth = 40
	}

	title := "New Job"
	if f.editing {
		title = "Edit Job"
	}

	inputWidth := formWidth - 16

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  " + title))
	b.WriteString("  ")
	if f.oneShot {
		b.WriteString(lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("ONE-SHOT"))
	} else {
		b.WriteString(mutedItemStyle.Render("recurring"))
	}
	b.WriteString("\n\n")

	for i := 0; i < fieldCount; i++ {
		label := formLabelStyle.Render(fmt.Sprintf("  %-10s", fieldLabels[i]+":"))

		// Override schedule placeholder in one-shot mode
		if i == fieldSchedule && f.oneShot {
			f.inputs[i].Placeholder = "Date/time: 'tomorrow at 3pm' or '2026-03-22 14:30'"
		} else if i == fieldSchedule {
			f.inputs[i].Placeholder = fieldHints[fieldSchedule]
		}

		f.inputs[i].Width = inputWidth
		rendered := lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1).
			Render(f.inputs[i].View())

		b.WriteString(label + " " + rendered)
		b.WriteString("\n")

		if i == fieldSchedule && !f.oneShot {
			b.WriteString(renderPicker(&f.picker, inputWidth))
			b.WriteString("\n")
		}

		if i == fieldWorkDir && f.completer.active {
			b.WriteString(renderCompletions(&f.completer, inputWidth))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if f.picker.focused {
		b.WriteString("  " +
			helpBinding("←/→", "column") + helpSep() +
			helpBinding("↑/↓", "scroll") + helpSep() +
			helpBinding("space", "mode") + helpSep() +
			helpBinding("tab", "next") + helpSep() +
			helpBinding("esc", "cancel"))
	} else if f.activeField == fieldWorkDir && f.completer.active {
		b.WriteString("  " +
			helpBinding("↑/↓", "select") + helpSep() +
			helpBinding("→/enter", "open") + helpSep() +
			helpBinding("←", "parent") + helpSep() +
			helpBinding("tab", "next") + helpSep() +
			helpBinding("esc", "close"))
	} else {
		b.WriteString("  " +
			helpBinding("tab", "next field") + helpSep() +
			helpBinding("shift+tab", "prev") + helpSep() +
			helpBinding("ctrl+o", "one-shot") + helpSep() +
			helpBinding("enter", "save") + helpSep() +
			helpBinding("esc", "cancel"))
	}

	return formStyle.Width(formWidth).Render(b.String())
}

func renderConfirmDialog(message string) string {
	content := confirmTitleStyle.Render("Confirm") + "\n\n" +
		detailValueStyle.Render(message) + "\n\n" +
		helpKeyStyle.Render("y") + helpDescStyle.Render(" yes  ") +
		helpKeyStyle.Render("n") + helpDescStyle.Render(" no")

	return confirmStyle.Width(40).Render(content)
}
