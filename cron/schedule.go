package cron

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dayMap = map[string]string{
	"sunday": "0", "sun": "0",
	"monday": "1", "mon": "1",
	"tuesday": "2", "tue": "2",
	"wednesday": "3", "wed": "3",
	"thursday": "4", "thu": "4",
	"friday": "5", "fri": "5",
	"saturday": "6", "sat": "6",
}

// HumanToCron converts a human-readable schedule string to a cron expression.
// Returns the original string if no pattern matches (assumed to be raw cron).
func HumanToCron(input string) string {
	s := strings.TrimSpace(strings.ToLower(input))

	// every minute
	if s == "every minute" {
		return "* * * * *"
	}

	// every hour
	if s == "every hour" {
		return "0 * * * *"
	}

	// every N minutes
	if m := regexp.MustCompile(`^every (\d+) minutes?$`).FindStringSubmatch(s); m != nil {
		return fmt.Sprintf("*/%s * * * *", m[1])
	}

	// every N hours
	if m := regexp.MustCompile(`^every (\d+) hours?$`).FindStringSubmatch(s); m != nil {
		return fmt.Sprintf("0 */%s * * *", m[1])
	}

	// every day at HH:MM or HH:MMam/pm
	if m := regexp.MustCompile(`^every day at (.+)$`).FindStringSubmatch(s); m != nil {
		if h, mi, ok := parseTime(m[1]); ok {
			return fmt.Sprintf("%d %d * * *", mi, h)
		}
	}

	// every weekday at HH:MM
	if m := regexp.MustCompile(`^every weekday at (.+)$`).FindStringSubmatch(s); m != nil {
		if h, mi, ok := parseTime(m[1]); ok {
			return fmt.Sprintf("%d %d * * 1-5", mi, h)
		}
	}

	// every <dayname> at HH:MM
	if m := regexp.MustCompile(`^every (\w+) at (.+)$`).FindStringSubmatch(s); m != nil {
		if dow, ok := dayMap[m[1]]; ok {
			if h, mi, ok := parseTime(m[2]); ok {
				return fmt.Sprintf("%d %d * * %s", mi, h, dow)
			}
		}
	}

	return input
}

