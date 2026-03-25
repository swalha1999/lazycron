package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronToHuman converts a cron expression to a human-readable string.
func CronToHuman(expr string) string {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return expr
	}
	min, hour, dom, mon, dow := parts[0], parts[1], parts[2], parts[3], parts[4]

	// Detect one-shot-style expression: all 4 date fields are specific values
	if isValue(min) && isValue(hour) && isValue(dom) && isValue(mon) && dow == "*" {
		h, _ := strconv.Atoi(hour)
		m, _ := strconv.Atoi(min)
		d, _ := strconv.Atoi(dom)
		mo, _ := strconv.Atoi(mon)
		now := time.Now()
		year := now.Year()
		t := time.Date(year, time.Month(mo), d, h, m, 0, 0, time.Local)
		if t.Before(now) {
			t = time.Date(year+1, time.Month(mo), d, h, m, 0, 0, time.Local)
		}
		return t.Format("Jan 02, 2006 at 3:04 PM")
	}

	// Build the frequency part (how often within a day): minute + hour
	freq := describeFrequency(min, hour)

	// Build the date constraint part (which days): dom + mon + dow
	when := describeWhen(dom, mon, dow)

	if when == "" {
		return capitalize(freq)
	}
	return capitalize(freq + ", " + when)
}

// describeFrequency describes how often the job runs within a day based on min+hour.
func describeFrequency(min, hour string) string {
	// Both specific values → exact time
	if isValue(min) && isValue(hour) {
		h, _ := strconv.Atoi(hour)
		m, _ := strconv.Atoi(min)
		return "at " + formatTime(h, m)
	}

	// Specific minute, step hour → "every 2nd hour at minute 30"
	if isValue(min) && isStep(hour) {
		m, _ := strconv.Atoi(min)
		hourPart := describeStep(hour, "hour")
		if m == 0 {
			return hourPart
		}
		return fmt.Sprintf("%s at minute %d", hourPart, m)
	}

	// Specific minute, wildcard hour → "every hour at minute 30"
	if isValue(min) && hour == "*" {
		m, _ := strconv.Atoi(min)
		if m == 0 {
			return "every hour"
		}
		return fmt.Sprintf("every hour at minute %d", m)
	}

	// Step minute, specific hour → "every 5th minute during 3 PM hour"
	if isStep(min) && isValue(hour) {
		h, _ := strconv.Atoi(hour)
		return describeStep(min, "minute") + " during " + formatHour(h)
	}

	// Step minute, step hour → "every 5th minute, every 2nd hour"
	if isStep(min) && isStep(hour) {
		return describeStep(min, "minute") + ", " + describeStep(hour, "hour")
	}

	// Step minute, wildcard hour → "every 5th minute"
	if isStep(min) && hour == "*" {
		return describeStep(min, "minute")
	}

	// Wildcard minute, specific hour → "every minute during 3 PM hour"
	if min == "*" && isValue(hour) {
		h, _ := strconv.Atoi(hour)
		return "every minute during " + formatHour(h)
	}

	// Wildcard minute, step hour → "every minute, every 2nd hour"
	if min == "*" && isStep(hour) {
		return "every minute, " + describeStep(hour, "hour")
	}

	// Both wildcard
	return "every minute"
}

// describeWhen describes date constraints from dom, mon, dow fields.
// Returns empty string if there are no constraints (all wildcards).
func describeWhen(dom, mon, dow string) string {
	var parts []string

	if dom != "*" {
		parts = append(parts, describeDateField(dom, "day"))
	}
	if mon != "*" {
		parts = append(parts, describeDateField(mon, "month"))
	}
	if dow != "*" {
		parts = append(parts, describeDow(dow))
	}

	return strings.Join(parts, ", ")
}

func describeDateField(field, unit string) string {
	if isStep(field) {
		return describeStep(field, unit)
	}
	if isValue(field) {
		v, _ := strconv.Atoi(field)
		switch unit {
		case "day":
			return fmt.Sprintf("on day %d", v)
		case "month":
			return "in " + monthName(v)
		}
	}
	return field
}

