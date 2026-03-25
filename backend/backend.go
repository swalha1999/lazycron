package backend

import (
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/monitor"
)

// Backend abstracts all cron and history operations for a single server.
type Backend interface {
	Name() string
	ReadJobs() ([]cron.Job, error)
	WriteJobs(jobs []cron.Job) error
	RunJob(id, name, command string) (string, error)
	LoadHistory() ([]history.Entry, error)
	WriteHistory(jobID, jobName, output string, success bool) error
	DeleteHistory(filePath string) error
	EnsureRecordScript() error
	GetTimezone() (string, int, error) // returns timezone name and offset in seconds
	GetRunningJobs() ([]monitor.RunningJob, error)
	KillJob(pid int) error
	Close() error
}
