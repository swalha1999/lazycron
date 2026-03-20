package cron

import (
	"testing"
	"time"
)

// --- matchField ---

func TestMatchField_Wildcard(t *testing.T) {
	if !matchField("*", 0, 0, 59) {
		t.Error("wildcard should match any value")
	}
	if !matchField("*", 59, 0, 59) {
		t.Error("wildcard should match max value")
	}
}

func TestMatchField_ExactValue(t *testing.T) {
	if !matchField("5", 5, 0, 59) {
		t.Error("exact value should match")
	}
	if matchField("5", 6, 0, 59) {
		t.Error("exact value should not match different number")
	}
}

func TestMatchField_CommaList(t *testing.T) {
	if !matchField("1,15,30", 15, 0, 59) {
		t.Error("comma list should match included value")
	}
	if matchField("1,15,30", 20, 0, 59) {
		t.Error("comma list should not match excluded value")
	}
}

func TestMatchField_Range(t *testing.T) {
	tests := []struct {
		field string
		value int
		want  bool
	}{
		{"1-5", 1, true},
		{"1-5", 3, true},
		{"1-5", 5, true},
		{"1-5", 0, false},
		{"1-5", 6, false},
	}
	for _, tt := range tests {
		if got := matchField(tt.field, tt.value, 0, 59); got != tt.want {
			t.Errorf("matchField(%q, %d) = %v, want %v", tt.field, tt.value, got, tt.want)
		}
	}
}

func TestMatchField_Step(t *testing.T) {
	tests := []struct {
		field string
		value int
		want  bool
	}{
		{"*/5", 0, true},
		{"*/5", 5, true},
		{"*/5", 10, true},
		{"*/5", 3, false},
		{"*/15", 0, true},
		{"*/15", 15, true},
		{"*/15", 30, true},
		{"*/15", 45, true},
		{"*/15", 7, false},
	}
	for _, tt := range tests {
		if got := matchField(tt.field, tt.value, 0, 59); got != tt.want {
			t.Errorf("matchField(%q, %d) = %v, want %v", tt.field, tt.value, got, tt.want)
		}
	}
}

func TestMatchField_StepWithStart(t *testing.T) {
	// 5/10 means start at 5, then 15, 25, 35, 45, 55
	if !matchField("5/10", 5, 0, 59) {
		t.Error("step with start should match start value")
	}
	if !matchField("5/10", 15, 0, 59) {
		t.Error("step with start should match start+step")
	}
	if matchField("5/10", 0, 0, 59) {
		t.Error("step with start should not match value before start")
	}
	if matchField("5/10", 10, 0, 59) {
		t.Error("step with start should not match non-step value")
	}
}

func TestMatchField_InvalidStep(t *testing.T) {
	if matchField("*/0", 0, 0, 59) {
		t.Error("step of 0 should not match")
	}
	if matchField("*/abc", 0, 0, 59) {
		t.Error("non-numeric step should not match")
	}
}

func TestMatchField_InvalidRange(t *testing.T) {
	if matchField("a-b", 0, 0, 59) {
		t.Error("non-numeric range should not match")
	}
}

func TestMatchField_InvalidValue(t *testing.T) {
	if matchField("abc", 0, 0, 59) {
		t.Error("non-numeric value should not match")
	}
}

// --- matchesCron ---

