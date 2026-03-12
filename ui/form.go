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
	fieldLogFile  = 4
	fieldCount    = 5
)

var fieldLabels = [fieldCount]string{
	"Name",
	"Command",
	"Schedule",
	"Work Dir",
	"Log File",
}

var fieldHints = [fieldCount]string{
	"Identifier for this job",
	"Shell command to execute",
	"Cron expression or 'every day at 9am'",
	"Optional: working directory",
	"Optional: path to log file",
}

type formModel struct {
	inputs      [fieldCount]textinput.Model
	activeField int
	editing     bool
	editIndex   int
}

func newForm() formModel {
	f := formModel{}
	for i := 0; i < fieldCount; i++ {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = fieldHints[i]
		ti.CharLimit = 512
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
		ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
		f.inputs[i] = ti
	}
	return f
}

func newFormForEdit(job cron.Job, index int) formModel {
	f := formModel{
		editing:   true,
		editIndex: index,
	}

	for i := 0; i < fieldCount; i++ {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = fieldHints[i]
		ti.CharLimit = 512
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
		ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
		f.inputs[i] = ti
	}

	// Try to extract workdir and logfile from command
	cmd := job.Command
	workDir := ""
	logFile := ""

	if idx := strings.Index(cmd, " >> "); idx != -1 {
		logPart := cmd[idx:]
		cmd = strings.TrimSpace(cmd[:idx])
		logPart = strings.TrimPrefix(logPart, " >> ")
		logPart = strings.TrimSuffix(logPart, " 2>&1")
		logFile = strings.TrimSpace(logPart)
	}

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
	f.inputs[fieldLogFile].SetValue(logFile)

	return f
}

// focusActive focuses the current active field and returns the blink cmd.
func (f *formModel) focusActive() tea.Cmd {
	return f.inputs[f.activeField].Focus()
}

func (f *formModel) nextField() tea.Cmd {
	f.inputs[f.activeField].Blur()
	f.activeField = (f.activeField + 1) % fieldCount
	return f.inputs[f.activeField].Focus()
}

func (f *formModel) prevField() tea.Cmd {
	f.inputs[f.activeField].Blur()
	f.activeField = (f.activeField - 1 + fieldCount) % fieldCount
	return f.inputs[f.activeField].Focus()
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
	logFile := strings.TrimSpace(f.inputs[fieldLogFile].Value())

	if name == "" {
		return cron.Job{}, fmt.Errorf("name is required")
	}
	if command == "" {
		return cron.Job{}, fmt.Errorf("command is required")
	}
	if schedule == "" {
		return cron.Job{}, fmt.Errorf("schedule is required")
	}

	cronExpr := cron.HumanToCron(schedule)
	if err := cron.ValidateCron(cronExpr); err != nil {
		return cron.Job{}, fmt.Errorf("invalid schedule: %w", err)
	}

	finalCmd := command
	if workDir != "" {
		finalCmd = fmt.Sprintf("cd %s && %s", workDir, finalCmd)
	}
	if logFile != "" {
		finalCmd = fmt.Sprintf("%s >> %s 2>&1", finalCmd, logFile)
	}

	return cron.Job{
		Name:     name,
		Schedule: cronExpr,
		Command:  finalCmd,
		Enabled:  true,
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
	b.WriteString("\n\n")

	for i := 0; i < fieldCount; i++ {
		label := formLabelStyle.Render(fmt.Sprintf("  %-10s", fieldLabels[i]+":"))

		f.inputs[i].Width = inputWidth
		rendered := lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1).
			Render(f.inputs[i].View())

		b.WriteString(label + " " + rendered)
		b.WriteString("\n")

		// Show schedule conversion hint
		if i == fieldSchedule && f.inputs[fieldSchedule].Value() != "" {
			cronExpr := cron.HumanToCron(f.inputs[fieldSchedule].Value())
			if cronExpr != f.inputs[fieldSchedule].Value() {
				hint := mutedItemStyle.Render(fmt.Sprintf("             → %s", cronExpr))
				b.WriteString("  " + hint + "\n")
			}
			human := cron.CronToHuman(cronExpr)
			if human != cronExpr {
				hint := mutedItemStyle.Render(fmt.Sprintf("             = %s", human))
				b.WriteString("  " + hint + "\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("  tab: next field • shift+tab: prev • enter: save • esc: cancel"))

	content := b.String()

	return formStyle.Width(formWidth).Render(content)
}

func renderConfirmDialog(message string) string {
	content := confirmTitleStyle.Render("Confirm") + "\n\n" +
		detailValueStyle.Render(message) + "\n\n" +
		helpKeyStyle.Render("y") + helpDescStyle.Render(" yes  ") +
		helpKeyStyle.Render("n") + helpDescStyle.Render(" no")

	return confirmStyle.Width(40).Render(content)
}
