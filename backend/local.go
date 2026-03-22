package backend

import (
	"os"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/record"
)

// LocalBackend wraps existing cron/history packages for localhost operations.
type LocalBackend struct{}

func NewLocalBackend() *LocalBackend {
	return &LocalBackend{}
}

func (b *LocalBackend) Name() string { return "local" }

func (b *LocalBackend) ReadJobs() ([]cron.Job, error) {
	output, err := cron.ReadCrontab()
	if err != nil {
		return nil, err
	}
	return cron.Parse(output), nil
}

func (b *LocalBackend) WriteJobs(jobs []cron.Job) error {
	return cron.WriteCrontab(jobs)
}

func (b *LocalBackend) RunJob(name, command string) (string, error) {
	return cron.RunJobNow(name, command)
}

func (b *LocalBackend) LoadHistory() ([]history.Entry, error) {
	return history.LoadAll()
}

func (b *LocalBackend) WriteHistory(jobName, output string, success bool) error {
	return history.WriteEntry(jobName, output, success)
}

func (b *LocalBackend) DeleteHistory(filePath string) error {
	return os.Remove(filePath)
}

func (b *LocalBackend) EnsureRecordScript() error {
	return record.InstallRecord()
}

func (b *LocalBackend) Close() error { return nil }
