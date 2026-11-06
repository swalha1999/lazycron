package config

import (
	"reflect"
	"regexp"
	"testing"
	"time"
)

func TestNewSpaceIDFormat(t *testing.T) {
	re := regexp.MustCompile(`^sp[0-9a-f]{32}$`)
	a, b := NewSpaceID(), NewSpaceID()
	if !re.MatchString(a) || !re.MatchString(b) {
		t.Fatalf("bad space ids %q %q", a, b)
	}
	if a == b {
		t.Fatal("space ids should be random")
	}
}

func TestEnsureSpaceIDIsStable(t *testing.T) {
	var c Config
	if !c.EnsureSpaceID() {
		t.Fatal("first call should set an id")
	}
	id := c.Herdr.SpaceID
	if c.EnsureSpaceID() || c.Herdr.SpaceID != id {
		t.Fatal("second call must keep the id")
	}
}

func TestHoldDuration(t *testing.T) {
	cases := map[string]time.Duration{"": DefaultHold, "0": 0, "2h": 2 * time.Hour}
	for in, want := range cases {
		got, err := HerdrConfig{Hold: in}.HoldDuration()
		if err != nil || got != want {
			t.Errorf("Hold %q = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := (HerdrConfig{Hold: "soon"}).HoldDuration(); err == nil {
		t.Error("expected error for invalid hold")
	}
}

func TestDefaultsAndRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultRunner() != RunnerPlain || cfg.Herdr.KeepRunsOrDefault() != DefaultKeepRuns {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	cfg.Runner = RunnerHerdr
	cfg.Herdr.KeepRuns = 3
	cfg.EnsureSpaceID()
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	back, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if back.DefaultRunner() != RunnerHerdr || !reflect.DeepEqual(back.Herdr, cfg.Herdr) {
		t.Fatalf("round trip lost data: %+v", back)
	}
}

func TestValidRunner(t *testing.T) {
	for _, ok := range []string{"", "plain", "herdr"} {
		if !ValidRunner(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	if ValidRunner("docker") {
		t.Error("docker should be invalid")
	}
}
