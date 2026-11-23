package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

func testJob(id, name string) cron.Job {
	return cron.Job{ID: id, Name: name, Schedule: "0 * * * *", Command: "echo " + name, Enabled: true, Wrapped: true}
}

// newStaleModel writes jobs to a file backend and loads them into a TUI
// model with the jobs panel focused.
func newStaleModel(t *testing.T, jobs ...cron.Job) (Model, *backend.FileBackend) {
	t.Helper()
	dir := t.TempDir()
	b := backend.NewFileBackend(filepath.Join(dir, "crontab"), filepath.Join(dir, "history"))
	if err := b.WriteJobs(jobs); err != nil {
		t.Fatal(err)
	}
	m := NewModel(backend.NewManagerWithBackend(b), "test")
	m.mode = modeNormal
	m.focusPanel = panelJobs
	loaded, err := b.ReadJobs()
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(jobsLoadedMsg{jobs: loaded})
	return next.(Model), b
}

// changeOutside applies change to the backend's jobs, as the CLI, `lazycron
// sync` or `exec --once` would while the TUI is open.
func changeOutside(t *testing.T, b backend.Backend, change func([]cron.Job) []cron.Job) {
	t.Helper()
	jobs, err := b.ReadJobs()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.WriteJobs(change(jobs)); err != nil {
		t.Fatal(err)
	}
}

// awaitMsg runs cmd (expanding batches, each in its own goroutine so status
// ticks don't block) and returns the first message of type T.
func awaitMsg[T tea.Msg](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	found := make(chan T, 1)
	var run func(tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			for _, sub := range msg {
				go run(sub)
			}
		case T:
			select {
			case found <- msg:
			default:
			}
		}
	}
	go run(cmd)
	select {
	case msg := <-found:
		return msg
	case <-time.After(3 * time.Second):
		t.Fatalf("no %T produced", *new(T))
		panic("unreachable")
	}
}

// pressSpaceOn toggles the job with the given ID through the TUI and feeds
// the save result back into the model.
func pressSpaceOn(t *testing.T, m Model, id string) Model {
	t.Helper()
	i, err := indexOfJob(m.jobs, id)
	if err != nil {
		t.Fatal(err)
	}
	m.selectedRow = rowForJobIdx(m.jobListRows, i)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	saved := awaitMsg[jobSavedMsg](t, cmd)
	if saved.err != nil {
		t.Fatalf("save: %v", saved.err)
	}
	next, _ = next.(Model).Update(saved)
	return next.(Model)
}

func jobByID(t *testing.T, b backend.Backend, id string) (cron.Job, bool) {
	t.Helper()
	jobs, err := b.ReadJobs()
	if err != nil {
		t.Fatal(err)
	}
	i, err := indexOfJob(jobs, id)
	if err != nil {
		return cron.Job{}, false
	}
	return jobs[i], true
}

func assertScriptExists(t *testing.T, b *backend.FileBackend, id string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(b.ScriptsDir(), id+".sh")); err != nil {
		t.Fatalf("script for %s was removed: %v", id, err)
	}
}

func TestToggleKeepsJobAddedOutsideTUI(t *testing.T) {
	m, b := newStaleModel(t, testJob("aaaa1111", "a"))
	changeOutside(t, b, func(jobs []cron.Job) []cron.Job {
		return append(jobs, testJob("bbbb2222", "external"))
	})

	m = pressSpaceOn(t, m, "aaaa1111")

	if a, _ := jobByID(t, b, "aaaa1111"); a.Enabled {
		t.Fatal("toggle was not saved")
	}
	if _, ok := jobByID(t, b, "bbbb2222"); !ok {
		t.Fatal("job added outside the TUI was deleted")
	}
	assertScriptExists(t, b, "bbbb2222")
	if len(m.jobs) != 2 {
		t.Fatalf("TUI list not refreshed after save: %d jobs", len(m.jobs))
	}
}

func TestDisableCompletedOneShotsKeepsExternalJob(t *testing.T) {
	once := testJob("once1111", "once")
	once.OneShot = true
	m, b := newStaleModel(t, once)
	changeOutside(t, b, func(jobs []cron.Job) []cron.Job {
		return append(jobs, testJob("bbbb2222", "external"))
	})

	hist := []history.Entry{{JobID: "once1111", JobName: "once"}}
	msg := disableCompletedOneShots(m.manager.ActiveBackend(), hist)().(disableCompletedOneShotsMsg)
	if msg.err != nil || msg.disabled != 1 {
		t.Fatalf("disabled=%d err=%v", msg.disabled, msg.err)
	}

	if j, _ := jobByID(t, b, "once1111"); j.Enabled {
		t.Fatal("fired one-shot is still enabled")
	}
	if _, ok := jobByID(t, b, "bbbb2222"); !ok {
		t.Fatal("job added outside the TUI was deleted")
	}
	assertScriptExists(t, b, "bbbb2222")
}

func TestOneShotDisabledOutsideTUIStaysDisabled(t *testing.T) {
	once := testJob("once1111", "once")
	once.OneShot = true
	m, b := newStaleModel(t, once, testJob("aaaa1111", "a"))
	changeOutside(t, b, func(jobs []cron.Job) []cron.Job {
		i, _ := indexOfJob(jobs, "once1111")
		jobs[i].Enabled = false // what `exec --once` does after the run
		return jobs
	})

	pressSpaceOn(t, m, "aaaa1111")

	if j, _ := jobByID(t, b, "once1111"); j.Enabled {
		t.Fatal("unrelated TUI save re-armed the one-shot")
	}
}

func TestSaveOfJobDeletedOutsideTUIReportsError(t *testing.T) {
	m, b := newStaleModel(t, testJob("aaaa1111", "a"), testJob("cccc3333", "c"))
	changeOutside(t, b, func(jobs []cron.Job) []cron.Job {
		i, _ := indexOfJob(jobs, "aaaa1111")
		return append(jobs[:i], jobs[i+1:]...)
	})

	saved := awaitMsg[jobSavedMsg](t, saveJobs(m.manager.ActiveBackend(), setJobEnabled("aaaa1111", false)))
	if saved.err == nil {
		t.Fatal("expected an error for a job that no longer exists")
	}
	if len(saved.jobs) != 1 || saved.jobs[0].ID != "cccc3333" {
		t.Fatalf("expected the fresh list to come back, got %+v", saved.jobs)
	}
}

func TestSwapJobsReordersByID(t *testing.T) {
	jobs := []cron.Job{testJob("a", "a"), testJob("x", "x"), testJob("b", "b")}
	got, err := swapJobs("a", "b")(jobs)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "b" || got[1].ID != "x" || got[2].ID != "a" {
		t.Fatalf("order = %s %s %s", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestReplaceJobKeepsCurrentEnabledState(t *testing.T) {
	current := testJob("a", "old")
	current.Enabled = false
	edited := testJob("a", "new")
	got, err := replaceJob(edited)([]cron.Job{current})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "new" || got[0].Enabled {
		t.Fatalf("got %+v", got[0])
	}
}
