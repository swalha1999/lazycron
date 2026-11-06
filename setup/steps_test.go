package setup

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeHost answers shell commands from a table (first matching prefix
// wins) and keeps files in memory.
type fakeHost struct {
	os      string
	answers []answer
	files   map[string]string
	ran     []string
}

type answer struct {
	prefix string
	out    string
	err    error
}

func newFakeHost(osName string, answers ...answer) *fakeHost {
	return &fakeHost{os: osName, answers: answers, files: map[string]string{}}
}

func (f *fakeHost) OS() string   { return f.os }
func (f *fakeHost) Home() string { return "/home/u" }
func (f *fakeHost) User() string { return "u" }

func (f *fakeHost) Shell(cmd string) (string, error) {
	f.ran = append(f.ran, cmd)
	for _, a := range f.answers {
		if strings.HasPrefix(cmd, a.prefix) {
			return a.out, a.err
		}
	}
	return "", errors.New("unexpected command")
}

func (f *fakeHost) ReadFile(p string) ([]byte, error) {
	if c, ok := f.files[p]; ok {
		return []byte(c), nil
	}
	return nil, os.ErrNotExist
}

func (f *fakeHost) WriteFile(p string, data []byte, _ os.FileMode) error {
	f.files[p] = string(data)
	return nil
}

func (f *fakeHost) ranAny(prefix string) bool {
	for _, c := range f.ran {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func statusOf(t *testing.T, r Result, name string) Status {
	t.Helper()
	for _, s := range r.Steps {
		if s.Name == name {
			return s.Status
		}
	}
	t.Fatalf("no step %q in %+v", name, r.Steps)
	return ""
}

var (
	noHC       = answer{prefix: "command -v herdr-core", err: errors.New("exit 1")}
	hasHC      = answer{prefix: "command -v herdr-core", out: "/home/u/.local/bin/herdr-core"}
	hcVersion  = answer{prefix: "'/home/u/.local/bin/herdr-core' --version", out: "hc 0.1.18"}
	noLink     = answer{prefix: "readlink", err: errors.New("no link")}
	notEnabled = answer{prefix: "systemctl --user is-enabled", err: errors.New("disabled")}
	noLinger   = answer{prefix: "loginctl show-user", out: "no"}
)

func fullOpts() Options {
	return Options{InstallHerdr: true, Service: true, Linger: true, LazycronPath: "/opt/lazycron"}
}

func TestDryRunChangesNothing(t *testing.T) {
	h := newFakeHost("linux", noHC, noLink, notEnabled, noLinger)
	opts := fullOpts()
	opts.DryRun = true
	r := Run(h, opts)

	for _, name := range []string{"herdr-core", "lazycron link", "boot service", "linger"} {
		if got := statusOf(t, r, name); got != StatusWouldChange {
			t.Errorf("%s: %s, want would_change", name, got)
		}
	}
	for _, mutating := range []string{"curl", "ln ", "mkdir", "systemctl --user daemon-reload", "loginctl enable-linger"} {
		if h.ranAny(mutating) {
			t.Errorf("dry run ran %q", mutating)
		}
	}
	if len(h.files) != 0 {
		t.Errorf("dry run wrote files: %v", h.files)
	}
	if !r.HerdrReady {
		t.Error("dry run should report herdr as ready after the planned install")
	}
}

func TestFreshLinuxSetup(t *testing.T) {
	installed := false
	h := newFakeHost("linux",
		answer{prefix: "curl -fsSL " + HerdrCoreInstaller + " | sh", out: "hc 0.1.18 installed"},
		hcVersion, noLink,
		answer{prefix: "mkdir -p '/home/u/.lazycron/bin' && ln -sfn '/opt/lazycron'"},
		notEnabled,
		answer{prefix: "systemctl --user daemon-reload && systemctl --user enable --now lazycron-herdr.service"},
		noLinger,
		answer{prefix: "loginctl enable-linger 'u'"},
	)
	// herdr-core is missing until the installer runs.
	h.answers = append([]answer{noHC}, h.answers...)
	r := Run(&installingHost{fakeHost: h, installed: &installed}, fullOpts())

	if r.Failed() {
		t.Fatalf("setup failed: %+v", r.Steps)
	}
	for _, name := range []string{"herdr-core", "lazycron link", "boot service", "linger"} {
		if got := statusOf(t, r, name); got != StatusChanged {
			t.Errorf("%s: %s, want changed", name, got)
		}
	}
	unit := h.files["/home/u/.config/systemd/user/lazycron-herdr.service"]
	if !strings.Contains(unit, "ExecStart=/home/u/.local/bin/herdr-core server start") || !strings.Contains(unit, "WantedBy=default.target") {
		t.Fatalf("unexpected unit:\n%s", unit)
	}
	if r.HCVersion != "0.1.18" || !r.HerdrReady {
		t.Fatalf("result: %+v", r)
	}
}

// installingHost makes herdr-core discoverable once the installer ran.
type installingHost struct {
	*fakeHost
	installed *bool
}

func (h *installingHost) Shell(cmd string) (string, error) {
	if strings.HasPrefix(cmd, "curl ") {
		*h.installed = true
	}
	if strings.HasPrefix(cmd, "command -v herdr-core") && *h.installed {
		return "/home/u/.local/bin/herdr-core", nil
	}
	return h.fakeHost.Shell(cmd)
}

func TestSecondRunIsAllOK(t *testing.T) {
	h := newFakeHost("linux", hasHC, hcVersion,
		answer{prefix: "readlink", out: "/opt/lazycron"},
		answer{prefix: "systemctl --user is-enabled"},
		answer{prefix: "loginctl show-user", out: "yes"},
	)
	svc, _ := serviceFor(h, "/home/u/.local/bin/herdr-core")
	h.files[svc.Path] = svc.Content
	r := Run(h, fullOpts())
	for _, s := range r.Steps {
		if s.Status != StatusOK {
			t.Errorf("%s: %s (%s)", s.Name, s.Status, s.Detail)
		}
	}
}

func TestNoHerdrOptOut(t *testing.T) {
	h := newFakeHost("linux", noHC, noLink, noLinger, answer{prefix: "mkdir -p"}, answer{prefix: "loginctl enable-linger"})
	opts := fullOpts()
	opts.InstallHerdr = false
	r := Run(h, opts)
	if statusOf(t, r, "herdr-core") != StatusSkipped || r.HerdrReady || h.ranAny("curl") {
		t.Fatalf("opt-out must not install: %+v", r)
	}
	for _, s := range r.Steps {
		if s.Name == "boot service" {
			t.Fatal("no service without herdr-core")
		}
	}
}

func TestOldHerdrIsAWarning(t *testing.T) {
	h := newFakeHost("darwin", hasHC,
		answer{prefix: "'/home/u/.local/bin/herdr-core' --version", out: "hc 0.1.2"},
		answer{prefix: "readlink", out: "/opt/lazycron"},
		answer{prefix: "launchctl print", err: errors.New("not loaded")},
		answer{prefix: "launchctl bootout"},
	)
	r := Run(h, fullOpts())
	if statusOf(t, r, "herdr-core version") != StatusWarning {
		t.Fatalf("old version should warn: %+v", r.Steps)
	}
	if statusOf(t, r, "linger") != StatusSkipped {
		t.Fatal("linger is Linux-only")
	}
	plist := h.files["/home/u/Library/LaunchAgents/dev.lazycron.herdr-core.plist"]
	if !strings.Contains(plist, "<string>/home/u/.local/bin/herdr-core</string>") || !strings.Contains(plist, "RunAtLoad") {
		t.Fatalf("plist:\n%s", plist)
	}
	if h.ranAny("hc update") || h.ranAny("server stop") || h.ranAny("server restart") {
		t.Fatal("setup must never update or restart the core")
	}
}

func TestLingerFailureHasHint(t *testing.T) {
	h := newFakeHost("linux", hasHC, hcVersion, answer{prefix: "readlink", out: "/opt/lazycron"},
		answer{prefix: "systemctl --user is-enabled"}, noLinger,
		answer{prefix: "loginctl enable-linger", err: errors.New("access denied")})
	svc, _ := serviceFor(h, "/home/u/.local/bin/herdr-core")
	h.files[svc.Path] = svc.Content
	r := Run(h, fullOpts())
	if !r.Failed() || r.Steps[len(r.Steps)-1].Hint != "run: sudo loginctl enable-linger u" {
		t.Fatalf("unexpected: %+v", r.Steps)
	}
}
