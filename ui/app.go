package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/swalha1999/lazycron/cron"
)

type mode int

const (
	modeNormal mode = iota
	modeForm
	modeConfirmDelete
	modeHelp
	modeRunOutput
)

type statusType int

const (
	statusNone statusType = iota
	statusError
	statusSuccess
	statusInfo
)

type Model struct {
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
}

type jobsLoadedMsg struct {
	jobs []cron.Job
	err  error
}

type jobSavedMsg struct {
	err error
}

type jobRanMsg struct {
	name   string
	output string
	err    error
}

type clearStatusMsg struct {
	id int
}

func clearStatusAfter(id int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{id: id}
	})
}

func NewModel() Model {
	return Model{
		selected: 0,
		mode:     modeNormal,
	}
}

func (m Model) Init() tea.Cmd {
	return loadJobs
}

func loadJobs() tea.Msg {
	output, err := cron.ReadCrontab()
	if err != nil {
		return jobsLoadedMsg{err: err}
	}
	jobs := cron.Parse(output)
	return jobsLoadedMsg{jobs: jobs}
}

func saveJobs(jobs []cron.Job) tea.Cmd {
	return func() tea.Msg {
		err := cron.WriteCrontab(jobs)
		return jobSavedMsg{err: err}
	}
}

func runJob(name, command string) tea.Cmd {
	return func() tea.Msg {
		output, err := cron.RunJobNow(command)
		return jobRanMsg{name: name, output: output, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case jobsLoadedMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		m.jobs = msg.jobs
		if len(m.jobs) > 0 {
			m.statusMsg = fmt.Sprintf("Loaded %d job(s)", len(m.jobs))
			m.statusKind = statusInfo
			m.statusID++
			return m, clearStatusAfter(m.statusID, 2*time.Second)
		}
		return m, nil

	case jobSavedMsg:
		if msg.err != nil {
			m.statusMsg = "Save failed: " + msg.err.Error()
			m.statusKind = statusError
			m.statusID++
			return m, clearStatusAfter(m.statusID, 5*time.Second)
		}
		return m, nil

	case jobRanMsg:
		m.statusID++
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
			return m, nil
		}
		if msg.output != "" {
			m.runOutput = msg.output
			m.runJobName = msg.name
			m.runOutputFailed = false
			m.mode = modeRunOutput
			m.runOutputScroll = 0
			m.statusMsg = ""
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Job '%s' ran successfully", msg.name)
		m.statusKind = statusSuccess
		return m, clearStatusAfter(m.statusID, 4*time.Second)

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
	if m.mode == modeForm {
		cmd := m.form.updateInput(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case modeForm:
		return m.handleFormKey(msg)
	case modeConfirmDelete:
		return m.handleConfirmKey(msg)
	case modeHelp:
		return m.handleHelpKey(msg)
	case modeRunOutput:
		return m.handleRunOutputKey(msg)
	default:
		return m.handleNormalKey(msg)
	}
}

func (m Model) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit

	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}

	case "down", "j":
		if m.selected < len(m.jobs)-1 {
			m.selected++
		}

	case "n":
		m.mode = modeForm
		m.form = newForm()
		m.statusMsg = ""
		return m, m.form.focusActive()

	case "enter", "e":
		if len(m.jobs) > 0 {
			m.mode = modeForm
			m.form = newFormForEdit(m.jobs[m.selected], m.selected)
			m.statusMsg = ""
			return m, m.form.focusActive()
		}

	case "d":
		if len(m.jobs) > 0 {
			m.mode = modeConfirmDelete
			m.statusMsg = ""
		}

	case " ":
		if len(m.jobs) > 0 {
			m.jobs[m.selected].Enabled = !m.jobs[m.selected].Enabled
			status := "enabled"
			if !m.jobs[m.selected].Enabled {
				status = "disabled"
			}
			m.statusMsg = fmt.Sprintf("Job '%s' %s", m.jobs[m.selected].Name, status)
			m.statusKind = statusSuccess
			m.statusID++
			return m, tea.Batch(saveJobs(m.jobs), clearStatusAfter(m.statusID, 4*time.Second))
		}

	case "r":
		if len(m.jobs) > 0 {
			job := m.jobs[m.selected]
			m.statusMsg = fmt.Sprintf("Running '%s'...", job.Name)
			m.statusKind = statusInfo
			return m, runJob(job.Name, job.Command)
		}

	case "R":
		m.statusMsg = "Refreshing..."
		m.statusKind = statusInfo
		return m, loadJobs

	case "?":
		m.mode = modeHelp
		m.statusMsg = ""
	}
	return m, nil
}

