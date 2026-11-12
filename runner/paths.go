// Package runner executes job runs: as plain processes or inside herdr-core
// terminals. It records each run as a history entry and keeps a marker file
// while a run is in progress, so every front end (CLI, TUI, cron) sees the
// same runs.
package runner

import (
	"os"
	"path/filepath"

	"github.com/swalha1999/lazycron/config"
)

// Paths are the directories a runner reads and writes.
type Paths struct {
	Scripts string // <job-id>.sh files
	History string // finished runs, <run-id>.json
	Runs    string // in-progress markers <run-id>.json and exit files
	Hold    string // folder held run terminals wait in; its name names the herdr-app space
}

// LocalPaths returns the standard layout under ~/.lazycron.
func LocalPaths() Paths {
	home, _ := os.UserHomeDir()
	return PathsUnder(filepath.Join(home, ".lazycron"))
}

// PathsUnder returns the standard layout below a lazycron directory.
func PathsUnder(dir string) Paths {
	return Paths{
		Scripts: filepath.Join(dir, "scripts"),
		History: filepath.Join(dir, "history"),
		Runs:    filepath.Join(dir, "runs"),
		Hold:    filepath.Join(dir, config.DefaultSpaceName),
	}
}

// ScriptPath returns the script file of a job.
func (p Paths) ScriptPath(jobID string) string {
	return filepath.Join(p.Scripts, jobID+".sh")
}

// ExitFile returns where a herdr run's wrapper writes the job's exit code.
func (p Paths) ExitFile(runID string) string {
	return filepath.Join(p.Runs, runID+".exit")
}

// AgentFile returns where `lazycron agent` describes the Claude session a
// run started; the run's history entry picks it up.
func (p Paths) AgentFile(runID string) string {
	return filepath.Join(p.Runs, runID+".agent.json")
}
