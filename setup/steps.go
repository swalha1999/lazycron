package setup

import (
	"fmt"
	"path"
	"strings"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/runner"
)

// HerdrCoreInstaller is herdr's official core-only installer. It installs
// herdr-core into ~/.local/bin (no desktop app); setup runs it only when no
// herdr-core is present, so it never updates or restarts an existing core.
const HerdrCoreInstaller = "https://releases.herdr.dev/core/install.sh"

// Status of one setup step.
type Status string

const (
	StatusOK          Status = "ok"           // already as wanted
	StatusChanged     Status = "changed"      // setup changed it
	StatusWouldChange Status = "would_change" // dry-run: would change it
	StatusSkipped     Status = "skipped"      // not applicable or opted out
	StatusWarning     Status = "warning"      // works, but needs attention
	StatusFailed      Status = "failed"       // could not be done
)

// Step is the outcome of one setup step.
type Step struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// Options control setup.
type Options struct {
	DryRun       bool
	InstallHerdr bool   // install herdr-core when missing
	Service      bool   // install the boot service
	Linger       bool   // enable systemd linger (Linux)
	HCPath       string // known herdr-core path (skips the lookup)
	LazycronPath string // binary ~/.lazycron/bin/lazycron should point at ("" = look up `lazycron` on the host)

	// For a server (LazycronPath empty): install lazycron globally with the
	// official installer when it is missing or older than Version.
	InstallLazycron bool
	Version         string // this machine's lazycron version
}

// Result is what setup did, plus what later steps need.
type Result struct {
	Steps      []Step `json:"steps"`
	HCPath     string `json:"hc_path,omitempty"`
	HCVersion  string `json:"hc_version,omitempty"`
	HerdrReady bool   `json:"herdr_ready"`
}

// Failed reports whether any step failed.
func (r Result) Failed() bool {
	for _, s := range r.Steps {
		if s.Status == StatusFailed {
			return true
		}
	}
	return false
}

// runner applies steps to one host.
type stepRunner struct {
	h    Host
	opts Options
	res  Result

	lazycronTarget string // lazycron the lazycronInstall step found or installed
}

func (s *stepRunner) add(name string, status Status, detail, hint string) {
	s.res.Steps = append(s.res.Steps, Step{Name: name, Status: status, Detail: detail, Hint: hint})
}

// change performs apply unless dry-run, recording the outcome of step name.
func (s *stepRunner) change(name, detail string, apply func() error, hint string) bool {
	if s.opts.DryRun {
		s.add(name, StatusWouldChange, detail, "")
		return true
	}
	if err := apply(); err != nil {
		s.add(name, StatusFailed, fmt.Sprintf("%s: %v", detail, err), hint)
		return false
	}
	s.add(name, StatusChanged, detail, "")
	return true
}

// Run sets up host for herdr mode. It never stops, restarts or updates a
// running herdr-core.
func Run(h Host, opts Options) Result {
	s := &stepRunner{h: h, opts: opts}
	s.herdrCore()
	s.hcVersion()
	s.lazycronInstall()
	s.lazycronLink()
	if s.res.HCPath != "" {
		s.service()
	}
	s.linger()
	return s.res
}

const findHCScript = `command -v herdr-core 2>/dev/null || { [ -x "$HOME/.local/bin/herdr-core" ] && echo "$HOME/.local/bin/herdr-core"; }`

func (s *stepRunner) herdrCore() {
	const name = "herdr-core"
	if s.opts.HCPath != "" {
		s.res.HCPath = s.opts.HCPath
		s.add(name, StatusOK, s.opts.HCPath, "")
		return
	}
	if p, err := s.h.Shell(findHCScript); err == nil && p != "" {
		s.res.HCPath = p
		s.add(name, StatusOK, p, "")
		return
	}
	if !s.opts.InstallHerdr {
		s.add(name, StatusSkipped, "not installed; jobs run as plain cron",
			"install it with `curl -fsSL "+HerdrCoreInstaller+" | sh`, then run `lazycron setup` again")
		return
	}
	detail := "install herdr-core (a separate project, herdr.dev) with its official installer"
	ok := s.change(name, detail, func() error {
		out, err := s.h.Shell("curl -fsSL " + HerdrCoreInstaller + " | sh")
		if err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	}, "install it manually: curl -fsSL "+HerdrCoreInstaller+" | sh")
	if ok && s.opts.DryRun {
		s.res.HCPath = path.Join(s.h.Home(), ".local", "bin", "herdr-core")
		return
	}
	if p, err := s.h.Shell(findHCScript); ok && err == nil && p != "" {
		s.res.HCPath = p
	}
}

