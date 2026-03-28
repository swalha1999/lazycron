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

// DatetimeToCron converts a datetime string to a pinned cron expression.
// Supports strict formats: "2026-03-22 14:30", "2026-03-22T14:30"
// and natural language: "tomorrow at 3pm", "next monday at 9am", "march 22 at 2:30pm"
// Returns the cron expression, the resolved time, and any error.
func DatetimeToCron(input string) (string, time.Time, error) {
	s := strings.TrimSpace(input)
	now := time.Now()

	var resolved time.Time
	var ok bool

	// Try strict ISO-like formats first
	for _, layout := range []string{
		"2006-01-02 15:04",
		"2006-01-02T15:04",
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			resolved = t
			ok = true
			break
		}
	}

	// Try natural language
	if !ok {
		resolved, ok = parseNaturalDatetime(s, now)
	}

	if !ok {
		return "", time.Time{}, fmt.Errorf("could not parse datetime %q — try 'tomorrow at 3pm' or '2026-03-22 14:30'", input)
	}

	if !resolved.After(now) {
		return "", time.Time{}, fmt.Errorf("scheduled time %s is in the past", resolved.Format("2006-01-02 15:04"))
	}

	cronExpr := fmt.Sprintf("%d %d %d %d *", resolved.Minute(), resolved.Hour(), resolved.Day(), int(resolved.Month()))
	return cronExpr, resolved, nil
}

// parseNaturalDatetime parses natural language datetime relative to now.
func parseNaturalDatetime(s string, now time.Time) (time.Time, bool) {
	lower := strings.ToLower(s)

	// "tomorrow at <time>"
	if m := regexp.MustCompile(`^tomorrow at (.+)$`).FindStringSubmatch(lower); m != nil {
		if h, mi, ok := parseTime(m[1]); ok {
			tomorrow := now.AddDate(0, 0, 1)
			return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), h, mi, 0, 0, time.Local), true
		}
	}

	// "next <dayname> at <time>"
	if m := regexp.MustCompile(`^next (\w+) at (.+)$`).FindStringSubmatch(lower); m != nil {
		if dow, ok := dayMap[m[1]]; ok {
			if h, mi, ok := parseTime(m[2]); ok {
				target, _ := strconv.Atoi(dow)
				current := int(now.Weekday())
				daysAhead := (target - current + 7) % 7
				if daysAhead == 0 {
					daysAhead = 7 // "next Monday" when today is Monday means 7 days
				}
				d := now.AddDate(0, 0, daysAhead)
				return time.Date(d.Year(), d.Month(), d.Day(), h, mi, 0, 0, time.Local), true
			}
		}
	}

	// "<monthname> <day> at <time>" e.g. "march 22 at 2:30pm"
	if m := regexp.MustCompile(`^(\w+) (\d{1,2}) at (.+)$`).FindStringSubmatch(lower); m != nil {
		if month, ok := parseMonth(m[1]); ok {
			day, _ := strconv.Atoi(m[2])
			if h, mi, ok := parseTime(m[3]); ok {
				year := now.Year()
				t := time.Date(year, month, day, h, mi, 0, 0, time.Local)
				if !t.After(now) {
					t = time.Date(year+1, month, day, h, mi, 0, 0, time.Local)
				}
				return t, true
			}
		}
	}

	return time.Time{}, false
}

// parseMonth converts a month name to time.Month.
func parseMonth(s string) (time.Month, bool) {
	months := map[string]time.Month{
		"january": time.January, "jan": time.January,
		"february": time.February, "feb": time.February,
		"march": time.March, "mar": time.March,
		"april": time.April, "apr": time.April,
		"may": time.May,
		"june": time.June, "jun": time.June,
		"july": time.July, "jul": time.July,
		"august": time.August, "aug": time.August,
		"september": time.September, "sep": time.September,
		"october": time.October, "oct": time.October,
		"november": time.November, "nov": time.November,
		"december": time.December, "dec": time.December,
	}
	m, ok := months[strings.ToLower(s)]
	return m, ok
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

// fieldPart represents one component of a parsed cron field.
type fieldPart struct {
	start, end, step int
}

// contains reports whether value is matched by this field part.
func (p fieldPart) contains(value int) bool {
	if p.step > 0 {
		for v := p.start; v <= p.end; v += p.step {
			if v == value {
				return true
			}
		}
		return false
	}
	return value >= p.start && value <= p.end
}

// parseCronField parses a single cron field into its component parts.
// It returns an error if the field contains invalid syntax or out-of-range values.
func parseCronField(field string, min, max int) ([]fieldPart, error) {
	if field == "*" {
		return []fieldPart{{start: min, end: max}}, nil
	}

	var parts []fieldPart
	for _, tok := range strings.Split(field, ",") {
		base := tok
		stepVal := 0

		if strings.Contains(tok, "/") {
			sp := strings.SplitN(tok, "/", 2)
			base = sp[0]
			s, err := strconv.Atoi(sp[1])
			if err != nil {
				return nil, fmt.Errorf("invalid step %q", sp[1])
			}
			if s == 0 {
				return nil, fmt.Errorf("step must not be zero")
			}
			stepVal = s
		}

		switch {
		case base == "*":
			parts = append(parts, fieldPart{start: min, end: max, step: stepVal})

		case strings.Contains(base, "-"):
			rng := strings.SplitN(base, "-", 2)
			lo, err1 := strconv.Atoi(rng[0])
			hi, err2 := strconv.Atoi(rng[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid range %q", base)
			}
			if lo < min || hi > max || lo > hi {
				return nil, fmt.Errorf("range %d-%d out of bounds [%d-%d]", lo, hi, min, max)
			}
			parts = append(parts, fieldPart{start: lo, end: hi, step: stepVal})

		default:
			v, err := strconv.Atoi(base)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q", base)
			}
			if v < min || v > max {
				return nil, fmt.Errorf("value %d out of bounds [%d-%d]", v, min, max)
			}
			end := v
			if stepVal > 0 {
				end = max
			}
			parts = append(parts, fieldPart{start: v, end: end, step: stepVal})
		}
	}

	return parts, nil
}

func matchField(field string, value, min, max int) bool {
	parts, err := parseCronField(field, min, max)
	if err != nil {
		return false
	}
	for _, p := range parts {
		if p.contains(value) {
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
		if _, err := parseCronField(part, ranges[i][0], ranges[i][1]); err != nil {
			return fmt.Errorf("invalid %s field %q: %w", names[i], part, err)
		}
	}
	return nil
}
