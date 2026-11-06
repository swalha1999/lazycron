package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/runner"
)

func sampleRunState() runState {
	return runState{
		active: map[string]runner.Active{"busy": {RunID: "r9", JobID: "busy", Terminal: "t9", StartedAt: "2026-11-06T02:00:00Z"}},
		live:   map[string]bool{"t9": true, "t5": true, "t4": false},
	}
}

func TestTerminalBadges(t *testing.T) {
	s := sampleRunState()
	if s.terminalBadge("t5") == "" || s.terminalBadge("t4") == "" {
		t.Fatal("kept terminals need a badge")
	}
	if s.terminalBadge("t1") != "" || s.terminalBadge("") != "" {
		t.Fatal("removed or missing terminals get no badge")
	}
	if kept, live := s.terminalKept("t4"); !kept || live {
		t.Fatalf("t4: kept=%v live=%v", kept, live)
	}
}

func TestJobRowShowsRunningBadge(t *testing.T) {
	s := sampleRunState()
	busy := renderJobRow(cron.Job{ID: "busy", Name: "Busy", Schedule: "* * * * *", Enabled: true, Wrapped: true}, 20, nil, s)
	idle := renderJobRow(cron.Job{ID: "idle", Name: "Idle", Schedule: "* * * * *", Enabled: true, Wrapped: true}, 20, nil, s)
	if !strings.Contains(busy, "⟳") || strings.Contains(idle, "⟳") {
		t.Fatalf("busy=%q idle=%q", busy, idle)
	}
}

func TestHistoryDetailShowsRunFields(t *testing.T) {
	code := 3
	e := &history.Entry{JobName: "J", Timestamp: "2026-11-06T02:00:00Z", Success: boolPtr(false),
		ExitCode: &code, Runner: "herdr", Terminal: "t5", RunID: "r1", OutputTruncated: true}
	out := renderHistoryDetail(e, 80, sampleRunState())
	for _, want := range []string{"Exit", "3", "herdr", "t5", "open", "attach", "r1", "truncated"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail lacks %q:\n%s", want, out)
		}
	}
}

func TestAttachTarget(t *testing.T) {
	m := Model{
		jobs:              []cron.Job{{ID: "busy", Name: "Busy"}},
		history:           []history.Entry{{JobName: "Old", Terminal: "t4"}, {JobName: "Gone", Terminal: "t1"}},
		runs:              sampleRunState(),
		focusPanel:        panelJobs,
		collapsedProjects: map[string]bool{},
	}
	m.rebuildRows()
	m.selectedRow = rowForJobIdx(m.jobListRows, 0)
	if term, _ := m.attachTarget(); term != "t9" {
		t.Fatalf("jobs panel: want running terminal t9, got %q", term)
	}
	m.focusPanel = panelHistory
	if term, _ := m.attachTarget(); term != "t4" {
		t.Fatalf("history panel: want kept terminal t4, got %q", term)
	}
	m.historySelected = 1
	if term, name := m.attachTarget(); term != "" || name != "Gone" {
		t.Fatalf("removed terminal must not be attachable: %q", term)
	}
}

func TestHerdrNoticeOnlyOnceAndOnlyWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // no hc anywhere
	local := backend.NewLocalBackend()

	if _, ok := checkHerdrNotice(local)().(herdrNoticeMsg); !ok {
		t.Fatal("expected the notice when herdr-core is missing")
	}
	if msg := checkHerdrNotice(backend.NewFileBackend(filepath.Join(home, "c"), filepath.Join(home, "h")))(); msg != nil {
		t.Fatal("no notice for the --cron-file backend")
	}

	m := Model{mode: modeHerdrNotice}
	next, _ := m.handleHerdrNoticeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if next.(Model).mode != modeNormal {
		t.Fatal("n should close the notice")
	}
	if msg := checkHerdrNotice(local)(); msg != nil {
		t.Fatal("the notice must not come back after it was answered")
	}
	cfg, _ := config.Load()
	if !cfg.Herdr.Prompted {
		t.Fatal("answer not remembered")
	}

	// With herdr-core present there is nothing to ask.
	t.Setenv("HOME", t.TempDir())
	bin := filepath.Join(os.Getenv("HOME"), ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "herdr-core"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if msg := checkHerdrNotice(local)(); msg != nil {
		t.Fatal("no notice when herdr-core is installed")
	}
}
