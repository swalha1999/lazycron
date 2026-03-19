package cron

import (
	"strings"
	"testing"
)

// recPath is the record binary path used in test fixtures.
var recPath = recordBinPath()

// helper to build a current-format wrapped command.
func wrapCmd(cmd, name string) string {
	return WrapWithRecord(cmd, name)
}

// helper to build a legacy-format wrapped command.
func legacyWrap(cmd, name string) string {
	return "{ " + cmd + "; } 2>&1 | " + recPath + " " + `"` + name + `"`
}

// --- StripRecord ---

func TestStripRecord_CurrentFormat(t *testing.T) {
	raw := wrapCmd("echo hello", "test-job")
	got := StripRecord(raw)
	if got != "echo hello" {
		t.Errorf("StripRecord(current) = %q, want %q", got, "echo hello")
	}
}

func TestStripRecord_CurrentFormat_ComplexCommand(t *testing.T) {
	cmd := `cd /tmp && df -h / | tail -1 | awk '{print $5}'`
	raw := wrapCmd(cmd, "disk-check")
	got := StripRecord(raw)
	if got != cmd {
		t.Errorf("StripRecord(current complex) = %q, want %q", got, cmd)
	}
}

func TestStripRecord_LegacyFormat(t *testing.T) {
	raw := legacyWrap("echo hello", "test-job")
	got := StripRecord(raw)
	if got != "echo hello" {
		t.Errorf("StripRecord(legacy) = %q, want %q", got, "echo hello")
	}
}

func TestStripRecord_LegacyFormatWithTee(t *testing.T) {
	raw := "{ echo hello; } 2>&1 | tee -a /tmp/log.txt | " + recPath + ` "test-job"`
	got := StripRecord(raw)
	if got != "echo hello" {
		t.Errorf("StripRecord(legacy+tee) = %q, want %q", got, "echo hello")
	}
}

func TestStripRecord_NoWrapper(t *testing.T) {
	raw := "echo hello"
	got := StripRecord(raw)
	if got != "echo hello" {
		t.Errorf("StripRecord(none) = %q, want %q", got, "echo hello")
	}
}

// --- IsCurrentFormat ---

