package cron

import (
	"os"
	"path/filepath"
	"strings"
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
	if string(raw) != "#!/usr/bin/env bash\n"+ScriptPreamble+"echo hello world\n" {
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

func TestWriteScript_RefusesSelfRef(t *testing.T) {
	dir := withFakeScriptsDir(t)

	selfRef := "bash '" + dir + "/abc12345.sh'"
	err := WriteScript("abc12345", selfRef)
	if err == nil {
		t.Fatal("expected error when command is a script ref")
	}
	if !strings.Contains(err.Error(), "self-referential") {
		t.Errorf("error should mention self-referential, got %q", err.Error())
	}

	// Crucial: no file should have been written. A file with a self-ref body
	// becomes a fork bomb the first time it runs.
	if _, statErr := os.Stat(ScriptPath("abc12345")); !os.IsNotExist(statErr) {
		t.Error("script file should not exist after self-ref guard fires")
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
		{"bash /Users/me/.lazycron/scripts/abc12345.sh", true},
		{"bash /home/ubuntu/.lazycron/scripts/abc12345.sh", true},
		{"sh /Users/me/.lazycron/scripts/abc12345.sh", true}, // legacy
		{"sh /home/ubuntu/.lazycron/scripts/abc12345.sh", true}, // legacy
		{"echo hello", false},
		{"bash /tmp/other-script.sh", false},
		{"sh /tmp/other-script.sh", false},
		{"sh", false},
		{"bash", false},
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

// --- ScriptRefPath ---

func TestScriptRefPath(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"bash '/home/u/.lazycron/scripts/x.sh'", "/home/u/.lazycron/scripts/x.sh"},
		{"sh '/home/u/.lazycron/scripts/x.sh'", "/home/u/.lazycron/scripts/x.sh"},
		{"bash /home/u/.lazycron/scripts/x.sh", "/home/u/.lazycron/scripts/x.sh"},
		{"echo not-a-ref", ""},
		{"bash /tmp/other.sh", ""},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := ScriptRefPath(tt.command)
			if got != tt.want {
				t.Errorf("ScriptRefPath(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

// --- resolveScript ---

func TestResolveScript_Success(t *testing.T) {
	dir := withFakeScriptsDir(t)
	WriteScript("abc12345", "echo resolved")

	ref := "bash " + dir + "/abc12345.sh"
	got := resolveScript(ref)
	if got != "echo resolved" {
		t.Errorf("resolveScript(%q) = %q, want %q", ref, got, "echo resolved")
	}
}

func TestResolveScript_LegacyShPrefix(t *testing.T) {
	dir := withFakeScriptsDir(t)
	WriteScript("abc12345", "echo legacy")

	ref := "sh " + dir + "/abc12345.sh"
	got := resolveScript(ref)
	if got != "echo legacy" {
		t.Errorf("resolveScript(%q) = %q, want %q", ref, got, "echo legacy")
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

// --- StripProjectCd ---

func TestStripProjectCd(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCmd     string
		wantProject string
	}{
		{
			name:        "tilde path",
			input:       "cd ~/.lazycron/projects/foo && .sandcastle/node_modules/.bin/tsx --env-file-if-exists=.sandcastle/.env .sandcastle/jobs/x.ts",
			wantCmd:     ".sandcastle/node_modules/.bin/tsx --env-file-if-exists=.sandcastle/.env .sandcastle/jobs/x.ts",
			wantProject: "~/.lazycron/projects/foo",
		},
		{
			name:        "absolute path",
			input:       "cd /home/user/.lazycron/projects/foo && echo hi",
			wantCmd:     "echo hi",
			wantProject: "/home/user/.lazycron/projects/foo",
		},
		{
			name:        "single-quoted path with spaces",
			input:       "cd '/home/me/path with spaces' && do-thing",
			wantCmd:     "do-thing",
			wantProject: "/home/me/path with spaces",
		},
		{
			name:        "double-quoted path",
			input:       `cd "/var/lib/foo" && echo hi`,
			wantCmd:     "echo hi",
			wantProject: "/var/lib/foo",
		},
		{
			name:        "no cd prefix at all",
			input:       "echo hi",
			wantCmd:     "echo hi",
			wantProject: "",
		},
		{
			name:        "cd without && returns original",
			input:       "cd /tmp",
			wantCmd:     "cd /tmp",
			wantProject: "",
		},
		{
			name:        "cd then semicolon (not &&) returns original",
			input:       "cd /tmp; echo hi",
			wantCmd:     "cd /tmp; echo hi",
			wantProject: "",
		},
		{
			name:        "extra whitespace around &&",
			input:       "cd /foo   &&   echo bar",
			wantCmd:     "echo bar",
			wantProject: "/foo",
		},
		{
			name:        "unterminated single quote returns original",
			input:       "cd '/foo && bar",
			wantCmd:     "cd '/foo && bar",
			wantProject: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotCmd, gotProject := StripProjectCd(tc.input)
			if gotCmd != tc.wantCmd {
				t.Errorf("cmd = %q, want %q", gotCmd, tc.wantCmd)
			}
			if gotProject != tc.wantProject {
				t.Errorf("project = %q, want %q", gotProject, tc.wantProject)
			}
		})
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
