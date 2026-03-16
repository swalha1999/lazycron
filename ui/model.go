package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

type mode int

const (
	modeNormal mode = iota
	modeForm
	modeConfirmDelete
	modeHelp
	modeRunOutput
	modeAddServer
	modeConfirmDeleteServer
	modePasswordPrompt
)

type statusType int

const (
	statusNone statusType = iota
	statusError
	statusSuccess
	statusInfo
)

const (
	panelServers = 0
	panelJobs    = 1
	panelHistory = 2
	panelDetail  = 3
	panelCount   = 4
)

type Model struct {
	manager *backend.Manager

	jobs     []cron.Job
	selected int
	mode     mode
	form     formModel
	width    int
	height   int

	statusMsg  string
	statusKind statusType
	statusID   int

	runOutput       string
	runJobName      string
	runOutputFailed bool
	runOutputScroll int

	history         []history.Entry
	historySelected int
	focusPanel      int
	lastLeftPanel   int
	historyScroll   int
	detailScroll    int

	// Server panel state
	serverSelected  int
	serverSwitching bool
	serverForm      serverFormModel

	// Password prompt state
	passwordInput      textinput.Model
	passwordServerIdx  int
}

func newPasswordInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Enter password"
	ti.CharLimit = 256
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '*'
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
	return ti
}

func NewModel(mgr *backend.Manager) Model {
	return Model{
		manager:    mgr,
		selected:   0,
		mode:       modeNormal,
		focusPanel: panelServers,
	}
}