func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Handle picker-specific keys when picker is focused
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
			// Save from picker
			m.form.inputs[fieldSchedule].SetValue(m.form.picker.Expression())
			m.form.picker.focused = false
			// Fall through to enter/save handling below
		case "shift+tab":
			cmd := m.form.prevField()
			return m, cmd
		default:
			return m, nil
		}
	}

	// Handle completer navigation when suggestions are visible
	if m.form.activeField == fieldWorkDir && m.form.completer.active {
		switch key {
		case "down":
			m.form.completer.selectNext()
			return m, nil
		case "up":
			m.form.completer.selectPrev()
			return m, nil
		case "enter", "right":
			// Drill into selected directory
			if m.form.completer.selected >= 0 {
				path := m.form.completer.drillIn()
				if path != "" {
					m.form.inputs[fieldWorkDir].SetValue(path)
					m.form.inputs[fieldWorkDir].CursorEnd()
				}
				return m, nil
			}
			// No selection: right does nothing, enter falls through to save
			if key == "right" {
				return m, nil
			}
		case "left":
			// Drill out to parent directory
			path := m.form.completer.drillOut()
			m.form.inputs[fieldWorkDir].SetValue(path)
			m.form.inputs[fieldWorkDir].CursorEnd()
			return m, nil
		case "esc":
			// Close suggestions, keep typed value
			m.form.completer.reset()
			return m, nil
		}
	}

	switch key {
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

		if m.form.editing {
			job.Enabled = m.jobs[m.form.editIndex].Enabled
			m.jobs[m.form.editIndex] = job
			m.statusMsg = fmt.Sprintf("Updated job '%s'", job.Name)
		} else {
			m.jobs = append(m.jobs, job)
			m.selected = len(m.jobs) - 1
			m.statusMsg = fmt.Sprintf("Created job '%s'", job.Name)
		}
		m.statusKind = statusSuccess
		m.mode = modeNormal
		m.statusID++
		return m, tea.Batch(saveJobs(m.jobs), clearStatusAfter(m.statusID, 4*time.Second))

	case "tab":
		cmd := m.form.nextField()
		return m, cmd
	case "shift+tab":
		cmd := m.form.prevField()
		return m, cmd
	default:
		cmd := m.form.updateInput(msg)
		// Update path completer when typing in Work Dir
		if m.form.activeField == fieldWorkDir {
			m.form.completer.update(m.form.inputs[fieldWorkDir].Value())
		}
		return m, cmd
	}
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if len(m.jobs) > 0 {
			name := m.jobs[m.selected].Name
			m.jobs = append(m.jobs[:m.selected], m.jobs[m.selected+1:]...)
			if m.selected >= len(m.jobs) && m.selected > 0 {
				m.selected--
			}
			m.statusMsg = fmt.Sprintf("Deleted job '%s'", name)
			m.statusKind = statusSuccess
			m.mode = modeNormal
			m.statusID++
			return m, tea.Batch(saveJobs(m.jobs), clearStatusAfter(m.statusID, 4*time.Second))
		}
	case "n", "N", "esc":
		m.mode = modeNormal
		m.statusMsg = "Cancelled"
		m.statusKind = statusInfo
		m.statusID++
		return m, clearStatusAfter(m.statusID, 3*time.Second)
	}
	return m, nil
}

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "?":
		m.mode = modeNormal
	}
	return m, nil
}

