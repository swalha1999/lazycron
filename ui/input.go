package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// newStyledInput returns a textinput.Model preconfigured with the project's
// standard prompt/placeholder/text/cursor styles. Callers set placeholder,
// char limit, echo mode, etc. on the returned model.
func newStyledInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
	return ti
}
