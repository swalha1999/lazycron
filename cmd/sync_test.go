package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

// --- readAgentJobs ---

// agentJob returns a minimal agent job file with the given front matter lines.
func agentJob(frontMatter string) string {
	return "---\n" + frontMatter + "\n---\nDo the task.\n"
}

func TestReadAgentJobs_Valid(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "fix-agent.md", agentJob("name: Fix Agent\ncron: \"0 9 * * 1-5\"\ntag: BP\ntag_color: \"#f38ba8\""))

	jobs, err := readAgentJobs(dir, "myproject")
	if err != nil {
		t.Fatalf("readAgentJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	j := jobs[0]
	if j.ID != "myproject-fix-agent" {
		t.Errorf("ID = %q, want myproject-fix-agent (project-scoped)", j.ID)
	}
	if j.Name != "Fix Agent" {
		t.Errorf("Name = %q", j.Name)
	}
	if j.Schedule != "0 9 * * 1-5" {
		t.Errorf("Schedule = %q", j.Schedule)
	}
	want := `"$HOME/.lazycron/bin/lazycron" agent .lazycron/jobs/fix-agent.md --project 'myproject'`
	if j.Command != want {
		t.Errorf("Command = %q, want %q", j.Command, want)
	}
	if j.Project != "myproject" {
		t.Errorf("Project = %q", j.Project)
	}
	if j.Tag != "BP" || j.TagColor != "#f38ba8" {
		t.Errorf("Tag/Color = %q/%q", j.Tag, j.TagColor)
	}
}

func TestReadAgentJobs_HumanSchedule(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "audit.md", agentJob("name: Audit\ncron: every sunday at 3am"))

	jobs, err := readAgentJobs(dir, "p")
	if err != nil {
		t.Fatalf("readAgentJobs: %v", err)
	}
	if jobs[0].Schedule != "0 3 * * 0" {
		t.Errorf("Schedule = %q, want 0 3 * * 0", jobs[0].Schedule)
	}
}

func TestReadAgentJobs_MultipleSortedAndIgnoresOtherFiles(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "zebra.md", agentJob("name: Z\ncron: \"0 1 * * *\""))
	writeJobFile(t, dir, "apple.md", agentJob("name: A\ncron: \"0 2 * * *\""))
	writeJobFile(t, dir, "old.ts", `export const cron = "0 3 * * *";`)

	jobs, err := readAgentJobs(dir, "p")
	if err != nil {
		t.Fatalf("readAgentJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(jobs))
	}
	if jobs[0].ID != "p-apple" || jobs[1].ID != "p-zebra" {
		t.Errorf("expected sorted scoped IDs p-apple, p-zebra; got %s, %s", jobs[0].ID, jobs[1].ID)
	}
}

func TestReadAgentJobs_MissingMetadataFails(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "bad.md", "just a prompt, no front matter")

	_, err := readAgentJobs(dir, "p")
	if err == nil {
		t.Fatal("expected error for missing front matter, got nil")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Errorf("error should reference filename, got: %v", err)
	}
}

func TestReadAgentJobs_InvalidCronFails(t *testing.T) {
	dir := t.TempDir()
	writeJobFile(t, dir, "broken-cron.md", agentJob("name: Broken\ncron: not a cron expression"))

	if _, err := readAgentJobs(dir, "p"); err == nil {
		t.Fatal("expected error for invalid cron, got nil")
	}
}

func TestReadAgentJobs_EmptyDir(t *testing.T) {
	jobs, err := readAgentJobs(t.TempDir(), "p")
	if err != nil {
		t.Fatalf("readAgentJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

// TestReadAgentJobs_CrossProjectScoping ensures two projects with the same
// job filename produce different IDs, so syncing project B never overwrites
// project A's entry on a shared machine.
func TestReadAgentJobs_CrossProjectScoping(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	writeJobFile(t, dirA, "worker.md", agentJob("name: Worker\ncron: every 4 hours"))
	writeJobFile(t, dirB, "worker.md", agentJob("name: Worker\ncron: every 4 hours"))

	jobsA, err := readAgentJobs(dirA, "project-a")
	if err != nil {
		t.Fatalf("readAgentJobs A: %v", err)
	}
	jobsB, err := readAgentJobs(dirB, "project-b")
	if err != nil {
		t.Fatalf("readAgentJobs B: %v", err)
	}
	if jobsA[0].ID != "project-a-worker" || jobsB[0].ID != "project-b-worker" {
		t.Errorf("unexpected scoped IDs: A=%q B=%q", jobsA[0].ID, jobsB[0].ID)
	}
}

func TestLegacyDirHint(t *testing.T) {
	dir := t.TempDir()
	if legacyDirHint(dir) != "" {
		t.Fatal("no hint expected without .sandcastle/")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".sandcastle", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(legacyDirHint(dir), ".lazycron/jobs/") {
		t.Errorf("expected a migration hint, got %q", legacyDirHint(dir))
	}
	if err := os.MkdirAll(filepath.Join(dir, ".lazycron", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if legacyDirHint(dir) != "" {
		t.Error("no hint expected once .lazycron/jobs/ exists")
	}
}

// TestMergeJobs_LegacyBareIDMigration covers the upgrade path from
// pre-scoped-ID lazycron versions: an existing job on the remote with
// a bare ID (`worker-agent`) and Project="lazycron" should be matched
// against an incoming scoped ID (`lazycron-worker-agent`) and updated
// in place rather than left orphaned next to the new entry.
func TestMergeJobs_LegacyBareIDMigration(t *testing.T) {
	existing := []cron.Job{
		{ID: "worker-agent", Name: "Worker", Schedule: "0 9 * * 1-5", Command: "old", Project: "lazycron", Enabled: true},
	}
	incoming := []cron.Job{
		{ID: "lazycron-worker-agent", Name: "Worker", Schedule: "0 9 * * 1-5", Command: "new", Project: "lazycron", Enabled: true},
	}

	merged, added, updated, _ := mergeJobs(existing, incoming)
	if added != 0 || updated != 1 {
		t.Errorf("legacy migration: added=%d updated=%d, want 0/1", added, updated)
	}
	if len(merged) != 1 {
		t.Errorf("expected 1 merged job (in-place upgrade), got %d", len(merged))
	}
	if merged[0].Command != "new" {
		t.Errorf("merged command = %q, want %q", merged[0].Command, "new")
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

// --- helpers ---

func writeJobFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