func (m Model) handleRunOutputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = modeNormal
		m.runOutputScroll = 0
	case "j", "down":
		m.runOutputScroll++
	case "k", "up":
		if m.runOutputScroll > 0 {
			m.runOutputScroll--
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Top bar
	topBar := renderTopBar(m.mode, m.width)

	// Bottom bar
	bottomBar := renderBottomBar(m.mode, m.statusMsg, m.statusKind, m.width)

	// Main content area: fill remaining space after top bar (1 line) and bottom bar
	contentHeight := m.height - 1 - lipgloss.Height(bottomBar)

	panels := renderPanels(m.jobs, m.selected, m.width, contentHeight)

	var content string
	switch m.mode {
	case modeForm:
		fg := renderForm(&m.form, m.width)
		content = overlay(panels, fg, m.width, contentHeight)

	case modeConfirmDelete:
		jobName := ""
		if len(m.jobs) > 0 {
			jobName = m.jobs[m.selected].Name
		}
		fg := renderConfirmDialog(fmt.Sprintf("Delete job '%s'?", jobName))
		content = overlay(panels, fg, m.width, contentHeight)

	case modeHelp:
		fg := renderHelpScreen()
		content = overlay(panels, fg, m.width, contentHeight)

	case modeRunOutput:
		fg := renderRunOutput(m.runJobName, m.runOutput, m.runOutputFailed, m.runOutputScroll, m.width, contentHeight)
		content = overlay(panels, fg, m.width, contentHeight)

	default:
		content = panels
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		topBar,
		content,
		bottomBar,
	)
}

