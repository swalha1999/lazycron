package cron

import (
	"os"
	"path/filepath"
	"testing"
)

// --- WriteScript / ReadScriptCommand ---

func TestWriteReadScript(t *testing.T) {
	withFakeScriptsDir(t)

	err := WriteScript("abc12345", "echo hello world")
	if err != nil {
		t.Fatalf("WriteScript: %v", err)
	}

	path := ScriptPath("abc12345")
	content, err := ReadScriptCommand(path)
	if err != nil {
		t.Fatalf("ReadScriptCommand: %v", err)
	}
	if content != "echo hello world" {
		t.Errorf("content = %q, want %q", content, "echo hello world")
	}

	// Verify raw file has shebang and preamble
	raw, _ := os.ReadFile(path)
	if string(raw) != "#!/bin/sh\n"+ScriptPreamble+"echo hello world\n" {
		t.Errorf("raw file = %q, want shebang + preamble + command", string(raw))
	}

	// Verify permissions
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0700 {
		t.Errorf("permissions = %o, want 0700", info.Mode().Perm())
	}
}

func TestWriteScript_MultilineCommand(t *testing.T) {
	withFakeScriptsDir(t)

	cmd := "cd /repo && claude -p --dangerously-skip-permissions \"Very long prompt with\nmultiple lines\""
	err := WriteScript("def67890", cmd)
	if err != nil {
		t.Fatalf("WriteScript: %v", err)
	}

	content, err := ReadScriptCommand(ScriptPath("def67890"))
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

	WriteScript("aabbccdd", "echo bye")
	path := ScriptPath("aabbccdd")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("script should exist before delete")
	}

	err := DeleteScript("aabbccdd")
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
		{ID: "aa112233", Name: "job-a", Command: "echo a"},
		{ID: "bb445566", Name: "job-b", Command: "echo b"},
	}

	err := SyncScripts(jobs)
	if err != nil {
		t.Fatalf("SyncScripts: %v", err)
	}

	// Active scripts should exist (named by ID)
	for _, id := range []string{"aa112233", "bb445566"} {
		if _, err := os.Stat(ScriptPath(id)); os.IsNotExist(err) {
			t.Errorf("script for %q should exist", id)
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
		{"sh /Users/me/.lazycron/scripts/abc12345.sh", true},
		{"sh /home/ubuntu/.lazycron/scripts/abc12345.sh", true},
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
	WriteScript("abc12345", "echo resolved")

	ref := "sh " + dir + "/abc12345.sh"
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
		id   string
		want string
	}{
		{"abc12345", dir + "/abc12345.sh"},
		{"deadbeef", dir + "/deadbeef.sh"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := ScriptPath(tt.id)
			if got != tt.want {
				t.Errorf("ScriptPath(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}
