package cron

import "fmt"

// FieldDiff describes a single field that differs between two Jobs.
type FieldDiff struct {
	Field string
	Old   string
	New   string
}

// jobComparator describes one comparable field on Job: how to detect a
// difference and how to render either side as a string for display.
type jobComparator struct {
	name    string
	differs func(a, b Job) bool
	format  func(j Job) string
}

// jobComparators is the single source of truth for which Job fields count as
// a change. Both DiffJob and JobsDiffer iterate over this list, so adding a
// new comparable field on Job is a single edit here instead of a coordinated
// edit across sync.go and diff.go.
var jobComparators = []jobComparator{
	{"name", func(a, b Job) bool { return a.Name != b.Name }, func(j Job) string { return j.Name }},
	{"schedule", func(a, b Job) bool { return a.Schedule != b.Schedule }, func(j Job) string { return fmt.Sprintf("%q", j.Schedule) }},
	{"command", func(a, b Job) bool { return a.Command != b.Command }, func(j Job) string { return j.Command }},
	{"enabled", func(a, b Job) bool { return a.Enabled != b.Enabled }, func(j Job) string { return fmt.Sprintf("%v", j.Enabled) }},
	{"wrapped", func(a, b Job) bool { return a.Wrapped != b.Wrapped }, func(j Job) string { return fmt.Sprintf("%v", j.Wrapped) }},
	{"once", func(a, b Job) bool { return a.OneShot != b.OneShot }, func(j Job) string { return fmt.Sprintf("%v", j.OneShot) }},
	{"tag", func(a, b Job) bool { return a.Tag != b.Tag }, func(j Job) string { return j.Tag }},
	{"tag_color", func(a, b Job) bool { return a.TagColor != b.TagColor }, func(j Job) string { return j.TagColor }},
	{"project", func(a, b Job) bool { return a.Project != b.Project }, func(j Job) string { return j.Project }},
}

// DiffJob returns the list of fields that differ between a and b.
func DiffJob(a, b Job) []FieldDiff {
	var changes []FieldDiff
	for _, c := range jobComparators {
		if c.differs(a, b) {
			changes = append(changes, FieldDiff{
				Field: c.name,
				Old:   c.format(a),
				New:   c.format(b),
			})
		}
	}
	return changes
}

// JobsDiffer reports whether a and b differ in any comparable field.
func JobsDiffer(a, b Job) bool {
	for _, c := range jobComparators {
		if c.differs(a, b) {
			return true
		}
	}
	return false
}
