package cron

import "testing"

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
