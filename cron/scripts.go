package cron

import (
	"os"
	"path/filepath"
	"regexp"
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
func ScriptPath(jobName string) string {
	return filepath.Join(scriptsDir(), sanitizeJobName(jobName)+".sh")
}

var sanitizeRe = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeJobName converts a job name to a filesystem-safe slug.
func sanitizeJobName(name string) string {
	return strings.Trim(sanitizeRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

// WriteScript writes a job's command to its script file.
func WriteScript(jobName, command string) error {
	dir := scriptsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(ScriptPath(jobName), []byte(BuildScriptContent(command)), 0700)
}

// ScriptPreamble is the profile-sourcing block prepended to every script.
const ScriptPreamble = "# Source user profile for PATH and environment variables.\n" +
	"for __lc_rc in \"$HOME/.profile\" \"$HOME/.bashrc\" \"$HOME/.zshrc\"; do\n" +
	"  [ -f \"$__lc_rc\" ] && . \"$__lc_rc\" 2>/dev/null\n" +
	"done\n" +
	"unset __lc_rc\n"

// BuildScriptContent returns a complete script with shebang, preamble, and command.
func BuildScriptContent(command string) string {
	return "#!/bin/sh\n" + ScriptPreamble + command + "\n"
}

// StripShebang removes the shebang line and preamble from script content,
// returning just the command.
func StripShebang(content string) string {
	if strings.HasPrefix(content, "#!/bin/sh\n") {
		content = content[len("#!/bin/sh\n"):]
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
func DeleteScript(jobName string) error {
	err := os.Remove(ScriptPath(jobName))
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
		filename := sanitizeJobName(j.Name) + ".sh"
		active[filename] = true
		if err := WriteScript(j.Name, j.Command); err != nil {
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

// scriptRefMarker is the path component that identifies a lazycron script reference.
var scriptRefMarker = filepath.Join(".lazycron", "scripts")

// IsScriptRef reports whether a command is a reference to a lazycron script.
func IsScriptRef(command string) bool {
	return strings.HasPrefix(command, "sh ") &&
		strings.Contains(command, scriptRefMarker)
}

// resolveScript reads the actual command from a script file reference.
// If the command is not a script ref or reading fails, it returns the original command.
func resolveScript(command string) string {
	if !IsScriptRef(command) {
		return command
	}
	path := strings.TrimPrefix(command, "sh ")
	path = strings.Trim(path, "'\"")
	content, err := ReadScriptCommand(path)
	if err != nil {
		return command
	}
	return content
}
