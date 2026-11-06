package setup

import (
	"errors"
	"testing"
)

func healthyAnswers() []answer {
	return []answer{
		{prefix: "command -v crontab", out: "/usr/bin/crontab"},
		{prefix: "pgrep", out: "123"},
		{prefix: "command -v lazycron", out: "/usr/local/bin/lazycron"},
		{prefix: "[ -x '/home/u/.lazycron/bin/lazycron' ]", out: "/usr/local/bin/lazycron"},
		hasHC, hcVersion,
		{prefix: "'/home/u/.local/bin/herdr-core' server status --json", out: `{"version":"0.1.18","live":3}`},
		{prefix: "systemctl --user is-enabled"},
		{prefix: "loginctl show-user", out: "yes"},
		{prefix: "command -v claude", out: "/usr/bin/claude"},
		{prefix: "claude auth status --json", out: `{"loggedIn":true}`},
	}
}

func TestDoctorHealthy(t *testing.T) {
	steps := Doctor(newFakeHost("linux", healthyAnswers()...), DoctorOptions{Runner: "herdr"})
	if len(steps) != 9 {
		t.Fatalf("want 9 checks, got %d: %+v", len(steps), steps)
	}
	for _, s := range steps {
		if s.Status != StatusOK {
			t.Errorf("%s: %s (%s)", s.Name, s.Status, s.Detail)
		}
	}
}

func TestDoctorMissingHerdrDependsOnRunner(t *testing.T) {
	answers := []answer{
		{prefix: "command -v crontab", out: "/usr/bin/crontab"},
		{prefix: "pgrep", err: errors.New("none")},
		noHC, noLinger,
		{prefix: "command -v claude", err: errors.New("none")},
	}
	for runner, want := range map[string]Status{"herdr": StatusFailed, "plain": StatusWarning} {
		steps := Doctor(newFakeHost("linux", answers...), DoctorOptions{Runner: runner})
		got := map[string]Status{}
		for _, s := range steps {
			got[s.Name] = s.Status
		}
		if got["herdr-core"] != want || got["linger"] != want || got["lazycron link"] != want {
			t.Errorf("runner %s: %v", runner, got)
		}
		if got["cron daemon"] != StatusFailed || got["claude"] != StatusSkipped || got["PATH"] != StatusWarning {
			t.Errorf("runner %s: %v", runner, got)
		}
		if _, ok := got["herdr-core server"]; ok {
			t.Error("no server check without herdr-core")
		}
	}
}
