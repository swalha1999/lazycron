package ui

import (
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
)

type statusType int

const (
	statusNone statusType = iota
	statusError
	statusSuccess
	statusInfo
)

const (
	panelJobs    = 0
	panelHistory = 1
	panelDetail  = 2
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

	history         []history.Entry
	historySelected int
	focusPanel      int
	lastLeftPanel   int
	historyScroll   int
	detailScroll    int
}

func NewModel() Model {
	return Model{
		selected: 0,
		mode:     modeNormal,
	}
}
