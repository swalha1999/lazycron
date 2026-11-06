package runner

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/internal/fakehcbuild"
)

type fixture struct {
	t     *testing.T
	exec  *Executor
	state string // fake hc state dir
}

// newFixture returns an executor over temp dirs; with herdr it drives a
// freshly built fake hc.
func newFixture(t *testing.T, herdr bool) *fixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	f := &fixture{t: t, exec: &Executor{
		Paths:    PathsUnder(filepath.Join(home, ".lazycron")),
		SpaceID:  "sp" + strings.Repeat("a", 32),
		KeepRuns: 5,
		MinPoll:  10 * time.Millisecond,
	}}
	if herdr {
		bin, state := fakehcbuild.Build(t)
		f.state = state
		f.exec.HC = &HC{Bin: bin, Cmd: LocalCommander{}}
	}
	return f
}

func (f *fixture) script(jobID, body string) {
	f.t.Helper()
	if err := cron.WriteScriptIn(f.exec.Paths.Scripts, jobID, body); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) run(req Request) history.Entry {
	f.t.Helper()
	if req.Trigger == "" {
		req.Trigger = TriggerManual
	}
	e, err := f.exec.Run(req)
	if err != nil {
		f.t.Fatalf("Run: %v", err)
	}
	return e
}

func (f *fixture) calls() string {
	data, _ := os.ReadFile(filepath.Join(f.state, "calls.log"))
	return string(data)
}

func assertRecorded(t *testing.T, f *fixture, e history.Entry) {
	t.Helper()
	back, err := history.LoadEntry(filepath.Join(f.exec.Paths.History, history.EntryFileName(e.RunID)))
	if err != nil {
		t.Fatalf("history entry not written: %v", err)
	}
	if back.RunID != e.RunID || back.JobID != e.JobID {
		t.Fatalf("stored entry mismatch: %+v", back)
	}
	if active, _ := LoadActive(f.exec.Paths.Runs); len(active) != 0 {
		t.Fatalf("marker left behind: %+v", active)
	}
}

func TestPlainRunRecordsExitAndOutput(t *testing.T) {
	f := newFixture(t, false)
	f.script("job1", "echo out; echo err >&2; exit 3")
	e := f.run(Request{JobID: "job1", JobName: "Job One", Runner: config.RunnerPlain})

	if *e.Success || e.ExitCode == nil || *e.ExitCode != 3 {
		t.Fatalf("want exit 3 failure, got %+v", e)
	}
	if e.Output != "out\nerr" || e.Runner != config.RunnerPlain || e.FallbackReason != "" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	assertRecorded(t, f, e)
}

func TestHerdrRunPropagatesExitCodeAndLabels(t *testing.T) {
	f := newFixture(t, true)
	f.script("job2", "echo from-herdr; exit 7")
	e := f.run(Request{JobID: "job2", JobName: "Job Two", Runner: config.RunnerHerdr})

	if e.ExitCode == nil || *e.ExitCode != 7 || *e.Success {
		t.Fatalf("want exit 7, got %+v", e)
	}
	if e.Runner != config.RunnerHerdr || e.Terminal == "" || e.Output != "from-herdr" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	calls := f.calls()
	for _, want := range []string{"run --bg --no-parent --no-notify --name cron-job2-",
		"--label herdr.space=" + f.exec.SpaceID, "--label lazycron.job=job2", "--label lazycron.run=" + e.RunID} {
		if !strings.Contains(calls, want) {
			t.Errorf("hc run call missing %q:\n%s", want, calls)
		}
	}
	assertRecorded(t, f, e)
}

func TestHerdrRunHoldKeepsTerminalLiveButReturns(t *testing.T) {
	f := newFixture(t, true)
	f.exec.Hold = time.Hour
	f.script("held", "echo done-work")
	start := time.Now()
	e := f.run(Request{JobID: "held", JobName: "Held", Runner: config.RunnerHerdr})
	if time.Since(start) > 20*time.Second {
		t.Fatal("hold must not delay the result")
	}
	if !*e.Success || e.Output != "done-work" {
		t.Fatalf("unexpected entry: %+v", e)
	}
	term, err := f.exec.HC.Get(e.Terminal)
	if err != nil || !term.Live() {
		t.Fatalf("held terminal should be live: %+v %v", term, err)
	}
	_ = f.exec.HC.Kill(e.Terminal)
}

func TestHerdrFallsBackWhenRunFails(t *testing.T) {
	f := newFixture(t, true)
	t.Setenv("FAKEHC_FAIL_RUN", "1")
	f.script("fb", "echo plain-anyway")
	e := f.run(Request{JobID: "fb", JobName: "FB", Runner: config.RunnerHerdr})
	if e.Runner != config.RunnerPlain || !strings.Contains(e.FallbackReason, "core unreachable") {
		t.Fatalf("expected plain fallback, got %+v", e)
	}
	if !*e.Success || e.Output != "plain-anyway" {
		t.Fatalf("job must still run: %+v", e)
	}
}