func (s *stepRunner) hcVersion() {
	const name = "herdr-core version"
	if s.res.HCPath == "" {
		return
	}
	if s.opts.DryRun && s.lastStatus() == StatusWouldChange {
		s.add(name, StatusSkipped, "checked after install", "")
		s.res.HerdrReady = true
		return
	}
	out, err := s.h.Shell(cron.ShellQuote(s.res.HCPath) + " --version")
	if err != nil {
		s.add(name, StatusFailed, fmt.Sprintf("%v: %s", err, out), "reinstall herdr-core")
		return
	}
	v := strings.TrimPrefix(strings.TrimSpace(out), "hc ")
	s.res.HCVersion = v
	if !runner.VersionAtLeast(v, runner.MinHCVersion) {
		s.add(name, StatusWarning, fmt.Sprintf("%s is older than %s", v, runner.MinHCVersion),
			fmt.Sprintf("update when no agents are running: hc update --version %s", runner.MinHCVersion))
	} else {
		s.add(name, StatusOK, v, "")
	}
	s.res.HerdrReady = true
}

func (s *stepRunner) lastStatus() Status {
	if len(s.res.Steps) == 0 {
		return ""
	}
	return s.res.Steps[len(s.res.Steps)-1].Status
}

func (s *stepRunner) lazycronLink() {
	const name = "lazycron link"
	link := lazycronLinkPath(s.h)
	target := s.opts.LazycronPath
	if target == "" {
		target = s.lazycronTarget
	}
	if target == "" {
		out, err := s.h.Shell("command -v lazycron")
		if err != nil || out == "" || out == link {
			if current, err := s.h.Shell("readlink " + cron.ShellQuote(link)); err == nil && current != "" {
				s.add(name, StatusOK, link+" -> "+current, "")
				return
			}
			s.add(name, StatusWarning, "lazycron is not installed on this host; herdr-mode lines fall back to plain runs",
				"run `lazycron setup` again without --no-lazycron, or install it there")
			return
		}
		target = out
	}
	if current, err := s.h.Shell("readlink " + cron.ShellQuote(link)); err == nil && current == target {
		s.add(name, StatusOK, link+" -> "+target, "")
		return
	}
	s.change(name, link+" -> "+target, func() error {
		out, err := s.h.Shell(fmt.Sprintf("mkdir -p %s && ln -sfn %s %s", cron.ShellQuote(path.Dir(link)), cron.ShellQuote(target), cron.ShellQuote(link)))
		if err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	}, "")
}

// lazycronLinkPath is ~/.lazycron/bin/lazycron on host, the path cron lines call.
func lazycronLinkPath(h Host) string {
	return path.Join(h.Home(), ".lazycron", "bin", "lazycron")
}

func (s *stepRunner) service() {
	const name = "boot service"
	if !s.opts.Service {
		s.add(name, StatusSkipped, "disabled with --no-service", "")
		return
	}
	svc, ok := serviceFor(s.h, s.res.HCPath)
	if !ok {
		s.add(name, StatusSkipped, "no supported service manager on "+s.h.OS(), "")
		return
	}
	current, _ := s.h.ReadFile(svc.Path)
	_, enabledErr := s.h.Shell(svc.Enabled)
	if string(current) == svc.Content && enabledErr == nil {
		s.add(name, StatusOK, svc.Path, "")
		return
	}
	s.change(name, "write and enable "+svc.Path, func() error {
		if err := s.h.WriteFile(svc.Path, []byte(svc.Content), 0o644); err != nil {
			return err
		}
		if out, err := s.h.Shell(svc.Enable); err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	}, "check `systemctl --user status "+systemdUnitName+"`")
}

func (s *stepRunner) linger() {
	const name = "linger"
	if s.h.OS() != "linux" {
		s.add(name, StatusSkipped, "only needed on Linux (launchd agents need a login on macOS)", "")
		return
	}
	if !s.opts.Linger {
		s.add(name, StatusSkipped, "disabled with --no-linger", "")
		return
	}
	user := s.h.User()
	if out, err := s.h.Shell("loginctl show-user " + cron.ShellQuote(user) + " --property=Linger --value"); err == nil && strings.TrimSpace(out) == "yes" {
		s.add(name, StatusOK, "enabled for "+user, "")
		return
	}
	s.change(name, "enable linger for "+user+" so the user's services run with nobody logged in", func() error {
		if out, err := s.h.Shell("loginctl enable-linger " + cron.ShellQuote(user)); err != nil {
			return fmt.Errorf("%v: %s", err, out)
		}
		return nil
	}, "run: sudo loginctl enable-linger "+user)
}
