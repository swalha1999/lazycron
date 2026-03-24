package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/ui"
)

// testEnv sets up a temporary crontab file and history directory.
type testEnv struct {
	cronFile   string
	historyDir string
	fb         *backend.FileBackend
	mgr        *backend.Manager
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	cronFile := filepath.Join(dir, "crontab")
	histDir := filepath.Join(dir, "history")
	if err := os.MkdirAll(histDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fb := backend.NewFileBackend(cronFile, histDir)
	mgr := backend.NewManagerWithBackend(fb)
	return &testEnv{
		cronFile:   cronFile,
		historyDir: histDir,
		fb:         fb,
		mgr:        mgr,
	}
}

// prePopulate writes jobs to the crontab file.
func (e *testEnv) prePopulate(t *testing.T, jobs []cron.Job) {
	t.Helper()
	if err := e.fb.WriteJobs(jobs); err != nil {
		t.Fatalf("prePopulate: %v", err)
	}
}

// readJobs reads jobs from the crontab file.
func (e *testEnv) readJobs(t *testing.T) []cron.Job {
	t.Helper()
	jobs, err := e.fb.ReadJobs()
	if err != nil {
		t.Fatalf("readJobs: %v", err)
	}
	return jobs
}

// cronFileContents returns the raw crontab file contents.
func (e *testEnv) cronFileContents(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.cronFile)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("reading cron file: %v", err)
	}
	return string(data)
}

// initModel creates a new TUI model and processes its Init command.
func (e *testEnv) initModel(t *testing.T) tea.Model {
	t.Helper()
	m := ui.NewModel(e.mgr, "test")
	cmd := m.Init()
	result := processCmd(m, cmd)
	// Dismiss the startup splash screen
	result, _ = result.Update(tea.KeyMsg{Type: tea.KeyEsc})
	return result
}

// execCmd runs a tea.Cmd with a short timeout. Returns the message or nil if
// the command blocks (e.g. tick-based commands like blink or historyTick).
func execCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(50 * time.Millisecond):
		return nil // skip blocking commands (ticks, blinks)
	}
}

// processCmd executes a tea.Cmd and feeds the resulting message back to the model.
// It recursively processes batch commands. It skips tick-based commands
// (blink, clearStatus, historyTick) that would block.
func processCmd(m tea.Model, cmd tea.Cmd) tea.Model {
	msg := execCmd(cmd)
	if msg == nil {
		return m
	}

	// Handle batch messages: tea.BatchMsg is []tea.Cmd in bubbletea v1.3.10
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = processCmd(m, c)
		}
		return m
	}

	var newCmd tea.Cmd
	m, newCmd = m.Update(msg)
	return processCmd(m, newCmd)
}

// sendKey sends a single-character key message to the model and processes the resulting command.
func sendKey(m tea.Model, key string) tea.Model {
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return processCmd(m, cmd)
}

// sendSpecialKey sends a special key (enter, tab, esc, space, etc.) to the model.
func sendSpecialKey(m tea.Model, keyType tea.KeyType) tea.Model {
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: keyType})
	return processCmd(m, cmd)
}

// sendString types a string character by character.
func sendString(m tea.Model, s string) tea.Model {
	for _, ch := range s {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		m = processCmd(m, cmd)
	}
	return m
}

// sampleJob creates a simple enabled job for pre-populating.
func sampleJob(name, command, schedule string) cron.Job {
	return cron.Job{
		ID:       cron.GenerateID(),
		Name:     name,
		Schedule: schedule,
		Command:  command,
		Enabled:  true,
		Wrapped:  true,
	}
}

// --- Test Cases ---

func TestCreateBlankJob(t *testing.T) {
	env := newTestEnv(t)
	m := env.initModel(t)

	// Focus on jobs panel: tab
	m = sendSpecialKey(m, tea.KeyTab)

	// Open new job menu: n
	m = sendKey(m, "n")

	// Choose blank form: b
	m = sendKey(m, "b")

	// Type name
	m = sendString(m, "my-job")

	// Tab to command field
	m = sendSpecialKey(m, tea.KeyTab)

	// Type command
	m = sendString(m, "echo hello")

	// Tab to schedule field (already has default "* * * * *" from picker)
	m = sendSpecialKey(m, tea.KeyTab)

	// Tab through the cron picker
	m = sendSpecialKey(m, tea.KeyTab)

	// Now on WorkDir — skip it. Submit with enter.
	m = sendSpecialKey(m, tea.KeyEnter)

	// Verify
	jobs := env.readJobs(t)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d\ncrontab contents:\n%s", len(jobs), env.cronFileContents(t))
	}
	if jobs[0].Name != "my-job" {
		t.Errorf("job name = %q, want %q", jobs[0].Name, "my-job")
	}
}

func TestDeleteJob(t *testing.T) {
	env := newTestEnv(t)
	env.prePopulate(t, []cron.Job{sampleJob("delete-me", "echo bye", "* * * * *")})
	m := env.initModel(t)

	// Focus jobs panel
	m = sendSpecialKey(m, tea.KeyTab)

	// Move past group header to the job row
	m = sendKey(m, "j")

	// Delete: D
	m = sendKey(m, "D")

	// Confirm: y
	m = sendKey(m, "y")

	// Verify
	jobs := env.readJobs(t)
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after delete, got %d", len(jobs))
	}
}

