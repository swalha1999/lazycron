package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig holds the project-local config from .lazycron/config.yaml.
// It's distinct from the global ~/.lazycron/config.yml (server registry) —
// this one travels with the repo and identifies a single project.
type ProjectConfig struct {
	// Name is the project's display name and the directory used on remote
	// machines under ~/.lazycron/projects/<name>/.
	Name string `yaml:"name"`
}

// ProjectConfigPath returns the conventional path to a project config given
// the project's .lazycron directory.
func ProjectConfigPath(lazycronDir string) string {
	return filepath.Join(lazycronDir, "config.yaml")
}

// LoadProjectConfig reads .lazycron/config.yaml from lazycronDir.
// Returns (nil, nil) if the file doesn't exist — callers should treat that
// as "no project config", not an error.
func LoadProjectConfig(lazycronDir string) (*ProjectConfig, error) {
	path := ProjectConfigPath(lazycronDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// SaveProjectConfig writes a ProjectConfig to .lazycron/config.yaml.
// Creates lazycronDir if it doesn't exist.
func SaveProjectConfig(lazycronDir string, cfg *ProjectConfig) error {
	if err := os.MkdirAll(lazycronDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", lazycronDir, err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}
	return os.WriteFile(ProjectConfigPath(lazycronDir), data, 0o644)
}

// ResolveProjectName returns the project name to use for a given working
// directory, applying the precedence: explicit override > config.yaml name
// > basename of cwd. cwd should be an absolute path.
func ResolveProjectName(override string, cfg *ProjectConfig, cwd string) string {
	if override != "" {
		return override
	}
	if cfg != nil && cfg.Name != "" {
		return cfg.Name
	}
	return filepath.Base(cwd)
}
