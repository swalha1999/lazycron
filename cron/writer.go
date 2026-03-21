package cron

import (
	"fmt"
	"os"
	"os/exec"
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

	// runShell executes a shell command and returns trimmed output.
	runShell = func(command string) (string, error) {
		cmd := exec.Command("sh", "-c", command)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
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
	var b strings.Builder
	for i, job := range jobs {
		b.WriteString(job.CrontabLine())
		if i < len(jobs)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	return b.String()
}

// WriteCrontab writes jobs to crontab via `crontab -`.
// It first syncs script files to ~/.lazycron/scripts/.
func WriteCrontab(jobs []Job) error {
	if err := SyncScripts(jobs); err != nil {
		return fmt.Errorf("sync scripts: %w", err)
	}
	content := FormatCrontab(jobs)
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

// RunJobNow ensures the script file is up-to-date and runs it.
// Returns the combined stdout/stderr output and any error.
func RunJobNow(name, command string) (string, error) {
	path := ScriptPath(name)

	// Only write the script if it doesn't exist yet (SyncScripts handles normal updates).
	// This avoids overwriting a good script with a potentially stale in-memory command.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := WriteScript(name, command); err != nil {
			return "", fmt.Errorf("write script: %w", err)
		}
	}

	return runShell("sh '" + path + "'")
}
