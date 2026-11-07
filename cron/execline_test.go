package cron

import (
	"path/filepath"
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
	if raw != "'"+LazycronBinPath()+"' exec 'backup' --name 'Nightly '\\''backup'\\''' --once" {
		t.Fatalf("exec line not clean: %q", raw)
	}
	if !IsCurrentFormat(raw) {
		t.Fatal("exec line should count as current format")
	}
	if got := jobCommand(raw, ScriptsDir()); got != "bash '"+ScriptPath("backup")+"'" {
		t.Fatalf("jobCommand = %q", got)
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
	withDefaultRunner(t, config.RunnerPlain)
	content := FormatCrontab([]Job{
		{ID: "x", Name: "X", Schedule: "* * * * *", Command: "true", Enabled: true, Runner: config.RunnerHerdr},
		{ID: "p", Name: "P", Schedule: "* * * * *", Command: "true", Enabled: true},
	})
	out := RelocateCrontab(content, "/r/.lazycron/scripts", "/r/.lazycron/bin")
	if strings.Contains(out, ScriptsDir()) || strings.Contains(out, LazycronBinPath()) || strings.Contains(out, RecordBinPath()) {
		t.Fatalf("local paths left: %s", out)
	}
	for _, want := range []string{"'/r/.lazycron/bin/lazycron' exec 'x'", "/r/.lazycron/scripts/p.sh", "/r/.lazycron/bin/record"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}

func TestExecLineHasNoFallback(t *testing.T) {
	withDefaultRunner(t, config.RunnerHerdr)
	line := Job{ID: "x", Name: "X", Schedule: "* * * * *", Command: "true", Enabled: true}.CrontabLine()
	if strings.Contains(line, "||") || strings.Contains(line, "record") || strings.Contains(line, ".sh") {
		t.Fatalf("herdr line should only call lazycron exec: %s", line)
	}
}

func TestLegacyExecLineParsesAndNeedsRewrite(t *testing.T) {
	withDefaultRunner(t, config.RunnerHerdr)
	legacy := "# Old @id:old\n0 * * * * '" + LazycronBinPath() + "' exec 'old' --name 'Old' || { " +
		WrapWithRecord("bash '"+ScriptPath("old")+"'", "old", "Old") + "; }\n"
	jobs := Parse(legacy)
	if len(jobs) != 1 {
		t.Fatalf("parsed %d jobs", len(jobs))
	}
	j := jobs[0]
	if !j.ExecLine || j.Wrapped || !j.NeedsRewrite() {
		t.Fatalf("legacy exec line should parse as an old format needing rewrite: %+v", j)
	}
	if j.ID != "old" || j.Name != "Old" {
		t.Fatalf("legacy parse mismatch: %+v", j)
	}
	rewritten := Parse(j.CrontabLine())[0]
	if rewritten.NeedsRewrite() || !rewritten.Wrapped {
		t.Fatalf("rewritten line should be current: %+v", rewritten)
	}
}

func TestParseInResolvesScriptsInGivenDir(t *testing.T) {
	withDefaultRunner(t, config.RunnerHerdr)
	dir := filepath.Join(t.TempDir(), ".lazycron", "scripts") // a server's or crontab file's scripts dir
	if err := WriteScriptIn(dir, "s1", "echo from-dir"); err != nil {
		t.Fatal(err)
	}
	line := Job{ID: "s1", Name: "S", Schedule: "* * * * *", Command: "x", Enabled: true}.CrontabLine()
	if got := ParseIn(line, dir)[0].Command; got != "echo from-dir" {
		t.Fatalf("Command = %q, want the script's command from %s", got, dir)
	}
}

func TestFormatCrontabForUsesGivenDefault(t *testing.T) {
	withDefaultRunner(t, config.RunnerPlain)
	jobs := []Job{
		{ID: "a", Name: "A", Schedule: "* * * * *", Command: "true", Enabled: true},
		{ID: "b", Name: "B", Schedule: "* * * * *", Command: "true", Enabled: true, Runner: config.RunnerPlain},
	}
	out := FormatCrontabFor(jobs, config.RunnerHerdr)
	if !strings.Contains(out, " exec 'a'") || strings.Contains(out, " exec 'b'") {
		t.Fatalf("server default must apply only to jobs without an override:\n%s", out)
	}
	if strings.Contains(FormatCrontab(jobs), " exec ") {
		t.Fatal("FormatCrontab must keep using DefaultRunner")
	}
}
