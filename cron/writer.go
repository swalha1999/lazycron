package cron

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// These function variables wrap system calls so tests can replace them.
var (
	// runCrontab executes a crontab command with optional stdin.
	runCrontab = func(stdin string, args ...string) (string, error) {
		cmd := exec.Command("crontab", args...)
		if stdin != "" {
			cmd.Stdin = strings.NewReader(stdin)
		}
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// lookPath checks if a command exists in PATH.
	lookPath = exec.LookPath
)

// ReadCrontab reads the current crontab.
func ReadCrontab() (string, error) {
	out, err := runCrontab("", "-l")
	if err != nil {
		if strings.Contains(out, "no crontab for") {
			return "", nil
		}
		return "", fmt.Errorf("crontab -l: %s", strings.TrimSpace(out))
	}
	return out, nil
}

// FormatCrontab formats jobs into crontab file content.
func FormatCrontab(jobs []Job) string {
	return FormatCrontabFor(jobs, DefaultRunner)
}

// FormatCrontabFor formats jobs for a crontab whose default runner is
// defaultRunner.
func FormatCrontabFor(jobs []Job, defaultRunner string) string {
	var b strings.Builder
	for i, job := range jobs {
		b.WriteString(job.CrontabLineFor(defaultRunner))
		if i < len(jobs)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

// RelocateCrontab rewrites the local lazycron paths in formatted crontab
// content for another location: scriptsDir replaces ~/.lazycron/scripts and,
// when non-empty, binDir replaces ~/.lazycron/bin (record and lazycron links).
func RelocateCrontab(content, scriptsDir, binDir string) string {
	content = strings.ReplaceAll(content, ScriptsDir(), scriptsDir)
	if binDir != "" {
		content = strings.ReplaceAll(content, filepath.Dir(RecordBinPath()), binDir)
	}
	return content
}

// WriteCrontab writes jobs to crontab via `crontab -`, keeping the current
// crontab's foreign lines (env assignments, @reboot entries, comments).
// It first syncs script files to ~/.lazycron/scripts/.
func WriteCrontab(jobs []Job) error {
	existing, err := ReadCrontab()
	if err != nil {
		return err
	}
	if err := SyncScripts(jobs); err != nil {
		return fmt.Errorf("sync scripts: %w", err)
	}
	content := KeepForeignLines(existing, FormatCrontab(jobs))
	out, err := runCrontab(content, "-")
	if err != nil {
		return fmt.Errorf("crontab -: %s", strings.TrimSpace(out))
	}
	return nil
}

// CheckCrontabAvailable checks if the crontab command exists.
func CheckCrontabAvailable() error {
	_, err := lookPath("crontab")
	if err != nil {
		return fmt.Errorf("crontab command not found in PATH — lazycron requires crontab to be installed")
	}
	return nil
}
