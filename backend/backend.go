package backend

import (
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

// Backend abstracts all cron and history operations for a single server.
type Backend interface {
	Name() string
	ReadJobs() ([]cron.Job, error)
	WriteJobs(jobs []cron.Job) error
	RunJob(command string) (string, error)
	LoadHistory() ([]history.Entry, error)
	WriteHistory(jobName, output string, success bool) error
	EnsureRecordScript() error
	Close() error
}