func TestMatchesCron(t *testing.T) {
	tests := []struct {
		name  string
		t     time.Time
		parts []string
		want  bool
	}{
		{
			"every minute matches any time",
			time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
			[]string{"*", "*", "*", "*", "*"},
			true,
		},
		{
			"specific time matches",
			time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC), // Sunday
			[]string{"0", "9", "*", "*", "*"},
			true,
		},
		{
			"specific time does not match wrong minute",
			time.Date(2025, 6, 15, 9, 5, 0, 0, time.UTC),
			[]string{"0", "9", "*", "*", "*"},
			false,
		},
		{
			"weekday constraint matches",
			time.Date(2025, 6, 16, 9, 0, 0, 0, time.UTC), // Monday=1
			[]string{"0", "9", "*", "*", "1"},
			true,
		},
		{
			"weekday constraint rejects wrong day",
			time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC), // Sunday=0
			[]string{"0", "9", "*", "*", "1"},
			false,
		},
		{
			"day of month matches",
			time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			[]string{"0", "0", "1", "1", "*"},
			true,
		},
		{
			"weekday range 1-5",
			time.Date(2025, 6, 18, 9, 0, 0, 0, time.UTC), // Wednesday=3
			[]string{"0", "9", "*", "*", "1-5"},
			true,
		},
		{
			"step minutes",
			time.Date(2025, 6, 15, 10, 15, 0, 0, time.UTC),
			[]string{"*/15", "*", "*", "*", "*"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesCron(tt.t, tt.parts)
			if got != tt.want {
				t.Errorf("matchesCron(%v, %v) = %v, want %v", tt.t, tt.parts, got, tt.want)
			}
		})
	}
}

// --- ValidateCron ---

func TestValidateCron_Valid(t *testing.T) {
	valid := []string{
		"* * * * *",
		"0 9 * * *",
		"*/5 * * * *",
		"0 0 1 1 *",
		"0 9 * * 1-5",
		"0,30 * * * *",
		"0 9 * * 0,6",
		"0 */2 * * *",
		"5/10 * * * *",
		"0 0 1-15 * *",
	}
	for _, expr := range valid {
		t.Run(expr, func(t *testing.T) {
			if err := ValidateCron(expr); err != nil {
				t.Errorf("ValidateCron(%q) unexpected error: %v", expr, err)
			}
		})
	}
}

func TestValidateCron_WrongFieldCount(t *testing.T) {
	tests := []string{
		"* * * *",
		"* * * * * *",
		"*",
		"",
	}
	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			if err := ValidateCron(expr); err == nil {
				t.Errorf("ValidateCron(%q) expected error for wrong field count", expr)
			}
		})
	}
}

func TestValidateCron_OutOfRange(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"minute too high", "60 * * * *"},
		{"hour too high", "0 24 * * *"},
		{"day too high", "0 0 32 * *"},
		{"day too low", "0 0 0 * *"},
		{"month too high", "0 0 * 13 *"},
		{"month too low", "0 0 * 0 *"},
		{"dow too high", "0 0 * * 7"},
		{"minute negative", "-1 * * * *"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCron(tt.expr); err == nil {
				t.Errorf("ValidateCron(%q) expected error for out-of-range", tt.expr)
			}
		})
	}
}

func TestValidateCron_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{"non-numeric", "abc * * * *"},
		{"bad step", "*/abc * * * *"},
		{"bad range", "a-b * * * *"},
		{"range reversed", "10-5 * * * *"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCron(tt.expr); err == nil {
				t.Errorf("ValidateCron(%q) expected error for invalid syntax", tt.expr)
			}
		})
	}
}

// --- NextRuns ---

func TestNextRuns_InvalidExpr(t *testing.T) {
	result := NextRuns("bad", 5)
	if result != nil {
		t.Errorf("NextRuns with invalid expr should return nil, got %v", result)
	}
}

func TestNextRuns_ReturnsRequestedCount(t *testing.T) {
	// "every minute" should always find results quickly
	results := NextRuns("* * * * *", 3)
	if len(results) != 3 {
		t.Fatalf("NextRuns(every minute, 3) returned %d results, want 3", len(results))
	}

	// Results should be in ascending order
	for i := 1; i < len(results); i++ {
		if !results[i].After(results[i-1]) {
			t.Errorf("results not in order: %v is not after %v", results[i], results[i-1])
		}
	}
}

func TestNextRuns_AllInFuture(t *testing.T) {
	results := NextRuns("0 * * * *", 2)
	now := time.Now()
	for _, r := range results {
		if !r.After(now) {
			t.Errorf("result %v is not after now %v", r, now)
		}
	}
}

func TestNextRuns_ZeroCount(t *testing.T) {
	results := NextRuns("* * * * *", 0)
	if len(results) != 0 {
		t.Errorf("NextRuns with 0 count returned %d results", len(results))
	}
}

