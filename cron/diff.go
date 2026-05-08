package cron

import "fmt"

// FieldDiff describes a single Job field that differs between two jobs.
type FieldDiff struct {
	Field string
	Old   string
	New   string
}

// jobFieldComparators is the single source of truth for which Job fields
// participate in equality and diffing. Both DiffJob and JobsDiffer iterate
// this list, so adding a new comparable field to Job is a single edit here.
var jobFieldComparators = []struct {
	name    string
	differs func(a, b Job) bool
	show    func(j Job) string
}{
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

// DiffJob returns the list of fields that differ between a and b, in the
// canonical order defined by jobFieldComparators.
func DiffJob(a, b Job) []FieldDiff {
	var diffs []FieldDiff
	for _, c := range jobFieldComparators {
		if c.differs(a, b) {
			diffs = append(diffs, FieldDiff{Field: c.name, Old: c.show(a), New: c.show(b)})
		}
	}
	return diffs
}

// JobsDiffer reports whether a and b differ in any comparable field.
func JobsDiffer(a, b Job) bool {
	return len(DiffJob(a, b)) > 0
}