func TestToggleDisable(t *testing.T) {
	env := newTestEnv(t)
	env.prePopulate(t, []cron.Job{sampleJob("toggle-me", "echo hi", "* * * * *")})
	m := env.initModel(t)

	// Focus jobs panel
	m = sendSpecialKey(m, tea.KeyTab)

	// Move past group header to the job row
	m = sendKey(m, "j")

	// Toggle disable: space
	m = sendSpecialKey(m, tea.KeySpace)

	// Verify
	content := env.cronFileContents(t)
	if !strings.Contains(content, "#DISABLED") {
		t.Fatalf("expected job to be disabled, crontab:\n%s", content)
	}
}

func TestToggleReEnable(t *testing.T) {
	env := newTestEnv(t)
	disabledJob := sampleJob("toggle-me", "echo hi", "* * * * *")
	disabledJob.Enabled = false
	env.prePopulate(t, []cron.Job{disabledJob})
	m := env.initModel(t)

	// Focus jobs panel
	m = sendSpecialKey(m, tea.KeyTab)

	// Move past group header to the job row
	m = sendKey(m, "j")

	// Toggle enable: space
	m = sendSpecialKey(m, tea.KeySpace)

	// Verify
	content := env.cronFileContents(t)
	if strings.Contains(content, "#DISABLED") {
		t.Fatalf("expected job to be enabled, crontab:\n%s", content)
	}
}

func TestEditJob(t *testing.T) {
	env := newTestEnv(t)
	env.prePopulate(t, []cron.Job{sampleJob("old-name", "echo old", "* * * * *")})
	m := env.initModel(t)

	// Focus jobs panel
	m = sendSpecialKey(m, tea.KeyTab)

	// Move past group header to the job row
	m = sendKey(m, "j")

	// Edit: e
	m = sendKey(m, "e")

	// Clear the name field and type new name.
	// The name field is focused. Select all and clear.
	m = sendSpecialKey(m, tea.KeyCtrlA)
	m = sendSpecialKey(m, tea.KeyCtrlK)
	m = sendString(m, "new-name")

	// Submit
	m = sendSpecialKey(m, tea.KeyEnter)

	// Verify
	jobs := env.readJobs(t)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "new-name" {
		t.Errorf("job name = %q, want %q", jobs[0].Name, "new-name")
	}
}

func TestCancelForm(t *testing.T) {
	env := newTestEnv(t)
	m := env.initModel(t)

	// Focus jobs panel, open new job menu, choose blank form
	m = sendSpecialKey(m, tea.KeyTab)
	m = sendKey(m, "n")
	m = sendKey(m, "b")

	// Type some data
	m = sendString(m, "temp-job")

	// Cancel with esc
	m = sendSpecialKey(m, tea.KeyEscape)

	// Verify: no file should exist or it should be empty
	jobs := env.readJobs(t)
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after cancel, got %d", len(jobs))
	}
}

func TestCreateMultipleJobs(t *testing.T) {
	env := newTestEnv(t)
	m := env.initModel(t)

	// Create first job
	m = sendSpecialKey(m, tea.KeyTab) // focus jobs panel
	m = sendKey(m, "n")
	m = sendKey(m, "b")
	m = sendString(m, "job-one")
	m = sendSpecialKey(m, tea.KeyTab) // → command
	m = sendString(m, "echo one")
	m = sendSpecialKey(m, tea.KeyTab) // → schedule (has default)
	m = sendSpecialKey(m, tea.KeyTab) // → picker
	m = sendSpecialKey(m, tea.KeyTab) // → workdir
	m = sendSpecialKey(m, tea.KeyEnter)

	// Create second job (we're back in normal mode on jobs panel)
	m = sendKey(m, "n")
	m = sendKey(m, "b")
	m = sendString(m, "job-two")
	m = sendSpecialKey(m, tea.KeyTab) // → command
	m = sendString(m, "echo two")
	m = sendSpecialKey(m, tea.KeyTab) // → schedule (has default)
	m = sendSpecialKey(m, tea.KeyTab) // → picker
	m = sendSpecialKey(m, tea.KeyTab) // → workdir
	m = sendSpecialKey(m, tea.KeyEnter)

	// Verify
	jobs := env.readJobs(t)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d\ncrontab:\n%s", len(jobs), env.cronFileContents(t))
	}
}

func TestDeleteSecondJob(t *testing.T) {
	env := newTestEnv(t)
	env.prePopulate(t, []cron.Job{
		sampleJob("first", "echo 1", "* * * * *"),
		sampleJob("second", "echo 2", "0 9 * * *"),
	})
	m := env.initModel(t)

	// Focus jobs panel
	m = sendSpecialKey(m, tea.KeyTab)

	// Move past group header to first job, then to second job
	m = sendKey(m, "j")
	m = sendKey(m, "j")

	// Delete: D
	m = sendKey(m, "D")

	// Confirm: y
	m = sendKey(m, "y")

	// Verify
	jobs := env.readJobs(t)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "first" {
		t.Errorf("remaining job = %q, want %q", jobs[0].Name, "first")
	}
}