// --- DatetimeToCron ---

func TestDatetimeToCron_StrictFormats(t *testing.T) {
	// Use a future date
	future := time.Now().AddDate(1, 0, 0)
	dateStr := future.Format("2006-01-02") + " 14:30"

	expr, resolved, err := DatetimeToCron(dateStr)
	if err != nil {
		t.Fatalf("DatetimeToCron(%q) error: %v", dateStr, err)
	}
	if resolved.Hour() != 14 || resolved.Minute() != 30 {
		t.Errorf("resolved time = %v, want 14:30", resolved)
	}
	expected := "30 14 " + future.Format("2") + " " + future.Format("1") + " *"
	if expr != expected {
		t.Errorf("DatetimeToCron = %q, want %q", expr, expected)
	}
}

func TestDatetimeToCron_ISOFormat(t *testing.T) {
	future := time.Now().AddDate(1, 0, 0)
	dateStr := future.Format("2006-01-02") + "T09:00"

	expr, _, err := DatetimeToCron(dateStr)
	if err != nil {
		t.Fatalf("DatetimeToCron(%q) error: %v", dateStr, err)
	}
	expected := "0 9 " + future.Format("2") + " " + future.Format("1") + " *"
	if expr != expected {
		t.Errorf("DatetimeToCron = %q, want %q", expr, expected)
	}
}

func TestDatetimeToCron_TomorrowAt(t *testing.T) {
	expr, resolved, err := DatetimeToCron("tomorrow at 3pm")
	if err != nil {
		t.Fatalf("DatetimeToCron('tomorrow at 3pm') error: %v", err)
	}

	tomorrow := time.Now().AddDate(0, 0, 1)
	if resolved.Day() != tomorrow.Day() {
		t.Errorf("resolved day = %d, want %d", resolved.Day(), tomorrow.Day())
	}
	if resolved.Hour() != 15 || resolved.Minute() != 0 {
		t.Errorf("resolved time = %d:%d, want 15:00", resolved.Hour(), resolved.Minute())
	}

	expected := "0 15 " + tomorrow.Format("2") + " " + tomorrow.Format("1") + " *"
	if expr != expected {
		t.Errorf("DatetimeToCron = %q, want %q", expr, expected)
	}
}

func TestDatetimeToCron_NextDayAt(t *testing.T) {
	_, resolved, err := DatetimeToCron("next monday at 9am")
	if err != nil {
		t.Fatalf("DatetimeToCron('next monday at 9am') error: %v", err)
	}
	if resolved.Weekday() != time.Monday {
		t.Errorf("resolved weekday = %v, want Monday", resolved.Weekday())
	}
	if resolved.Hour() != 9 || resolved.Minute() != 0 {
		t.Errorf("resolved time = %d:%d, want 9:00", resolved.Hour(), resolved.Minute())
	}
}

func TestDatetimeToCron_PastTimeError(t *testing.T) {
	past := time.Now().AddDate(-1, 0, 0)
	dateStr := past.Format("2006-01-02") + " 14:30"

	_, _, err := DatetimeToCron(dateStr)
	if err == nil {
		t.Error("DatetimeToCron should return error for past time")
	}
}

func TestDatetimeToCron_InvalidInput(t *testing.T) {
	_, _, err := DatetimeToCron("not a date")
	if err == nil {
		t.Error("DatetimeToCron should return error for invalid input")
	}
}

func TestDatetimeToCron_MonthDayAt(t *testing.T) {
	// Use a month that's far in the future to ensure it's not in the past
	future := time.Now().AddDate(1, 0, 0)
	monthName := future.Format("January")
	dayStr := future.Format("2")
	input := monthName + " " + dayStr + " at 2:30pm"

	_, resolved, err := DatetimeToCron(input)
	if err != nil {
		t.Fatalf("DatetimeToCron(%q) error: %v", input, err)
	}
	if resolved.Hour() != 14 || resolved.Minute() != 30 {
		t.Errorf("resolved time = %d:%d, want 14:30", resolved.Hour(), resolved.Minute())
	}
}
