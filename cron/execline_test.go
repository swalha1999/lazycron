package cron

import (
	"strings"
	"testing"

	"github.com/swalha1999/lazycron/config"
)

func withDefaultRunner(t *testing.T, runner string) {
	t.Helper()
	orig := DefaultRunner
	DefaultRunner = runner
	t.Cleanup(func() { DefaultRunner = orig })
}

func TestHerdrLineRoundTrip(t *testing.T) {
	withDefaultRunner(t, config.RunnerHerdr)
	job := Job{ID: "backup", Name: "Nightly 'backup'", Schedule: "0 2 * * *", Command: "echo hi", Enabled: true, OneShot: true}
	line := job.CrontabLine()

	cmdLine := strings.SplitN(line, "\n", 2)[1]
	_, raw, _ := splitCronLine(cmdLine)
	if !IsExecLine(raw) {
		t.Fatalf("expected exec line, got %q", raw)
	}
	if !strings.Contains(raw, LazycronBinPath()+"' exec 'backup' --name 'Nightly '\\''backup'\\''' --once; else ") {
		t.Fatalf("exec call not as expected: %q", raw)
	}
	if !IsCurrentFormat(raw) {
		t.Fatal("exec line should count as current format")
	}
	if got := StripRecord(raw); got != "bash '"+ScriptPath("backup")+"'" {
		t.Fatalf("StripRecord = %q", got)
	}

	jobs := Parse(line)
	if len(jobs) != 1 {
		t.Fatalf("parsed %d jobs", len(jobs))
	}
	got := jobs[0]
	if got.ID != "backup" || got.Name != job.Name || !got.OneShot || !got.Wrapped || got.Runner != "" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestPlainLineUnchangedByDefault(t *testing.T) {
	withDefaultRunner(t, config.RunnerPlain)
	job := Job{ID: "a1", Name: "A", Schedule: "* * * * *", Command: "true", Enabled: true}
	if strings.Contains(job.CrontabLine(), "exec") {
		t.Fatalf("plain runner must keep the record line: %s", job.CrontabLine())
	}
}

func TestRunnerOverrideMarker(t *testing.T) {
	withDefaultRunner(t, config.RunnerPlain)
	job := Job{ID: "a1", Name: "Agent", Schedule: "* * * * *", Command: "true", Enabled: false,
		Runner: config.RunnerHerdr, OneShot: true, Tag: "AI", TagColor: "#fff", Project: "p"}
	line := job.CrontabLine()
	if !strings.HasPrefix(line, "# Agent @id:a1 @runner:herdr @once [AI:#fff] {p}\n#DISABLED ") {
		t.Fatalf("unexpected line: %s", line)
	}
	if !strings.Contains(line, " exec 'a1'") {
		t.Fatal("override should produce an exec line")
	}
	got := Parse(line)[0]
	if got.Runner != config.RunnerHerdr || got.Name != "Agent" || got.Project != "p" || got.Tag != "AI" || !got.OneShot || got.Enabled {
		t.Fatalf("parse mismatch: %+v", got)
	}
}

func TestExtractRunnerIgnoresUnknown(t *testing.T) {
	name, runner := extractRunner("Job @runner:docker")
	if runner != "" || name != "Job @runner:docker" {
		t.Fatalf("got %q %q", name, runner)
	}
}

func TestRelocateCrontab(t *testing.T) {
	withDefaultRunner(t, config.RunnerHerdr)
	content := FormatCrontab([]Job{{ID: "x", Name: "X", Schedule: "* * * * *", Command: "true", Enabled: true}})
	out := RelocateCrontab(content, "/r/.lazycron/scripts", "/r/.lazycron/bin")
	if strings.Contains(out, ScriptsDir()) || strings.Contains(out, LazycronBinPath()) || strings.Contains(out, RecordBinPath()) {
		t.Fatalf("local paths left: %s", out)
	}
	for _, want := range []string{"/r/.lazycron/scripts/x.sh", "/r/.lazycron/bin/lazycron", "/r/.lazycron/bin/record"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}
