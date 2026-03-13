package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

type clearStatusMsg struct {
	id int
}

type historyTickMsg struct{}

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

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadJobs, loadHistory, historyTick())
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

func loadHistory() tea.Msg {
	entries, err := history.LoadAll()
	return historyLoadedMsg{entries: entries, err: err}
}

func saveHistory(jobName, output string, success bool) tea.Cmd {
	return func() tea.Msg {
		err := history.WriteEntry(jobName, output, success)
		return historySavedMsg{err: err}
	}
}
