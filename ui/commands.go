package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

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

type historyLoadedMsg struct {
	entries []history.Entry
	err     error
}

type historySavedMsg struct {
	err error
}

type historyDeletedMsg struct {
	err error
}

type clearStatusMsg struct {
	id int
}

type historyTickMsg struct{}

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

func splashTimer() tea.Cmd {
	return tea.Tick(1200*time.Millisecond, func(time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	b := m.manager.ActiveBackend()
	return tea.Batch(loadJobs(b), loadHistory(b), historyTick(), splashTimer())
}

// disableCompletedOneShotsMsg is sent after one-shot jobs have been auto-disabled.
type disableCompletedOneShotsMsg struct {
	disabled int
	err      error
}

// disableCompletedOneShots checks enabled one-shot jobs against history
// and disables any that have already executed. This acts as a backup for the
// shell-based auto-disable in record.sh, which can fail on macOS due to
// crontab permission restrictions in cron execution context.
func disableCompletedOneShots(b backend.Backend, jobs []cron.Job, hist []history.Entry) tea.Cmd {
	// Copy the slice so we don't mutate the model from a command
	updated := make([]cron.Job, len(jobs))
	copy(updated, jobs)

	return func() tea.Msg {
		// Build set of job names that have history entries
		ran := make(map[string]bool, len(hist))
		for _, e := range hist {
			ran[e.JobName] = true
		}

		// Find enabled one-shot jobs that have already run
		count := 0
		for i := range updated {
			if updated[i].OneShot && updated[i].Enabled && ran[updated[i].Name] {
				updated[i].Enabled = false
				count++
			}
		}

		if count == 0 {
			return disableCompletedOneShotsMsg{}
		}

		err := b.WriteJobs(updated)
		return disableCompletedOneShotsMsg{disabled: count, err: err}
	}
}

func loadJobs(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		jobs, err := b.ReadJobs()
		return jobsLoadedMsg{jobs: jobs, err: err}
	}
}

func saveJobs(b backend.Backend, jobs []cron.Job) tea.Cmd {
	return func() tea.Msg {
		err := b.WriteJobs(jobs)
		return jobSavedMsg{err: err}
	}
}

func runJob(b backend.Backend, name, command string) tea.Cmd {
	return func() tea.Msg {
		output, err := b.RunJob(name, command)
		return jobRanMsg{name: name, output: output, err: err}
	}
}

func loadHistory(b backend.Backend) tea.Cmd {
	return func() tea.Msg {
		entries, err := b.LoadHistory()
		return historyLoadedMsg{entries: entries, err: err}
	}
}

func deleteHistory(b backend.Backend, filePath string) tea.Cmd {
	return func() tea.Msg {
		err := b.DeleteHistory(filePath)
		return historyDeletedMsg{err: err}
	}
}

func saveHistory(b backend.Backend, jobName, output string, success bool) tea.Cmd {
	return func() tea.Msg {
		err := b.WriteHistory(jobName, output, success)
		return historySavedMsg{err: err}
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
