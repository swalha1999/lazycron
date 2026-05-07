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

// stubInit replaces scaffoldSandcastleConfig + writeEnsureRepoLib for tests, restoring on cleanup.
func stubInit(t *testing.T, scaffold func(string, string) error, write func(string) error) {
	t.Helper()
	origScaffold := scaffoldSandcastleConfig
	origWrite := writeEnsureRepoLib
	scaffoldSandcastleConfig = scaffold
	writeEnsureRepoLib = write
	t.Cleanup(func() {
		scaffoldSandcastleConfig = origScaffold
		writeEnsureRepoLib = origWrite
	})
}

func resetInitFlags(t *testing.T) {
	t.Helper()
	prevAgents, prevName := initWithAgents, initName
	initWithAgents, initName = false, ""
	t.Cleanup(func() {
		initWithAgents, initName = prevAgents, prevName
	})
}

func TestRunInit_AddsSandcastleToGitignore(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	initName = "my-proj"

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), ".sandcastle") {
		t.Errorf("expected .gitignore to include .sandcastle, got: %s", string(gi))
	}

	if _, err := os.Stat(filepath.Join(dir, ".lazycron")); !os.IsNotExist(err) {
		t.Errorf("expected no .lazycron/ to be created, got err=%v", err)
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

	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if got := strings.Count(string(data), ".sandcastle"); got != 1 {
		t.Errorf("expected .sandcastle to appear exactly once in .gitignore, got %d times", got)
	}
}

func TestRunInit_WithAgents_StubbedHooks(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	initWithAgents = true
	initName = "agents-test"

	scaffoldCalls := 0
	writeCalls := 0
	resolvedDir, _ := filepath.EvalSymlinks(dir)
	stubInit(t,
		func(cwd, projectName string) error {
			scaffoldCalls++
			if cwd != resolvedDir && cwd != dir {
				t.Errorf("scaffold called with cwd %q, want %q (or %q)", cwd, resolvedDir, dir)
			}
			if projectName != "agents-test" {
				t.Errorf("scaffold called with projectName %q, want %q", projectName, "agents-test")
			}
			return os.MkdirAll(filepath.Join(cwd, ".sandcastle"), 0o755)
		},
		func(cwd string) error {
			writeCalls++
			return os.MkdirAll(filepath.Join(cwd, ".sandcastle", "lib"), 0o755)
		},
	)

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if scaffoldCalls != 1 {
		t.Errorf("scaffoldSandcastleConfig calls = %d, want 1", scaffoldCalls)
	}
	if writeCalls != 1 {
		t.Errorf("writeEnsureRepoLib calls = %d, want 1", writeCalls)
	}

	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(gi), ".sandcastle") {
		t.Errorf("expected .gitignore to include .sandcastle, got: %s", string(gi))
	}
}

func TestAppendGitignore_Idempotent(t *testing.T) {
	dir := chdirTemp(t)

	if err := appendGitignore(dir, ".sandcastle"); err != nil {
		t.Fatal(err)
	}
	if err := appendGitignore(dir, ".sandcastle"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if got := strings.Count(string(data), ".sandcastle"); got != 1 {
		t.Errorf("expected .sandcastle to appear exactly once, got %d times", got)
	}
}

func TestAppendGitignore_AddsNewlineWhenNeeded(t *testing.T) {
	dir := chdirTemp(t)
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules"), 0o644)

	if err := appendGitignore(dir, ".sandcastle"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	want := "node_modules\n.sandcastle\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}