func describeDow(field string) string {
	if isStep(field) {
		return describeStep(field, "weekday")
	}
	if isValue(field) {
		v, _ := strconv.Atoi(field)
		return "on " + dowName(v)
	}
	// Ranges like 1-5
	if strings.Contains(field, "-") && !strings.Contains(field, "/") {
		rng := strings.SplitN(field, "-", 2)
		lo, err1 := strconv.Atoi(rng[0])
		hi, err2 := strconv.Atoi(rng[1])
		if err1 == nil && err2 == nil {
			if lo == 1 && hi == 5 {
				return "on weekdays"
			}
			return fmt.Sprintf("on %s through %s", dowName(lo), dowName(hi))
		}
	}
	return field
}

// describeStep converts "*/N" into "every Nth <unit>" and
// "start-end/N" into "every Nth <unit> from <start> to <end>".
func describeStep(field, unit string) string {
	if isRangeStep(field) {
		return describeRangeStep(field, unit)
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(field, "*/"))
	if n == 1 {
		return "every " + unit
	}
	return fmt.Sprintf("every %s %s", ordinal(n), unit)
}

// describeRangeStep converts "start-end/step" into a human-readable string.
func describeRangeStep(field, unit string) string {
	slashIdx := strings.Index(field, "/")
	rangePart := field[:slashIdx]
	step, _ := strconv.Atoi(field[slashIdx+1:])
	dashIdx := strings.Index(rangePart, "-")
	lo, _ := strconv.Atoi(rangePart[:dashIdx])
	hi, _ := strconv.Atoi(rangePart[dashIdx+1:])

	var stepStr string
	if step == 1 {
		stepStr = "every " + unit
	} else {
		stepStr = fmt.Sprintf("every %s %s", ordinal(step), unit)
	}

	switch unit {
	case "hour":
		return fmt.Sprintf("%s from %s to %s", stepStr, formatHourOnly(lo), formatHourOnly(hi))
	case "minute":
		return fmt.Sprintf("%s from %d to %d", stepStr, lo, hi)
	default:
		return fmt.Sprintf("%s from %d to %d", stepStr, lo, hi)
	}
}

func ordinal(n int) string {
	if n >= 11 && n <= 13 {
		return fmt.Sprintf("%dth", n)
	}
	switch n % 10 {
	case 1:
		return fmt.Sprintf("%dst", n)
	case 2:
		return fmt.Sprintf("%dnd", n)
	case 3:
		return fmt.Sprintf("%drd", n)
	default:
		return fmt.Sprintf("%dth", n)
	}
}

func isStep(field string) bool {
	return strings.HasPrefix(field, "*/") || isRangeStep(field)
}

// isRangeStep checks for patterns like "8-20/2" (start-end/step).
func isRangeStep(field string) bool {
	slashIdx := strings.Index(field, "/")
	if slashIdx < 0 {
		return false
	}
	rangePart := field[:slashIdx]
	stepPart := field[slashIdx+1:]
	dashIdx := strings.Index(rangePart, "-")
	if dashIdx < 0 {
		return false
	}
	_, err1 := strconv.Atoi(rangePart[:dashIdx])
	_, err2 := strconv.Atoi(rangePart[dashIdx+1:])
	_, err3 := strconv.Atoi(stepPart)
	return err1 == nil && err2 == nil && err3 == nil
}

func isValue(field string) bool {
	_, err := strconv.Atoi(field)
	return err == nil
}

func formatTime(h, m int) string {
	t := time.Date(2000, 1, 1, h, m, 0, 0, time.Local)
	if m == 0 {
		return t.Format("3 PM")
	}
	return t.Format("3:04 PM")
}

func formatHour(h int) string {
	t := time.Date(2000, 1, 1, h, 0, 0, 0, time.Local)
	return t.Format("3 PM") + " hour"
}

func formatHourOnly(h int) string {
	t := time.Date(2000, 1, 1, h, 0, 0, 0, time.Local)
	return t.Format("3 PM")
}

func monthName(m int) string {
	if m >= 1 && m <= 12 {
		return time.Month(m).String()
	}
	return strconv.Itoa(m)
}

func dowName(d int) string {
	names := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	if d >= 0 && d < len(names) {
		return names[d]
	}
	return strconv.Itoa(d)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] -= 32
	}
	return string(runes)
}
