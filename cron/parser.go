package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Job represents a single cron job entry.
type Job struct {
	Name     string
	Schedule string
	Command  string
	Enabled  bool
}

// recordBinPath returns the path to ~/.lazycron/bin/record.
func recordBinPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "bin", "record")
}

// WrapWithRecord wraps a command to pipe output through the record binary.
func WrapWithRecord(command, jobName string) string {
	return fmt.Sprintf("{ %s; } 2>&1 | %s %q", command, recordBinPath(), jobName)
}

// StripRecord removes the record pipe wrapper from a command, returning the inner command.
func StripRecord(command string) string {
	recPath := recordBinPath()

	// Look for | <record-path> "name" suffix
	pipeIdx := strings.LastIndex(command, "| "+recPath+" ")
	if pipeIdx == -1 {
		// Also try with | tee -a ... | record
		pipeIdx = strings.LastIndex(command, "| "+recPath+" ")
	}
	if pipeIdx == -1 {
		return command
	}

	inner := strings.TrimSpace(command[:pipeIdx])

	// Also strip tee if present: | tee -a logfile | record
	if teeIdx := strings.LastIndex(inner, "| tee -a "); teeIdx != -1 {
		inner = strings.TrimSpace(inner[:teeIdx])
	}

	// Strip { ...; } 2>&1 wrapper
	if strings.HasPrefix(inner, "{ ") && strings.HasSuffix(inner, "; } 2>&1") {
		inner = strings.TrimPrefix(inner, "{ ")
		inner = strings.TrimSuffix(inner, "; } 2>&1")
		inner = strings.TrimSpace(inner)
	}

	return inner
}

// CrontabLine returns the crontab representation of a job,
// including the name comment and optionally the disabled prefix.
func (j Job) CrontabLine() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", j.Name)

	// Wrap command with record pipe
	wrappedCmd := WrapWithRecord(j.Command, j.Name)

	if !j.Enabled {
		fmt.Fprintf(&b, "#DISABLED %s %s", j.Schedule, wrappedCmd)
	} else {
		fmt.Fprintf(&b, "%s %s", j.Schedule, wrappedCmd)
	}
	return b.String()
}

// Parse parses crontab -l output into a slice of Jobs.
func Parse(output string) []Job {
	lines := strings.Split(output, "\n")
	var jobs []Job
	autoIndex := 1

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines
		if line == "" {
			i++
			continue
		}

		// Check if this is a name comment: # job-name
		// But not a disabled line or env variable
		if isNameComment(line) {
			name := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			i++
			if i < len(lines) {
				jobLine := strings.TrimSpace(lines[i])
				if jobLine == "" {
					i++
					continue
				}
				if job, ok := parseJobLine(jobLine, name); ok {
					jobs = append(jobs, job)
					i++
					continue
				}
			}
			continue
		}

		// Regular cron line or disabled line without a preceding name comment
		if job, ok := parseJobLine(line, ""); ok {
			if job.Name == "" {
				job.Name = fmt.Sprintf("job-%d", autoIndex)
				autoIndex++
			}
			jobs = append(jobs, job)
		}
		i++
	}

	return jobs
}

func isNameComment(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	// Not a disabled line
	if strings.HasPrefix(line, "#DISABLED ") {
		return false
	}
	// Not an env assignment like # PATH=... or shell comment with =
	rest := strings.TrimPrefix(line, "#")
	rest = strings.TrimSpace(rest)
	// Must not be empty
	if rest == "" {
		return false
	}
	// Should look like a simple name (no = sign in first word)
	if strings.Contains(strings.Fields(rest)[0], "=") {
		return false
	}
	return true
}

func parseJobLine(line, name string) (Job, bool) {
	// Disabled job
	if strings.HasPrefix(line, "#DISABLED ") {
		rest := strings.TrimPrefix(line, "#DISABLED ")
		schedule, command, ok := splitCronLine(rest)
		if !ok {
			return Job{}, false
		}
		return Job{
			Name:     name,
			Schedule: schedule,
			Command:  StripRecord(command),
			Enabled:  false,
		}, true
	}

	// Skip other comments
	if strings.HasPrefix(line, "#") {
		return Job{}, false
	}

	// Active job
	schedule, command, ok := splitCronLine(line)
	if !ok {
		return Job{}, false
	}
	return Job{
		Name:     name,
		Schedule: schedule,
		Command:  StripRecord(command),
		Enabled:  true,
	}, true
}

// splitCronLine splits a cron line into schedule (5 fields) and command.
func splitCronLine(line string) (schedule, command string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return "", "", false
	}
	schedule = strings.Join(fields[:5], " ")

	// Walk past the first 5 whitespace-separated fields to find where the command starts.
	pos := 0
	for i := 0; i < 5; i++ {
		// Skip leading whitespace
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}
		// Skip the field
		for pos < len(line) && line[pos] != ' ' && line[pos] != '\t' {
			pos++
		}
	}
	command = strings.TrimSpace(line[pos:])
	return schedule, command, true
}
