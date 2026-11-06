package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/swalha1999/lazycron/setup"
)

func TestDiffRunsEvents(t *testing.T) {
	now := time.Unix(0, 0)
	running := runView{ID: "r1", State: stateRunning}
	events, state := diffRuns(nil, []runView{running}, now)
	if len(events) != 1 || events[0].Type != eventStarted {
		t.Fatalf("first sight of a running run: %+v", events)
	}
	events, state = diffRuns(state, []runView{running}, now)
	if len(events) != 0 {
		t.Fatalf("no change, no events: %+v", events)
	}
	done := runView{ID: "r1", State: stateFailed}
	quick := runView{ID: "r2", State: stateSucceeded}
	events, _ = diffRuns(state, []runView{quick, done}, now)
	if len(events) != 3 || events[0].Type != eventFailed || events[1].Type != eventStarted || events[2].Type != eventFinished {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestEventsStreamsRunLifecycle(t *testing.T) {
	env := newTestEnv(t)
	wait := detachInProcess(t, env)
	env.mustRun(addArgs("ev", "Ev", "every hour", "sleep 1; exit 2")...)
	go func() {
		time.Sleep(300 * time.Millisecond)
		startFirstJobInBackground(t, env)
	}()
	r := env.mustRun("events", "--count", "2", "--for", "20s")
	wait()
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"type":"run.started"`) || !strings.Contains(lines[1], `"type":"run.failed"`) {
		t.Fatalf("unexpected events:\n%s", r.stdout)
	}
}

// startFirstJobInBackground starts a run like `run --no-wait`, without the
// shared cobra flag state that the concurrent `events` command is using.
func startFirstJobInBackground(t *testing.T, env *testEnv) {
	b := mustFileBackend(env)
	jobs, err := b.ReadJobs()
	if err != nil || len(jobs) == 0 {
		t.Errorf("read jobs: %v", err)
		return
	}
	job := jobs[0]
	job.Runner = "plain" // explicit, so this goroutine never reads cron.DefaultRunner
	if err := startBackground(b, job); err != nil {
		t.Errorf("start: %v", err)
	}
}

func TestDoctorReportsAndFails(t *testing.T) {
	env := newTestEnv(t)
	useScriptedHost(t, env)
	r := env.run("doctor")
	var rep doctorReport
	r.decode(t, &rep)
	if len(rep.Checks) == 0 {
		t.Fatal("no checks")
	}
	names := map[string]setup.Status{}
	for _, c := range rep.Checks {
		names[c.Name] = c.Status
	}
	for _, want := range []string{"crontab", "herdr-core", "jobs", "runs"} {
		if _, ok := names[want]; !ok {
			t.Errorf("missing check %s in %v", want, names)
		}
	}
	if rep.OK != (r.code == 0) {
		t.Fatalf("ok=%v but exit %d", rep.OK, r.code)
	}
}

func TestServerCommands(t *testing.T) {
	env := newTestEnv(t)
	add := []string{"server", "add", "--name", "vps", "--host", "10.0.0.2", "--user", "me"}
	var res serverResult
	env.mustRun(add...).decode(t, &res)
	if res.Action != actionCreated {
		t.Fatalf("add: %+v", res)
	}
	env.mustRun(add...).decode(t, &res)
	if res.Action != actionUnchanged {
		t.Fatalf("re-add: %+v", res)
	}
	if r := env.run("server", "add", "--name", "vps", "--host", "10.0.0.3", "--user", "me"); r.code != exitConflict {
		t.Fatalf("conflicting add: exit %d", r.code)
	}
	var ls struct {
		Servers []struct{ Name string }
	}
	env.mustRun("server", "ls").decode(t, &ls)
	if len(ls.Servers) != 1 || ls.Servers[0].Name != "vps" {
		t.Fatalf("ls: %+v", ls)
	}
	env.mustRun("server", "rm", "vps", "--dry-run")
	env.mustRun("server", "ls").decode(t, &ls)
	if len(ls.Servers) != 1 {
		t.Fatal("dry-run rm removed the server")
	}
	env.mustRun("server", "rm", "vps")
	if r := env.run("server", "rm", "vps"); r.code != exitNotFound {
		t.Fatalf("rm missing: exit %d", r.code)
	}
}

func TestSkillInstallIsIdempotent(t *testing.T) {
	env := newTestEnv(t)
	dir := filepath.Join(env.home, "skills")
	var res struct{ Action, Path string }
	env.mustRun("skill", "install", "--dir", dir).decode(t, &res)
	if res.Action != actionCreated {
		t.Fatalf("install: %+v", res)
	}
	data, err := os.ReadFile(res.Path)
	if err != nil || !strings.HasPrefix(string(data), "---\nname: lazycron\n") {
		t.Fatalf("SKILL.md: %v\n%s", err, data)
	}
	env.mustRun("skill", "install", "--dir", dir).decode(t, &res)
	if res.Action != actionUnchanged {
		t.Fatalf("reinstall: %+v", res)
	}
}

// TestAgentHelpCoversEveryCommand keeps the agent guide in parity with the
// command tree: every user-facing command must be documented there.
func TestAgentHelpCoversEveryCommand(t *testing.T) {
	internal := map[string]bool{"exec": true, "help": true, "completion": true, "agent-help": true,
		"init": true, "sync": true, "diff": true, "pull": true, "skill": true}
	for _, c := range rootCmd.Commands() {
		name := c.Name()
		if internal[name] || c.Hidden {
			continue
		}
		if !strings.Contains(agentGuide, "lazycron "+name) && !strings.Contains(agentGuide, "| "+name) &&
			!strings.Contains(agentGuide, " "+name+" <") && !strings.Contains(agentGuide, "lazycron "+name+" ") {
			t.Errorf("agent-help does not mention `%s`", name)
		}
	}
	out := runCLI(t, "agent-help").stdout
	if !strings.Contains(out, "# lazycron for agents") {
		t.Fatal("agent-help printed nothing")
	}
}
