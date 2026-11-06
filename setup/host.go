// Package setup prepares a machine for lazycron's herdr mode: herdr-core
// installed, the lazycron link cron calls, a user service that starts the
// core at boot, systemd linger so it runs with nobody logged in, and the
// crontab rewritten to the current format. Every step is idempotent and
// supports dry-run.
package setup

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// Host is a machine setup acts on: this one or a server over SSH.
type Host interface {
	OS() string // runtime.GOOS values: "linux", "darwin"
	Home() string
	User() string
	// Shell runs a sh command line and returns its combined output.
	Shell(command string) (string, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, mode os.FileMode) error
}

// LocalHost is this machine.
type LocalHost struct{}

func (LocalHost) OS() string { return runtime.GOOS }

func (LocalHost) Home() string {
	home, _ := os.UserHomeDir()
	return home
}

func (LocalHost) User() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

func (LocalHost) Shell(command string) (string, error) {
	out, err := exec.Command("sh", "-c", command).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (LocalHost) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (LocalHost) WriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}
