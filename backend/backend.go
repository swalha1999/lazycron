package backend

import (
	"os/exec"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/monitor"
	"github.com/swalha1999/lazycron/runner"
)

// Backend abstracts all cron and history operations for a single server.
type Backend interface {
	Name() string
	ReadJobs() ([]cron.Job, error)
	WriteJobs(jobs []cron.Job) error
	LoadHistory() ([]history.Entry, error)
	DeleteHistory(filePath string) error
	EnsureRecordScript() error
	GetTimezone() (string, int, error) // returns timezone name and offset in seconds
	GetRunningJobs() ([]monitor.RunningJob, error)
	KillJob(pid int) error
	Close() error

	// ExecJob runs a job now through the run engine (herdr or plain, as the
	// job says), waits for it and returns its recorded history entry.
	ExecJob(job cron.Job, req ExecRequest) (history.Entry, error)
	// StartJob starts a run in the background and returns at once; the run
	// shows up in ActiveRuns and later in LoadHistory under req.RunID.
	StartJob(job cron.Job, req ExecRequest) error
	// ActiveRuns returns runs in progress, newest first.
	ActiveRuns() ([]runner.Active, error)
	// KillProcessGroup ends a plain run's process group (SIGTERM).
	KillProcessGroup(pgid int) error
	// Herdr returns a herdr-core client on the target.
	Herdr() (*runner.HC, error)
	// InteractiveCommand returns a command that runs argv on the target with
	// this terminal's stdio (used for `hc attach`).
	InteractiveCommand(argv []string) (*exec.Cmd, error)

	// CopyProjectFiles transfers a local directory to the project's location
	// on the target. remoteSubpath is appended to the backend's project root
	// (~/.lazycron/projects/ on remote; not used locally). excludes are
	// tar-style paths relative to localDir's root, e.g. "./.env".
	// On LocalBackend this is a no-op (the project already lives in cwd).
	CopyProjectFiles(localDir, remoteSubpath string, excludes []string) error

	// CheckAgentDeps reports which of AgentDeps are missing on the target.
	CheckAgentDeps() (missing []string, err error)

	// ProjectDir returns the absolute path to the project's working directory
	// on the target — what cron entries `cd` into before invoking the agent.
	// LocalBackend returns the current working directory; RemoteBackend
	// returns ~/.lazycron/projects/<projectName> with the remote home
	// resolved to an absolute path (no literal `~`, since cron commands are
	// single-quoted and `~` does not expand inside single quotes).
	ProjectDir(projectName string) (string, error)
}

// AgentDeps are the programs an agent job needs on the machine that runs it:
// lazycron to run it, Claude Code, git and gh for the work, herdr-core for
// the terminal. Each must be installed (on PATH, or in /usr/local/bin or
// ~/.local/bin, where installers put them).
var AgentDeps = []string{"lazycron", "claude", "git", "gh", "herdr-core"}

// agentDepDirs are checked besides PATH: non-login shells (cron, ssh
// commands) often lack them.
var agentDepDirs = []string{"/usr/local/bin", "$HOME/.local/bin"}
