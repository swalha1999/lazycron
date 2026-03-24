package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

type mode int

const (
	modeSplash mode = iota
	modeNormal
	modeForm
	modeConfirmDelete
	modeHelp
	modeRunOutput
	modeAddServer
	modeConfirmDeleteServer
	modePasswordPrompt
	modeNewJobChoice
	modeTemplatePicker
	modeConfirmDeleteHistory
	modeProjectPrompt
	modeSearch
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

	// Template picker state
	templatePicker templatePickerModel

	// Confirm dialog state
	confirmYes bool // true = "yes" selected, false = "no" selected

	// Project grouping state
	collapsedProjects map[string]bool
	jobListRows       []listRow       // cached visual rows for grouped job list
	selectedRow       int             // visual row index in grouped job list
	projectInput      textinput.Model // for quick-assign project prompt

	// Search/filter state
	searchInput textinput.Model
	searchQuery string       // active filter text
	searchPanel int          // panel the filter applies to (-1 = none)
	searchJobMatch map[int]bool // set of matching job indices (nil = all)

	// App version (for self-update)
	version string
}

// setStatus updates the status bar and returns a Cmd that clears it after d.
func (m *Model) setStatus(msg string, kind statusType, d time.Duration) tea.Cmd {
	m.statusMsg = msg
	m.statusKind = kind
	m.statusID++
	return clearStatusAfter(m.statusID, d)
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

func newProjectInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Project name (empty to clear)"
	ti.CharLimit = 64
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
	return ti
}

func NewModel(mgr *backend.Manager, version string) Model {
	return Model{
		manager:           mgr,
		selected:          0,
		mode:              modeSplash,
		focusPanel:        panelServers,
		collapsedProjects: make(map[string]bool),
		searchPanel:       -1,
		version:           version,
	}
}

// activeDirLister returns a DirLister for the active backend.
// Returns nil for local backends (completer falls back to os.ReadDir).
func (m *Model) activeDirLister() DirLister {
	b := m.manager.ActiveBackend()
	if rb, ok := b.(*backend.RemoteBackend); ok {
		return rb.DirLister()
	}
	return nil
}
