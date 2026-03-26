package cmd

import (
	"testing"
	"time"

	"github.com/swalha1999/lazycron/history"
)

func TestFormatStatus(t *testing.T) {
	yes := true
	no := false

	tests := []struct {
		name    string
		success *bool
		want    string
	}{
		{"success", &yes, "\u2713"},
		{"failure", &no, "\u2717"},
		{"unknown", nil, "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatus(tt.success)
			if got != tt.want {
				t.Errorf("formatStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	formatted, ts := formatTimestamp("2026-03-24T09:00:00Z")
	if formatted != "2026-03-24 09:00" {
		t.Errorf("formatted = %q, want %q", formatted, "2026-03-24 09:00")
	}
	if ts.IsZero() {
		t.Error("expected non-zero time")
	}
}

func TestFormatTimestamp_Invalid(t *testing.T) {
	formatted, ts := formatTimestamp("not-a-date")
	if formatted != "not-a-date" {
		t.Errorf("formatted = %q, want raw input", formatted)
	}
	if !ts.IsZero() {
		t.Error("expected zero time for invalid input")
	}
}

func TestFormatDetail_Success(t *testing.T) {
	yes := true
	e := history.Entry{Success: &yes, Output: "all good"}
	detail := formatDetail(e, time.Time{})
	if detail != "" {
		t.Errorf("expected empty detail for success, got %q", detail)
	}
}

func TestFormatDetail_FailureWithOutput(t *testing.T) {
	no := false
	e := history.Entry{Success: &no, Output: "pg_dump: connection refused\nsome detail"}
	detail := formatDetail(e, time.Time{})
	if detail != "pg_dump: connection refused" {
		t.Errorf("detail = %q, want first line of output", detail)
	}
}

func TestFormatDetail_FailureNoOutput(t *testing.T) {
	no := false
	e := history.Entry{Success: &no, Output: ""}
	detail := formatDetail(e, time.Time{})
	if detail != "(failed)" {
		t.Errorf("detail = %q, want %q", detail, "(failed)")
	}
}
