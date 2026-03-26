package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

// --- readJobFiles ---

func TestReadJobFiles_Valid(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, dir, "db-backup.yaml", `
name: Database Backup
schedule: "0 3 * * *"
command: pg_dump mydb
project: backend
tag: DB
tag_color: "#a6e3a1"
`)

	writeYAML(t, dir, "log-rotate.yaml", `
name: Log Rotation
schedule: "0 0 * * 0"
command: logrotate /etc/logrotate.conf
`)

	jobs, err := readJobFiles(dir, nil)
	if err != nil {
		t.Fatalf("readJobFiles: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	// Find db-backup job (file order may vary)
	var dbJob cron.Job
	for _, j := range jobs {
		if j.ID == "db-backup" {
			dbJob = j
		}
	}
	if dbJob.ID == "" {
		t.Fatal("db-backup job not found")
	}
	if dbJob.Name != "Database Backup" {
		t.Errorf("name = %q, want %q", dbJob.Name, "Database Backup")
	}
	if dbJob.Schedule != "0 3 * * *" {
		t.Errorf("schedule = %q, want %q", dbJob.Schedule, "0 3 * * *")
	}
	if dbJob.Project != "backend" {
		t.Errorf("project = %q, want %q", dbJob.Project, "backend")
	}
	if dbJob.Tag != "DB" {
		t.Errorf("tag = %q, want %q", dbJob.Tag, "DB")
	}
	if !dbJob.Enabled {
		t.Error("expected enabled=true by default")
	}
}

func TestReadJobFiles_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad-job.yaml", `not: valid: yaml: [`)

	_, err := readJobFiles(dir, nil)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestReadJobFiles_InvalidID(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "DB-BACKUP.yaml", `
name: Bad ID
schedule: "* * * * *"
command: echo hi
`)

	_, err := readJobFiles(dir, nil)
	if err == nil {
		t.Fatal("expected error for uppercase filename")
	}
}

func TestReadJobFiles_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no-name.yaml", `
schedule: "* * * * *"
command: echo hi
`)

	_, err := readJobFiles(dir, nil)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestReadJobFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	jobs, err := readJobFiles(dir, nil)
	if err != nil {
		t.Fatalf("readJobFiles: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestReadJobFiles_ExplicitDisabled(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "disabled-job.yaml", `
name: Disabled Job
schedule: "* * * * *"
command: echo off
enabled: false
`)

	jobs, err := readJobFiles(dir, nil)
	if err != nil {
		t.Fatalf("readJobFiles: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Enabled {
		t.Error("expected enabled=false")
	}
}

// --- readJobFiles with env vars ---

func TestReadJobFiles_EnvSubstitution(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, dir, "db-backup.yaml", `
name: Database Backup
schedule: "0 3 * * *"
command: pg_dump -h ${DB_HOST} ${DB_NAME}
`)

	vars := map[string]string{
		"DB_HOST": "prod-db.example.com",
		"DB_NAME": "appdb",
	}

	jobs, err := readJobFiles(dir, vars)
	if err != nil {
		t.Fatalf("readJobFiles: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	want := "pg_dump -h prod-db.example.com appdb"
	if jobs[0].Command != want {
		t.Errorf("command = %q, want %q", jobs[0].Command, want)
	}
}

func TestReadJobFiles_UndefinedVarError(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, dir, "db-backup.yaml", `
name: Database Backup
schedule: "0 3 * * *"
command: pg_dump -h ${DB_HOST} ${DB_NAME}
`)

	vars := map[string]string{"DB_HOST": "localhost"}

	_, err := readJobFiles(dir, vars)
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
}

func TestReadJobFiles_NilVarsSkipsSubstitution(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, dir, "job.yaml", `
name: Test Job
schedule: "* * * * *"
command: echo ${NOT_SUBSTITUTED}
`)

	// nil vars means no substitution — ${...} is preserved as-is.
	jobs, err := readJobFiles(dir, nil)
	if err != nil {
		t.Fatalf("readJobFiles: %v", err)
	}
	if jobs[0].Command != "echo ${NOT_SUBSTITUTED}" {
		t.Errorf("command = %q, expected literal ${NOT_SUBSTITUTED}", jobs[0].Command)
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
	// TUI job should still be there
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

func writeYAML(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
