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

func TestDiffFields_MultipleChanges(t *testing.T) {
	old := cron.Job{Name: "A", Schedule: "* * * * *", Command: "echo a", Enabled: true, Project: "alpha"}
	new := cron.Job{Name: "B", Schedule: "0 3 * * *", Command: "echo b", Enabled: false, Project: "beta"}

	changes := diffFields(old, new)
	if len(changes) != 5 {
		t.Fatalf("expected 5 changes, got %d", len(changes))
	}

	fields := make(map[string]bool)
	for _, c := range changes {
		fields[c.Field] = true
	}
	for _, f := range []string{"name", "schedule", "command", "enabled", "project"} {
		if !fields[f] {
			t.Errorf("missing change for field %q", f)
		}
	}
}

func TestDiffFields_NoChanges(t *testing.T) {
	j := cron.Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true}
	changes := diffFields(j, j)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
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
