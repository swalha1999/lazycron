package backend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

func TestFileBackend_ReadWriteJobs(t *testing.T) {
	dir := t.TempDir()
	cronFile := filepath.Join(dir, "crontab")
	histDir := filepath.Join(dir, "history")

	fb := NewFileBackend(cronFile, histDir)

	// Reading non-existent file returns empty.
	jobs, err := fb.ReadJobs()
	if err != nil {
		t.Fatalf("ReadJobs on missing file: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}

	// Write jobs and read back.
	want := []cron.Job{
		{ID: "abc12345", Name: "hello", Schedule: "* * * * *", Command: "echo hi", Enabled: true, Wrapped: true},
	}
	if err := fb.WriteJobs(want); err != nil {
		t.Fatalf("WriteJobs: %v", err)
	}

	got, err := fb.ReadJobs()
	if err != nil {
		t.Fatalf("ReadJobs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 job, got %d", len(got))
	}
	if got[0].Name != "hello" {
		t.Errorf("name = %q, want %q", got[0].Name, "hello")
	}
}

func TestFileBackend_History(t *testing.T) {
	dir := t.TempDir()
	cronFile := filepath.Join(dir, "crontab")
	histDir := filepath.Join(dir, "history")

	fb := NewFileBackend(cronFile, histDir)

	// Write a history entry.
	if err := fb.WriteHistory("abc12345", "test-job", "output", true); err != nil {
		t.Fatalf("WriteHistory: %v", err)
	}

	entries, err := fb.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].JobName != "test-job" {
		t.Errorf("JobName = %q, want %q", entries[0].JobName, "test-job")
	}

	// Delete the history entry.
	if err := fb.DeleteHistory(entries[0].FilePath); err != nil {
		t.Fatalf("DeleteHistory: %v", err)
	}
	entries, err = fb.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory after delete: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", len(entries))
	}
}

func TestFileBackend_DeleteHistoryOutsideDir(t *testing.T) {
	dir := t.TempDir()
	cronFile := filepath.Join(dir, "crontab")
	histDir := filepath.Join(dir, "history")

	fb := NewFileBackend(cronFile, histDir)

	// Attempting to delete a file outside the history dir should fail.
	err := fb.DeleteHistory("/tmp/some-other-file")
	if err == nil {
		t.Fatal("expected error deleting file outside history dir")
	}
}

func TestFileBackend_Name(t *testing.T) {
	fb := NewFileBackend("/tmp/crontab", "/tmp/history")
	if fb.Name() != "file:/tmp/crontab" {
		t.Errorf("Name = %q", fb.Name())
	}
}

func TestFileBackend_EmptyFileReturnsNoJobs(t *testing.T) {
	dir := t.TempDir()
	cronFile := filepath.Join(dir, "crontab")
	histDir := filepath.Join(dir, "history")

	// Create an empty file.
	if err := os.WriteFile(cronFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	fb := NewFileBackend(cronFile, histDir)
	jobs, err := fb.ReadJobs()
	if err != nil {
		t.Fatalf("ReadJobs on empty file: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(jobs))
	}
}
