package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Runner names. A job runs either as a plain cron command or inside a
// herdr-core terminal.
const (
	RunnerPlain = "plain"
	RunnerHerdr = "herdr"
)

// Defaults for the herdr runner.
const (
	DefaultKeepRuns  = 5
	DefaultHold      = 72 * time.Hour
	DefaultSpaceName = "Cron"
)

// HerdrConfig holds settings for running jobs through herdr-core.
type HerdrConfig struct {
	// SpaceID is the herdr-app space run terminals are labeled into
	// (herdr.space). "sp" followed by 32 lowercase hex characters.
	SpaceID string `yaml:"space_id,omitempty"`
	// KeepRuns is how many finished run terminals to keep per job.
	KeepRuns int `yaml:"keep_runs,omitempty"`
	// Hold is how long a finished run terminal stays live so herdr-app can
	// show it (Go duration, e.g. "72h"; "0" ends it with the job).
	Hold string `yaml:"hold,omitempty"`
	// Bin overrides the herdr-core binary (default: hc on PATH or ~/.local/bin).
	Bin string `yaml:"bin,omitempty"`
	// Prompted records that the TUI already showed the herdr-core notice.
	Prompted bool `yaml:"prompted,omitempty"`
}

// ValidRunner reports whether name is a known runner ("" means default).
func ValidRunner(name string) bool {
	return name == "" || name == RunnerPlain || name == RunnerHerdr
}

// DefaultRunner returns the configured default runner, RunnerPlain if unset.
func (c *Config) DefaultRunner() string {
	if c.Runner == "" {
		return RunnerPlain
	}
	return c.Runner
}

// KeepRunsOrDefault returns KeepRuns, or DefaultKeepRuns when unset.
func (h HerdrConfig) KeepRunsOrDefault() int {
	if h.KeepRuns <= 0 {
		return DefaultKeepRuns
	}
	return h.KeepRuns
}

// HoldDuration parses Hold, returning DefaultHold when unset.
func (h HerdrConfig) HoldDuration() (time.Duration, error) {
	if h.Hold == "" {
		return DefaultHold, nil
	}
	if h.Hold == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(h.Hold)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid herdr.hold %q: want a duration like 72h or 0", h.Hold)
	}
	return d, nil
}

// NewSpaceID returns a fresh herdr space id: "sp" + 32 lowercase hex.
func NewSpaceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return "sp" + hex.EncodeToString(b)
}

// EnsureSpaceID gives the config a space id if it has none and reports
// whether it changed (the caller saves).
func (c *Config) EnsureSpaceID() bool {
	if c.Herdr.SpaceID != "" {
		return false
	}
	c.Herdr.SpaceID = NewSpaceID()
	return true
}
