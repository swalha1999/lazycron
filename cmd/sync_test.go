package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

// --- readSandcastleJobs ---

func TestReadSandcastleJobs_Valid(t *testing.T) {
	dir := t.TempDir()
	writeTS(t, dir, "fix-agent.ts", `export const cron = "0 9 * * 1-5";
export const name = "Fix Agent";
export const tag = "BP";
export const tagColor = "#f38ba8";
// rest of agent...
`)

	jobs, err := readSandcastleJobs(dir, "myproject")
	if err != nil {
		t.Fatalf("readSandcastleJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	j := jobs[0]
	if j.ID != "fix-agent" {
		t.Errorf("ID = %q, want fix-agent", j.ID)
	}
	if j.Name != "Fix Agent" {
		t.Errorf("Name = %q", j.Name)
	}
	if j.Schedule != "0 9 * * 1-5" {
		t.Errorf("Schedule = %q", j.Schedule)
	}
	if j.Command != ".sandcastle/node_modules/.bin/tsx --env-file-if-exists=.sandcastle/.env .sandcastle/jobs/fix-agent.ts" {
		t.Errorf("Command = %q", j.Command)
	}
	if j.Project != "myproject" {
		t.Errorf("Project = %q", j.Project)
	}
	if j.Tag != "BP" || j.TagColor != "#f38ba8" {
		t.Errorf("Tag/Color = %q/%q", j.Tag, j.TagColor)
	}
}

func TestReadSandcastleJobs_MultipleSorted(t *testing.T) {
	dir := t.TempDir()
	writeTS(t, dir, "zebra.ts", `export const cron = "0 1 * * *";
export const name = "Z";`)
	writeTS(t, dir, "apple.ts", `export const cron = "0 2 * * *";
export const name = "A";`)

	jobs, err := readSandcastleJobs(dir, "p")
	if err != nil {
		t.Fatalf("readSandcastleJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(jobs))
	}
	if jobs[0].ID != "apple" || jobs[1].ID != "zebra" {
		t.Errorf("expected sorted order; got %s, %s", jobs[0].ID, jobs[1].ID)
	}
}

func TestReadSandcastleJobs_MissingMetadataFails(t *testing.T) {
	dir := t.TempDir()
	writeTS(t, dir, "bad.ts", `// no exports here, just code
console.log("hello");`)

	_, err := readSandcastleJobs(dir, "p")
	if err == nil {
		t.Fatal("expected error for missing metadata, got nil")
	}
	if !strings.Contains(err.Error(), "bad.ts") {
		t.Errorf("error should reference filename, got: %v", err)
	}
}

func TestReadSandcastleJobs_InvalidCronFails(t *testing.T) {
	dir := t.TempDir()
	writeTS(t, dir, "broken-cron.ts", `export const cron = "not a cron expression";
export const name = "Broken";`)

	_, err := readSandcastleJobs(dir, "p")
	if err == nil {
		t.Fatal("expected error for invalid cron, got nil")
	}
}

func TestReadSandcastleJobs_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	jobs, err := readSandcastleJobs(dir, "p")
	if err != nil {
		t.Fatalf("readSandcastleJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

// --- remoteProjectPath ---

func TestRemoteProjectPath(t *testing.T) {
	if got := remoteProjectPath("", "ignored", "/cwd/here"); got != "/cwd/here" {
		t.Errorf("local mode: got %q, want /cwd/here", got)
	}
	if got := remoteProjectPath("vm1", "myproj", "/anything"); got != "~/.lazycron/projects/myproj" {
		t.Errorf("remote mode: got %q", got)
	}
}

// --- mergeJobs ---

func TestMergeJobs_AllNew(t *testing.T) {
	existing := []cron.Job{}
	incoming := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "echo backup"},
		{ID: "log-rotate", Name: "Log Rotate", Schedule: "0 0 * * 0", Command: "echo rotate"},
	}

	merged, added, updated, unchanged := mergeJobs(existing, incoming)
	if added != 2 || updated != 0 || unchanged != 0 {
		t.Errorf("counts = %d/%d/%d, want 2/0/0", added, updated, unchanged)
	}
	if len(merged) != 2 {
		t.Errorf("merged len = %d, want 2", len(merged))
	}
}

func TestMergeJobs_AllUnchanged(t *testing.T) {
	jobs := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "echo backup", Enabled: true},
	}
	incoming := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "echo backup", Enabled: true},
	}

	_, added, updated, unchanged := mergeJobs(jobs, incoming)
	if added != 0 || updated != 0 || unchanged != 1 {
		t.Errorf("counts = %d/%d/%d, want 0/0/1", added, updated, unchanged)
	}
}

