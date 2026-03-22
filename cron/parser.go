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
	Wrapped  bool   // true if the raw command uses the current record wrapper format
	Tag      string // optional colored tag displayed after the name
	TagColor string // hex color for the tag, e.g. "#f38ba8"
	OneShot  bool   // true if this job should run once and then self-disable
	Project  string // optional project group for organizing jobs
}

// recordBinPath returns the path to ~/.lazycron/bin/record.
func recordBinPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "bin", "record")
}

// wrapPrefix is the start of the current record wrapper format.
const wrapPrefix = `__lc_out=$({ `

// wrapEndMarker separates the inner command from the exit-code capture.
const wrapEndMarker = `; } 2>&1); __lc_ec=$?;`

// WrapWithRecord wraps a command so its output and exit code are captured
// and piped through the record binary for history tracking.
func WrapWithRecord(command, jobName string) string {
	return fmt.Sprintf(`%s%s%s echo "$__lc_out" | %s %q "$__lc_ec"`,
		wrapPrefix, command, wrapEndMarker, recordBinPath(), jobName)
}

// WrapWithRecordOnce wraps a command like WrapWithRecord but appends --once
// so the record script auto-disables the crontab entry after execution.
func WrapWithRecordOnce(command, jobName string) string {
	return fmt.Sprintf(`%s%s%s echo "$__lc_out" | %s %q "$__lc_ec" --once`,
		wrapPrefix, command, wrapEndMarker, recordBinPath(), jobName)
}

// StripRecord removes the record wrapper from a raw crontab command,
// returning the user's original command.
func StripRecord(command string) string {
	// Current format: __lc_out=$({ cmd; } 2>&1); __lc_ec=$?; echo "$__lc_out" | record "name" "$__lc_ec"
	if strings.HasPrefix(command, wrapPrefix) {
		if endIdx := strings.Index(command, wrapEndMarker); endIdx != -1 {
			return strings.TrimSpace(command[len(wrapPrefix):endIdx])
		}
	}

	// Legacy format: { cmd; } 2>&1 | record "name"
	recPath := recordBinPath()
	pipeIdx := strings.LastIndex(command, "| "+recPath+" ")
	if pipeIdx == -1 {
		return command
	}

	inner := strings.TrimSpace(command[:pipeIdx])

	// Strip tee if present (legacy): | tee -a logfile | record
	if teeIdx := strings.LastIndex(inner, "| tee -a "); teeIdx != -1 {
		inner = strings.TrimSpace(inner[:teeIdx])
	}

	// Strip { ...; } 2>&1 wrapper
	if strings.HasPrefix(inner, "{ ") && strings.HasSuffix(inner, "; } 2>&1") {
		inner = inner[len("{ ") : len(inner)-len("; } 2>&1")]
		inner = strings.TrimSpace(inner)
	}

	return inner
}

// IsCurrentFormat reports whether a raw crontab command uses the current
// record wrapper format (as opposed to a legacy or missing wrapper).
func IsCurrentFormat(rawCommand string) bool {
	return strings.HasPrefix(rawCommand, wrapPrefix)
}

