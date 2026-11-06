package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/internal/fakehcbuild"
)

// useHerdr configures the env for the herdr runner backed by a fake hc,
// with no hold so terminals end with their job.
func (e *testEnv) useHerdr() (hcState string) {
	e.t.Helper()
	bin, state := fakehcbuild.Build(e.t)
	cfg := "runner: herdr\nherdr:\n  bin: " + bin + "\n  hold: \"0\"\n  space_id: sp" + strings.Repeat("0", 32) + "\n"
	if err := os.MkdirAll(filepath.Join(e.home, ".lazycron"), 0o700); err != nil {
		e.t.Fatal(err)
	}
	writeFile(e.t, filepath.Join(e.home, ".lazycron", "config.yml"), cfg)
	return state
}

// detachInProcess makes --no-wait run the exec in a goroutine (the test
// binary cannot be re-executed as lazycron). It returns a wait func.
func detachInProcess(t *testing.T, env *testEnv) func() {
	t.Helper()
	var wg sync.WaitGroup
	orig := backend.SpawnDetached
	backend.SpawnDetached = func(args []string) error {
		// args: --cron-file <f> exec <id> --name <n> --runner <r> --trigger <t> --run-id <id>
		opts := map[string]string{}
		for i := 4; i+1 < len(args); i += 2 {
			opts[args[i]] = args[i+1]
		}
		job := cron.Job{ID: args[3], Name: opts["--name"], Runner: opts["--runner"]}
		fb := backend.NewFileBackend(env.cronFile, env.cronFile+".history")
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = fb.ExecJob(job, backend.ExecRequest{RunID: opts["--run-id"], Trigger: opts["--trigger"]})
		}()
		return nil
	}
	t.Cleanup(func() { backend.SpawnDetached = orig })
	return wg.Wait
}

func TestRunPlainRecordsAndLists(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("hello", "Hello", "every hour", "echo hello-world")...)

	var run runView
	env.mustRun("run", "hello").decode(t, &run)
	if run.State != stateSucceeded || run.Runner != "plain" || run.Output == nil || *run.Output != "hello-world" {
		t.Fatalf("unexpected run: %+v", run)
	}

	var list struct{ Runs []runView }
	env.mustRun("runs", "hello").decode(t, &list)
	if len(list.Runs) != 1 || list.Runs[0].ID != run.ID || list.Runs[0].Trigger != "manual" {
		t.Fatalf("runs: %+v", list.Runs)
	}

	var logged runView
	env.mustRun("logs", run.ID).decode(t, &logged)
	if logged.Output == nil || *logged.Output != "hello-world" {
		t.Fatalf("logs <run>: %+v", logged)
	}
	if out := env.mustRun("logs", "Hello", "--json=false").stdout; !strings.Contains(out, "✓") {
		t.Fatalf("logs <job> text: %q", out)
	}

	var waited runView
	env.mustRun("wait", "hello").decode(t, &waited)
	if waited.ID != run.ID {
		t.Fatalf("wait <job> should return the finished run: %+v", waited)
	}

	var detail jobDetail
	env.mustRun("get", "hello").decode(t, &detail)
	if detail.LastRun == nil || detail.LastRun.ID != run.ID {
		t.Fatalf("get should show the last run: %+v", detail.LastRun)
	}
}

func TestRunFailingJobExitsOne(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("bad", "Bad", "every hour", "echo boom; exit 9")...)
	r := env.run("run", "bad")
	if r.code != exitFailure || r.errorKind(t) != kindJobFailed {
		t.Fatalf("exit %d stderr %s", r.code, r.stderr)
	}
	var run runView
	r.decode(t, &run)
	if run.State != stateFailed || run.ExitCode == nil || *run.ExitCode != 9 {
		t.Fatalf("unexpected run: %+v", run)
	}
}

func TestRunDryRunDoesNotRun(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("x", "X", "every hour", "true")...)
	env.mustRun("run", "x", "--dry-run")
	var list struct{ Runs []runView }
	env.mustRun("runs").decode(t, &list)
	if len(list.Runs) != 0 {
		t.Fatal("dry run must not run the job")
	}
}

