package setup

import (
	"fmt"
	"path"
)

// Service names. The unit only runs `hc server start` at boot: herdr-core
// then runs itself (on Linux as a transient systemd-run unit) and cron's
// `lazycron exec` finds it, or starts one on demand.
const (
	systemdUnitName = "lazycron-herdr.service"
	launchdLabel    = "dev.lazycron.herdr-core"
)

// serviceFile describes the boot service for a host.
type serviceFile struct {
	Path    string
	Content string
	// Enable is the shell command that registers and starts it.
	Enable string
	// Enabled is a shell command that succeeds when it is already registered.
	Enabled string
}

// serviceFor returns the boot service for host's OS, or false when the OS
// has no supported service manager.
func serviceFor(h Host, hcPath string) (serviceFile, bool) {
	switch h.OS() {
	case "linux":
		p := path.Join(h.Home(), ".config", "systemd", "user", systemdUnitName)
		return serviceFile{
			Path:    p,
			Content: systemdUnit(hcPath),
			Enable:  "systemctl --user daemon-reload && systemctl --user enable --now " + systemdUnitName,
			Enabled: "systemctl --user is-enabled --quiet " + systemdUnitName,
		}, true
	case "darwin":
		p := path.Join(h.Home(), "Library", "LaunchAgents", launchdLabel+".plist")
		return serviceFile{
			Path:    p,
			Content: launchdPlist(hcPath),
			Enable:  fmt.Sprintf("launchctl bootout gui/$(id -u)/%s 2>/dev/null; launchctl bootstrap gui/$(id -u) %q", launchdLabel, p),
			Enabled: fmt.Sprintf("launchctl print gui/$(id -u)/%s >/dev/null 2>&1", launchdLabel),
		}, true
	}
	return serviceFile{}, false
}

func systemdUnit(hcPath string) string {
	return fmt.Sprintf(`# Managed by lazycron setup. Starts herdr-core at boot so scheduled
# lazycron runs have a core even when nobody is logged in (needs linger).
[Unit]
Description=herdr-core for lazycron scheduled runs

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=%s server start

[Install]
WantedBy=default.target
`, hcPath)
}

func launchdPlist(hcPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<!-- Managed by lazycron setup: starts herdr-core at login for lazycron runs. -->
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>server</string>
    <string>start</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
</dict>
</plist>
`, launchdLabel, hcPath)
}