func TestIsCurrentFormat(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"current", wrapCmd("echo hello", "j"), true},
		{"legacy", legacyWrap("echo hello", "j"), false},
		{"no wrapper", "echo hello", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCurrentFormat(tt.raw); got != tt.want {
				t.Errorf("IsCurrentFormat = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- WrapWithRecord / StripRecord roundtrip ---

func TestWrapStripRoundtrip(t *testing.T) {
	commands := []string{
		"echo hello",
		"cd /var/log && tail -f syslog",
		`find /tmp -type f -mtime +7 -delete`,
		`tar -czf ~/backups/weekly-$(date +\%Y\%m\%d).tar.gz ~/Documents`,
		`df -h / | tail -1 | awk '{print $5}' | logger -t disk-usage`,
	}
	for _, cmd := range commands {
		t.Run(cmd[:min(len(cmd), 30)], func(t *testing.T) {
			wrapped := WrapWithRecord(cmd, "test-job")
			got := StripRecord(wrapped)
			if got != cmd {
				t.Errorf("roundtrip failed:\n  in:  %q\n  out: %q", cmd, got)
			}
		})
	}
}

// --- CrontabLine ---

func TestCrontabLine_Enabled(t *testing.T) {
	j := Job{Name: "my-job", Schedule: "0 9 * * *", Command: "echo hi", Enabled: true, Wrapped: true}
	line := j.CrontabLine()

	if !strings.HasPrefix(line, "# my-job\n") {
		t.Errorf("missing name comment: %q", line)
	}
	if !strings.Contains(line, "0 9 * * * "+wrapPrefix) {
		t.Errorf("expected schedule + wrapped command: %q", line)
	}
}

func TestCrontabLine_Disabled(t *testing.T) {
	j := Job{Name: "my-job", Schedule: "0 9 * * *", Command: "echo hi", Enabled: false, Wrapped: true}
	line := j.CrontabLine()

	if !strings.Contains(line, "#DISABLED 0 9 * * * ") {
		t.Errorf("expected #DISABLED prefix: %q", line)
	}
}

// --- Parse ---

func TestParse_CurrentFormat(t *testing.T) {
	input := "# my-job\n* * * * * " + wrapCmd("echo hello", "my-job")
	jobs := Parse(input)

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	assertEqual(t, "Name", j.Name, "my-job")
	assertEqual(t, "Schedule", j.Schedule, "* * * * *")
	assertEqual(t, "Command", j.Command, "echo hello")
	assertBool(t, "Enabled", j.Enabled, true)
	assertBool(t, "Wrapped", j.Wrapped, true)
}

func TestParse_LegacyFormat(t *testing.T) {
	input := "# old-job\n*/5 * * * * " + legacyWrap("echo legacy", "old-job")
	jobs := Parse(input)

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	assertEqual(t, "Name", j.Name, "old-job")
	assertEqual(t, "Schedule", j.Schedule, "*/5 * * * *")
	assertEqual(t, "Command", j.Command, "echo legacy")
	assertBool(t, "Enabled", j.Enabled, true)
	assertBool(t, "Wrapped", j.Wrapped, false)
}

func TestParse_NoWrapper(t *testing.T) {
	input := "# bare-job\n0 3 * * * echo bare"
	jobs := Parse(input)

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	assertEqual(t, "Name", j.Name, "bare-job")
	assertEqual(t, "Command", j.Command, "echo bare")
	assertBool(t, "Wrapped", j.Wrapped, false)
}

func TestParse_Disabled(t *testing.T) {
	input := "# dis-job\n#DISABLED 0 2 * * 0 " + wrapCmd("echo off", "dis-job")
	jobs := Parse(input)

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	assertEqual(t, "Name", j.Name, "dis-job")
	assertEqual(t, "Command", j.Command, "echo off")
	assertBool(t, "Enabled", j.Enabled, false)
	assertBool(t, "Wrapped", j.Wrapped, true)
}

func TestParse_DisabledLegacy(t *testing.T) {
	input := "# old-dis\n#DISABLED */5 * * * * " + legacyWrap("echo old", "old-dis")
	jobs := Parse(input)

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	j := jobs[0]
	assertBool(t, "Enabled", j.Enabled, false)
	assertBool(t, "Wrapped", j.Wrapped, false)
	assertEqual(t, "Command", j.Command, "echo old")
}

func TestParse_AutoName(t *testing.T) {
	input := "* * * * * echo no-comment"
	jobs := Parse(input)

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	assertEqual(t, "Name", jobs[0].Name, "job-1")
}

func TestParse_AutoNameIncrement(t *testing.T) {
	input := "* * * * * echo first\n0 * * * * echo second"
	jobs := Parse(input)

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	assertEqual(t, "Name[0]", jobs[0].Name, "job-1")
	assertEqual(t, "Name[1]", jobs[1].Name, "job-2")
}

func TestParse_SkipsEmptyLines(t *testing.T) {
	input := "\n\n# my-job\n\n* * * * * echo hello\n\n"
	jobs := Parse(input)

	// Name comment followed by empty line means the name is consumed
	// but the empty line makes it skip, then "* * * * * echo hello"
	// is parsed as a standalone job with auto-name.
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	assertEqual(t, "Name", jobs[0].Name, "job-1")
}

func TestParse_SkipsEnvComments(t *testing.T) {
	input := "# PATH=/usr/bin\n* * * * * echo hello"
	jobs := Parse(input)

	// "# PATH=/usr/bin" has = in first word, so not a name comment.
	// "* * * * * echo hello" is a standalone job.
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	assertEqual(t, "Name", jobs[0].Name, "job-1")
}

func TestParse_SkipsBareComments(t *testing.T) {
	input := "#\n* * * * * echo hello"
	jobs := Parse(input)

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	assertEqual(t, "Name", jobs[0].Name, "job-1")
}

func TestParse_MultipleJobs(t *testing.T) {
	input := "# job-a\n0 9 * * * " + wrapCmd("echo a", "job-a") +
		"\n# job-b\n0 17 * * * " + legacyWrap("echo b", "job-b") +
		"\n0 0 * * * echo c"
	jobs := Parse(input)

	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	assertEqual(t, "jobs[0].Name", jobs[0].Name, "job-a")
	assertBool(t, "jobs[0].Wrapped", jobs[0].Wrapped, true)

	assertEqual(t, "jobs[1].Name", jobs[1].Name, "job-b")
	assertBool(t, "jobs[1].Wrapped", jobs[1].Wrapped, false)

	assertEqual(t, "jobs[2].Name", jobs[2].Name, "job-1")
	assertBool(t, "jobs[2].Wrapped", jobs[2].Wrapped, false)
}

func TestParse_EmptyInput(t *testing.T) {
	jobs := Parse("")
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestParse_TooFewFields(t *testing.T) {
	input := "# bad\n* * * * not-enough"
	jobs := Parse(input)
	// 5 fields total = schedule (5) + 0 command fields → rejected
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs for too-few fields, got %d", len(jobs))
	}
}

// --- splitCronLine ---

func TestSplitCronLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantSch  string
		wantCmd  string
		wantOk   bool
	}{
		{
			"simple",
			"0 9 * * * echo hello",
			"0 9 * * *", "echo hello", true,
		},
		{
			"tabs",
			"0\t9\t*\t*\t*\techo hello",
			"0 9 * * *", "echo hello", true,
		},
		{
			"extra spaces",
			"0  9  *  *  *  echo   hello   world",
			"0 9 * * *", "echo   hello   world", true,
		},
		{
			"too few fields",
			"0 9 * *",
			"", "", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sch, cmd, ok := splitCronLine(tt.line)
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if !ok {
				return
			}
			assertEqual(t, "schedule", sch, tt.wantSch)
			assertEqual(t, "command", cmd, tt.wantCmd)
		})
	}
}