func renderPanels(jobs []cron.Job, selected, width, height int) string {
	listWidth := width * 2 / 5
	detailWidth := width - listWidth - 4 // account for borders

	if listWidth < 20 {
		listWidth = 20
	}

	var selectedJob *cron.Job
	if selected >= 0 && selected < len(jobs) {
		selectedJob = &jobs[selected]
	}

	innerHeight := height - 2 // border (top + bottom)
	if innerHeight < 3 {
		innerHeight = 3
	}

	// Left panel
	listContent := renderJobList(jobs, selected, listWidth-4, innerHeight)
	leftPanel := activePanelStyle.
		Width(listWidth).
		Height(innerHeight).
		Render(panelTitleStyle.Render(" Jobs") + "\n" + listContent)

	// Right panel
	detailContent := renderDetail(selectedJob, detailWidth-4)
	rightPanel := panelStyle.
		Width(detailWidth).
		Height(innerHeight).
		Render(panelTitleStyle.Render(" Details") + "\n" + detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func renderTopBar(m mode, width int) string {
	title := titleStyle.Render("lazycron")
	var modeStr string
	switch m {
	case modeNormal:
		modeStr = modeStyle.Render("NORMAL")
	case modeForm:
		modeStr = modeStyle.Render("EDIT")
	case modeConfirmDelete:
		modeStr = modeStyle.Render("CONFIRM")
	case modeHelp:
		modeStr = modeStyle.Render("HELP")
	case modeRunOutput:
		modeStr = modeStyle.Render("OUTPUT")
	}

	spacer := strings.Repeat(" ", max(0, width-lipgloss.Width(title)-lipgloss.Width(modeStr)))
	bar := title + spacer + modeStr

	return topBarStyle.Width(width).Render(bar)
}

func renderBottomBar(m mode, statusMsg string, statusKind statusType, width int) string {
	var help string
	switch m {
	case modeNormal:
		help = helpBinding("n", "new") + helpSep() +
			helpBinding("enter", "edit") + helpSep() +
			helpBinding("d", "delete") + helpSep() +
			helpBinding("space", "toggle") + helpSep() +
			helpBinding("r", "run") + helpSep() +
			helpBinding("R", "refresh") + helpSep() +
			helpBinding("?", "help") + helpSep() +
			helpBinding("q", "quit")
	case modeForm:
		help = helpBinding("tab", "next") + helpSep() +
			helpBinding("shift+tab", "prev") + helpSep() +
			helpBinding("enter", "save") + helpSep() +
			helpBinding("esc", "cancel")
	case modeConfirmDelete:
		help = helpBinding("y", "confirm") + helpSep() +
			helpBinding("n", "cancel")
	case modeHelp:
		help = helpBinding("esc", "back")
	case modeRunOutput:
		help = helpBinding("j/k", "scroll") + helpSep() +
			helpBinding("esc", "close")
	}

	// Status message
	var status string
	if statusMsg != "" {
		switch statusKind {
		case statusError:
			status = errorStyle.Render(" ✗ " + statusMsg)
		case statusSuccess:
			status = successStyle.Render(" ✓ " + statusMsg)
		default:
			status = statusBarStyle.Render(" " + statusMsg)
		}
	}

	helpLine := statusBarStyle.Width(width).Render(help)
	if status != "" {
		statusLine := lipgloss.NewStyle().
			Width(width).
			Render(status)
		return lipgloss.JoinVertical(lipgloss.Left, statusLine, helpLine)
	}
	return helpLine
}

func helpBinding(key, desc string) string {
	return helpKeyStyle.Render(key) + helpDescStyle.Render(" "+desc)
}

func helpSep() string {
	return helpDescStyle.Render("  ")
}

func renderHelpScreen() string {
	bindings := []struct{ key, desc string }{
		{"n", "Create new job"},
		{"enter / e", "Edit selected job"},
		{"d", "Delete selected job"},
		{"space", "Toggle enable/disable"},
		{"r", "Run job now"},
		{"R", "Refresh from crontab"},
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"?", "Show/hide help"},
		{"q", "Quit"},
	}

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  Keybindings") + "\n\n")
	for _, bind := range bindings {
		key := helpKeyStyle.Render(fmt.Sprintf("  %-14s", bind.key))
		desc := detailValueStyle.Render(bind.desc)
		b.WriteString(key + " " + desc + "\n")
	}

	content := b.String()
	return formStyle.Width(44).Render(content)
}

func renderRunOutput(name, output string, failed bool, scroll, width, maxHeight int) string {
	boxWidth := width - 10
	if boxWidth > 80 {
		boxWidth = 80
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	var b strings.Builder

	// Title with status indicator
	if failed {
		b.WriteString(errorStyle.Render("  ✗ Run Failed: " + name))
	} else {
		b.WriteString(successStyle.Render("  ✓ Run Output: " + name))
	}
	b.WriteString("\n\n")

	if output == "" {
		b.WriteString(mutedItemStyle.Render("  (no output)"))
		b.WriteString("\n")
	} else {
		// Word-wrap each line of output
		var allLines []string
		for _, rawLine := range strings.Split(output, "\n") {
			if rawLine == "" {
				allLines = append(allLines, "")
			} else {
				wrapped := wrapText(rawLine, boxWidth-8)
				allLines = append(allLines, wrapped...)
			}
		}

		// Calculate visible window
		visibleLines := maxHeight - 10
		if visibleLines < 3 {
			visibleLines = 3
		}

		startLine := scroll
		if len(allLines) <= visibleLines {
			startLine = 0
		} else if startLine > len(allLines)-visibleLines {
			startLine = len(allLines) - visibleLines
		}

		endLine := startLine + visibleLines
		if endLine > len(allLines) {
			endLine = len(allLines)
		}

		for _, line := range allLines[startLine:endLine] {
			b.WriteString("  " + detailValueStyle.Render(line) + "\n")
		}

		if len(allLines) > visibleLines {
			scrollInfo := fmt.Sprintf("  [lines %d–%d of %d]", startLine+1, endLine, len(allLines))
			b.WriteString(mutedItemStyle.Render(scrollInfo) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("  esc: close • j/k: scroll"))

	return formStyle.Width(boxWidth).Render(b.String())
}

// overlay centers the fgBox on top of bg, preserving bg on both sides.
func overlay(bg, fgBox string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fgBox, "\n")

	// Pad bg to height
	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	fgHeight := len(fgLines)
	fgWidth := lipgloss.Width(fgBox)

	// Center position
	topOffset := (height - fgHeight) / 2
	leftOffset := (width - fgWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}
	rightOffset := leftOffset + fgWidth

	for i, fgLine := range fgLines {
		row := topOffset + i
		if row >= 0 && row < len(bgLines) {
			bgLine := bgLines[row]
			// Left portion of bg (ANSI-aware truncate to leftOffset chars)
			left := ansi.Truncate(bgLine, leftOffset, "")
			// Right portion of bg (skip leftOffset+fgWidth chars)
			right := ansi.TruncateLeft(bgLine, rightOffset, "")
			bgLines[row] = left + fgLine + right
		}
	}

	return strings.Join(bgLines[:height], "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
