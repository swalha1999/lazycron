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

// WriteCrontab writes jobs to crontab via `crontab -`.
func WriteCrontab(jobs []Job) error {
	var b strings.Builder
	for i, job := range jobs {
		b.WriteString(job.CrontabLine())
		if i < len(jobs)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(b.String())
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
func RunJobNow(command string) error {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
