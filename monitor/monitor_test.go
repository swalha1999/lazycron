package monitor

import (
	"fmt"
	"testing"
	"time"
)

func TestParseElapsedTime(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"5", 5 * time.Second, false},
		{"30", 30 * time.Second, false},
		{"01:30", 90 * time.Second, false},
		{"05:45", 5*time.Minute + 45*time.Second, false},
		{"01:30:45", 1*time.Hour + 30*time.Minute + 45*time.Second, false},
		{"12:00:00", 12 * time.Hour, false},
		{"2-03:30:45", 2*24*time.Hour + 3*time.Hour + 30*time.Minute + 45*time.Second, false},
		{"invalid", 0, true},
		{"01:02:03:04", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseElapsedTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseElapsedTime(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseElapsedTime(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.expected {
				t.Errorf("parseElapsedTime(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseRunningJobs(t *testing.T) {
	psOutput := `  PID     ELAPSED STAT  %CPU %MEM   RSS COMMAND
 1234       00:05 S      0.0  0.1  1024 sh /home/user/.lazycron/scripts/abc123.sh
 5678       01:30 R      2.5  0.5  4096 sh '/Users/admin/.lazycron/scripts/def456.sh'
 9012       12:45:30 S    0.0  0.0   512 /bin/bash
 3456    2-03:30:00 S    1.0  1.2 12288 sh /home/user/.lazycron/scripts/abc123.sh
 7890       00:01 S      0.0  0.0   256 grep lazycron`

	jobs, err := ParseRunningJobs(psOutput, "/home/user/.lazycron/scripts")
	if err != nil {
		t.Fatalf("ParseRunningJobs() error: %v", err)
	}

	if len(jobs) != 3 {
		t.Fatalf("parseRunningJobs() returned %d jobs, want 3", len(jobs))
	}

	// Check first job
	if jobs[0].PID != 1234 {
		t.Errorf("jobs[0].PID = %d, want 1234", jobs[0].PID)
	}
	if jobs[0].JobID != "abc123" {
		t.Errorf("jobs[0].JobID = %q, want %q", jobs[0].JobID, "abc123")
	}
	if jobs[0].Elapsed != 5*time.Second {
		t.Errorf("jobs[0].Elapsed = %v, want 5s", jobs[0].Elapsed)
	}

	// Check second job (with quotes in path)
	if jobs[1].PID != 5678 {
		t.Errorf("jobs[1].PID = %d, want 5678", jobs[1].PID)
	}
	if jobs[1].JobID != "def456" {
		t.Errorf("jobs[1].JobID = %q, want %q", jobs[1].JobID, "def456")
	}
	if jobs[1].Elapsed != 90*time.Second {
		t.Errorf("jobs[1].Elapsed = %v, want 1m30s", jobs[1].Elapsed)
	}

	// Check third job (with days)
	if jobs[2].PID != 3456 {
		t.Errorf("jobs[2].PID = %d, want 3456", jobs[2].PID)
	}
	expected := 2*24*time.Hour + 3*time.Hour + 30*time.Minute
	if jobs[2].Elapsed != expected {
		t.Errorf("jobs[2].Elapsed = %v, want %v", jobs[2].Elapsed, expected)
	}
}

func TestParseRunningJobs_EmptyOutput(t *testing.T) {
	jobs, err := ParseRunningJobs("", "/home/user/.lazycron/scripts")
	if err != nil {
		t.Fatalf("ParseRunningJobs() error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("ParseRunningJobs() returned %d jobs, want 0", len(jobs))
	}
}

func TestParseRunningJobs_NoMatches(t *testing.T) {
	psOutput := `  PID     ELAPSED STAT  %CPU %MEM   RSS COMMAND
 1234       00:05 S      0.0  0.0   512 /bin/bash
 5678       01:30 S      0.0  0.0   256 vim test.txt`

	jobs, err := ParseRunningJobs(psOutput, "/home/user/.lazycron/scripts")
	if err != nil {
		t.Fatalf("ParseRunningJobs() error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("ParseRunningJobs() returned %d jobs, want 0", len(jobs))
	}
}

func TestGroupByJob(t *testing.T) {
	jobs := []RunningJob{
		{PID: 1234, JobID: "abc123", Elapsed: 5 * time.Second},
		{PID: 5678, JobID: "def456", Elapsed: 90 * time.Second},
		{PID: 9012, JobID: "abc123", Elapsed: 120 * time.Second},
		{PID: 3456, JobID: "abc123", Elapsed: 180 * time.Second},
		{PID: 7890, JobID: "xyz789", Elapsed: 10 * time.Second},
	}

	groups := GroupByJob(jobs)

	// Should have 3 unique jobs
	if len(groups) != 3 {
		t.Fatalf("GroupByJob() returned %d groups, want 3", len(groups))
	}

	// Find abc123 group
	var abc123Group *JobGroup
	for i := range groups {
		if groups[i].JobID == "abc123" {
			abc123Group = &groups[i]
			break
		}
	}

	if abc123Group == nil {
		t.Fatal("abc123 group not found")
	}

	if len(abc123Group.Instances) != 3 {
		t.Errorf("abc123 group has %d instances, want 3", len(abc123Group.Instances))
	}
}

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{5*time.Minute + 45*time.Second, "5m 45s"},
		{1*time.Hour + 30*time.Minute, "1h 30m"},
		{12 * time.Hour, "12h 0m"},
		{25 * time.Hour, "1d 1h"},
		{2*24*time.Hour + 3*time.Hour, "2d 3h"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatElapsed(tt.duration)
			if got != tt.expected {
				t.Errorf("FormatElapsed(%v) = %q, want %q", tt.duration, got, tt.expected)
			}
		})
	}
}

func TestKillJob(t *testing.T) {
	// Mock killCommand
	called := false
	var calledPID int
	oldKillCommand := killCommand
	defer func() { killCommand = oldKillCommand }()

	killCommand = func(pid int) error {
		called = true
		calledPID = pid
		return nil
	}

	err := KillJob(1234)
	if err != nil {
		t.Errorf("KillJob(1234) error: %v", err)
	}
	if !called {
		t.Error("killCommand was not called")
	}
	if calledPID != 1234 {
		t.Errorf("killCommand called with pid=%d, want 1234", calledPID)
	}
}

func TestKillJob_Error(t *testing.T) {
	// Mock killCommand to return error
	oldKillCommand := killCommand
	defer func() { killCommand = oldKillCommand }()

	killCommand = func(pid int) error {
		return fmt.Errorf("process not found")
	}

	err := KillJob(9999)
	if err == nil {
		t.Error("KillJob(9999) expected error, got nil")
	}
}

func TestGetRunningJobs_Integration(t *testing.T) {
	// Mock psCommand
	oldPsCommand := psCommand
	defer func() { psCommand = oldPsCommand }()

	psCommand = func() (string, error) {
		return `  PID     ELAPSED STAT  %CPU %MEM   RSS COMMAND
 1234       00:05 S      0.5  0.2  2048 sh /home/user/.lazycron/scripts/test123.sh
 5678       01:30 R      1.2  0.3  3072 sh /home/user/.lazycron/scripts/test456.sh`, nil
	}

	jobs, err := GetRunningJobs("/home/user/.lazycron/scripts")
	if err != nil {
		t.Fatalf("GetRunningJobs() error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("GetRunningJobs() returned %d jobs, want 2", len(jobs))
	}
}
