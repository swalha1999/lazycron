package record

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSelfLink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, "opt", "lazycron")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := executable
	t.Cleanup(func() { executable = orig })
	executable = func() (string, error) { return bin, nil }

	// HOME is under the temp dir, so pretend temp is elsewhere.
	t.Setenv("TMPDIR", "/nonexistent-tmp")
	InstallSelfLink()
	InstallSelfLink() // idempotent

	got, err := os.Readlink(SelfLinkPath())
	if err != nil {
		t.Fatalf("link missing: %v", err)
	}
	want, _ := filepath.EvalSymlinks(bin)
	if got != want {
		t.Fatalf("link -> %q, want %q", got, want)
	}
}

func TestInstallSelfLinkSkipsTestBinaries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	InstallSelfLink() // running as a .test binary
	if _, err := os.Lstat(SelfLinkPath()); !os.IsNotExist(err) {
		t.Fatal("test binary must not be linked")
	}
}
