package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func addArgs(id, name, schedule, command string, extra ...string) []string {
	return append([]string{"add", "--id", id, "--name", name, "--schedule", schedule, "--command", command}, extra...)
}

func TestAddIsIdempotentAndDetectsConflicts(t *testing.T) {
	env := newTestEnv(t)
	var res mutationResult
	env.mustRun(addArgs("backup", "Backup", "every day at 2am", "echo hi")...).decode(t, &res)
	if res.Action != actionCreated || res.Job.Schedule != "0 2 * * *" {
		t.Fatalf("unexpected: %+v", res)
	}
	env.mustRun(addArgs("backup", "Backup", "0 2 * * *", "echo hi")...).decode(t, &res)
	if res.Action != actionUnchanged {
		t.Fatalf("second add should be unchanged: %+v", res)
	}
	r := env.run(addArgs("backup", "Backup", "0 3 * * *", "echo hi")...)
	if r.code != exitConflict || r.errorKind(t) != kindConflict {
		t.Fatalf("expected conflict, got %d %s", r.code, r.stderr)
	}
	if n := len(env.jobs()); n != 1 {
		t.Fatalf("want 1 job, got %d", n)
	}
}

func TestAddWithoutIDMatchesByName(t *testing.T) {
	env := newTestEnv(t)
	args := []string{"add", "-n", "Report", "-s", "every hour", "-c", "date"}
	env.mustRun(args...)
	var res mutationResult
	env.mustRun(args...).decode(t, &res)
	if res.Action != actionUnchanged || len(env.jobs()) != 1 {
		t.Fatalf("re-adding by name should be a no-op: %+v", res)
	}
}

func TestAddDryRunWritesNothing(t *testing.T) {
	env := newTestEnv(t)
	var res mutationResult
	env.mustRun(addArgs("x", "X", "every hour", "true", "--dry-run")...).decode(t, &res)
	if !res.DryRun || res.Action != actionCreated {
		t.Fatalf("unexpected: %+v", res)
	}
	if _, err := os.Stat(env.cronFile); !os.IsNotExist(err) {
		t.Fatal("dry run must not create the crontab")
	}
}

func TestAddValidatesInput(t *testing.T) {
	env := newTestEnv(t)
	for _, args := range [][]string{
		addArgs("x", "X", "not a schedule", "true"),
		addArgs("Bad ID", "X", "every hour", "true"),
		addArgs("x", "X", "every hour", "true", "--runner", "docker"),
		addArgs("x", "a\n* * * * * evil", "every hour", "true"),
		addArgs("x", "Disk 90% alert", "every hour", "true"),
		addArgs("x", "X", "every hour", "true", "--tag", "a\rb"),
		addArgs("x", "X", "every hour", "true", "--project", "50%"),
		{"add", "--name", "X"},
	} {
		if r := env.run(args...); r.code != exitUsage {
			t.Errorf("%v: exit %d, want usage (stderr %s)", args, r.code, r.stderr)
		}
	}
}

func TestUnsafeLabelsAreRejected(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("job1", "Job One", "every hour", "true")...)
	for _, args := range [][]string{
		{"edit", "job1", "--name", "Disk 90% alert"},
		{"edit", "job1", "--project", "a\nb"},
	} {
		if r := env.run(args...); r.code != exitUsage {
			t.Errorf("%v: exit %d, want usage (stderr %s)", args, r.code, r.stderr)
		}
	}
	file := filepath.Join(env.home, "evil.yml")
	writeFile(t, file, "jobs:\n  - name: \"a\\n* * * * * evil\"\n    schedule: every hour\n    command: x\n")
	if r := env.run("apply", file); r.code != exitUsage {
		t.Errorf("apply: exit %d, want usage (stderr %s)", r.code, r.stderr)
	}
	if jobs := env.jobs(); len(jobs) != 1 || jobs[0].Name != "Job One" || jobs[0].Project != "" {
		t.Fatalf("crontab changed: %+v", jobs)
	}
}

func TestGetEditEnableDisableRm(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("job1", "Job One", "every hour", "echo one", "--project", "p1")...)

	var detail jobDetail
	env.mustRun("get", "Job One").decode(t, &detail)
	if detail.ID != "job1" || detail.Project != "p1" || detail.LastRun != nil || len(detail.NextRuns) == 0 {
		t.Fatalf("unexpected get: %+v", detail)
	}
	if r := env.run("get", "nope"); r.code != exitNotFound {
		t.Fatalf("get missing: exit %d", r.code)
	}

	var res mutationResult
	env.mustRun("edit", "job1", "--schedule", "every day at 3am", "--tag", "AI").decode(t, &res)
	if res.Action != actionUpdated || res.Job.Schedule != "0 3 * * *" || res.Job.Tag != "AI" || len(res.Changes) < 2 {
		t.Fatalf("unexpected edit: %+v", res)
	}
	env.mustRun("edit", "job1", "--tag", "AI").decode(t, &res)
	if res.Action != actionUnchanged {
		t.Fatalf("no-op edit: %+v", res)
	}
	if r := env.run("edit", "job1", "--workdir", "/tmp"); r.code != exitUsage {
		t.Fatalf("--workdir alone: exit %d", r.code)
	}

	env.mustRun("disable", "job1").decode(t, &res)
	if res.Action != actionUpdated || res.Job.Enabled {
		t.Fatalf("disable: %+v", res)
	}
	env.mustRun("disable", "job1").decode(t, &res)
	if res.Action != actionUnchanged {
		t.Fatalf("second disable should be a no-op: %+v", res)
	}
	env.mustRun("enable", "job1").decode(t, &res)
	if !res.Job.Enabled {
		t.Fatal("enable failed")
	}

	env.mustRun("rm", "job1", "--dry-run").decode(t, &res)
	if res.Action != actionDeleted || len(env.jobs()) != 1 {
		t.Fatal("dry-run rm must keep the job")
	}
	env.mustRun("rm", "job1")
	if len(env.jobs()) != 0 {
		t.Fatal("rm did not delete")
	}
	if r := env.run("rm", "job1"); r.code != exitNotFound {
		t.Fatalf("rm missing: exit %d", r.code)
	}
}

