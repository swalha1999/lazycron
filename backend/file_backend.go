package backend

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/monitor"
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

func (b *FileBackend) RunJob(id, name, command string) (string, error) {
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

func (b *FileBackend) WriteHistory(jobID, jobName, output string, success bool) error {
	if err := os.MkdirAll(b.historyDir, 0o755); err != nil {
		return err
	}
	return history.WriteEntryTo(b.historyDir, jobID, jobName, output, success)
}

func (b *FileBackend) DeleteHistory(filePath string) error {
	// Safety: only delete files within our history directory.
	// Clean and resolve both paths to absolute form to prevent path traversal.
	absHistoryDir, err := filepath.Abs(filepath.Clean(b.historyDir))
	if err != nil {
		return fmt.Errorf("failed to resolve history dir: %w", err)
	}

	absFilePath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Use filepath.Rel to check if the file is within the history directory.
	// If Rel returns a path that starts with "..", the file is outside.
	rel, err := filepath.Rel(absHistoryDir, absFilePath)
	if err != nil {
		return fmt.Errorf("refusing to delete file outside history dir")
	}

	// Check if the relative path escapes the directory (contains ..)
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("refusing to delete file outside history dir")
	}

	return os.Remove(absFilePath)
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

// RunInProject runs the command directly with no project-dir wrap.
// Suitable for unit tests; not used in production paths.
func (b *FileBackend) RunInProject(projectName, command string, stdout, stderr io.Writer) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// ProjectDir returns the current working directory — file backend has no
// remote home concept and is used for tests/dry-run.
func (b *FileBackend) ProjectDir(projectName string) (string, error) {
	return os.Getwd()
}
