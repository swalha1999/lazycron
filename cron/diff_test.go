package cron

import "testing"

func TestJobsDiffer_Identical(t *testing.T) {
	j := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true, Wrapped: true}
	if JobsDiffer(j, j) {
		t.Error("identical jobs should not differ")
	}
}

func TestJobsDiffer_WrappedOnly(t *testing.T) {
	a := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true, Wrapped: false}
	b := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true, Wrapped: true}
	if !JobsDiffer(a, b) {
		t.Error("difference in Wrapped should be detected")
	}
}

func TestDiffJob_NoChanges(t *testing.T) {
	j := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true}
	if changes := DiffJob(j, j); len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestDiffJob_WrappedOnly(t *testing.T) {
	a := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true, Wrapped: false}
	b := Job{Name: "A", Schedule: "* * * * *", Command: "echo", Enabled: true, Wrapped: true}
	changes := DiffJob(a, b)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Field != "wrapped" {
		t.Errorf("expected field 'wrapped', got %q", changes[0].Field)
	}
	if changes[0].Old != "false" || changes[0].New != "true" {
		t.Errorf("expected false → true, got %s → %s", changes[0].Old, changes[0].New)
	}
}

func TestDiffJob_AllFields(t *testing.T) {
	a := Job{
		Name: "old", Schedule: "* * * * *", Command: "echo old",
		Enabled: false, Wrapped: false, OneShot: false,
		Tag: "X", TagColor: "#000000", Project: "alpha",
	}
	b := Job{
		Name: "new", Schedule: "0 3 * * *", Command: "echo new",
		Enabled: true, Wrapped: true, OneShot: true,
		Tag: "Y", TagColor: "#ffffff", Project: "beta",
	}

	changes := DiffJob(a, b)
	want := []string{"name", "schedule", "command", "enabled", "wrapped", "once", "tag", "tag_color", "project"}
	if len(changes) != len(want) {
		t.Fatalf("expected %d changes, got %d", len(want), len(changes))
	}
	for i, w := range want {
		if changes[i].Field != w {
			t.Errorf("changes[%d].Field = %q, want %q", i, changes[i].Field, w)
		}
	}
}

func TestDiffJob_ScheduleQuoted(t *testing.T) {
	a := Job{Schedule: "* * * * *"}
	b := Job{Schedule: "0 3 * * *"}
	changes := DiffJob(a, b)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].Old != `"* * * * *"` || changes[0].New != `"0 3 * * *"` {
		t.Errorf("schedule should render quoted, got %s → %s", changes[0].Old, changes[0].New)
	}
}

func TestJobsDiffer_MatchesDiffJob(t *testing.T) {
	cases := []struct {
		a, b Job
	}{
		{Job{Name: "A"}, Job{Name: "A"}},
		{Job{Name: "A"}, Job{Name: "B"}},
		{Job{Wrapped: false}, Job{Wrapped: true}},
		{Job{OneShot: false}, Job{OneShot: true}},
	}
	for i, c := range cases {
		got := JobsDiffer(c.a, c.b)
		want := len(DiffJob(c.a, c.b)) > 0
		if got != want {
			t.Errorf("case %d: JobsDiffer=%v, len(DiffJob)>0=%v", i, got, want)
		}
	}
}
