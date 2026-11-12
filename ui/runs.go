package ui

import (
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/runner"
	"github.com/swalha1999/lazycron/setup"
)

// runState is what the TUI knows about runs beyond finished history: runs in
// progress and which herdr terminals are still open.
type runState struct {
	active map[string]runner.Active // newest in-progress run per job id
	live   map[string]bool          // herdr terminal address -> program still running
	agents map[string]string        // in-progress run id -> its live Claude session terminal
}

// agentLabel marks the terminal of an agent run's Claude session.
const agentLabel = "lazycron.agent"

// loadRunState reads in-progress runs and, when any run has a herdr
// terminal, the terminals' states (one `hc ls --all`).
func loadRunState(b backend.Backend, entries []history.Entry) runState {
	s := runState{active: map[string]runner.Active{}, live: map[string]bool{}, agents: map[string]string{}}
	active, _ := b.ActiveRuns()
	needTerminals := false
	for _, a := range active { // newest first
		if a.Stale {
			continue
		}
		if _, seen := s.active[a.JobID]; !seen {
			s.active[a.JobID] = a
		}
		needTerminals = needTerminals || a.Terminal != ""
	}
	for _, e := range entries {
		needTerminals = needTerminals || e.Terminal != ""
	}
	if !needTerminals {
		return s
	}
	hc, err := b.Herdr()
	if err != nil {
		return s
	}
	terms, err := hc.List(nil)
	if err != nil {
		return s
	}
	for _, t := range terms {
		s.live[t.Address] = t.Live()
	}
	for _, a := range s.active {
		if a.Terminal == "" {
			continue
		}
		agents, err := hc.List(map[string]string{runner.LabelRun: a.RunID, agentLabel: "claude"})
		if err != nil {
			continue
		}
		for _, t := range agents {
			if t.Live() {
				s.agents[a.RunID] = t.Address
			}
		}
	}
	return s
}

// runTerminal is the terminal to show for a run: its Claude session while
// that is still there, otherwise the run's own terminal.
func (s runState) runTerminal(e history.Entry) string {
	if e.Agent != nil {
		if kept, _ := s.terminalKept(e.Agent.Terminal); kept {
			return e.Agent.Terminal
		}
	}
	return e.Terminal
}

// activeTerminal is the terminal to attach to for a run in progress.
func (s runState) activeTerminal(a runner.Active) string {
	if t := s.agents[a.RunID]; t != "" {
		return t
	}
	return a.Terminal
}

// running returns the job's run in progress, if any.
func (s runState) running(jobID string) (runner.Active, bool) {
	a, ok := s.active[jobID]
	return a, ok
}

// terminalKept reports whether a run's terminal still exists, and whether
// its program is live (running, or held open for viewing).
func (s runState) terminalKept(addr string) (kept, live bool) {
	if addr == "" {
		return false, false
	}
	live, kept = s.live[addr]
	return kept, live
}

var (
	runningBadgeStyle  = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	terminalBadgeStyle = lipgloss.NewStyle().Foreground(colorMauve)
)

// runningBadge marks a job with a run in progress.
func runningBadge() string {
	return runningBadgeStyle.Render("⟳")
}

// terminalBadge marks a history entry whose herdr terminal can be attached.
func (s runState) terminalBadge(addr string) string {
	kept, live := s.terminalKept(addr)
	switch {
	case live:
		return terminalBadgeStyle.Render("⧉")
	case kept:
		return mutedItemStyle.Render("⧉")
	}
	return ""
}

// attachTarget returns the terminal the attach key should open: in the jobs
// panel the job's running terminal, in the history panel the entry's.
func (m Model) attachTarget() (string, string) {
	switch m.focusPanel {
	case panelJobs:
		if idx := m.currentJobIndex(); idx >= 0 && !m.isOnHeader() {
			job := m.jobs[idx]
			if a, ok := m.runs.running(job.ID); ok && a.Terminal != "" {
				return m.runs.activeTerminal(a), job.Name
			}
			return "", job.Name
		}
	case panelHistory:
		if m.historySelected >= 0 && m.historySelected < len(m.history) {
			e := m.history[m.historySelected]
			if t := m.runs.runTerminal(e); t != "" {
				if kept, _ := m.runs.terminalKept(t); kept {
					return t, e.JobName
				}
			}
			return "", e.JobName
		}
	}
	return "", ""
}