func parseTime(s string) (hour, minute int, ok bool) {
	s = strings.TrimSpace(strings.ToLower(s))

	// Try 12h format: 9am, 9:30pm, 9 am, 9:30 pm
	s = strings.ReplaceAll(s, " ", "")
	if m := regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?(am|pm)$`).FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi := 0
		if m[2] != "" {
			mi, _ = strconv.Atoi(m[2])
		}
		if m[3] == "pm" && h != 12 {
			h += 12
		}
		if m[3] == "am" && h == 12 {
			h = 0
		}
		if h >= 0 && h <= 23 && mi >= 0 && mi <= 59 {
			return h, mi, true
		}
	}

	// Try 24h format: 9:00, 14:30
	if m := regexp.MustCompile(`^(\d{1,2}):(\d{2})$`).FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		if h >= 0 && h <= 23 && mi >= 0 && mi <= 59 {
			return h, mi, true
		}
	}

	return 0, 0, false
}

// CronToHuman converts a cron expression to a human-readable string.
func CronToHuman(expr string) string {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return expr
	}
	min, hour, dom, mon, dow := parts[0], parts[1], parts[2], parts[3], parts[4]

	// every minute
	if min == "*" && hour == "*" && dom == "*" && mon == "*" && dow == "*" {
		return "Every minute"
	}

	// every N minutes
	if strings.HasPrefix(min, "*/") && hour == "*" && dom == "*" && mon == "*" && dow == "*" {
		n := strings.TrimPrefix(min, "*/")
		return fmt.Sprintf("Every %s minutes", n)
	}

	// every hour
	if min != "*" && !strings.Contains(min, "/") && hour == "*" && dom == "*" && mon == "*" && dow == "*" {
		return fmt.Sprintf("Every hour at minute %s", min)
	}

	// every N hours
	if strings.HasPrefix(hour, "*/") && dom == "*" && mon == "*" && dow == "*" {
		n := strings.TrimPrefix(hour, "*/")
		return fmt.Sprintf("Every %s hours", n)
	}

	// specific time
	if min != "*" && hour != "*" && !strings.Contains(min, "/") && !strings.Contains(hour, "/") {
		h, _ := strconv.Atoi(hour)
		m, _ := strconv.Atoi(min)
		timeStr := formatTime(h, m)

		if dom == "*" && mon == "*" {
			if dow == "*" {
				return fmt.Sprintf("Every day at %s", timeStr)
			}
			if dow == "1-5" {
				return fmt.Sprintf("Every weekday at %s", timeStr)
			}
			if dayName, ok := dowToName(dow); ok {
				return fmt.Sprintf("Every %s at %s", dayName, timeStr)
			}
		}
	}

	return expr
}

func formatTime(h, m int) string {
	t := time.Date(2000, 1, 1, h, m, 0, 0, time.Local)
	if m == 0 {
		return t.Format("3 PM")
	}
	return t.Format("3:04 PM")
}

func dowToName(dow string) (string, bool) {
	names := map[string]string{
		"0": "Sunday", "1": "Monday", "2": "Tuesday", "3": "Wednesday",
		"4": "Thursday", "5": "Friday", "6": "Saturday",
	}
	if name, ok := names[dow]; ok {
		return name, true
	}
	return "", false
}

// NextRuns computes the next n run times for a cron expression.
func NextRuns(expr string, n int) []time.Time {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil
	}

	now := time.Now()
	var results []time.Time

	// Brute force: check each minute for up to 366 days
	t := now.Truncate(time.Minute).Add(time.Minute)
	limit := t.Add(366 * 24 * time.Hour)

	for t.Before(limit) && len(results) < n {
		if matchesCron(t, parts) {
			results = append(results, t)
		}
		t = t.Add(time.Minute)
	}

	return results
}

func matchesCron(t time.Time, parts []string) bool {
	return matchField(parts[0], t.Minute(), 0, 59) &&
		matchField(parts[1], t.Hour(), 0, 23) &&
		matchField(parts[2], t.Day(), 1, 31) &&
		matchField(parts[3], int(t.Month()), 1, 12) &&
		matchField(parts[4], int(t.Weekday()), 0, 6)
}

func matchField(field string, value, min, max int) bool {
	if field == "*" {
		return true
	}

	for _, part := range strings.Split(field, ",") {
		// Handle step values
		if strings.Contains(part, "/") {
			sp := strings.SplitN(part, "/", 2)
			step, err := strconv.Atoi(sp[1])
			if err != nil || step == 0 {
				continue
			}
			start := min
			if sp[0] != "*" {
				start, err = strconv.Atoi(sp[0])
				if err != nil {
					continue
				}
			}
			for v := start; v <= max; v += step {
				if v == value {
					return true
				}
			}
			continue
		}

		// Handle ranges
		if strings.Contains(part, "-") {
			rng := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(rng[0])
			hi, err2 := strconv.Atoi(rng[1])
			if err1 == nil && err2 == nil && value >= lo && value <= hi {
				return true
			}
			continue
		}

		// Simple value
		if v, err := strconv.Atoi(part); err == nil && v == value {
			return true
		}
	}

	return false
}

// ValidateCron checks if a cron expression is valid (5 fields).
func ValidateCron(expr string) error {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return fmt.Errorf("cron expression must have 5 fields, got %d", len(parts))
	}

	names := []string{"minute", "hour", "day of month", "month", "day of week"}
	ranges := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 6}}

	for i, part := range parts {
		if err := validateField(part, ranges[i][0], ranges[i][1]); err != nil {
			return fmt.Errorf("invalid %s field %q: %w", names[i], part, err)
		}
	}
	return nil
}

func validateField(field string, min, max int) error {
	if field == "*" {
		return nil
	}

	for _, part := range strings.Split(field, ",") {
		base := part
		step := ""
		if strings.Contains(part, "/") {
			sp := strings.SplitN(part, "/", 2)
			base = sp[0]
			step = sp[1]
			if _, err := strconv.Atoi(step); err != nil {
				return fmt.Errorf("invalid step %q", step)
			}
		}

		if base == "*" {
			continue
		}

		if strings.Contains(base, "-") {
			rng := strings.SplitN(base, "-", 2)
			lo, err1 := strconv.Atoi(rng[0])
			hi, err2 := strconv.Atoi(rng[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("invalid range %q", base)
			}
			if lo < min || hi > max || lo > hi {
				return fmt.Errorf("range %d-%d out of bounds [%d-%d]", lo, hi, min, max)
			}
			continue
		}

		v, err := strconv.Atoi(base)
		if err != nil {
			return fmt.Errorf("invalid value %q", base)
		}
		if v < min || v > max {
			return fmt.Errorf("value %d out of bounds [%d-%d]", v, min, max)
		}
	}

	return nil
}
