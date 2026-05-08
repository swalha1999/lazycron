package cmd

import (
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

func TestComputeDiff_AllNew(t *testing.T) {
	existing := []cron.Job{}
	incoming := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "pg_dump mydb"},
	}

	entries := computeDiff(existing, incoming)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != diffNew {
		t.Errorf("expected diffNew, got %d", entries[0].Kind)
	}
}

func TestComputeDiff_AllUnchanged(t *testing.T) {
	jobs := []cron.Job{
		{ID: "db-backup", Name: "DB Backup", Schedule: "0 3 * * *", Command: "pg_dump mydb", Enabled: true},
	}

	entries := computeDiff(jobs, jobs)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != diffUnchanged {
		t.Errorf("expected diffUnchanged, got %d", entries[0].Kind)
	}
}

func TestComputeDiff_Updated(t *testing.T) {
	existing := []cron.Job{
		{ID: "log-rotate", Name: "Log Rotation", Schedule: "0 0 * * 0", Command: "logrotate", Enabled: true},
	}
	incoming := []cron.Job{
		{ID: "log-rotate", Name: "Log Rotation", Schedule: "0 0 * * 1", Command: "logrotate", Enabled: true},
	}

	entries := computeDiff(existing, incoming)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Kind != diffUpdated {
		t.Errorf("expected diffUpdated, got %d", entries[0].Kind)
	}
	if len(entries[0].Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(entries[0].Changes))
	}
	if entries[0].Changes[0].Field != "schedule" {
		t.Errorf("expected schedule change, got %q", entries[0].Changes[0].Field)
	}
}

func TestComputeDiff_Mixed(t *testing.T) {
	existing := []cron.Job{
		{ID: "unchanged-job", Name: "Same", Schedule: "* * * * *", Command: "echo same", Enabled: true},
		{ID: "update-me", Name: "Old Name", Schedule: "0 3 * * *", Command: "echo old", Enabled: true},
	}
	incoming := []cron.Job{
		{ID: "unchanged-job", Name: "Same", Schedule: "* * * * *", Command: "echo same", Enabled: true},
		{ID: "update-me", Name: "New Name", Schedule: "0 3 * * *", Command: "echo old", Enabled: true},
		{ID: "brand-new", Name: "New Job", Schedule: "0 0 * * *", Command: "echo new", Enabled: true},
	}

	entries := computeDiff(existing, incoming)

	var newCount, updatedCount, unchangedCount int
	for _, e := range entries {
		switch e.Kind {
		case diffNew:
			newCount++
		case diffUpdated:
			updatedCount++
		case diffUnchanged:
			unchangedCount++
		}
	}

	if newCount != 1 || updatedCount != 1 || unchangedCount != 1 {
		t.Errorf("counts = %d/%d/%d, want 1/1/1", newCount, updatedCount, unchangedCount)
	}
}

func TestComputeDiff_WrappedOnlyChange(t *testing.T) {
	// Regression: when only Wrapped differs, sync would update the job but
	// diff used to render `~ <name>` with no field info. After unifying the
	// comparators, diff must surface the wrapped change.
	existing := []cron.Job{
		{ID: "j", Name: "J", Schedule: "* * * * *", Command: "echo", Enabled: true, Wrapped: false},
	}
	incoming := []cron.Job{
		{ID: "j", Name: "J", Schedule: "* * * * *", Command: "echo", Enabled: true, Wrapped: true},
	}

	entries := computeDiff(existing, incoming)
	if len(entries) != 1 || entries[0].Kind != diffUpdated {
		t.Fatalf("expected 1 diffUpdated entry, got %+v", entries)
	}
	if len(entries[0].Changes) != 1 || entries[0].Changes[0].Field != "wrapped" {
		t.Errorf("expected single wrapped change, got %+v", entries[0].Changes)
	}
}

func TestHasChanges_NoChanges(t *testing.T) {
	entries := []diffEntry{{Kind: diffUnchanged}, {Kind: diffUnchanged}}
	if hasChanges(entries) {
		t.Error("expected no changes")
	}
}

func TestHasChanges_WithChanges(t *testing.T) {
	entries := []diffEntry{{Kind: diffUnchanged}, {Kind: diffNew}}
	if !hasChanges(entries) {
		t.Error("expected changes")
	}
}
