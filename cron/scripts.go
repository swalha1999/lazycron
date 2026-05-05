package cron

import (
	"os"
	"path/filepath"
	"strings"
)

// scriptsDir returns the path to ~/.lazycron/scripts/.
// It is a variable so tests can override it.
var scriptsDir = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "scripts")
}

// ScriptsDir returns the current scripts directory path.
func ScriptsDir() string {
	return scriptsDir()
}

// ScriptPath returns the full path for a job's script file.
func ScriptPath(jobID string) string {
	return filepath.Join(scriptsDir(), jobID+".sh")
}

// WriteScript writes a job's command to its script file.
func WriteScript(jobID, command string) error {
	dir := scriptsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(ScriptPath(jobID), []byte(BuildScriptContent(command)), 0700)
}

// ScriptPreamble is the profile-sourcing block prepended to every script.
//
// We run scripts with bash (not /bin/sh) because on macOS /bin/sh is bash
// in POSIX-strict mode, and sourcing .zshrc — which routinely has zsh-only
// syntax (e.g. compdef directives in completion files) — triggers a parse
// error that kills the script silently, before the actual command runs.
// Bash without --posix tolerates the same parse errors and continues.
const ScriptPreamble = "# Source user profile for PATH and environment variables.\n" +
	"for __lc_rc in \"$HOME/.profile\" \"$HOME/.bashrc\" \"$HOME/.zshrc\"; do\n" +
	"  [ -f \"$__lc_rc\" ] && . \"$__lc_rc\" 2>/dev/null || true\n" +
	"done\n" +
	"unset __lc_rc\n"

// BuildScriptContent returns a complete script with shebang, preamble, and command.
func BuildScriptContent(command string) string {
	return "#!/usr/bin/env bash\n" + ScriptPreamble + command + "\n"
}

// StripShebang removes the shebang line and preamble from script content,
// returning just the command. Tolerates both legacy `#!/bin/sh` scripts
// and the current `#!/usr/bin/env bash`.
func StripShebang(content string) string {
	for _, sh := range []string{"#!/usr/bin/env bash\n", "#!/bin/bash\n", "#!/bin/sh\n"} {
		if strings.HasPrefix(content, sh) {
			content = content[len(sh):]
			break
		}
	}
	content = strings.TrimPrefix(content, ScriptPreamble)
	return strings.TrimRight(content, "\n")
}

// ReadScriptCommand reads a script file and returns the command
// (stripping the shebang and profile-sourcing preamble).
func ReadScriptCommand(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return StripShebang(string(data)), nil
}

// DeleteScript removes a job's script file.
func DeleteScript(jobID string) error {
	err := os.Remove(ScriptPath(jobID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SyncScripts writes script files for all jobs and removes orphans.
func SyncScripts(jobs []Job) error {
	dir := scriptsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	active := make(map[string]bool)
	for _, j := range jobs {
		filename := j.ID + ".sh"
		active[filename] = true
		if err := WriteScript(j.ID, j.Command); err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !active[e.Name()] && strings.HasSuffix(e.Name(), ".sh") {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}

	return nil
}

// ShellQuote wraps a string in single quotes with proper escaping,
// making it safe to embed in a shell command.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// StripProjectCd removes a leading `cd <path> && ` prefix from a command,
// returning the stripped command and the extracted project path. If the
// command does not begin with such a prefix, the original command is
// returned with an empty projectPath.
//
// The recognised forms are produced by `lazycron sync` when injecting the
// project working directory:
//   - cd /home/user/.lazycron/projects/foo && rest...
//   - cd ~/.lazycron/projects/foo && rest...
//   - cd '/path with spaces' && rest...
func StripProjectCd(command string) (stripped, projectPath string) {
	if !strings.HasPrefix(command, "cd ") {
		return command, ""
	}
	rest := command[len("cd "):]

	var path string
	switch {
	case strings.HasPrefix(rest, "'"):
		end := strings.Index(rest[1:], "'")
		if end == -1 {
			return command, ""
		}
		path = rest[1 : 1+end]
		rest = rest[1+end+1:]
	case strings.HasPrefix(rest, `"`):
		end := strings.Index(rest[1:], `"`)
		if end == -1 {
			return command, ""
		}
		path = rest[1 : 1+end]
		rest = rest[1+end+1:]
	default:
		sp := strings.Index(rest, " ")
		if sp == -1 {
			return command, ""
		}
		path = rest[:sp]
		rest = rest[sp:]
	}

	rest = strings.TrimLeft(rest, " ")
	if !strings.HasPrefix(rest, "&&") {
		return command, ""
	}
	rest = strings.TrimLeft(rest[2:], " ")
	return rest, path
}

// scriptRefMarker is the path component that identifies a lazycron script reference.
var scriptRefMarker = filepath.Join(".lazycron", "scripts")

// scriptRefPrefixes are the invocation prefixes lazycron has used over time.
// Current writes use "bash "; older crontabs may still contain "sh ".
var scriptRefPrefixes = []string{"bash ", "sh "}

// IsScriptRef reports whether a command is a reference to a lazycron script.
func IsScriptRef(command string) bool {
	if !strings.Contains(command, scriptRefMarker) {
		return false
	}
	for _, p := range scriptRefPrefixes {
		if strings.HasPrefix(command, p) {
			return true
		}
	}
	return false
}

// resolveScript reads the actual command from a script file reference.
// If the command is not a script ref or reading fails, it returns the original command.
func resolveScript(command string) string {
	if !IsScriptRef(command) {
		return command
	}
	var path string
	for _, p := range scriptRefPrefixes {
		if strings.HasPrefix(command, p) {
			path = strings.TrimPrefix(command, p)
			break
		}
	}
	path = strings.Trim(path, "'\"")
	content, err := ReadScriptCommand(path)
	if err != nil {
		return command
	}
	return content
}
