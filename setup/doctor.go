package setup

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/runner"
)

// DoctorOptions say what the checked host is expected to do.
type DoctorOptions struct {
	Runner string // the default runner jobs on this host use
	HCPath string // configured herdr-core path, if any
}

// Doctor checks that host can run lazycron jobs as configured. Checks that
// only matter for herdr mode are warnings when the runner is plain.
// Nothing is changed, but note that asking herdr-core for its status starts
// a core when none is running (that is how hc works).
func Doctor(h Host, opts DoctorOptions) []Step {
	d := &doctor{h: h, herdr: opts.Runner == "herdr"}
	d.crontab()
	d.cronDaemon()
	d.path()
	d.lazycronLink()
	hc := d.herdrCore(opts.HCPath)
	if hc != "" {
		d.coreRunning(hc)
		d.bootService()
	}
	d.linger()
	d.claude()
	return d.steps
}

type doctor struct {
	h     Host
	herdr bool
	steps []Step
}

func (d *doctor) add(name string, st Status, detail, hint string) {
	d.steps = append(d.steps, Step{Name: name, Status: st, Detail: detail, Hint: hint})
}

// herdrProblem is a failure in herdr mode and a warning otherwise.
func (d *doctor) herdrProblem() Status {
	if d.herdr {
		return StatusFailed
	}
	return StatusWarning
}

func (d *doctor) crontab() {
	if p, err := d.h.Shell("command -v crontab"); err == nil && p != "" {
		d.add("crontab", StatusOK, p, "")
		return
	}
	d.add("crontab", StatusFailed, "crontab not found", "install cron (cronie on Arch, cron on Debian/Ubuntu)")
}

func (d *doctor) cronDaemon() {
	if d.h.OS() == "darwin" {
		d.add("cron daemon", StatusOK, "managed by launchd", "")
		return
	}
	if out, err := d.h.Shell("pgrep -x crond || pgrep -x cron || pgrep -x cronie"); err == nil && out != "" {
		d.add("cron daemon", StatusOK, "running", "")
		return
	}
	d.add("cron daemon", StatusFailed, "no cron daemon is running",
		"start it, e.g. sudo systemctl enable --now cronie (Arch) or cron (Debian/Ubuntu)")
}

// path checks that people and agents can call lazycron by name. Cron itself
// does not need it: crontab lines use absolute paths.
func (d *doctor) path() {
	if p, err := d.h.Shell("command -v lazycron"); err == nil && p != "" {
		d.add("PATH", StatusOK, "lazycron is "+p, "")
		return
	}
	d.add("PATH", StatusWarning, "lazycron is not on PATH", "add its install directory to PATH")
}

func (d *doctor) lazycronLink() {
	link := path.Join(d.h.Home(), ".lazycron", "bin", "lazycron")
	if out, err := d.h.Shell(fmt.Sprintf("[ -x %s ] && readlink %s", cron.ShellQuote(link), cron.ShellQuote(link))); err == nil {
		d.add("lazycron link", StatusOK, link+" -> "+out, "")
		return
	}
	d.add("lazycron link", d.herdrProblem(), link+" is missing; herdr-mode jobs will not run", "run `lazycron setup`")
}

func (d *doctor) herdrCore(configured string) string {
	hc := configured
	if hc == "" {
		hc, _ = d.h.Shell(findHCScript)
	}
	if hc == "" {
		d.add("herdr-core", d.herdrProblem(), "not installed; jobs run plainly",
			"install it with `lazycron setup` (or curl -fsSL "+HerdrCoreInstaller+" | sh)")
		return ""
	}
	out, err := d.h.Shell(cron.ShellQuote(hc) + " --version")
	v := strings.TrimPrefix(strings.TrimSpace(out), "hc ")
	switch {
	case err != nil:
		d.add("herdr-core", StatusFailed, fmt.Sprintf("%s does not run: %v", hc, err), "reinstall herdr-core")
		return ""
	case !runner.VersionAtLeast(v, runner.MinHCVersion):
		d.add("herdr-core", StatusWarning, fmt.Sprintf("%s is %s, older than %s", hc, v, runner.MinHCVersion),
			"update when no agents are running: hc update --version "+runner.MinHCVersion)
	default:
		d.add("herdr-core", StatusOK, hc+" "+v, "")
	}
	return hc
}

func (d *doctor) coreRunning(hc string) {
	out, err := d.h.Shell(cron.ShellQuote(hc) + " server status --json")
	var st struct {
		Version string `json:"version"`
		Live    int    `json:"live"`
	}
	if err != nil || json.Unmarshal([]byte(out), &st) != nil {
		d.add("herdr-core server", d.herdrProblem(), "not reachable: "+strings.TrimSpace(out), "check `hc server status`")
		return
	}
	d.add("herdr-core server", StatusOK, fmt.Sprintf("running %s, %d live terminals", st.Version, st.Live), "")
}

func (d *doctor) bootService() {
	svc, ok := serviceFor(d.h, "")
	if !ok {
		return
	}
	if _, err := d.h.Shell(svc.Enabled); err == nil {
		d.add("boot service", StatusOK, svc.Path, "")
		return
	}
	d.add("boot service", d.herdrProblem(), "herdr-core is not started at boot", "run `lazycron setup`")
}

func (d *doctor) linger() {
	if d.h.OS() != "linux" {
		return
	}
	out, err := d.h.Shell("loginctl show-user " + cron.ShellQuote(d.h.User()) + " --property=Linger --value")
	if err == nil && strings.TrimSpace(out) == "yes" {
		d.add("linger", StatusOK, "enabled", "")
		return
	}
	d.add("linger", d.herdrProblem(), "user services stop when you log out, so herdr-core may be down at cron time",
		"run `lazycron setup` (or sudo loginctl enable-linger "+d.h.User()+")")
}

func (d *doctor) claude() {
	p, err := d.h.Shell("command -v claude")
	if err != nil || p == "" {
		d.add("claude", StatusSkipped, "Claude Code not installed (only needed for agent jobs)", "")
		return
	}
	out, err := d.h.Shell("claude auth status --json")
	var st struct {
		LoggedIn bool `json:"loggedIn"`
	}
	if err != nil || json.Unmarshal([]byte(out), &st) != nil || !st.LoggedIn {
		d.add("claude", StatusWarning, p+": not logged in; agent jobs will fail", "run `claude auth login`")
		return
	}
	d.add("claude", StatusOK, p+": logged in", "")
}
