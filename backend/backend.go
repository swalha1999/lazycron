package backend

import (
	"io"

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

	// CopyProjectFiles transfers a local directory to the project's location
	// on the target. remoteSubpath is appended to the backend's project root
	// (~/.lazycron/projects/ on remote; not used locally). excludes are
	// tar-style paths relative to localDir's root, e.g. "./.env".
	// On LocalBackend this is a no-op (the project already lives in cwd).
	CopyProjectFiles(localDir, remoteSubpath string, excludes []string) error

	// CheckAgentDeps reports which of {docker, node, npx} are missing on the
	// target. An empty slice means all are present. err is non-nil only on
	// transport failure (e.g. SSH down).
	CheckAgentDeps() (missing []string, err error)

	// RunInProject runs a shell command from within the project directory on
	// the target. On LocalBackend this is exec.Command with Dir=cwd. On
	// RemoteBackend it's `cd ~/.lazycron/projects/<name> && <cmd>`. Output
	// is streamed to the caller-provided stdout/stderr writers.
	RunInProject(projectName, command string, stdout, stderr io.Writer) error
}
