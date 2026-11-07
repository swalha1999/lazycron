package setup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/swalha1999/lazycron/cron"
)

// LazycronInstaller is lazycron's official installer. It installs the latest
// release to /usr/local/bin (with sudo when that isn't writable).
const LazycronInstaller = "https://get.lazycron.com"

// lazycronInstallCmd runs the installer without its herdr-core step, which
// setup's own herdr-core step already handled.
const lazycronInstallCmd = "curl -fsSL " + LazycronInstaller + " | sh -s -- --no-herdr"

// defaultInstallPath is where the installer puts lazycron.
const defaultInstallPath = "/usr/local/bin/lazycron"

// lazycronInstall makes sure a server has lazycron installed globally, since
// cron there runs and records every job through it. It runs only when setup
// targets another host (Options.LazycronPath empty) and remembers the binary
// for the link step.
func (s *stepRunner) lazycronInstall() {
	const name = "lazycron"
	if s.opts.LazycronPath != "" {
		return // this machine: the running binary is lazycron
	}
	link := lazycronLinkPath(s.h)
	installed := s.hostLazycron(link)
	if installed == "" {
		if !s.opts.InstallLazycron {
			s.add(name, StatusWarning, "not installed; herdr-mode lines fall back to plain runs",
				"install it with `"+lazycronInstallCmd+"`, then run `lazycron setup` again")
			return
		}
		if s.runInstaller(name, "install lazycron with "+lazycronInstallCmd) {
			s.lazycronTarget = s.hostLazycron(link)
			if s.lazycronTarget == "" && s.opts.DryRun {
				s.lazycronTarget = defaultInstallPath
			}
		}
		return
	}

	s.lazycronTarget = installed
	have := s.versionOf(installed)
	if s.opts.InstallLazycron && olderVersion(have, s.opts.Version) {
		s.runInstaller(name, fmt.Sprintf("update lazycron %s (this machine has %s) with %s", have, s.opts.Version, lazycronInstallCmd))
		return
	}
	s.add(name, StatusOK, strings.TrimSpace(installed+" "+have), "")
}

// runInstaller runs the official installer on the host as step name.
func (s *stepRunner) runInstaller(name, detail string) bool {
	return s.change(name, detail, func() error {
		out, err := s.h.Shell(lazycronInstallCmd)
		if err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	}, "it needs write access to /usr/local/bin (or passwordless sudo); run `curl -fsSL "+LazycronInstaller+" | sh` on the host yourself, then `lazycron setup` again")
}

// hostLazycron returns the lazycron on the host's PATH (the installer's
// /usr/local/bin is checked even when a non-login shell lacks it), ignoring
// the link setup manages. "" when there is none.
func (s *stepRunner) hostLazycron(link string) string {
	out, err := s.h.Shell(`command -v lazycron || { [ -x ` + defaultInstallPath + ` ] && echo ` + defaultInstallPath + `; }`)
	if err != nil || out == "" || out == link {
		return ""
	}
	return out
}

// versionOf returns the version a lazycron binary reports, or "".
func (s *stepRunner) versionOf(bin string) string {
	out, err := s.h.Shell(cron.ShellQuote(bin) + " --version")
	if err != nil {
		return ""
	}
	fields := strings.Fields(out) // "lazycron version 0.14.0"
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// olderVersion reports whether have is an older release than want. Unknown
// or non-release versions (dev builds) never count as older, so setup never
// loops reinstalling the latest release over a newer local build.
func olderVersion(have, want string) bool {
	h, okH := parseVersion(have)
	w, okW := parseVersion(want)
	if !okH || !okW {
		return false
	}
	for i := range w {
		if h[i] != w[i] {
			return h[i] < w[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		if j := strings.IndexAny(p, "-+"); j >= 0 {
			p = p[:j]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
