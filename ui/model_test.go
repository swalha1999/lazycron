package ui

import (
	"testing"

	"github.com/swalha1999/lazycron/history"
)

func boolPtr(v bool) *bool { return &v }

func TestBuildLastRunStatus(t *testing.T) {
	// History is sorted by timestamp descending (most recent first).
	entries := []history.Entry{
		{JobName: "backup", Success: boolPtr(true), Timestamp: "2026-03-24T10:00:00Z"},
		{JobName: "sync", Success: boolPtr(false), Timestamp: "2026-03-24T09:00:00Z"},
		{JobName: "backup", Success: boolPtr(false), Timestamp: "2026-03-24T08:00:00Z"}, // older, should be ignored
	}

	status := buildLastRunStatus(entries)

	// "backup" should reflect the most recent (first) entry: true
	if s, ok := status["backup"]; !ok || s == nil || *s != true {
		t.Errorf("backup: got %v, want true", status["backup"])
	}

	// "sync" should be false
	if s, ok := status["sync"]; !ok || s == nil || *s != false {
		t.Errorf("sync: got %v, want false", status["sync"])
	}

	// "unknown" should not be present
	if _, ok := status["unknown"]; ok {
		t.Error("unknown job should not be in status map")
	}
}

func TestBuildLastRunStatus_Empty(t *testing.T) {
	status := buildLastRunStatus(nil)
	if len(status) != 0 {
		t.Errorf("expected empty map, got %d entries", len(status))
	}
}

func TestBuildLastRunStatus_NilSuccess(t *testing.T) {
	entries := []history.Entry{
		{JobName: "legacy", Success: nil, Timestamp: "2026-03-24T10:00:00Z"},
	}

	status := buildLastRunStatus(entries)
	if s, ok := status["legacy"]; !ok {
		t.Error("legacy should be in map")
	} else if s != nil {
		t.Errorf("legacy: got %v, want nil", *s)
	}
}
