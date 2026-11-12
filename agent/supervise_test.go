package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newWorkspace checks out a real worktree for a supervise test.
func newWorkspace(t *testing.T) workspace {
	t.Helper()
	_, clone := newOrigin(t)
	ws := workspace{repo: clone, dir: filepath.Join(t.TempDir(), "wt")}
	if err := AddWorktree(ws.repo, ws.dir, "origin/trunk"); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	return ws
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestSupervise_LostWaitKillsAgentAndRemovesWorktree(t *testing.T) {
	ws := newWorkspace(t)
	hc := &fakeHC{waitErrs: maxWaitFailures}
	err := supervise(newSession(hc), ws, "job")
	if err == nil || !strings.Contains(err.Error(), "t9") {
		t.Fatalf("error should name the terminal: %v", err)
	}
	if !hc.called("kill t9") {
		t.Errorf("agent left running; calls %v", hc.calls)
	}
	if exists(ws.dir) {
		t.Error("worktree leaked")
	}
}

func TestSupervise_UnkillableAgentKeepsWorktree(t *testing.T) {
	ws := newWorkspace(t)
	hc := &fakeHC{waitErrs: maxWaitFailures, killErr: errors.New("no such terminal")}
	err := supervise(newSession(hc), ws, "job")
	if err == nil || !strings.Contains(err.Error(), "hc kill t9") {
		t.Fatalf("error should tell how to stop the agent: %v", err)
	}
	if !exists(ws.dir) {
		t.Error("worktree removed under a possibly running agent")
	}
}

func TestSupervise_DoneRemovesWorktree(t *testing.T) {
	ws := newWorkspace(t)
	hc := &fakeHC{waits: []string{`{"ended":true,"reason":"status","status":"done","turn":1}`}}
	if err := supervise(newSession(hc), ws, "job"); err != nil {
		t.Fatal(err)
	}
	if exists(ws.dir) {
		t.Error("worktree leaked")
	}
}

func TestSupervise_BlockedKeepsWorktree(t *testing.T) {
	ws := newWorkspace(t)
	hc := &fakeHC{waits: []string{`{"ended":true,"reason":"status","status":"blocked","turn":1}`}}
	err := supervise(newSession(hc), ws, "job")
	if !errors.Is(err, ErrBlocked) || !strings.Contains(err.Error(), "hc attach t9") {
		t.Fatalf("supervise = %v", err)
	}
	if !exists(ws.dir) {
		t.Error("a blocked agent's worktree must be kept")
	}
}
