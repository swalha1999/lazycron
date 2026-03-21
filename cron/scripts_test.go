package cron

import (
	"os"
	"path/filepath"
	"testing"
)

// --- sanitizeJobName ---

func TestSanitizeJobName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-job", "my-job"},
		{"My Job", "my-job"},
		{"Security Review Agent", "security-review-agent"},
		{"hello_world", "hello-world"},
		{"  spaces  ", "spaces"},
		{"UPPERCASE", "uppercase"},
		{"a--b", "a-b"},
		{"special!@#chars", "special-chars"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeJobName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeJobName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- WriteScript / ReadScriptCommand ---

func TestWriteReadScript(t *testing.T) {
	withFakeScriptsDir(t)

	err := WriteScript("test-job", "echo hello world")
	if err != nil {
		t.Fatalf("WriteScript: %v", err)
	}

	path := ScriptPath("test-job")
	content, err := ReadScriptCommand(path)
	if err != nil {
		t.Fatalf("ReadScriptCommand: %v", err)
	}
	if content != "echo hello world" {
		t.Errorf("content = %q, want %q", content, "echo hello world")
	}

	// Verify raw file has shebang
	raw, _ := os.ReadFile(path)
	if string(raw) != "#!/bin/sh\necho hello world\n" {
		t.Errorf("raw file = %q, want shebang + command", string(raw))
	}

	// Verify permissions
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0755 {
		t.Errorf("permissions = %o, want 0755", info.Mode().Perm())
	}
}

func TestWriteScript_MultilineCommand(t *testing.T) {
	withFakeScriptsDir(t)

	cmd := "cd /repo && claude -p --dangerously-skip-permissions \"Very long prompt with\nmultiple lines\""
	err := WriteScript("agent-job", cmd)
	if err != nil {
		t.Fatalf("WriteScript: %v", err)
	}

	content, err := ReadScriptCommand(ScriptPath("agent-job"))
	if err != nil {
		t.Fatalf("ReadScriptCommand: %v", err)
	}
	if content != cmd {
		t.Errorf("content = %q, want %q", content, cmd)
	}
}

// --- DeleteScript ---

func TestDeleteScript(t *testing.T) {
	withFakeScriptsDir(t)

	WriteScript("to-delete", "echo bye")
	path := ScriptPath("to-delete")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("script should exist before delete")
	}

	err := DeleteScript("to-delete")
	if err != nil {
		t.Fatalf("DeleteScript: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("script should not exist after delete")
	}
}

func TestDeleteScript_NotFound(t *testing.T) {
	withFakeScriptsDir(t)

	err := DeleteScript("nonexistent")
	if err != nil {
		t.Errorf("deleting nonexistent script should not error, got: %v", err)
	}
}

// --- SyncScripts ---

func TestSyncScripts(t *testing.T) {
	dir := withFakeScriptsDir(t)

	// Create an orphan script
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "orphan.sh"), []byte("#!/bin/sh\necho old\n"), 0755)

	jobs := []Job{
		{Name: "job-a", Command: "echo a"},
		{Name: "job-b", Command: "echo b"},
	}

	err := SyncScripts(jobs)
	if err != nil {
		t.Fatalf("SyncScripts: %v", err)
	}

	// Active scripts should exist
	for _, name := range []string{"job-a", "job-b"} {
		if _, err := os.Stat(ScriptPath(name)); os.IsNotExist(err) {
			t.Errorf("script for %q should exist", name)
		}
	}

	// Orphan should be removed
	if _, err := os.Stat(filepath.Join(dir, "orphan.sh")); !os.IsNotExist(err) {
		t.Error("orphan script should be deleted")
	}
}

// --- IsScriptRef ---

func TestIsScriptRef(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"sh /Users/me/.lazycron/scripts/my-job.sh", true},
		{"sh /home/ubuntu/.lazycron/scripts/my-job.sh", true},
		{"echo hello", false},
		{"sh /tmp/other-script.sh", false},
		{"sh", false},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := IsScriptRef(tt.command)
			if got != tt.want {
				t.Errorf("IsScriptRef(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// --- resolveScript ---

func TestResolveScript_Success(t *testing.T) {
	dir := withFakeScriptsDir(t)
	WriteScript("test-job", "echo resolved")

	ref := "sh " + dir + "/test-job.sh"
	got := resolveScript(ref)
	if got != "echo resolved" {
		t.Errorf("resolveScript(%q) = %q, want %q", ref, got, "echo resolved")
	}
}

func TestResolveScript_NotScriptRef(t *testing.T) {
	got := resolveScript("echo hello")
	if got != "echo hello" {
		t.Errorf("resolveScript should return non-ref as-is, got %q", got)
	}
}

func TestResolveScript_FileMissing(t *testing.T) {
	withFakeScriptsDir(t)
	ref := "sh /nonexistent/.lazycron/scripts/missing.sh"
	got := resolveScript(ref)
	if got != ref {
		t.Errorf("resolveScript should return ref as-is when file missing, got %q", got)
	}
}

// --- ScriptPath ---

func TestScriptPath(t *testing.T) {
	dir := withFakeScriptsDir(t)

	tests := []struct {
		name string
		want string
	}{
		{"my-job", dir + "/my-job.sh"},
		{"My Agent Job", dir + "/my-agent-job.sh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScriptPath(tt.name)
			if got != tt.want {
				t.Errorf("ScriptPath(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
