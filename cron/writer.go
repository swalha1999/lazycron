package cron

import (
	"fmt"
	"os/exec"
	"strings"
)

// ReadCrontab reads the current crontab.
func ReadCrontab() (string, error) {
	cmd := exec.Command("crontab", "-l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "no crontab for") {
			return "", nil
		}
		return "", fmt.Errorf("crontab -l: %s", strings.TrimSpace(outStr))
	}
	return string(out), nil
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
func WriteCrontab(jobs []Job) error {
	content := FormatCrontab(jobs)
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab -: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CheckCrontabAvailable checks if the crontab command exists.
func CheckCrontabAvailable() error {
	_, err := exec.LookPath("crontab")
	if err != nil {
		return fmt.Errorf("crontab command not found in PATH — lazycron requires crontab to be installed")
	}
	return nil
}

// RunJobNow runs a job command immediately in a shell.
// Returns the combined stdout/stderr output and any error.
func RunJobNow(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	return output, err
}
