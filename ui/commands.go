package ui

import (
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/runner"
)

type jobsLoadedMsg struct {
	jobs []cron.Job
	err  error
}

type jobSavedMsg struct {
	jobs []cron.Job // the jobs as written (or as read, when err != nil)
	err  error
}

type jobRanMsg struct {
	id     string
	name   string
	output string
	err    error
}

type historyLoadedMsg struct {
	entries []history.Entry
	err     error
	runs    runState
}

type historyDeletedMsg struct {
	err error
}

type clearStatusMsg struct {
	id int
}

type historyTickMsg struct{}

type timeTickMsg struct{}

type splashDoneMsg struct{}

// Server-related messages
type serverConnectedMsg struct {
	index int
	err   error
}

type serverDataLoadedMsg struct {
	index   int
	jobs    []cron.Job
	history []history.Entry
	err     error
}

func clearStatusAfter(id int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{id: id}
	})
}

func historyTick() tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg {
		return historyTickMsg{}
	})
}

func timeTick() tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg {
		return timeTickMsg{}
	})
}

func splashTimer() tea.Cmd {
	return tea.Tick(1200*time.Millisecond, func(time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	b := m.manager.ActiveBackend()
	// Get timezone for local server (index 0)
	tzName, tzOffset, tzErr := b.GetTimezone()
	if tzErr == nil {
		m.manager.SetServerTimezone(0, tzName, tzOffset)
	}
	return tea.Batch(loadJobs(b), loadHistory(b), historyTick(), timeTick(), splashTimer(), checkHerdrNotice(b))
}

// disableCompletedOneShotsMsg is sent after one-shot jobs have been auto-disabled.
type disableCompletedOneShotsMsg struct {
	disabled int
	jobs     []cron.Job
	err      error
}

// disableCompletedOneShots checks enabled one-shot jobs against history
// and disables any that have already executed. This acts as a backup for the
// shell-based auto-disable in record.sh, which can fail on macOS due to
// crontab permission restrictions in cron execution context. It works on a
// fresh read of the crontab, never the TUI's in-memory list.
func disableCompletedOneShots(b backend.Backend, hist []history.Entry) tea.Cmd {
	count := 0
	mutation := disableRanOneShots(hist, &count)
	return func() tea.Msg {
		jobs, err := applyMutation(b, mutation)
		if errors.Is(err, errNoChange) {
			return disableCompletedOneShotsMsg{}
		}
		return disableCompletedOneShotsMsg{disabled: count, jobs: jobs, err: err}
	}
}

func loadJobs(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		jobs, err := b.ReadJobs()
		return jobsLoadedMsg{jobs: jobs, err: err}
	}
}

// saveJobs applies mutation to the backend's current jobs and writes them.
func saveJobs(b backend.Backend, mutation jobMutation) tea.Cmd {
	return func() tea.Msg {
		jobs, err := applyMutation(b, mutation)
		return jobSavedMsg{jobs: jobs, err: err}
	}
}

// runJob runs a job through the backend's run engine (herdr or plain), which
// records the history entry itself.
func runJob(b backend.Backend, job cron.Job) tea.Cmd {
	return func() tea.Msg {
		entry, err := b.ExecJob(job, backend.ExecRequest{Trigger: runner.TriggerManual})
		if err == nil && (entry.Success == nil || !*entry.Success) {
			err = runFailure(entry)
		}
		return jobRanMsg{id: job.ID, name: job.Name, output: entry.Output, err: err}
	}
}

// runFailure describes why a recorded run failed.
func runFailure(e history.Entry) error {
	switch {
	case e.Signal != 0:
		return fmt.Errorf("killed by signal %d", e.Signal)
	case e.ExitCode != nil:
		return fmt.Errorf("exit status %d", *e.ExitCode)
	default:
		return fmt.Errorf("run ended without an exit code")
	}
}

func loadHistory(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		entries, err := b.LoadHistory()
		return historyLoadedMsg{entries: entries, err: err, runs: loadRunState(b, entries)}
	}
}

func deleteHistory(b backend.Backend, filePath string) tea.Cmd {
	return func() tea.Msg {
		err := b.DeleteHistory(filePath)
		return historyDeletedMsg{err: err}
	}
}

func connectServer(mgr *backend.Manager, index int) tea.Cmd {
	return connectServerWithPassword(mgr, index, "")
}

func connectServerWithPassword(mgr *backend.Manager, index int, password string) tea.Cmd {
	return func() tea.Msg {
		mgr.SetServerStatus(index, backend.ConnConnecting, "")
		b := mgr.BackendAt(index)
		if b == nil {
			return serverConnectedMsg{index: index, err: nil}
		}
		if password != "" {
			if rb, ok := b.(*backend.RemoteBackend); ok {
				rb.SetPassword(password)
			}
		}
		err := b.EnsureRecordScript()
		if err != nil {
			mgr.SetServerStatus(index, backend.ConnError, err.Error())
			return serverConnectedMsg{index: index, err: err}
		}
		// Get timezone information
		tzName, tzOffset, tzErr := b.GetTimezone()
		if tzErr == nil {
			mgr.SetServerTimezone(index, tzName, tzOffset)
		}
		mgr.SetServerStatus(index, backend.ConnConnected, "")
		return serverConnectedMsg{index: index, err: nil}
	}
}

func loadServerData(mgr *backend.Manager, index int) tea.Cmd {
	return func() tea.Msg {
		b := mgr.BackendAt(index)
		if b == nil {
			return serverDataLoadedMsg{index: index}
		}
		jobs, jobErr := b.ReadJobs()
		hist, histErr := b.LoadHistory()
		var err error
		if jobErr != nil {
			err = jobErr
		} else if histErr != nil {
			err = histErr
		}
		return serverDataLoadedMsg{
			index:   index,
			jobs:    jobs,
			history: hist,
			err:     err,
		}
	}
}
