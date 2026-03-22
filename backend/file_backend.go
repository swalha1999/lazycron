package backend

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

// FileBackend reads/writes a plain crontab file and uses a temp history
// directory instead of the system crontab. Useful for testing and dry-run.
type FileBackend struct {
	cronFile   string
	historyDir string
}

// NewFileBackend creates a backend backed by a crontab file and history directory.
func NewFileBackend(cronFile, historyDir string) *FileBackend {
	return &FileBackend{
		cronFile:   cronFile,
		historyDir: historyDir,
	}
}

func (b *FileBackend) Name() string { return "file:" + b.cronFile }

func (b *FileBackend) ReadJobs() ([]cron.Job, error) {
	data, err := os.ReadFile(b.cronFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return cron.Parse(string(data)), nil
}

func (b *FileBackend) WriteJobs(jobs []cron.Job) error {
	content := cron.FormatCrontab(jobs)
	return os.WriteFile(b.cronFile, []byte(content), 0o644)
}

func (b *FileBackend) RunJob(name, command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (b *FileBackend) LoadHistory() ([]history.Entry, error) {
	if err := os.MkdirAll(b.historyDir, 0o755); err != nil {
		return nil, err
	}
	return history.LoadAllFrom(b.historyDir)
}

func (b *FileBackend) WriteHistory(jobName, output string, success bool) error {
	if err := os.MkdirAll(b.historyDir, 0o755); err != nil {
		return err
	}
	return history.WriteEntryTo(b.historyDir, jobName, output, success)
}

func (b *FileBackend) DeleteHistory(filePath string) error {
	// Safety: only delete files within our history directory.
	if !strings.HasPrefix(filePath, b.historyDir) {
		return fmt.Errorf("refusing to delete file outside history dir")
	}
	return os.Remove(filePath)
}

func (b *FileBackend) EnsureRecordScript() error { return nil }

func (b *FileBackend) Close() error { return nil }