// attachPreparedMsg carries the command that attaches to a terminal.
type attachPreparedMsg struct {
	cmd  *exec.Cmd
	name string
	err  error
}

// attachDoneMsg reports the end of an attach session.
type attachDoneMsg struct{ err error }

// prepareAttach builds `hc attach <terminal>` on the active backend.
func prepareAttach(b backend.Backend, terminal, name string) tea.Cmd {
	return func() tea.Msg {
		hc, err := b.Herdr()
		if err != nil {
			return attachPreparedMsg{err: err}
		}
		c, err := b.InteractiveCommand(hc.AttachArgs(terminal))
		return attachPreparedMsg{cmd: c, name: name, err: err}
	}
}

// startAttach handles the attach key.
func (m Model) startAttach() (tea.Model, tea.Cmd) {
	terminal, name := m.attachTarget()
	if terminal == "" {
		if name == "" {
			return m, nil
		}
		return m, m.setStatus(fmt.Sprintf("'%s' has no herdr terminal to attach to", name), statusInfo, 3*time.Second)
	}
	return m, prepareAttach(m.manager.ActiveBackend(), terminal, name)
}

// handleAttachPrepared suspends the TUI and runs hc attach.
func (m Model) handleAttachPrepared(msg attachPreparedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, m.setStatus("Attach failed: "+msg.err.Error(), statusError, 5*time.Second)
	}
	return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg { return attachDoneMsg{err: err} })
}

// herdrNoticeMsg asks the TUI to show the one-time herdr-core notice.
type herdrNoticeMsg struct{}

// herdrInstalledMsg reports the end of the herdr-core installer.
type herdrInstalledMsg struct{ err error }

// checkHerdrNotice decides once per machine whether to offer installing
// herdr-core: only for the local backend, when it is missing and the user
// has not been asked before.
func checkHerdrNotice(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		if _, local := b.(*backend.LocalBackend); !local {
			return nil
		}
		cfg, err := config.Load()
		if err != nil || cfg.Herdr.Prompted {
			return nil
		}
		if _, err := runner.FindHC(cfg.Herdr.Bin); err == nil {
			return nil
		}
		return herdrNoticeMsg{}
	}
}

// markHerdrPrompted remembers that the notice was shown.
func markHerdrPrompted() {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	cfg.Herdr.Prompted = true
	_ = config.Save(cfg)
}

func (m Model) handleHerdrNoticeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		markHerdrPrompted()
		m.mode = modeNormal
		c := exec.Command("sh", "-c", "curl -fsSL "+setup.HerdrCoreInstaller+" | sh")
		return m, tea.ExecProcess(c, func(err error) tea.Msg { return herdrInstalledMsg{err: err} })
	case "n", "N", "esc", "q":
		markHerdrPrompted()
		m.mode = modeNormal
		return m, m.setStatus("Jobs run as plain cron. Install herdr-core later with `lazycron setup`.", statusInfo, 5*time.Second)
	}
	return m, nil
}

func renderHerdrNotice() string {
	body := formTitleStyle.Render("  herdr-core is not installed") + "\n\n" +
		detailValueStyle.Render("  lazycron can run each job in a herdr-core terminal you can") + "\n" +
		detailValueStyle.Render("  watch live, open later or take over (herdr-app shows them in") + "\n" +
		detailValueStyle.Render("  a \"Cron\" space). Without it, jobs run as plain cron jobs.") + "\n\n" +
		mutedItemStyle.Render("  herdr-core is a separate project (herdr.dev). Installing runs") + "\n" +
		mutedItemStyle.Render("  its official installer into ~/.local/bin.") + "\n\n" +
		helpBinding("y", "install now") + helpSep() + helpBinding("n", "not now (won't ask again)")
	return formStyle.Width(68).Render(body)
}
