package cron

import (
	"strings"
	"testing"
)

func TestCronToHuman(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		// All wildcards
		{"* * * * *", "Every minute"},

		// Minute variations
		{"*/5 * * * *", "Every 5th minute"},
		{"*/1 * * * *", "Every minute"},
		{"0 * * * *", "Every hour"},
		{"30 * * * *", "Every hour at minute 30"},

		// Hour variations
		{"0 */2 * * *", "Every 2nd hour"},
		{"0 */1 * * *", "Every hour"},
		{"5 */2 * * *", "Every 2nd hour at minute 5"},
		{"* */2 * * *", "Every minute, every 2nd hour"},
		{"*/10 */2 * * *", "Every 10th minute, every 2nd hour"},

		// Range/step hours
		{"30 8-20/2 * * *", "Every 2nd hour from 8 AM to 8 PM at minute 30"},
		{"0 8-20/2 * * *", "Every 2nd hour from 8 AM to 8 PM"},
		{"* 8-20/2 * * *", "Every minute, every 2nd hour from 8 AM to 8 PM"},
		{"*/10 8-20/2 * * *", "Every 10th minute, every 2nd hour from 8 AM to 8 PM"},
		{"0 9-17/1 * * *", "Every hour from 9 AM to 5 PM"},

		// Specific times
		{"30 8 * * *", "At 8:30 AM"},
		{"0 9 * * *", "At 9 AM"},
		{"0 0 * * *", "At 12 AM"},
		{"0 14 * * *", "At 2 PM"},

		// Day of month
		{"* * */2 * *", "Every minute, every 2nd day"},
		{"0 */3 */2 * *", "Every 3rd hour, every 2nd day"},
		{"30 14 1 * *", "At 2:30 PM, on day 1"},

		// Month
		{"* * * */3 *", "Every minute, every 3rd month"},

		// Day of week
		{"0 9 * * 3", "At 9 AM, on Wednesday"},
		{"0 9 * * 0", "At 9 AM, on Sunday"},
		{"0 9 * * 1-5", "At 9 AM, on weekdays"},
		{"0 0 * * 0", "At 12 AM, on Sunday"},
		{"* * * * */2", "Every minute, every 2nd weekday"},

		// Invalid
		{"bad", "bad"},
		{"* * *", "* * *"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := CronToHuman(tt.expr)
			if got != tt.want {
				t.Errorf("CronToHuman(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

func TestOrdinal(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "1st"},
		{2, "2nd"},
		{3, "3rd"},
		{4, "4th"},
		{5, "5th"},
		{10, "10th"},
		{11, "11th"},
		{12, "12th"},
		{13, "13th"},
		{21, "21st"},
		{22, "22nd"},
		{23, "23rd"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := ordinal(tt.n)
			if got != tt.want {
				t.Errorf("ordinal(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// --- HumanToCron ---

func TestHumanToCron(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"every minute", "* * * * *"},
		{"every hour", "0 * * * *"},
		{"every 5 minutes", "*/5 * * * *"},
		{"every 1 minute", "*/1 * * * *"},
		{"every 2 hours", "0 */2 * * *"},
		{"every 1 hour", "0 */1 * * *"},
		{"every day at 9am", "0 9 * * *"},
		{"every day at 9:30am", "30 9 * * *"},
		{"every day at 2pm", "0 14 * * *"},
		{"every day at 2:30pm", "30 14 * * *"},
		{"every day at 12am", "0 0 * * *"},
		{"every day at 12pm", "0 12 * * *"},
		{"every day at 14:30", "30 14 * * *"},
		{"every day at 0:00", "0 0 * * *"},
		{"every weekday at 9am", "0 9 * * 1-5"},
		{"every weekday at 9:30am", "30 9 * * 1-5"},
		{"every monday at 9am", "0 9 * * 1"},
		{"every friday at 5pm", "0 17 * * 5"},
		{"every sunday at 12pm", "0 12 * * 0"},
		{"every saturday at 8:00", "0 8 * * 6"},
		// Case insensitivity
		{"Every Day At 9AM", "0 9 * * *"},
		{"EVERY HOUR", "0 * * * *"},
		// Pass-through for unrecognized patterns
		{"0 9 * * *", "0 9 * * *"},
		{"something else", "something else"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := HumanToCron(tt.input)
			if got != tt.want {
				t.Errorf("HumanToCron(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseTime ---

func TestParseTime(t *testing.T) {
	tests := []struct {
		input      string
		wantHour   int
		wantMinute int
		wantOK     bool
	}{
		// 12-hour format
		{"9am", 9, 0, true},
		{"9:30am", 9, 30, true},
		{"12pm", 12, 0, true},
		{"12am", 0, 0, true},
		{"11:59pm", 23, 59, true},
		{"1pm", 13, 0, true},
		{"9 am", 9, 0, true},
		{"9:30 pm", 21, 30, true},
		// 24-hour format
		{"0:00", 0, 0, true},
		{"9:00", 9, 0, true},
		{"14:30", 14, 30, true},
		{"23:59", 23, 59, true},
		// Invalid
		{"25:00", 0, 0, false},
		{"9:60", 0, 0, false},
		{"not-a-time", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, m, ok := parseTime(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseTime(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if h != tt.wantHour || m != tt.wantMinute {
				t.Errorf("parseTime(%q) = (%d, %d), want (%d, %d)", tt.input, h, m, tt.wantHour, tt.wantMinute)
			}
		})
	}
}

// --- HumanToCron/CronToHuman roundtrip ---

func TestHumanCronRoundtrip(t *testing.T) {
	// Human → Cron → Human should produce a recognizable description
	inputs := []string{
		"every minute",
		"every hour",
		"every 5 minutes",
		"every day at 9am",
		"every day at 2:30pm",
	}
	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			cron := HumanToCron(input)
			human := CronToHuman(cron)
			// The round-tripped description should not be the raw cron expression
			if human == cron {
				t.Errorf("HumanToCron(%q) = %q, CronToHuman gave back raw cron", input, cron)
			}
			// Should produce some human-readable text
			if strings.TrimSpace(human) == "" {
				t.Errorf("CronToHuman(%q) returned empty string", cron)
			}
		})
	}
}
