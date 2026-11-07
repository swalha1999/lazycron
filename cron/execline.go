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
//	'<bin>' exec '<id>' --name '<name>' [--once]
//
// lazycron exec runs the job's script (~/.lazycron/scripts/<id>.sh) in herdr,
// or plainly when herdr-core is unavailable, and records it. Lines written
// before v0.16 appended ` || { <record line>; }` as a fallback for a missing
// lazycron; they still parse, count as an old format, and `lazycron setup`
// rewrites them.
const (
	execLineCall           = "' exec '"
	legacyExecLineFallback = " || { "
	legacyExecLineSuffix   = "; }"
)

// ExecLine is the crontab command that runs j through `lazycron exec`.
func ExecLine(j Job) string {
	call := fmt.Sprintf("%s exec %s --name %s", ShellQuote(LazycronBinPath()), ShellQuote(j.ID), ShellQuote(j.Name))
	if j.OneShot {
		call += " --once"
	}
	return call
}

// IsExecLine reports whether a raw crontab command is a herdr exec line,
// current or legacy.
func IsExecLine(rawCommand string) bool {
	return strings.HasPrefix(rawCommand, "'") && strings.Index(rawCommand, execLineCall) > 0
}

// isLegacyExecLine reports whether an exec line still carries the old
// `|| { <record line>; }` fallback.
func isLegacyExecLine(rawCommand string) bool {
	fallback := strings.Index(rawCommand, legacyExecLineFallback)
	return IsExecLine(rawCommand) && fallback > strings.Index(rawCommand, execLineCall) &&
		strings.HasSuffix(rawCommand, legacyExecLineSuffix)
}

// execLineRecord returns the record line inside a legacy exec line.
func execLineRecord(rawCommand string) string {
	start := strings.Index(rawCommand, legacyExecLineFallback) + len(legacyExecLineFallback)
	return rawCommand[start : len(rawCommand)-len(legacyExecLineSuffix)]
}

// execLineID returns the job id an exec line runs.
func execLineID(rawCommand string) string {
	rest := rawCommand[strings.Index(rawCommand, execLineCall)+len(execLineCall):]
	if end := strings.Index(rest, "'"); end >= 0 {
		return rest[:end]
	}
	return ""
}

// execLineCommand is the script reference a current exec line runs, with
// the script in scriptsDir (which differs for servers and crontab files).
func execLineCommand(rawCommand, scriptsDir string) string {
	return "bash '" + filepath.Join(scriptsDir, execLineID(rawCommand)+".sh") + "'"
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
