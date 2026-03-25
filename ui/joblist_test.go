package ui

import (
	"testing"

	"github.com/swalha1999/lazycron/history"
)

func boolPtr(b bool) *bool { return &b }

func TestBuildLastRunStatus(t *testing.T) {
	tests := []struct {
		name    string
		entries []history.Entry
		wantLen int
		checks  map[string]*bool // job ID → expected success value
	}{
		{
			name:    "empty history",
			entries: nil,
			wantLen: 0,
		},
		{
			name: "single successful entry",
			entries: []history.Entry{
				{JobID: "abc123", Success: boolPtr(true), Timestamp: "2026-03-25T10:00:00Z"},
			},
			wantLen: 1,
			checks:  map[string]*bool{"abc123": boolPtr(true)},
		},
		{
			name: "single failed entry",
			entries: []history.Entry{
				{JobID: "abc123", Success: boolPtr(false), Timestamp: "2026-03-25T10:00:00Z"},
			},
			wantLen: 1,
			checks:  map[string]*bool{"abc123": boolPtr(false)},
		},
		{
			name: "most recent entry wins (already sorted newest first)",
			entries: []history.Entry{
				{JobID: "abc123", Success: boolPtr(true), Timestamp: "2026-03-25T12:00:00Z"},
				{JobID: "abc123", Success: boolPtr(false), Timestamp: "2026-03-25T10:00:00Z"},
			},
			wantLen: 1,
			checks:  map[string]*bool{"abc123": boolPtr(true)},
		},
		{
			name: "multiple jobs",
			entries: []history.Entry{
				{JobID: "job1", Success: boolPtr(true), Timestamp: "2026-03-25T12:00:00Z"},
				{JobID: "job2", Success: boolPtr(false), Timestamp: "2026-03-25T11:00:00Z"},
				{JobID: "job1", Success: boolPtr(false), Timestamp: "2026-03-25T10:00:00Z"},
			},
			wantLen: 2,
			checks: map[string]*bool{
				"job1": boolPtr(true),
				"job2": boolPtr(false),
			},
		},
		{
			name: "entries with empty job ID are skipped",
			entries: []history.Entry{
				{JobID: "", Success: boolPtr(true), Timestamp: "2026-03-25T12:00:00Z"},
				{JobID: "abc123", Success: boolPtr(false), Timestamp: "2026-03-25T10:00:00Z"},
			},
			wantLen: 1,
			checks:  map[string]*bool{"abc123": boolPtr(false)},
		},
		{
			name: "nil success pointer preserved",
			entries: []history.Entry{
				{JobID: "abc123", Success: nil, Timestamp: "2026-03-25T10:00:00Z"},
			},
			wantLen: 1,
			checks:  map[string]*bool{"abc123": nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLastRunStatus(tt.entries)
			if len(result) != tt.wantLen {
				t.Errorf("got len %d, want %d", len(result), tt.wantLen)
			}
			for id, wantSuccess := range tt.checks {
				gotSuccess, ok := result[id]
				if !ok {
					t.Errorf("missing key %q", id)
					continue
				}
				if wantSuccess == nil {
					if gotSuccess != nil {
						t.Errorf("key %q: got %v, want nil", id, *gotSuccess)
					}
				} else if gotSuccess == nil {
					t.Errorf("key %q: got nil, want %v", id, *wantSuccess)
				} else if *gotSuccess != *wantSuccess {
					t.Errorf("key %q: got %v, want %v", id, *gotSuccess, *wantSuccess)
				}
			}
		})
	}
}
