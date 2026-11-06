package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/setup"
)

// scriptedHost is a setup.Host where herdr-core is installed and everything
// else is already in place, except what the test says.
type scriptedHost struct {
	home  string
	ran   []string
	files map[string]string
}

func (h *scriptedHost) OS() string   { return "linux" }
func (h *scriptedHost) Home() string { return h.home }
func (h *scriptedHost) User() string { return "u" }
func (h *scriptedHost) Shell(c string) (string, error) {
	h.ran = append(h.ran, c)
	switch {
	case strings.HasPrefix(c, "command -v herdr-core"):
		return "/opt/herdr-core", nil
	case strings.HasSuffix(c, "--version"):
		return "hc 0.1.18", nil
	case strings.HasPrefix(c, "loginctl show-user"):
		return "yes", nil
	case strings.HasPrefix(c, "readlink"), strings.HasPrefix(c, "systemctl --user is-enabled"):
		return "", errors.New("missing")
	}
	return "", nil
}
func (h *scriptedHost) ReadFile(p string) ([]byte, error) { return nil, os.ErrNotExist }
func (h *scriptedHost) WriteFile(p string, d []byte, _ os.FileMode) error {
	h.files[p] = string(d)
	return nil
}

func useScriptedHost(t *testing.T, env *testEnv) *scriptedHost {
	h := &scriptedHost{home: env.home, files: map[string]string{}}
	orig := localSetupHost
	localSetupHost = func() setup.Host { return h }
	t.Cleanup(func() { localSetupHost = orig })
	return h
}

func TestSetupDryRunChangesNothing(t *testing.T) {
	env := newTestEnv(t)
	host := useScriptedHost(t, env)
	env.mustRun(addArgs("j", "J", "every hour", "true")...)
	before, _ := os.ReadFile(env.cronFile)

	var rep setupReport
	env.mustRun("setup", "--dry-run").decode(t, &rep)
	if !rep.DryRun || rep.Runner != config.RunnerHerdr {
		t.Fatalf("unexpected report: %+v", rep)
	}
	after, _ := os.ReadFile(env.cronFile)
	if string(before) != string(after) || len(host.files) != 0 {
		t.Fatal("dry run changed files")
	}
	if _, err := os.Stat(filepath.Join(env.home, ".lazycron", "config.yml")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote the config")
	}
	for _, c := range host.ran {
		if strings.HasPrefix(c, "systemctl --user daemon-reload") || strings.HasPrefix(c, "ln ") || strings.Contains(c, "curl") {
			t.Fatalf("dry run ran %q", c)
		}
	}
}

func TestSetupSwitchesToHerdrAndMigratesCrontab(t *testing.T) {
	env := newTestEnv(t)
	host := useScriptedHost(t, env)
	env.mustRun(addArgs("j", "J", "every hour", "echo hi")...)
	if data, _ := os.ReadFile(env.cronFile); strings.Contains(string(data), " exec ") {
		t.Fatal("plain default should write record lines")
	}

	var rep setupReport
	env.mustRun("setup").decode(t, &rep)
	if rep.Runner != config.RunnerHerdr || rep.Failed() {
		t.Fatalf("setup: %+v", rep)
	}
	data, _ := os.ReadFile(env.cronFile)
	if !strings.Contains(string(data), " exec 'j' --name 'J'") {
		t.Fatalf("crontab not migrated:\n%s", data)
	}
	cfg, err := config.Load()
	if err != nil || cfg.Runner != config.RunnerHerdr || cfg.Herdr.SpaceID == "" {
		t.Fatalf("config not updated: %+v %v", cfg, err)
	}
	if !strings.Contains(host.files[filepath.Join(env.home, ".config/systemd/user/lazycron-herdr.service")], "ExecStart=/opt/herdr-core server start") {
		t.Fatalf("unit not written: %v", host.files)
	}
	if jobs := env.jobs(); jobs[0].Command != "echo hi" || jobs[0].ID != "j" {
		t.Fatalf("migration changed the job: %+v", jobs[0])
	}

	// Second run: crontab and config already match.
	env.mustRun("setup").decode(t, &rep)
	for _, s := range rep.Steps {
		if (s.Name == "crontab" || s.Name == "config") && s.Status != setup.StatusOK {
			t.Errorf("second setup: %s %s (%s)", s.Name, s.Status, s.Detail)
		}
	}

	// And back to plain.
	env.mustRun("setup", "--runner", "plain").decode(t, &rep)
	if data, _ := os.ReadFile(env.cronFile); strings.Contains(string(data), " exec ") {
		t.Fatalf("--runner plain should restore record lines:\n%s", data)
	}
}
