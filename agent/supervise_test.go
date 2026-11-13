package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testRunID = "run-1"

// newTracker checks out a real worktree for a supervise test and tracks a
// session in it, recorded under a temporary runs dir.
func newTracker(t *testing.T) *tracker {
	t.Helper()
	_, clone := newOrigin(t)
	wt := filepath.Join(t.TempDir(), "wt")
	if err := AddWorktree(clone, wt, "origin/trunk"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	return &tracker{runsDir: t.TempDir(), runID: testRunID, rec: SessionRecord{SessionID: "sid", Dir: wt, Repo: clone}}
}

// savedRecord reads back the session record supervise left for the run.
func savedRecord(t *testing.T, tr *tracker) SessionRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(tr.runsDir, tr.runID+".agent.json"))
	if err != nil {
		t.Fatalf("no session record: %v", err)
	}
	var rec SessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// assertResumable checks the run left what `lazycron resume` needs: the
// worktree and a record of the session with the given status.
func assertResumable(t *testing.T, tr *tracker, status string) {
	t.Helper()
	if !exists(tr.rec.Dir) {
		t.Error("worktree removed; the session can't be resumed in it")
	}
	want := SessionRecord{Terminal: "t9", SessionID: "sid", Dir: tr.rec.Dir, Repo: tr.rec.Repo, Status: status}
	if got := savedRecord(t, tr); got != want {
		t.Errorf("record = %+v, want %+v", got, want)
	}
}

func TestSupervise_LostWaitStopsAgentAndKeepsSessionResumable(t *testing.T) {
	tr := newTracker(t)
	hc := &fakeHC{waitErrs: maxWaitFailures}
	err := supervise(newSession(hc), tr, "job")
	if err == nil || !strings.Contains(err.Error(), "terminal t9") || !strings.Contains(err.Error(), "lazycron resume "+testRunID) {
		t.Fatalf("error should name the terminal and how to continue: %v", err)
	}
	if !hc.called("kill t9") {
		t.Errorf("agent left running; calls %v", hc.calls)
	}
	assertResumable(t, tr, statusLost)
}

func TestSupervise_UnkillableLostAgentSaysHowToStopIt(t *testing.T) {
	tr := newTracker(t)
	hc := &fakeHC{waitErrs: maxWaitFailures, killErr: errors.New("no such terminal")}
	err := supervise(newSession(hc), tr, "job")
	if err == nil || !strings.Contains(err.Error(), "hc kill t9") || !strings.Contains(err.Error(), tr.rec.Dir) {
		t.Fatalf("error should tell how to stop the agent and where it works: %v", err)
	}
	assertResumable(t, tr, statusLost)
}

func TestSupervise_DoneLeavesSessionOpenAndResumable(t *testing.T) {
	tr := newTracker(t)
	hc := &fakeHC{waits: []string{`{"ended":true,"reason":"status","status":"done","turn":1}`}}
	if err := supervise(newSession(hc), tr, "job"); err != nil {
		t.Fatal(err)
	}
	if hc.called("kill") {
		t.Errorf("a finished session must stay open; calls %v", hc.calls)
	}
	assertResumable(t, tr, "done")
}

func TestSupervise_ErrorFailsRunAndLeavesSessionOpen(t *testing.T) {
	tr := newTracker(t)
	hc := &fakeHC{waits: []string{`{"ended":true,"reason":"status","status":"error","turn":1}`}}
	err := supervise(newSession(hc), tr, "job")
	if err == nil || !strings.Contains(err.Error(), "hc read t9") {
		t.Fatalf("supervise = %v", err)
	}
	if hc.called("kill") {
		t.Errorf("an errored session must stay open; calls %v", hc.calls)
	}
	assertResumable(t, tr, "error")
}

func TestSupervise_BlockedLeavesSessionOpen(t *testing.T) {
	tr := newTracker(t)
	hc := &fakeHC{waits: []string{`{"ended":true,"reason":"status","status":"blocked","turn":1}`}}
	err := supervise(newSession(hc), tr, "job")
	if !errors.Is(err, ErrBlocked) || !strings.Contains(err.Error(), "hc attach t9") {
		t.Fatalf("supervise = %v", err)
	}
	if hc.called("kill") {
		t.Errorf("a blocked session must stay open; calls %v", hc.calls)
	}
	assertResumable(t, tr, "blocked")
}

func TestSupervise_FailedStartRemovesWorktree(t *testing.T) {
	tr := newTracker(t)
	err := supervise(newSession(&fakeHC{runErr: errors.New("herdr-core is down")}), tr, "job")
	if err == nil {
		t.Fatal("supervise should fail when the terminal can't be started")
	}
	if exists(tr.rec.Dir) {
		t.Error("worktree leaked: no session ever ran in it")
	}
	if exists(filepath.Join(tr.runsDir, tr.runID+".agent.json")) {
		t.Error("a session that never started must not be recorded")
	}
}

func TestTrackerContinueHint_OutsideALazycronRun(t *testing.T) {
	tr := &tracker{rec: SessionRecord{Terminal: "t9", SessionID: "sid", Dir: "/wt"}}
	hint := tr.continueHint()
	for _, want := range []string{"hc attach t9", "claude --resume sid", "/wt"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint %q lacks %q", hint, want)
		}
	}
}
