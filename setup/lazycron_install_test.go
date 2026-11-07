package setup

import (
	"errors"
	"strings"
	"testing"
)

var (
	noLazycron        = answer{prefix: "command -v lazycron", err: errors.New("exit 1")}
	globalLazycron    = answer{prefix: "command -v lazycron", out: "/usr/local/bin/lazycron"}
	installerOK       = answer{prefix: lazycronInstallCmd, out: "lazycron v0.15.0 installed"}
	installerNeedSudo = answer{prefix: lazycronInstallCmd, out: "sudo: a terminal is required", err: errors.New("exit 1")}
	linkOK            = answer{prefix: "readlink", out: "/usr/local/bin/lazycron"}
	linkMissing       = answer{prefix: "readlink", err: errors.New("no link")}
	linked            = answer{prefix: "mkdir -p", out: ""}
)

func globalVersion(v string) answer {
	return answer{prefix: "'/usr/local/bin/lazycron' --version", out: "lazycron version " + v}
}

// runInstallAndLink runs the two lazycron steps the way Run does.
func runInstallAndLink(h Host, opts Options) Result {
	s := &stepRunner{h: h, opts: opts}
	s.lazycronInstall()
	s.lazycronLink()
	return s.res
}

func TestServerWithoutLazycron_RunsOfficialInstallerAndLinks(t *testing.T) {
	// First lookup finds nothing; after the installer it is on PATH.
	h := &sequenceHost{fakeHost: newFakeHost("linux", installerOK, linkMissing, linked),
		lookups: []answer{noLazycron, globalLazycron, globalLazycron}}
	res := runInstallAndLink(h, Options{InstallLazycron: true, Version: "0.15.0"})

	if got := statusOf(t, res, "lazycron"); got != StatusChanged {
		t.Fatalf("lazycron = %s, steps %+v", got, res.Steps)
	}
	if !h.ranAny("curl -fsSL https://get.lazycron.com | sh -s -- --no-herdr") {
		t.Errorf("official installer not run; ran %v", h.ran)
	}
	if got := statusOf(t, res, "lazycron link"); got != StatusChanged {
		t.Errorf("link = %s", got)
	}
	if !h.ranAny("mkdir -p '/home/u/.lazycron/bin' && ln -sfn '/usr/local/bin/lazycron' '/home/u/.lazycron/bin/lazycron'") {
		t.Errorf("link not pointed at the global install; ran %v", h.ran)
	}
}

func TestServerLazycronCurrent_NothingToDo(t *testing.T) {
	h := newFakeHost("linux", globalLazycron, globalVersion("0.15.0"), linkOK)
	res := runInstallAndLink(h, Options{InstallLazycron: true, Version: "0.15.0"})
	if statusOf(t, res, "lazycron") != StatusOK || statusOf(t, res, "lazycron link") != StatusOK {
		t.Errorf("steps %+v", res.Steps)
	}
	if h.ranAny("curl") {
		t.Error("installer must not run when lazycron is current")
	}
}

func TestServerLazycronOlder_RerunsInstaller(t *testing.T) {
	h := newFakeHost("linux", globalLazycron, globalVersion("0.14.0"), installerOK, linkOK)
	res := runInstallAndLink(h, Options{InstallLazycron: true, Version: "0.15.0"})
	if got := statusOf(t, res, "lazycron"); got != StatusChanged {
		t.Fatalf("lazycron = %s", got)
	}
	if !h.ranAny(lazycronInstallCmd) {
		t.Error("installer not rerun for an older lazycron")
	}
}

func TestServerLazycronNewerOrDevBuild_LeftAlone(t *testing.T) {
	for _, local := range []string{"0.14.0", "dev"} {
		h := newFakeHost("linux", globalLazycron, globalVersion("0.15.0"), linkOK)
		res := runInstallAndLink(h, Options{InstallLazycron: true, Version: local})
		if statusOf(t, res, "lazycron") != StatusOK || h.ranAny("curl") {
			t.Errorf("local %s: steps %+v ran %v", local, res.Steps, h.ran)
		}
	}
}

func TestServerWithoutLazycron_OptOutWarns(t *testing.T) {
	h := newFakeHost("linux", noLazycron, linkMissing)
	res := runInstallAndLink(h, Options{InstallLazycron: false, Version: "0.15.0"})
	if got := statusOf(t, res, "lazycron"); got != StatusWarning {
		t.Errorf("lazycron = %s", got)
	}
	if h.ranAny("curl") {
		t.Error("installer ran despite --no-lazycron")
	}
}

func TestServerWithoutLazycron_DryRunChangesNothing(t *testing.T) {
	h := newFakeHost("linux", noLazycron, linkMissing)
	res := runInstallAndLink(h, Options{DryRun: true, InstallLazycron: true, Version: "0.15.0"})
	if statusOf(t, res, "lazycron") != StatusWouldChange || statusOf(t, res, "lazycron link") != StatusWouldChange {
		t.Errorf("steps %+v", res.Steps)
	}
	if h.ranAny("curl") || h.ranAny("mkdir") {
		t.Errorf("dry run changed the host: %v", h.ran)
	}
}

func TestServerInstallerFailure_HasHint(t *testing.T) {
	h := newFakeHost("linux", noLazycron, installerNeedSudo, linkMissing)
	res := runInstallAndLink(h, Options{InstallLazycron: true, Version: "0.15.0"})
	if got := statusOf(t, res, "lazycron"); got != StatusFailed {
		t.Fatalf("lazycron = %s", got)
	}
	if res.Steps[0].Hint == "" {
		t.Error("a failed install needs a hint")
	}
}

func TestThisMachineSkipsInstall(t *testing.T) {
	h := newFakeHost("linux", linkOK)
	s := &stepRunner{h: h, opts: Options{LazycronPath: "/usr/local/bin/lazycron", InstallLazycron: true}}
	s.lazycronInstall()
	if len(s.res.Steps) != 0 || h.ranAny("curl") {
		t.Errorf("local setup must not run the installer: %+v", s.res.Steps)
	}
}

func TestOlderVersion(t *testing.T) {
	cases := []struct {
		have, want string
		older      bool
	}{
		{"0.14.0", "0.15.0", true},
		{"0.15.0", "0.15.0", false},
		{"0.16.0", "0.15.0", false},
		{"0.9.9", "0.10.0", true},
		{"v1.2.3", "1.2.4", true},
		{"0.15.0", "dev", false},
		{"", "0.15.0", false},
	}
	for _, c := range cases {
		if got := olderVersion(c.have, c.want); got != c.older {
			t.Errorf("olderVersion(%q, %q) = %v", c.have, c.want, got)
		}
	}
}

// sequenceHost answers successive `command -v lazycron` lookups from a list,
// so a test can model lazycron appearing after the installer runs.
type sequenceHost struct {
	*fakeHost
	lookups []answer
}

func (h *sequenceHost) Shell(cmd string) (string, error) {
	if len(h.lookups) > 0 && strings.HasPrefix(cmd, h.lookups[0].prefix) {
		a := h.lookups[0]
		h.lookups = h.lookups[1:]
		h.fakeHost.ran = append(h.fakeHost.ran, cmd)
		return a.out, a.err
	}
	return h.fakeHost.Shell(cmd)
}