func TestHerdrMissingFallsBack(t *testing.T) {
	f := newFixture(t, false)
	f.exec.HCUnavailable = "herdr-core (hc) not found"
	f.script("nohc", "true")
	e := f.run(Request{JobID: "nohc", JobName: "x", Runner: config.RunnerHerdr})
	if e.Runner != config.RunnerPlain || e.FallbackReason != "herdr-core (hc) not found" || !*e.Success {
		t.Fatalf("unexpected: %+v", e)
	}
}

func TestHerdrKilledRunRecordsSignal(t *testing.T) {
	f := newFixture(t, true)
	f.script("slow", "sleep 30")
	f.exec.Started = func(a Active) {
		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = f.exec.HC.Kill(a.Terminal)
		}()
	}
	e := f.run(Request{JobID: "slow", JobName: "Slow", Runner: config.RunnerHerdr})
	if *e.Success || e.Signal == 0 || e.ExitCode != nil {
		t.Fatalf("expected killed run, got %+v", e)
	}
}

func TestHerdrTruncatedOutputIsFlagged(t *testing.T) {
	f := newFixture(t, true)
	t.Setenv("FAKEHC_TRUNCATE", "1")
	f.script("big", "seq 1 5")
	e := f.run(Request{JobID: "big", JobName: "Big", Runner: config.RunnerHerdr})
	if !e.OutputTruncated {
		t.Fatalf("expected output_truncated: %+v", e)
	}
}

func TestRetentionKeepsNewestRuns(t *testing.T) {
	f := newFixture(t, true)
	f.exec.KeepRuns = 2
	f.script("ret", "true")
	var last history.Entry
	for i := 0; i < 4; i++ {
		last = f.run(Request{JobID: "ret", JobName: "Ret", Runner: config.RunnerHerdr})
	}
	terms, err := f.exec.HC.List(map[string]string{LabelJob: "ret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(terms) != 2 {
		t.Fatalf("want 2 kept terminals, got %d", len(terms))
	}
	found := false
	for _, term := range terms {
		found = found || term.Address == last.Terminal
	}
	if !found {
		t.Fatal("newest run's terminal must be kept")
	}
}

func TestOnceDisablesJob(t *testing.T) {
	f := newFixture(t, false)
	f.script("one", "true")
	var disabled string
	f.exec.DisableOnce = func(id string) error { disabled = id; return nil }
	f.run(Request{JobID: "one", JobName: "One", Runner: config.RunnerPlain, Once: true})
	if disabled != "one" {
		t.Fatalf("DisableOnce not called, got %q", disabled)
	}
}

func TestTerminalName(t *testing.T) {
	re := regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	runID := NewRunID(strings.Repeat("x", 64), time.Now())
	for _, job := range []string{"a", "nightly-backup", strings.Repeat("x", 64)} {
		name := terminalName(job, runID)
		if !re.MatchString(name) {
			t.Errorf("bad terminal name %q", name)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		have, want string
		ok         bool
	}{
		{"0.1.18", "0.1.18", true}, {"0.1.19", "0.1.18", true}, {"0.2.0", "0.1.18", true},
		{"0.1.9", "0.1.18", false}, {"0.1.18-rc1", "0.1.18", true}, {"1.0", "0.1.18", true},
	}
	for _, c := range cases {
		if got := VersionAtLeast(c.have, c.want); got != c.ok {
			t.Errorf("VersionAtLeast(%q,%q)=%v", c.have, c.want, got)
		}
	}
}

func TestStripTrailer(t *testing.T) {
	in := "work\n\n" + finishedMarker + " exit 0 at 2026-11-06 02:00. Kept for viewing"
	if got := stripTrailer(in); got != "work" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadActiveMarksStale(t *testing.T) {
	dir := t.TempDir()
	alive := Active{RunID: "b", JobID: "j", ExecPID: os.Getpid()}
	dead := Active{RunID: "a", JobID: "j", ExecPID: 1 << 22}
	for _, a := range []Active{alive, dead} {
		if err := WriteActive(dir, a); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := LoadActive(dir)
	if err != nil || len(runs) != 2 {
		t.Fatalf("got %v %v", runs, err)
	}
	if runs[0].RunID != "b" || runs[0].Stale || !runs[1].Stale {
		t.Fatalf("unexpected staleness/order: %+v", runs)
	}
}

func TestFindHCPrefersConfigured(t *testing.T) {
	p := filepath.Join(t.TempDir(), "my-hc")
	if err := os.WriteFile(p, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindHC(p)
	if err != nil || got != p {
		t.Fatalf("FindHC = %q, %v", got, err)
	}
	if _, err := FindHC(p + "-missing"); err == nil {
		t.Fatal("missing configured binary must error")
	}
}

func TestExtraLabelsAddAndRemove(t *testing.T) {
	f := newFixture(t, true)
	f.exec.ExtraLabels = map[string]string{"lazycron-test": "1", LabelSpace: ""}
	f.script("lab", "true")
	f.run(Request{JobID: "lab", JobName: "Lab", Runner: config.RunnerHerdr})
	calls := f.calls()
	if !strings.Contains(calls, "--label lazycron-test=1") || strings.Contains(calls, LabelSpace) {
		t.Fatalf("labels not applied:\n%s", calls)
	}
}
