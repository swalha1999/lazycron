package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectConfig_Missing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil cfg, got %+v", cfg)
	}
}

func TestLoadProjectConfig_OK(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte("name: my-project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.Name != "my-project" {
		t.Fatalf("got %+v, want Name=my-project", cfg)
	}
}

func TestLoadProjectConfig_BadYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"),
		[]byte(":\nbroken yaml::\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProjectConfig(dir); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestSaveAndLoadProjectConfig_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	in := &ProjectConfig{Name: "round-trip"}
	if err := SaveProjectConfig(dir, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := LoadProjectConfig(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out == nil || out.Name != in.Name {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", out, in)
	}
}

func TestResolveProjectName(t *testing.T) {
	tests := []struct {
		name     string
		override string
		cfg      *ProjectConfig
		cwd      string
		want     string
	}{
		{"override wins", "explicit", &ProjectConfig{Name: "from-cfg"}, "/repos/cwd-name", "explicit"},
		{"cfg used when no override", "", &ProjectConfig{Name: "from-cfg"}, "/repos/cwd-name", "from-cfg"},
		{"cwd basename when cfg nil", "", nil, "/repos/cwd-name", "cwd-name"},
		{"cwd basename when cfg empty name", "", &ProjectConfig{}, "/repos/cwd-name", "cwd-name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveProjectName(tc.override, tc.cfg, tc.cwd)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
