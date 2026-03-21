package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFakeCrontab replaces runCrontab for the duration of a test.
// The fake receives stdin and args, returns output and error.
func withFakeCrontab(t *testing.T, fake func(stdin string, args ...string) (string, error)) {
	t.Helper()
	original := runCrontab
	runCrontab = fake
	t.Cleanup(func() { runCrontab = original })
}

// withFakeShell replaces runShell for the duration of a test.
func withFakeShell(t *testing.T, fake func(command string) (string, error)) {
	t.Helper()
	original := runShell
	runShell = fake
	t.Cleanup(func() { runShell = original })
}

// withFakeLookPath replaces lookPath for the duration of a test.
func withFakeLookPath(t *testing.T, fake func(file string) (string, error)) {
	t.Helper()
	original := lookPath
	lookPath = fake
	t.Cleanup(func() { lookPath = original })
}

// withFakeScriptsDir replaces scriptsDir with a temp directory for the duration of a test.
// The path includes ".lazycron/scripts" so IsScriptRef detection works correctly.
func withFakeScriptsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".lazycron", "scripts")
	os.MkdirAll(dir, 0755)
	original := scriptsDir
	scriptsDir = func() string { return dir }
	t.Cleanup(func() { scriptsDir = original })
	return dir
}

// --- ReadCrontab ---

func TestReadCrontab_Success(t *testing.T) {
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		return "0 9 * * * echo hello\n", nil
	})

	out, err := ReadCrontab()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "0 9 * * * echo hello\n" {
		t.Errorf("output = %q, want crontab content", out)
	}
}

func TestReadCrontab_NoCrontab(t *testing.T) {
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		return "no crontab for testuser", fmt.Errorf("exit status 1")
	})

	out, err := ReadCrontab()
	if err != nil {
		t.Fatalf("'no crontab' should not be an error, got: %v", err)
	}
	if out != "" {
		t.Errorf("output should be empty, got %q", out)
	}
}

func TestReadCrontab_OtherError(t *testing.T) {
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		return "permission denied", fmt.Errorf("exit status 1")
	})

	_, err := ReadCrontab()
	if err == nil {
		t.Fatal("expected error for permission denied")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error should contain 'permission denied', got %q", err.Error())
	}
}

func TestReadCrontab_PassesCorrectArgs(t *testing.T) {
	var gotArgs []string
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		gotArgs = args
		return "", nil
	})

	ReadCrontab()
	if len(gotArgs) != 1 || gotArgs[0] != "-l" {
		t.Errorf("expected args [-l], got %v", gotArgs)
	}
}

// --- WriteCrontab ---

func TestWriteCrontab_Success(t *testing.T) {
	withFakeScriptsDir(t)
	var gotStdin string
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		gotStdin = stdin
		return "", nil
	})

	jobs := []Job{
		{Name: "test-job", Schedule: "0 9 * * *", Command: "echo hi", Enabled: true, Wrapped: true},
	}
	err := WriteCrontab(jobs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stdin should contain formatted crontab content
	if !strings.Contains(gotStdin, "# test-job") {
		t.Errorf("stdin should contain job name comment, got %q", gotStdin)
	}
	if !strings.Contains(gotStdin, "0 9 * * *") {
		t.Errorf("stdin should contain schedule, got %q", gotStdin)
	}
}

func TestWriteCrontab_Error(t *testing.T) {
	withFakeScriptsDir(t)
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		return "bad crontab", fmt.Errorf("exit status 1")
	})

	err := WriteCrontab([]Job{{Name: "j", Schedule: "* * * * *", Command: "echo", Enabled: true}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad crontab") {
		t.Errorf("error should contain output, got %q", err.Error())
	}
}

func TestWriteCrontab_PassesCorrectArgs(t *testing.T) {
	withFakeScriptsDir(t)
	var gotArgs []string
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		gotArgs = args
		return "", nil
	})

	WriteCrontab(nil)
	if len(gotArgs) != 1 || gotArgs[0] != "-" {
		t.Errorf("expected args [-], got %v", gotArgs)
	}
}

// --- FormatCrontab ---