func TestLogsForJobWithoutRuns(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("quiet", "Quiet", "every hour", "true")...)
	var out struct{ Runs []runView }
	env.mustRun("logs", "quiet").decode(t, &out)
	if len(out.Runs) != 0 {
		t.Fatal("expected no runs")
	}
	if r := env.run("logs", "no-such-job"); r.code != exitNotFound {
		t.Fatalf("exit %d", r.code)
	}
}

func TestRunThroughHerdr(t *testing.T) {
	env := newTestEnv(t)
	state := env.useHerdr()
	env.mustRun(addArgs("agent", "Agent", "every hour", "echo via-herdr; exit 3")...)

	r := env.run("run", "agent")
	var run runView
	r.decode(t, &run)
	if r.code != exitFailure || run.Runner != "herdr" || run.Terminal == "" || run.ExitCode == nil || *run.ExitCode != 3 {
		t.Fatalf("unexpected herdr run (exit %d): %+v", r.code, run)
	}
	if *run.Output != "via-herdr" {
		t.Fatalf("output %q", *run.Output)
	}
	calls, _ := os.ReadFile(filepath.Join(state, "calls.log"))
	if !strings.Contains(string(calls), "--label lazycron.job=agent") {
		t.Fatalf("hc not called as expected:\n%s", calls)
	}

	data, _ := os.ReadFile(env.cronFile)
	if !strings.Contains(string(data), " exec 'agent' --name 'Agent'") {
		t.Fatalf("herdr default should write exec lines:\n%s", data)
	}
}

func TestNoWaitThenWaitAndKill(t *testing.T) {
	env := newTestEnv(t)
	wait := detachInProcess(t, env)
	env.mustRun(addArgs("slow", "Slow", "every hour", "sleep 20")...)

	var started runView
	env.mustRun("run", "slow", "--no-wait").decode(t, &started)
	if started.State != stateRunning || started.ID == "" {
		t.Fatalf("expected a running run: %+v", started)
	}

	r := env.run("wait", started.ID, "--timeout", "300ms")
	if r.code != exitTimeout || r.errorKind(t) != kindTimeout {
		t.Fatalf("wait timeout: exit %d %s", r.code, r.stderr)
	}

	var killed killResult
	env.mustRun("kill", started.ID, "--dry-run").decode(t, &killed)
	if killed.Action != "killed" || !killed.DryRun {
		t.Fatalf("dry-run kill: %+v", killed)
	}
	env.mustRun("kill", "slow").decode(t, &killed)
	if killed.Action != "killed" {
		t.Fatalf("kill: %+v", killed)
	}

	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("killed run did not end")
	}
	r = env.run("wait", started.ID)
	var final runView
	r.decode(t, &final)
	if r.code != exitFailure || final.State != stateFailed || final.Signal == 0 {
		t.Fatalf("after kill: exit %d %+v", r.code, final)
	}
}

func TestExecIsQuietAndRecords(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("cronjob", "Cron Job", "every hour", "echo from-cron")...)
	r := env.mustRun("exec", "cronjob", "--name", "Cron Job", "--runner", "plain")
	if r.stdout != "" {
		t.Fatalf("exec must print nothing for cron, got %q", r.stdout)
	}
	var list struct{ Runs []runView }
	env.mustRun("runs").decode(t, &list)
	if len(list.Runs) != 1 || list.Runs[0].Trigger != "cron" || list.Runs[0].State != stateSucceeded {
		t.Fatalf("exec run not recorded: %+v", list.Runs)
	}
	var entry struct {
		RunID  string `json:"run_id"`
		Output string `json:"output"`
	}
	env.mustRun("exec", "cronjob", "--runner", "plain", "--json").decode(t, &entry)
	if entry.RunID == "" || entry.Output != "from-cron" {
		t.Fatalf("exec --json: %+v", entry)
	}
}

func TestExecOnceDisablesJob(t *testing.T) {
	env := newTestEnv(t)
	env.mustRun(addArgs("one", "One", "2030-01-01 09:00", "true", "--once")...)
	env.mustRun("exec", "one", "--runner", "plain", "--once")
	if jobs := env.jobs(); jobs[0].Enabled {
		t.Fatal("one-shot job should be disabled after exec --once")
	}
}

func TestAttachNeedsTTY(t *testing.T) {
	env := newTestEnv(t)
	if r := env.run("attach", "anything"); r.code != exitUsage {
		t.Fatalf("exit %d %s", r.code, r.stderr)
	}
}
