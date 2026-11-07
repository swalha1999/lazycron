package backend

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/monitor"
	"github.com/swalha1999/lazycron/runner"
)

// FileBackend reads/writes a plain crontab file and uses a temp history
// directory instead of the system crontab. Useful for testing and dry-run.
// Job scripts live beside the file in <cronFile>.lazycron/scripts so the real
// ~/.lazycron/scripts is never touched.
type FileBackend struct {
	cronFile   string
	historyDir string
}

// ScriptsDir returns where this backend keeps job scripts. The path contains
// ".lazycron/scripts" so crontab lines referencing it parse as script refs.
func (b *FileBackend) ScriptsDir() string {
	return b.cronFile + ".lazycron/scripts"
}

// StateDir returns the backend's private lazycron directory (runs, holds).
func (b *FileBackend) StateDir() string {
	return b.cronFile + ".lazycron"
}

// HistoryDir returns the backend's history directory.
func (b *FileBackend) HistoryDir() string {
	return b.historyDir
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
	if err := cron.SyncScriptsIn(b.ScriptsDir(), jobs); err != nil {
		return fmt.Errorf("sync scripts: %w", err)
	}
	content := cron.RelocateCrontab(cron.FormatCrontab(jobs), b.ScriptsDir(), "")
	return os.WriteFile(b.cronFile, []byte(content), 0o644)
}

func (b *FileBackend) LoadHistory() ([]history.Entry, error) {
	if err := os.MkdirAll(b.historyDir, 0o755); err != nil {
		return nil, err
	}
	return history.LoadAllFrom(b.historyDir)
}

func (b *FileBackend) DeleteHistory(filePath string) error {
	safe, err := ensureInsideDir(b.historyDir, filePath)
	if err != nil {
		return err
	}
	return os.Remove(safe)
}

func (b *FileBackend) EnsureRecordScript() error { return nil }

// GetTimezone returns the local timezone name and UTC offset in seconds.
func (b *FileBackend) GetTimezone() (string, int, error) {
	now := time.Now()
	_, offset := now.Zone()
	tzName := now.Format("MST")
	return tzName, offset, nil
}

// GetRunningJobs returns empty list for file backend (monitoring not supported).
func (b *FileBackend) GetRunningJobs() ([]monitor.RunningJob, error) {
	return nil, nil
}

// KillJob is not supported for file backend.
func (b *FileBackend) KillJob(pid int) error {
	return fmt.Errorf("kill job not supported for file backend")
}

func (b *FileBackend) Close() error { return nil }

// CopyProjectFiles is a no-op on the file backend (used for tests/dry-run).
func (b *FileBackend) CopyProjectFiles(localDir, remoteSubpath string, excludes []string) error {
	return nil
}

// CheckAgentDeps reports nothing missing — keeps tests using FileBackend
// from being coupled to the host's installed binaries.
func (b *FileBackend) CheckAgentDeps() ([]string, error) {
	return nil, nil
}

// ProjectDir returns the current working directory — file backend has no
// remote home concept and is used for tests/dry-run.
func (b *FileBackend) ProjectDir(projectName string) (string, error) {
	return os.Getwd()
}

// Paths returns the run-engine layout private to this crontab file.
func (b *FileBackend) Paths() runner.Paths {
	p := runner.PathsUnder(b.StateDir())
	p.Scripts = b.ScriptsDir()
	p.History = b.historyDir
	return p
}

func (b *FileBackend) ExecJob(job cron.Job, req ExecRequest) (history.Entry, error) {
	return execLocally(b, b.Paths(), job, req)
}

func (b *FileBackend) StartJob(job cron.Job, req ExecRequest) error {
	return startLocally(b.Paths(), job, req, "--cron-file", b.cronFile)
}

func (b *FileBackend) ActiveRuns() ([]runner.Active, error) {
	return runner.LoadActive(b.Paths().Runs)
}

func (b *FileBackend) Herdr() (*runner.HC, error) { return localHC() }

func (b *FileBackend) InteractiveCommand(argv []string) (*exec.Cmd, error) {
	return localCommand(argv), nil
}

func (b *FileBackend) KillProcessGroup(pgid int) error { return killGroup(pgid) }