func TestFormatCrontab_SingleJob(t *testing.T) {
	withFakeScriptsDir(t)
	jobs := []Job{
		{Name: "my-job", Schedule: "0 9 * * *", Command: "echo hi", Enabled: true, Wrapped: true},
	}
	got := FormatCrontab(jobs)

	if !strings.Contains(got, "# my-job") {
		t.Errorf("should contain name comment: %q", got)
	}
	if !strings.Contains(got, "0 9 * * *") {
		t.Errorf("should contain schedule: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("should end with newline: %q", got)
	}
}

func TestFormatCrontab_MultipleJobs(t *testing.T) {
	withFakeScriptsDir(t)
	jobs := []Job{
		{Name: "job-a", Schedule: "0 9 * * *", Command: "echo a", Enabled: true, Wrapped: true},
		{Name: "job-b", Schedule: "0 17 * * *", Command: "echo b", Enabled: true, Wrapped: true},
	}
	got := FormatCrontab(jobs)

	if strings.Count(got, "# job-a") != 1 || strings.Count(got, "# job-b") != 1 {
		t.Errorf("should contain both job names: %q", got)
	}

	// Jobs should be separated by a newline
	aIdx := strings.Index(got, "# job-a")
	bIdx := strings.Index(got, "# job-b")
	if aIdx >= bIdx {
		t.Errorf("job-a should appear before job-b: %q", got)
	}
}

func TestFormatCrontab_Empty(t *testing.T) {
	got := FormatCrontab(nil)
	if got != "\n" {
		t.Errorf("empty jobs should produce just a newline, got %q", got)
	}
}

func TestFormatCrontab_DisabledJob(t *testing.T) {
	withFakeScriptsDir(t)
	jobs := []Job{
		{Name: "off-job", Schedule: "0 3 * * *", Command: "echo off", Enabled: false, Wrapped: true},
	}
	got := FormatCrontab(jobs)

	if !strings.Contains(got, "#DISABLED") {
		t.Errorf("disabled job should have #DISABLED prefix: %q", got)
	}
}

// --- CheckCrontabAvailable ---

func TestCheckCrontabAvailable_Found(t *testing.T) {
	withFakeLookPath(t, func(file string) (string, error) {
		return "/usr/bin/crontab", nil
	})

	err := CheckCrontabAvailable()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckCrontabAvailable_NotFound(t *testing.T) {
	withFakeLookPath(t, func(file string) (string, error) {
		return "", fmt.Errorf("not found")
	})

	err := CheckCrontabAvailable()
	if err == nil {
		t.Fatal("expected error when crontab not in PATH")
	}
	if !strings.Contains(err.Error(), "crontab command not found") {
		t.Errorf("error should mention crontab not found, got %q", err.Error())
	}
}

// --- RunJobNow ---

func TestRunJobNow_Success(t *testing.T) {
	withFakeScriptsDir(t)
	withFakeShell(t, func(command string) (string, error) {
		return "hello world", nil
	})

	out, err := RunJobNow("test-job", "echo hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("output = %q, want %q", out, "hello world")
	}
}

func TestRunJobNow_Failure(t *testing.T) {
	withFakeScriptsDir(t)
	withFakeShell(t, func(command string) (string, error) {
		return "command not found", fmt.Errorf("exit status 127")
	})

	out, err := RunJobNow("test-job", "nonexistent-command")
	if err == nil {
		t.Fatal("expected error")
	}
	if out != "command not found" {
		t.Errorf("output = %q, want error output", out)
	}
}

func TestRunJobNow_CreatesScriptIfMissing(t *testing.T) {
	dir := withFakeScriptsDir(t)
	var gotCommand string
	withFakeShell(t, func(command string) (string, error) {
		gotCommand = command
		return "", nil
	})

	RunJobNow("my-job", "cd /tmp && ls -la")

	// Should run via quoted script path
	expectedPath := "sh '" + dir + "/my-job.sh'"
	if gotCommand != expectedPath {
		t.Errorf("command = %q, want %q", gotCommand, expectedPath)
	}

	// Script file should exist with the command
	content, err := ReadScriptCommand(dir + "/my-job.sh")
	if err != nil {
		t.Fatalf("script file not found: %v", err)
	}
	if content != "cd /tmp && ls -la" {
		t.Errorf("script content = %q, want %q", content, "cd /tmp && ls -la")
	}
}

func TestRunJobNow_DoesNotOverwriteExistingScript(t *testing.T) {
	dir := withFakeScriptsDir(t)
	withFakeShell(t, func(command string) (string, error) {
		return "", nil
	})

	// Write a script with the "correct" command
	WriteScript("my-job", "echo correct")

	// Run with a "stale" command — should NOT overwrite
	RunJobNow("my-job", "echo stale")

	content, _ := ReadScriptCommand(dir + "/my-job.sh")
	if content != "echo correct" {
		t.Errorf("script was overwritten: got %q, want %q", content, "echo correct")
	}
}
