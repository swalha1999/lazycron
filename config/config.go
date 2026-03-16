package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ServerConfig represents a remote server entry in the config file.
type ServerConfig struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port,omitempty"`
	User     string `yaml:"user"`
	KeyPath  string `yaml:"key_path,omitempty"`
	UseAgent bool   `yaml:"use_agent,omitempty"`
}

// Config is the top-level configuration structure.
type Config struct {
	Servers []ServerConfig `yaml:"servers"`
}

// configPath returns the path to ~/.lazycron/config.yml.
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "config.yml")
}

// Load reads and parses ~/.lazycron/config.yml.
// Returns an empty config (not an error) if the file doesn't exist.
func Load() (*Config, error) {
	path := configPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Expand ~ in key paths
	for i := range cfg.Servers {
		if cfg.Servers[i].KeyPath != "" {
			cfg.Servers[i].KeyPath = ExpandHome(cfg.Servers[i].KeyPath)
		}
	}

	return &cfg, nil
}

// Save writes the config to ~/.lazycron/config.yml.
func Save(cfg *Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Collapse home dir back to ~ for key paths before saving
	saveCfg := *cfg
	saveCfg.Servers = make([]ServerConfig, len(cfg.Servers))
	copy(saveCfg.Servers, cfg.Servers)
	for i := range saveCfg.Servers {
		if saveCfg.Servers[i].KeyPath != "" {
			saveCfg.Servers[i].KeyPath = collapseHome(saveCfg.Servers[i].KeyPath)
		}
	}

	data, err := yaml.Marshal(&saveCfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// AddServer appends a server to the config and saves.
func AddServer(srv ServerConfig) error {
	cfg, err := Load()
	if err != nil {
		cfg = &Config{}
	}
	cfg.Servers = append(cfg.Servers, srv)
	return Save(cfg)
}

// RemoveServer removes a server by name from the config and saves.
func RemoveServer(name string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	filtered := cfg.Servers[:0]
	for _, s := range cfg.Servers {
		if s.Name != name {
			filtered = append(filtered, s)
		}
	}
	cfg.Servers = filtered
	return Save(cfg)
}

// ExpandHome replaces a leading ~/ with the user's home directory.
func ExpandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func collapseHome(path string) string {
	home, _ := os.UserHomeDir()
	if home != "" && len(path) > len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}
