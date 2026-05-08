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

func (b *LocalBackend) RunJob(id, name, command string) (string, error) {
	return cron.RunJobNow(id, command)
}

func (b *LocalBackend) LoadHistory() ([]history.Entry, error) {
	return history.LoadAll()
}

func (b *LocalBackend) WriteHistory(jobID, jobName, output string, success bool) error {
	return history.WriteEntry(jobID, jobName, output, success)
}

func (b *LocalBackend) DeleteHistory(filePath string) error {
	absHistoryDir, err := filepath.Abs(filepath.Clean(record.HistoryDir()))
	if err != nil {
		return fmt.Errorf("failed to resolve history dir: %w", err)
	}

	absFilePath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	rel, err := filepath.Rel(absHistoryDir, absFilePath)
	if err != nil {
		return fmt.Errorf("refusing to delete file outside history dir")
	}

	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("refusing to delete file outside history dir")
	}

	return os.Remove(absFilePath)
}

func (b *LocalBackend) EnsureRecordScript() error {
	return record.InstallRecord()
}

// GetTimezone returns the local timezone name and UTC offset in seconds.
func (b *LocalBackend) GetTimezone() (string, int, error) {
	now := time.Now()
	_, offset := now.Zone()
	tzName := now.Format("MST")
	return tzName, offset, nil
}

// GetRunningJobs returns all currently running lazycron jobs on the local system.
func (b *LocalBackend) GetRunningJobs() ([]monitor.RunningJob, error) {
	scriptsDir := cron.ScriptsDir()
	return monitor.GetRunningJobs(scriptsDir)
}

// KillJob kills a running job by PID on the local system.
func (b *LocalBackend) KillJob(pid int) error {
	return monitor.KillJob(pid)
}

func (b *LocalBackend) Close() error { return nil }

// CopyProjectFiles is a no-op locally — the project already lives where the
// user invoked lazycron from.
func (b *LocalBackend) CopyProjectFiles(localDir, remoteSubpath string, excludes []string) error {
	return nil
}

// CheckAgentDeps reports which of docker/node/npx are missing on $PATH.
func (b *LocalBackend) CheckAgentDeps() ([]string, error) {
	var missing []string
	for _, name := range []string{"docker", "node", "npx"} {
		if _, err := exec.LookPath(name); err != nil {
			missing = append(missing, name)
		}
	}
	return missing, nil
}

// RunInProject runs command from the current working directory and streams
// output. projectName is unused locally — included only for interface symmetry.
func (b *LocalBackend) RunInProject(projectName, command string, stdout, stderr io.Writer) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// ProjectDir returns the current working directory — local jobs run in cwd.
func (b *LocalBackend) ProjectDir(projectName string) (string, error) {
	return os.Getwd()
}
