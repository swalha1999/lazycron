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
		return j.Runner
	}
	return DefaultRunner
}

// LazycronBinPath returns ~/.lazycron/bin/lazycron, the stable link to the
// lazycron binary that herdr-mode crontab lines call.
func LazycronBinPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "bin", "lazycron")
}

// Markers of the herdr exec line:
//
//	if [ -x '<bin>' ]; then '<bin>' exec '<id>' --name '<name>'; else <record line>; fi
//
// The else branch is the plain record line, so a job still runs (and is
// recorded) if the lazycron binary is gone.
const (
	execLinePrefix = "if [ -x "
	execLineElse   = "; else "
	execLineSuffix = "; fi"
)

// ExecLine wraps a plain record line so cron runs the job through
// `lazycron exec`, falling back to the record line.
func ExecLine(j Job, recordLine string) string {
	bin := ShellQuote(LazycronBinPath())
	call := fmt.Sprintf("%s exec %s --name %s", bin, ShellQuote(j.ID), ShellQuote(j.Name))
	if j.OneShot {
		call += " --once"
	}
	return fmt.Sprintf("%s%s ]; then %s%s%s%s", execLinePrefix, bin, call, execLineElse, recordLine, execLineSuffix)
}

// IsExecLine reports whether a raw crontab command is a herdr exec line.
func IsExecLine(rawCommand string) bool {
	return strings.HasPrefix(rawCommand, execLinePrefix) &&
		strings.HasSuffix(rawCommand, execLineSuffix) &&
		strings.Contains(rawCommand, execLineElse) &&
		strings.Contains(rawCommand, " exec ")
}

// execLineFallback returns the record line inside an exec line.
func execLineFallback(rawCommand string) string {
	start := strings.Index(rawCommand, execLineElse) + len(execLineElse)
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