func TestMergeJobs_SomeUpdated(t *testing.T) {
	existing := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "echo backup", Enabled: true},
	}
	incoming := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 4 * * *", Command: "echo backup", Enabled: true},
	}

	merged, added, updated, unchanged := mergeJobs(existing, incoming)
	if added != 0 || updated != 1 || unchanged != 0 {
		t.Errorf("counts = %d/%d/%d, want 0/1/0", added, updated, unchanged)
	}
	if merged[0].Schedule != "0 4 * * *" {
		t.Errorf("schedule not updated: %q", merged[0].Schedule)
	}
}

func TestMergeJobs_ExistingPreserved(t *testing.T) {
	existing := []cron.Job{
		{ID: "abc12345", Name: "TUI Job", Schedule: "* * * * *", Command: "echo tui", Enabled: true},
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "echo backup", Enabled: true},
	}
	incoming := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "echo backup", Enabled: true},
	}

	merged, added, updated, unchanged := mergeJobs(existing, incoming)
	if added != 0 || updated != 0 || unchanged != 1 {
		t.Errorf("counts = %d/%d/%d, want 0/0/1", added, updated, unchanged)
	}
	if len(merged) != 2 {
		t.Fatalf("merged len = %d, want 2", len(merged))
	}
	if merged[0].ID != "abc12345" {
		t.Errorf("existing job not preserved: %q", merged[0].ID)
	}
}

func TestMergeJobs_MixedAddUpdateUnchanged(t *testing.T) {
	existing := []cron.Job{
		{ID: "unchanged-job", Name: "Same", Schedule: "* * * * *", Command: "echo same", Enabled: true},
		{ID: "update-me", Name: "Old Name", Schedule: "0 3 * * *", Command: "echo old", Enabled: true},
	}
	incoming := []cron.Job{
		{ID: "unchanged-job", Name: "Same", Schedule: "* * * * *", Command: "echo same", Enabled: true},
		{ID: "update-me", Name: "New Name", Schedule: "0 3 * * *", Command: "echo old", Enabled: true},
		{ID: "brand-new", Name: "New Job", Schedule: "0 0 * * *", Command: "echo new", Enabled: true},
	}

	merged, added, updated, unchanged := mergeJobs(existing, incoming)
	if added != 1 || updated != 1 || unchanged != 1 {
		t.Errorf("counts = %d/%d/%d, want 1/1/1", added, updated, unchanged)
	}
	if len(merged) != 3 {
		t.Fatalf("merged len = %d, want 3", len(merged))
	}
}

// --- jobNeedsUpdate ---

func TestJobNeedsUpdate_NoChange(t *testing.T) {
	j := cron.Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true}
	if jobNeedsUpdate(j, j) {
		t.Error("identical jobs should not need update")
	}
}

func TestJobNeedsUpdate_ScheduleChange(t *testing.T) {
	a := cron.Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true}
	b := cron.Job{Name: "A", Schedule: "0 3 * * *", Command: "echo", Enabled: true}
	if !jobNeedsUpdate(a, b) {
		t.Error("different schedule should need update")
	}
}

// --- helpers ---

func writeTS(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
