package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/cron"
)

const (
	fieldName    = 0
	fieldCommand = 1
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
	fields      [fieldCount]string
	activeField int
	editing     bool // true = edit mode, false = new job
	editIndex   int  // index of job being edited
}

func newForm() formModel {
	return formModel{}
}

func newFormForEdit(job cron.Job, index int) formModel {
	f := formModel{
		editing:   true,
		editIndex: index,
	}
	f.fields[fieldName] = job.Name

	// Try to extract workdir and logfile from command
	cmd := job.Command
	workDir := ""
	logFile := ""

	// Extract log file: >> /path 2>&1
	if idx := strings.Index(cmd, " >> "); idx != -1 {
		logPart := cmd[idx:]
		cmd = strings.TrimSpace(cmd[:idx])
		logPart = strings.TrimPrefix(logPart, " >> ")
		logPart = strings.TrimSuffix(logPart, " 2>&1")
		logFile = strings.TrimSpace(logPart)
	}

	// Extract workdir: cd /path &&
	if strings.HasPrefix(cmd, "cd ") {
		if idx := strings.Index(cmd, " && "); idx != -1 {
			workDir = strings.TrimPrefix(cmd[:idx], "cd ")
			cmd = strings.TrimSpace(cmd[idx+4:])
		}
	}

	f.fields[fieldCommand] = cmd
	f.fields[fieldSchedule] = job.Schedule
	f.fields[fieldWorkDir] = workDir
	f.fields[fieldLogFile] = logFile

	return f
}

func (f *formModel) nextField() {
	f.activeField = (f.activeField + 1) % fieldCount
}

func (f *formModel) prevField() {
	f.activeField = (f.activeField - 1 + fieldCount) % fieldCount
}

func (f *formModel) handleChar(ch rune) {
	f.fields[f.activeField] += string(ch)
}

func (f *formModel) handleBackspace() {
	s := f.fields[f.activeField]
	if len(s) > 0 {
		f.fields[f.activeField] = s[:len(s)-1]
	}
}

func (f *formModel) buildJob() (cron.Job, error) {
	name := strings.TrimSpace(f.fields[fieldName])
	command := strings.TrimSpace(f.fields[fieldCommand])
	schedule := strings.TrimSpace(f.fields[fieldSchedule])
	workDir := strings.TrimSpace(f.fields[fieldWorkDir])
	logFile := strings.TrimSpace(f.fields[fieldLogFile])

	if name == "" {
		return cron.Job{}, fmt.Errorf("name is required")
	}
	if command == "" {
		return cron.Job{}, fmt.Errorf("command is required")
	}
	if schedule == "" {
		return cron.Job{}, fmt.Errorf("schedule is required")
	}

	// Convert human schedule to cron
	cronExpr := cron.HumanToCron(schedule)
	if err := cron.ValidateCron(cronExpr); err != nil {
		return cron.Job{}, fmt.Errorf("invalid schedule: %w", err)
	}

	// Build final command
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

func (f *formModel) buildPreview() string {
	schedule := strings.TrimSpace(f.fields[fieldSchedule])
	command := strings.TrimSpace(f.fields[fieldCommand])
	workDir := strings.TrimSpace(f.fields[fieldWorkDir])
	logFile := strings.TrimSpace(f.fields[fieldLogFile])

	if schedule == "" && command == "" {
		return ""
	}

	cronExpr := cron.HumanToCron(schedule)
	finalCmd := command
	if workDir != "" {
		finalCmd = fmt.Sprintf("cd %s && %s", workDir, finalCmd)
	}
	if logFile != "" {
		finalCmd = fmt.Sprintf("%s >> %s 2>&1", finalCmd, logFile)
	}

	return fmt.Sprintf("%s %s", cronExpr, finalCmd)
}

func renderForm(f *formModel, width, height int) string {
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

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  " + title))
	b.WriteString("\n\n")

	for i := 0; i < fieldCount; i++ {
		label := formLabelStyle.Render(fmt.Sprintf("  %-10s", fieldLabels[i]+":"))
		value := f.fields[i]
		if value == "" && i != f.activeField {
			value = fieldHints[i]
		}

		var rendered string
		if i == f.activeField {
			cursor := "█"
			rendered = formActiveInputStyle.
				Width(formWidth - 16).
				Render(value + cursor)
		} else {
			if f.fields[i] == "" {
				rendered = formInactiveInputStyle.
					Width(formWidth - 16).
					Foreground(colorMuted).
					Render(value)
			} else {
				rendered = formInactiveInputStyle.
					Width(formWidth - 16).
					Render(value)
			}
		}

		b.WriteString(label + " " + rendered)
		b.WriteString("\n")

		// Show schedule conversion hint
		if i == fieldSchedule && f.fields[fieldSchedule] != "" {
			cronExpr := cron.HumanToCron(f.fields[fieldSchedule])
			if cronExpr != f.fields[fieldSchedule] {
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

	// Preview
	preview := f.buildPreview()
	if preview != "" {
		b.WriteString("\n")
		b.WriteString(formPreviewStyle.Render("  Preview: " + preview))
	}

	b.WriteString("\n\n")
	b.WriteString(mutedItemStyle.Render("  tab: next field • shift+tab: prev • enter: save • esc: cancel"))

	content := b.String()

	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		formStyle.Width(formWidth).Render(content),
	)
}

func renderConfirmDialog(message string, width, height int) string {
	content := confirmTitleStyle.Render("Confirm") + "\n\n" +
		detailValueStyle.Render(message) + "\n\n" +
		helpKeyStyle.Render("y") + helpDescStyle.Render(" yes  ") +
		helpKeyStyle.Render("n") + helpDescStyle.Render(" no")

	dialog := confirmStyle.Width(40).Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog)
}
