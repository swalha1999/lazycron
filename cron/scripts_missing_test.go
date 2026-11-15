package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseWithMissingScript writes job's crontab line for scripts in dir,
// parses it back without the script on disk and returns the parsed job.
func parseWithMissingScript(t *testing.T, dir string, job Job) Job {
	t.Helper()
	line := RelocateCrontab(job.CrontabLine(), dir, "")
	jobs := ParseIn(line, dir)
	if len(jobs) != 1 {
		t.Fatalf("parsed %d jobs, want 1", len(jobs))
	}
	return jobs[0]
}

func TestParse_MissingScriptIsFlagged(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".lazycron", "scripts")
	for _, runner := range []string{"plain", "herdr"} {
		job := Job{ID: "abc12345", Name: "j", Schedule: "* * * * *", Command: "echo hi", Enabled: true, Runner: runner}
		got := parseWithMissingScript(t, dir, job)
		if !got.ScriptMissing() {
			t.Errorf("runner %s: ScriptMissing() = false for command %q", runner, got.Command)
		}
	}
}

func TestParse_ReadableScriptIsNotFlagged(t *testing.T) {
	dir := withFakeScriptsDir(t)
	if err := WriteScriptIn(dir, "abc12345", "echo hi"); err != nil {
		t.Fatal(err)
	}
	job := Job{ID: "abc12345", Name: "j", Schedule: "* * * * *", Command: "echo hi", Enabled: true}
	got := ParseIn(job.CrontabLine(), dir)[0]
	if got.ScriptMissing() || got.Command != "echo hi" {
		t.Errorf("Command = %q, ScriptMissing = %v", got.Command, got.ScriptMissing())
	}
}

// Regression for #130: a job whose script is missing must never get a
// script that runs itself (a fork bomb on every cron tick).
func TestSyncScriptsIn_MissingScriptNotSelfReferenced(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".lazycron", "scripts")
	job := parseWithMissingScript(t, dir, Job{ID: "abc12345", Name: "j", Schedule: "* * * * *", Command: "echo hi", Enabled: true})
	other := Job{ID: "def67890", Name: "k", Schedule: "* * * * *", Command: "echo ok", Enabled: true}

	if err := SyncScriptsIn(dir, []Job{job, other}); err != nil {
		t.Fatalf("SyncScriptsIn: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "abc12345.sh")); !os.IsNotExist(err) {
		t.Errorf("missing script was written (err=%v)", err)
	}
	if got, _ := ReadScriptCommand(filepath.Join(dir, "def67890.sh")); got != "echo ok" {
		t.Errorf("other job's script = %q, want %q", got, "echo ok")
	}
}

func TestSyncScriptsIn_KeepsUnreadableScriptFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".lazycron", "scripts")
	if err := WriteScriptIn(dir, "abc12345", "echo original"); err != nil {
		t.Fatal(err)
	}
	ref := Job{ID: "abc12345", Command: "bash '" + filepath.Join(dir, "abc12345.sh") + "'"}

	if err := SyncScriptsIn(dir, []Job{ref}); err != nil {
		t.Fatalf("SyncScriptsIn: %v", err)
	}
	got, err := ReadScriptCommand(filepath.Join(dir, "abc12345.sh"))
	if err != nil || got != "echo original" {
		t.Errorf("script = %q (err %v), want it left as %q", got, err, "echo original")
	}
	if strings.Contains(got, "abc12345.sh") {
		t.Error("script references itself")
	}
}