// --- isNameComment ---

func TestIsNameComment(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"# my-job", true},
		{"# multi word name", true},
		{"#DISABLED * * * * * echo hi", false},
		{"# PATH=/usr/bin", false},
		{"#", false},
		{"# ", false},
		{"not a comment", false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := isNameComment(tt.line); got != tt.want {
				t.Errorf("isNameComment(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

// --- Full roundtrip: Parse → CrontabLine → Parse ---

func TestFullRoundtrip(t *testing.T) {
	original := Job{
		Name:     "roundtrip-test",
		Schedule: "30 14 * * 1-5",
		Command:  `cd /app && ./deploy.sh --env=prod`,
		Enabled:  true,
		Wrapped:  true,
	}

	// Serialize
	line := original.CrontabLine()

	// Parse back
	jobs := Parse(line)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job after roundtrip, got %d", len(jobs))
	}

	got := jobs[0]
	assertEqual(t, "Name", got.Name, original.Name)
	assertEqual(t, "Schedule", got.Schedule, original.Schedule)
	assertEqual(t, "Command", got.Command, original.Command)
	assertBool(t, "Enabled", got.Enabled, original.Enabled)
	assertBool(t, "Wrapped", got.Wrapped, true)
}

func TestFullRoundtrip_Disabled(t *testing.T) {
	original := Job{
		Name:     "disabled-roundtrip",
		Schedule: "0 3 * * *",
		Command:  "echo sleeping",
		Enabled:  false,
		Wrapped:  true,
	}

	line := original.CrontabLine()
	jobs := Parse(line)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	got := jobs[0]
	assertEqual(t, "Name", got.Name, original.Name)
	assertEqual(t, "Command", got.Command, original.Command)
	assertBool(t, "Enabled", got.Enabled, false)
	assertBool(t, "Wrapped", got.Wrapped, true)
}

// --- helpers ---

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertBool(t *testing.T, field string, got, want bool) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}

