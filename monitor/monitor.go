package monitor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RunningJob represents a single running job instance.
type RunningJob struct {
	PID       int           // Process ID
	JobID     string        // Job ID extracted from script path
	JobName   string        // Job name (will be populated by caller)
	Elapsed   time.Duration // How long the job has been running
	StartTime time.Time     // When the job started
	CPU       string        // CPU usage percentage
	Memory    string        // Memory usage (RSS)
	State     string        // Process state (R, S, D, etc.)
	Command   string        // Full command line
}

// JobGroup represents multiple instances of the same job.
type JobGroup struct {
	JobID     string
	JobName   string
	Instances []RunningJob
}

// psCommand is a variable to allow test overrides.
var psCommand = func() (string, error) {
	cmd := exec.Command("ps", "-eo", "pid,etime,state,%cpu,%mem,rss,command")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// killCommand is a variable to allow test overrides.
var killCommand = func(pid int) error {
	cmd := exec.Command("kill", strconv.Itoa(pid))
	return cmd.Run()
}

// GetRunningJobs detects all currently running lazycron jobs on the local system.
// It returns a slice of RunningJob instances.
func GetRunningJobs(scriptsDir string) ([]RunningJob, error) {
	output, err := psCommand()
	if err != nil {
		return nil, fmt.Errorf("ps command failed: %w", err)
	}

	return ParseRunningJobs(output, scriptsDir)
}

// ParseRunningJobs parses ps output and extracts running lazycron jobs.
// This function is pure and easily testable. It's exported so remote backends can use it.
func ParseRunningJobs(psOutput, scriptsDir string) ([]RunningJob, error) {
	lines := strings.Split(psOutput, "\n")

	// Pattern to match lazycron script execution
	// Matches: sh /path/to/.lazycron/scripts/{jobID}.sh
	// OR: sh '/path/to/.lazycron/scripts/{jobID}.sh'
	pattern := regexp.MustCompile(`sh\s+'?([^']+\.lazycron/scripts/([^/]+)\.sh)'?`)

	var jobs []RunningJob
	now := time.Now()

	for _, line := range lines {
		// Skip header and empty lines
		if strings.TrimSpace(line) == "" || strings.Contains(line, "PID") {
			continue
		}

		// Parse ps output: PID ELAPSED STATE %CPU %MEM RSS COMMAND
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		// Extract PID
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		// Extract elapsed time (format: [[dd-]hh:]mm:ss)
		elapsed, err := parseElapsedTime(fields[1])
		if err != nil {
			continue
		}

		// Extract state, CPU, memory
		state := fields[2]
		cpu := fields[3]
		// memPct := fields[4] // We use RSS instead of %MEM
		rss := fields[5]

		// Convert RSS (KB) to human readable format
		rssKB, _ := strconv.ParseFloat(rss, 64)
		memStr := formatMemory(rssKB)

		// Join remaining fields as command
		command := strings.Join(fields[6:], " ")

		// Check if this is a lazycron script
		matches := pattern.FindStringSubmatch(command)
		if matches == nil {
			continue
		}

		// Skip wrapper processes (record script wrappers that contain the script path but aren't the actual script)
		// These have __lc_out= in their command line
		if strings.Contains(command, "__lc_out=") {
			continue
		}

		// Extract job ID from script path
		jobID := matches[2]

		jobs = append(jobs, RunningJob{
			PID:       pid,
			JobID:     jobID,
			Elapsed:   elapsed,
			StartTime: now.Add(-elapsed),
			CPU:       cpu,
			Memory:    memStr,
			State:     state,
			Command:   command,
		})
	}

	return jobs, nil
}

// formatMemory converts KB to human-readable format.
func formatMemory(kb float64) string {
	if kb < 1024 {
		return fmt.Sprintf("%.0fK", kb)
	}
	mb := kb / 1024
	if mb < 1024 {
		return fmt.Sprintf("%.1fM", mb)
	}
	gb := mb / 1024
	return fmt.Sprintf("%.2fG", gb)
}

// parseElapsedTime converts ps etime format to time.Duration.
// Formats: ss, mm:ss, hh:mm:ss, dd-hh:mm:ss
func parseElapsedTime(etime string) (time.Duration, error) {
	var days, hours, minutes, seconds int

	// Check for days (dd-hh:mm:ss)
	if strings.Contains(etime, "-") {
		parts := strings.Split(etime, "-")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid etime format with days: %s", etime)
		}
		var err error
		days, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
		etime = parts[1]
	}

	// Parse time components (hh:mm:ss or mm:ss or ss)
	timeParts := strings.Split(etime, ":")
	switch len(timeParts) {
	case 1: // ss
		var err error
		seconds, err = strconv.Atoi(timeParts[0])
		if err != nil {
			return 0, err
		}
	case 2: // mm:ss
		var err error
		minutes, err = strconv.Atoi(timeParts[0])
		if err != nil {
			return 0, err
		}
		seconds, err = strconv.Atoi(timeParts[1])
		if err != nil {
			return 0, err
		}
	case 3: // hh:mm:ss
		var err error
		hours, err = strconv.Atoi(timeParts[0])
		if err != nil {
			return 0, err
		}
		minutes, err = strconv.Atoi(timeParts[1])
		if err != nil {
			return 0, err
		}
		seconds, err = strconv.Atoi(timeParts[2])
		if err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("invalid etime format: %s", etime)
	}

	total := time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second

	return total, nil
}

// GroupByJob groups running jobs by JobID.
func GroupByJob(jobs []RunningJob) []JobGroup {
	groups := make(map[string]*JobGroup)

	for _, job := range jobs {
		group, exists := groups[job.JobID]
		if !exists {
			group = &JobGroup{
				JobID:     job.JobID,
				JobName:   job.JobName,
				Instances: []RunningJob{},
			}
			groups[job.JobID] = group
		}
		group.Instances = append(group.Instances, job)
	}

	// Convert map to slice
	result := make([]JobGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, *group)
	}

	return result
}

// KillJob kills a running job by PID.
func KillJob(pid int) error {
	return killCommand(pid)
}

// FormatElapsed formats a duration in a human-readable way.
func FormatElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// GetScriptsDir returns the expected scripts directory path.
// This is used to filter ps output for lazycron jobs.
func GetScriptsDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/root"
	}
	return filepath.Join(home, ".lazycron", "scripts")
}