func TestAmbiguousNameIsConflict(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("a1", "Same", "every hour", "true")...)
	env.mustRun(addArgs("a2", "Same", "every day at 1am", "true")...)
	if r := env.run("get", "Same"); r.code != exitConflict {
		t.Fatalf("exit %d", r.code)
	}
}

func TestRunnerOverrideIsWrittenToCrontab(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("ag", "Agent", "every hour", "claude -p hi", "--runner", "herdr")...)
	data, err := os.ReadFile(env.cronFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "@runner:herdr") || !strings.Contains(string(data), " exec 'ag'") {
		t.Fatalf("crontab lacks herdr exec line:\n%s", data)
	}
	jobs := env.jobs()
	if jobs[0].Runner != "herdr" || !jobs[0].RunnerSet || jobs[0].Command != "claude -p hi" {
		t.Fatalf("unexpected job: %+v", jobs[0])
	}
	var res mutationResult
	env.mustRun("edit", "ag", "--runner", "").decode(t, &res)
	if res.Job.Runner != "plain" || res.Job.RunnerSet {
		t.Fatalf("clearing the override should fall back to the default: %+v", res.Job)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCreatesUpdatesAndPrunes(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("other", "Other project", "every hour", "true", "--project", "keep")...)
	env.mustRun(addArgs("old", "Old", "every hour", "true", "--project", "ops")...)

	file := filepath.Join(env.home, "jobs.yml")
	writeFile(t, file, `project: ops
jobs:
  - name: Nightly Backup
    schedule: every day at 2am
    command: backup.sh
    workdir: /srv
  - id: report
    name: Report
    schedule: "0 9 * * 1"
    command: report.sh
    runner: herdr
`)
	var res applyResult
	env.mustRun("apply", file, "--dry-run").decode(t, &res)
	if !res.DryRun || res.New != 2 || len(env.jobs()) != 2 {
		t.Fatalf("dry run: %+v", res)
	}

	env.mustRun("apply", file).decode(t, &res)
	if res.New != 2 || res.Deleted != 0 || !res.Changed {
		t.Fatalf("first apply: %+v", res)
	}
	env.mustRun("apply", file).decode(t, &res)
	if res.Changed || res.Unchanged != 2 {
		t.Fatalf("second apply must be a no-op: %+v", res)
	}

	writeFile(t, file, `project: ops
jobs:
  - id: report
    name: Report
    schedule: "0 10 * * 1"
    command: report.sh
`)
	env.mustRun("apply", file, "--prune").decode(t, &res)
	if res.Updated != 1 || res.Deleted != 2 {
		t.Fatalf("prune apply: %+v", res)
	}
	ids := map[string]bool{}
	for _, j := range env.jobs() {
		ids[j.ID] = true
	}
	if !ids["other"] || !ids["report"] || ids["old"] || ids["nightly-backup"] || len(ids) != 2 {
		t.Fatalf("prune must stay inside project ops: %v", ids)
	}
}

func TestApplyRejectsBadFiles(t *testing.T) {
	env := newTestEnv(t)
	file := filepath.Join(env.home, "bad.yml")
	writeFile(t, file, "jobs:\n  - name: A\n    schedule: every hour\n    command: x\n  - name: A\n    schedule: every day at 1am\n    command: y\n")
	if r := env.run("apply", file); r.code != exitUsage {
		t.Fatalf("duplicate ids: exit %d %s", r.code, r.stderr)
	}
	if r := env.run("apply", filepath.Join(env.home, "missing.yml")); r.code != exitNotFound {
		t.Fatalf("missing file: exit %d", r.code)
	}
}

func TestExportRoundTripsThroughApply(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("a", "A job", "every day at 2am", "echo a", "--workdir", "/tmp", "--tag", "T", "--tag-color", "#fff")...)
	env.mustRun(addArgs("b", "B job", "every hour", "echo b", "--disabled", "--runner", "herdr")...)
	env.mustRun(addArgs("c", "C job", "2030-01-02 03:04", "echo c", "--once")...)

	out := env.mustRun("export", "--json=false").stdout
	file := filepath.Join(env.home, "export.yml")
	writeFile(t, file, out)
	var res applyResult
	env.mustRun("apply", file).decode(t, &res)
	if res.Changed || res.Unchanged != 3 {
		t.Fatalf("export -> apply should be a no-op: %+v\n%s", res, out)
	}
}
