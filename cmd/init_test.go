package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp swaps into a fresh temp directory and restores cwd on cleanup.
// Returns the absolute path to the temp dir.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return dir
}

func resetInitFlags(t *testing.T) {
	t.Helper()
	prevAgents, prevName := initWithAgents, initName
	initWithAgents, initName = false, ""
	t.Cleanup(func() {
		initWithAgents, initName = prevAgents, prevName
	})
}

func TestRunInit_WritesConfigAndLocalGitignore(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	initName = "my-proj"

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("init must not touch the repo's .gitignore (.lazycron/ is committed), got err=%v", err)
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".lazycron", ".gitignore"))
	if err != nil {
		t.Fatalf("read .lazycron/.gitignore: %v", err)
	}
	for _, want := range []string{".env", "_pulled/"} {
		if !strings.Contains(string(gi), want) {
			t.Errorf(".lazycron/.gitignore should ignore %s, got: %s", want, gi)
		}
	}
	cfg, err := os.ReadFile(filepath.Join(dir, ".lazycron", "config.yaml"))
	if err != nil || !strings.Contains(string(cfg), "my-proj") {
		t.Errorf("config.yaml = %q, %v; want it to name my-proj", cfg, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lazycron", "jobs")); !os.IsNotExist(err) {
		t.Errorf("expected no jobs/ without --with-agents, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".sandcastle")); !os.IsNotExist(err) {
		t.Errorf("init must not create .sandcastle/, got err=%v", err)
	}
}

func TestRunInit_IsIdempotent(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	if err := runInit(nil, nil); err != nil {
		t.Fatalf("second runInit: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".lazycron", ".gitignore"))
	if string(data) != lazycronGitignore {
		t.Errorf(".lazycron/.gitignore changed on re-run: %q", data)
	}
}

func TestRunInit_WithAgents_WritesJobsAndKeepsEdits(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	initWithAgents = true

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	for _, f := range []string{"audit.md", "improve.md", "deps.md", "worker.md", "upstream-sync.md"} {
		if _, err := os.Stat(filepath.Join(dir, ".lazycron", "jobs", f)); err != nil {
			t.Errorf("missing job %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".lazycron", "context.md")); err != nil {
		t.Errorf("missing context.md: %v", err)
	}

	edited := filepath.Join(dir, ".lazycron", "jobs", "audit.md")
	if err := os.WriteFile(edited, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(nil, nil); err != nil {
		t.Fatalf("second runInit: %v", err)
	}
	if got, _ := os.ReadFile(edited); string(got) != "mine" {
		t.Errorf("re-running init overwrote an edited job: %q", got)
	}
}