// CrontabLine returns the crontab representation of a job,
// including the name comment and optionally the disabled prefix.
func (j Job) CrontabLine() string {
	var b strings.Builder
	nameComment := j.Name
	if j.OneShot {
		nameComment += " @once"
	}
	if j.Tag != "" {
		color := j.TagColor
		if color == "" {
			color = "#f38ba8"
		}
		nameComment += " [" + j.Tag + ":" + color + "]"
	}
	if j.Project != "" {
		nameComment += " {" + j.Project + "}"
	}
	fmt.Fprintf(&b, "# %s\n", nameComment)

	scriptCmd := "sh '" + ScriptPath(j.Name) + "'"
	var wrapped string
	if j.OneShot {
		wrapped = WrapWithRecordOnce(scriptCmd, j.Name)
	} else {
		wrapped = WrapWithRecord(scriptCmd, j.Name)
	}
	if !j.Enabled {
		fmt.Fprintf(&b, "#DISABLED %s %s", j.Schedule, wrapped)
	} else {
		fmt.Fprintf(&b, "%s %s", j.Schedule, wrapped)
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

		// Check if this is a name comment: # job-name [TAG:color]
		if isNameComment(line) {
			name := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			var oneShot bool
			name, oneShot = extractOnce(name)
			project := ""
			name, project = extractProject(name)
			tag, tagColor := "", ""
			name, tag, tagColor = extractTag(name)
			i++
			if i < len(lines) {
				jobLine := strings.TrimSpace(lines[i])
				if jobLine == "" {
					i++
					continue
				}
				if job, ok := parseJobLine(jobLine, name); ok {
					job.Tag = tag
					job.TagColor = tagColor
					job.OneShot = oneShot
					job.Project = project
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

// extractOnce checks for the @once marker in a name comment.
// Returns the clean name (without @once) and whether it was present.
func extractOnce(name string) (string, bool) {
	const marker = " @once"
	if idx := strings.Index(name, marker); idx != -1 {
		// Ensure @once is followed by nothing, whitespace, or [
		after := name[idx+len(marker):]
		after = strings.TrimSpace(after)
		if after == "" || strings.HasPrefix(after, "[") {
			clean := strings.TrimSpace(name[:idx] + " " + after)
			return clean, true
		}
	}
	return name, false
}

// extractTag parses a tag suffix from a name like "Job Name [PP:#f38ba8]".
// Returns the clean name, tag text, and tag color.
func extractTag(name string) (string, string, string) {
	openIdx := strings.LastIndex(name, "[")
	if openIdx == -1 || !strings.HasSuffix(name, "]") {
		return name, "", ""
	}
	inner := name[openIdx+1 : len(name)-1]
	parts := strings.SplitN(inner, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return name, "", ""
	}
	cleanName := strings.TrimSpace(name[:openIdx])
	return cleanName, parts[0], parts[1]
}

// extractProject parses a project suffix from a name like "Job Name {my-project}".
// Returns the clean name and project string.
func extractProject(name string) (string, string) {
	openIdx := strings.LastIndex(name, "{")
	if openIdx == -1 || !strings.HasSuffix(name, "}") {
		return name, ""
	}
	project := name[openIdx+1 : len(name)-1]
	if project == "" {
		return name, ""
	}
	cleanName := strings.TrimSpace(name[:openIdx])
	return cleanName, project
}

func isNameComment(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	if strings.HasPrefix(line, "#DISABLED ") {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	if rest == "" {
		return false
	}
	// Not an env assignment like PATH=...
	if strings.Contains(strings.Fields(rest)[0], "=") {
		return false
	}
	return true
}

func parseJobLine(line, name string) (Job, bool) {
	// Disabled job
	if strings.HasPrefix(line, "#DISABLED ") {
		rest := strings.TrimPrefix(line, "#DISABLED ")
		schedule, rawCmd, ok := splitCronLine(rest)
		if !ok {
			return Job{}, false
		}
		return Job{
			Name:     name,
			Schedule: schedule,
			Command:  resolveScript(StripRecord(rawCmd)),
			Enabled:  false,
			Wrapped:  IsCurrentFormat(rawCmd),
		}, true
	}

	// Skip other comments
	if strings.HasPrefix(line, "#") {
		return Job{}, false
	}

	// Active job
	schedule, rawCmd, ok := splitCronLine(line)
	if !ok {
		return Job{}, false
	}
	return Job{
		Name:     name,
		Schedule: schedule,
		Command:  resolveScript(StripRecord(rawCmd)),
		Enabled:  true,
		Wrapped:  IsCurrentFormat(rawCmd),
	}, true
}

// splitCronLine splits a cron line into schedule (5 fields) and command.
func splitCronLine(line string) (schedule, command string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return "", "", false
	}
	schedule = strings.Join(fields[:5], " ")

	// Walk past the first 5 whitespace-separated fields to preserve original spacing in command.
	pos := 0
	for i := 0; i < 5; i++ {
		for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
			pos++
		}
		for pos < len(line) && line[pos] != ' ' && line[pos] != '\t' {
			pos++
		}
	}
	command = strings.TrimSpace(line[pos:])
	return schedule, command, true
}
