package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// --- ExpandHome ---

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{"~/Documents", filepath.Join(home, "Documents")},
		{"~/.ssh/id_rsa", filepath.Join(home, ".ssh", "id_rsa")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", "~"}, // bare ~ without / is not expanded
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExpandHome(tt.input)
			if got != tt.want {
				t.Errorf("ExpandHome(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- collapseHome ---

func TestCollapseHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
		want  string
	}{
		{filepath.Join(home, "Documents"), "~/Documents"},
		{filepath.Join(home, ".ssh", "id_rsa"), "~/.ssh/id_rsa"},
		{"/other/path", "/other/path"},
		{home, home}, // exact home dir without trailing slash is not collapsed
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := collapseHome(tt.input)
			if got != tt.want {
				t.Errorf("collapseHome(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- Load / Save roundtrip ---

func TestLoadSave_Roundtrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	original := &Config{
		Servers: []ServerConfig{
			{
				Name:     "prod",
				Host:     "prod.example.com",
				Port:     22,
				User:     "deploy",
				KeyPath:  "~/.ssh/id_rsa",
				UseAgent: false,
			},
			{
				Name:     "staging",
				Host:     "staging.example.com",
				Port:     2222,
				User:     "admin",
				UseAgent: true,
			},
		},
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(loaded.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(loaded.Servers))
	}

	s := loaded.Servers[0]
	if s.Name != "prod" || s.Host != "prod.example.com" || s.Port != 22 || s.User != "deploy" {
		t.Errorf("server 0 mismatch: %+v", s)
	}
	// KeyPath should be expanded on load
	expectedKeyPath := filepath.Join(home, ".ssh", "id_rsa")
	if s.KeyPath != expectedKeyPath {
		t.Errorf("KeyPath = %q, want %q (expanded)", s.KeyPath, expectedKeyPath)
	}

	s2 := loaded.Servers[1]
	if s2.Name != "staging" || s2.Port != 2222 || !s2.UseAgent {
		t.Errorf("server 1 mismatch: %+v", s2)
	}
}

func TestSave_CollapsesKeyPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{
		Servers: []ServerConfig{
			{
				Name:    "test",
				Host:    "example.com",
				User:    "user",
				KeyPath: filepath.Join(home, ".ssh", "id_rsa"),
			},
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Read raw YAML to verify ~ is written
	data, err := os.ReadFile(filepath.Join(home, ".lazycron", "config.yml"))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	var raw Config
	yaml.Unmarshal(data, &raw)
	if raw.Servers[0].KeyPath != "~/.ssh/id_rsa" {
		t.Errorf("saved KeyPath = %q, want %q", raw.Servers[0].KeyPath, "~/.ssh/id_rsa")
	}
}

func TestSave_DoesNotMutateOriginal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	originalPath := filepath.Join(home, ".ssh", "id_rsa")
	cfg := &Config{
		Servers: []ServerConfig{
			{Name: "test", Host: "example.com", User: "user", KeyPath: originalPath},
		},
	}

	Save(cfg)

	// Original config should not be mutated
	if cfg.Servers[0].KeyPath != originalPath {
		t.Errorf("Save mutated original KeyPath: got %q, want %q", cfg.Servers[0].KeyPath, originalPath)
	}
}

// --- Load edge cases ---

func TestLoad_NoConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(cfg.Servers))
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".lazycron")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(":::invalid"), 0o600)

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_EmptyConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configDir := filepath.Join(home, ".lazycron")
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(""), 0o600)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(cfg.Servers))
	}
}

// --- AddServer / RemoveServer ---

func TestAddServer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := AddServer(ServerConfig{Name: "server-1", Host: "host1.com", User: "user1"})
	if err != nil {
		t.Fatalf("AddServer error: %v", err)
	}

	err = AddServer(ServerConfig{Name: "server-2", Host: "host2.com", User: "user2"})
	if err != nil {
		t.Fatalf("AddServer error: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(cfg.Servers))
	}
	if cfg.Servers[0].Name != "server-1" || cfg.Servers[1].Name != "server-2" {
		t.Errorf("server names mismatch: %v", cfg.Servers)
	}
}

func TestRemoveServer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	AddServer(ServerConfig{Name: "keep", Host: "host1.com", User: "user1"})
	AddServer(ServerConfig{Name: "remove", Host: "host2.com", User: "user2"})
	AddServer(ServerConfig{Name: "also-keep", Host: "host3.com", User: "user3"})

	err := RemoveServer("remove")
	if err != nil {
		t.Fatalf("RemoveServer error: %v", err)
	}

	cfg, _ := Load()
	if len(cfg.Servers) != 2 {
		t.Fatalf("expected 2 servers after removal, got %d", len(cfg.Servers))
	}
	for _, s := range cfg.Servers {
		if s.Name == "remove" {
			t.Error("removed server still present")
		}
	}
}

func TestRemoveServer_NotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	AddServer(ServerConfig{Name: "existing", Host: "host.com", User: "user"})

	err := RemoveServer("nonexistent")
	if err != nil {
		t.Fatalf("RemoveServer error: %v", err)
	}

	cfg, _ := Load()
	if len(cfg.Servers) != 1 {
		t.Errorf("expected 1 server unchanged, got %d", len(cfg.Servers))
	}
}

// --- Save file permissions ---

func TestSave_FilePermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	Save(&Config{})

	path := filepath.Join(home, ".lazycron", "config.yml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat error: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("config file permissions = %o, want 600", perm)
	}
}
