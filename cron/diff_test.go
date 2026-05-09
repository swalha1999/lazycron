package cron

import "testing"

func TestJobsDiffer_Identical(t *testing.T) {
	j := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true}
	if JobsDiffer(j, j) {
		t.Error("identical jobs should not differ")
	}
}

func TestJobsDiffer_ScheduleChange(t *testing.T) {
	a := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true}
	b := Job{Name: "A", Schedule: "0 3 * * *", Command: "echo", Enabled: true}
	if !JobsDiffer(a, b) {
		t.Error("schedule change should be detected")
	}
}

func TestJobsDiffer_WrappedChange(t *testing.T) {
	// Regression for issue #80: Wrapped used to be checked in sync but not
	// in diff, so a wrapped-only change rendered as a blank update line.
	a := Job{Name: "A", Wrapped: false}
	b := Job{Name: "A", Wrapped: true}
	if !JobsDiffer(a, b) {
		t.Error("wrapped change should be detected")
	}
	diffs := DiffJob(a, b)
	if len(diffs) != 1 || diffs[0].Field != "wrapped" {
		t.Errorf("expected single wrapped diff, got %+v", diffs)
	}
}

func TestDiffJob_NoChanges(t *testing.T) {
	j := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true}
	if d := DiffJob(j, j); len(d) != 0 {
		t.Errorf("expected 0 diffs, got %d", len(d))
	}
}

func TestDiffJob_AllFieldsCovered(t *testing.T) {
	// Two jobs that disagree on every comparable field should produce one
	// diff per comparator — this guards against accidentally dropping a
	// field from jobFieldComparators.
	a := Job{
		Name: "a", Schedule: "* * * * *", Command: "cmd-a", Enabled: false,
		Wrapped: false, OneShot: false, Tag: "ta", TagColor: "#aaa", Project: "pa",
	}
	b := Job{
		Name: "b", Schedule: "0 3 * * *", Command: "cmd-b", Enabled: true,
		Wrapped: true, OneShot: true, Tag: "tb", TagColor: "#bbb", Project: "pb",
	}
	diffs := DiffJob(a, b)
	if len(diffs) != len(jobFieldComparators) {
		t.Fatalf("expected %d diffs (one per comparator), got %d", len(jobFieldComparators), len(diffs))
	}
}

func TestDiffJob_FormatsScheduleQuoted(t *testing.T) {
	a := Job{Schedule: "* * * * *"}
	b := Job{Schedule: "0 3 * * *"}
	diffs := DiffJob(a, b)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Old != `"* * * * *"` || diffs[0].New != `"0 3 * * *"` {
		t.Errorf("expected quoted schedule strings, got old=%q new=%q", diffs[0].Old, diffs[0].New)
	}
}
