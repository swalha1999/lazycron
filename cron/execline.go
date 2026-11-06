package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/swalha1999/lazycron/config"
)

// DefaultRunner is the runner used by jobs without a per-job override. The
// CLI and TUI set it from ~/.lazycron/config.yml at startup.
var DefaultRunner = config.RunnerPlain

// EffectiveRunner returns the job's runner override or DefaultRunner.
func (j Job) EffectiveRunner() string {
	if j.Runner != "" {
		return j.Runner // without touching the process-wide default
	}
	return DefaultRunner
}

func (j Job) runnerWith(defaultRunner string) string {
	if j.Runner != "" {
		return j.Runner
	}
	return defaultRunner
}

// LazycronBinPath returns ~/.lazycron/bin/lazycron, the stable link to the
// lazycron binary that herdr-mode crontab lines call.
func LazycronBinPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "bin", "lazycron")
}

// The herdr exec line:
//
//	'<bin>' exec '<id>' --name '<name>' || { <record line>; }
//
// `lazycron exec` exits non-zero only when it could not start the job, so
// the record line after || runs the job plainly (and records it) if the
// lazycron binary is missing, crashes or rejects its arguments. A job never
// silently does not run.
const (
	execLineCall     = "' exec '"
	execLineFallback = " || { "
	execLineSuffix   = "; }"
)

// ExecLine wraps a plain record line so cron runs the job through
// `lazycron exec`, falling back to the record line.
func ExecLine(j Job, recordLine string) string {
	call := fmt.Sprintf("%s exec %s --name %s", ShellQuote(LazycronBinPath()), ShellQuote(j.ID), ShellQuote(j.Name))
	if j.OneShot {
		call += " --once"
	}
	return call + execLineFallback + recordLine + execLineSuffix
}

// IsExecLine reports whether a raw crontab command is a herdr exec line.
func IsExecLine(rawCommand string) bool {
	call := strings.Index(rawCommand, execLineCall)
	fallback := strings.Index(rawCommand, execLineFallback)
	return strings.HasPrefix(rawCommand, "'") && call > 0 && fallback > call &&
		strings.HasSuffix(rawCommand, execLineSuffix)
}

// execLineRecord returns the record line inside an exec line.
func execLineRecord(rawCommand string) string {
	start := strings.Index(rawCommand, execLineFallback) + len(execLineFallback)
	return rawCommand[start : len(rawCommand)-len(execLineSuffix)]
}

// runnerMarker tags a job's runner override in its name comment.
const runnerMarker = " @runner:"

// extractRunner parses a runner override like "Job Name @runner:herdr".
func extractRunner(name string) (string, string) {
	idx := strings.Index(name, runnerMarker)
	if idx == -1 {
		return name, ""
	}
	rest := name[idx+len(runnerMarker):]
	end := strings.IndexByte(rest, ' ')
	if end == -1 {
		end = len(rest)
	}
	runner := rest[:end]
	if runner == "" || !config.ValidRunner(runner) {
		return name, ""
	}
	return strings.TrimSpace(name[:idx] + rest[end:]), runner
}
