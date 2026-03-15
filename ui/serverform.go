package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/config"
)

const (
	srvFieldName     = 0
	srvFieldHost     = 1
	srvFieldPort     = 2
	srvFieldUser     = 3
	srvFieldPassword = 4
	srvFieldCount    = 5
)

var srvFieldLabels = [srvFieldCount]string{
	"Name",
	"Host",
	"Port",
	"User",
	"Password",
}

var srvFieldHints = [srvFieldCount]string{
	"Display name for this server",
	"Hostname or IP address",
	"SSH port (default: 22)",
	"SSH username",
	"Optional (keys auto-detected)",
}

type serverFormModel struct {
	inputs      [srvFieldCount]textinput.Model
	activeField int
}

func newServerForm() serverFormModel {
	f := serverFormModel{}
	for i := 0; i < srvFieldCount; i++ {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = srvFieldHints[i]
		ti.CharLimit = 256
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
		ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
		if i == srvFieldPassword {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '*'
		}
		f.inputs[i] = ti
	}
	f.inputs[srvFieldPort].SetValue("22")
	return f
}

func (f *serverFormModel) focusActive() tea.Cmd {
	return f.inputs[f.activeField].Focus()
}

func (f *serverFormModel) nextField() tea.Cmd {
	f.inputs[f.activeField].Blur()
	f.activeField = (f.activeField + 1) % srvFieldCount
	return f.focusActive()
}

func (f *serverFormModel) prevField() tea.Cmd {
	f.inputs[f.activeField].Blur()
	f.activeField = (f.activeField - 1 + srvFieldCount) % srvFieldCount
	return f.focusActive()
}

func (f *serverFormModel) updateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	f.inputs[f.activeField], cmd = f.inputs[f.activeField].Update(msg)
	return cmd
}

func (f *serverFormModel) buildServerConfig() (config.ServerConfig, error) {
	name := strings.TrimSpace(f.inputs[srvFieldName].Value())
	host := strings.TrimSpace(f.inputs[srvFieldHost].Value())
	portStr := strings.TrimSpace(f.inputs[srvFieldPort].Value())
	user := strings.TrimSpace(f.inputs[srvFieldUser].Value())
	password := f.inputs[srvFieldPassword].Value()

	if name == "" {
		return config.ServerConfig{}, fmt.Errorf("name is required")
	}
	if host == "" {
		return config.ServerConfig{}, fmt.Errorf("host is required")
	}
	if user == "" {
		return config.ServerConfig{}, fmt.Errorf("user is required")
	}

	port := 22
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			return config.ServerConfig{}, fmt.Errorf("invalid port number")
		}
		port = p
	}

	return config.ServerConfig{
		Name:     name,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
	}, nil
}

func renderServerForm(f *serverFormModel, width int) string {
	formWidth := width - 10
	if formWidth > 60 {
		formWidth = 60
	}
	if formWidth < 40 {
		formWidth = 40
	}

	inputWidth := formWidth - 16

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Add Server"))
	b.WriteString("\n\n")

	for i := 0; i < srvFieldCount; i++ {
		label := formLabelStyle.Render(fmt.Sprintf("  %-10s", srvFieldLabels[i]+":"))

		f.inputs[i].Width = inputWidth
		rendered := lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1).
			Render(f.inputs[i].View())

		b.WriteString(label + " " + rendered)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("  SSH keys are auto-detected from ~/.ssh/") + "\n\n")
	b.WriteString("  " +
		helpBinding("tab", "next field") + helpSep() +
		helpBinding("shift+tab", "prev") + helpSep() +
		helpBinding("enter", "save") + helpSep() +
		helpBinding("esc", "cancel"))

	return formStyle.Width(formWidth).Render(b.String())
}
