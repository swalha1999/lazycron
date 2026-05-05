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
	prevAgents, prevName, prevForce := initWithAgents, initName, initForce
	initWithAgents, initName, initForce = false, "", false
	t.Cleanup(func() {
		initWithAgents, initName, initForce = prevAgents, prevName, prevForce
	})
}

func TestRunInit_BasicScaffold(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	initName = "my-proj"

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	cfgPath := filepath.Join(dir, ".lazycron", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if !strings.Contains(string(data), "name: my-proj") {
		t.Errorf("expected `name: my-proj` in config, got: %s", string(data))
	}

	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gi), ".lazycron/.env") {
		t.Errorf("expected .gitignore to include .lazycron/.env, got: %s", string(gi))
	}
}

func TestRunInit_NameDefaultsToBasename(t *testing.T) {
	// Use a Go-friendly basename so it's not yaml-quoted as a number.
	parent := t.TempDir()
	dir := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	resetInitFlags(t)

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".lazycron", "config.yaml"))
	if !strings.Contains(string(data), "name: myproject") {
		t.Errorf("expected `name: myproject` in config, got: %s", string(data))
	}
}

func TestRunInit_RefuseExisting(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	if err := os.MkdirAll(filepath.Join(dir, ".lazycron"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := runInit(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got %v", err)
	}
}

func TestRunInit_ForceOverwrites(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	initForce = true
	initName = "second"

	// Pre-create with a different name.
	if err := os.MkdirAll(filepath.Join(dir, ".lazycron"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, ".lazycron", "config.yaml"), []byte("name: first\n"), 0o644)

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".lazycron", "config.yaml"))
	if !strings.Contains(string(data), "name: second") {
		t.Errorf("expected name: second after --force, got: %s", string(data))
	}
}

func TestRunInit_WithAgents_StubbedHooks(t *testing.T) {
	dir := chdirTemp(t)
	resetInitFlags(t)
	initWithAgents = true
	initName = "agents-test"

	scaffoldCalls := 0
	writeCalls := 0
	// Resolve symlinks since macOS /var -> /private/var; t.TempDir uses the
	// unresolved form but os.Getwd inside runInit returns the resolved one.
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
	if !strings.Contains(string(gi), ".sandcastle/.env") {
		t.Errorf("expected .gitignore to include .sandcastle/.env, got: %s", string(gi))
	}
}

func TestAppendGitignore_Idempotent(t *testing.T) {
	dir := chdirTemp(t)

	if err := appendGitignore(dir, ".lazycron/.env"); err != nil {
		t.Fatal(err)
	}
	if err := appendGitignore(dir, ".lazycron/.env"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if got := strings.Count(string(data), ".lazycron/.env"); got != 1 {
		t.Errorf("expected .lazycron/.env to appear exactly once, got %d times", got)
	}
}

func TestAppendGitignore_AddsNewlineWhenNeeded(t *testing.T) {
	dir := chdirTemp(t)
	// File without trailing newline.
	_ = os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules"), 0o644)

	if err := appendGitignore(dir, ".lazycron/.env"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	want := "node_modules\n.lazycron/.env\n"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}
