package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// MinHCVersion is the oldest herdr-core lazycron is tested with.
const MinHCVersion = "0.1.18"

// Labels lazycron puts on run terminals.
const (
	LabelSpace = "herdr.space"
	LabelJob   = "lazycron.job"
	LabelRun   = "lazycron.run"
)

// Commander runs a program and returns its stdout and stderr. LocalCommander
// runs on this machine; backends provide one for remote hosts.
type Commander interface {
	Output(args []string) (stdout, stderr []byte, err error)
}

// LocalCommander runs programs on this machine.
type LocalCommander struct{}

// Output runs args[0] with the rest as arguments.
func (LocalCommander) Output(args []string) ([]byte, []byte, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command(args[0], args[1:]...)
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// HC drives the herdr-core CLI.
type HC struct {
	Bin string
	Cmd Commander
}

// HCError is an error reported by hc as {"error":{kind,message,hint}}.
type HCError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *HCError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("hc: %s (%s; %s)", e.Message, e.Kind, e.Hint)
	}
	return fmt.Sprintf("hc: %s (%s)", e.Message, e.Kind)
}

// IsHCNotFound reports whether err is hc's not_found error.
func IsHCNotFound(err error) bool {
	var he *HCError
	return errors.As(err, &he) && he.Kind == "not_found"
}

// FindHC locates the herdr-core binary: the configured path, then hc or
// herdr-core on PATH, then ~/.local/bin (cron's PATH does not include it).
func FindHC(configured string) (string, error) {
	if configured != "" {
		if _, err := os.Stat(configured); err != nil {
			return "", fmt.Errorf("configured herdr-core %s: %w", configured, err)
		}
		return configured, nil
	}
	for _, name := range []string{"hc", "herdr-core"} {
		if p, err := exec.LookPath(name); err == nil && isHerdrCore(p) {
			return p, nil
		}
	}
	home, _ := os.UserHomeDir()
	for _, name := range []string{"herdr-core", "hc"} {
		p := filepath.Join(home, ".local", "bin", name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("herdr-core (hc) not found on PATH or in ~/.local/bin")
}

// isHerdrCore guards against other programs called hc (some window
// managers ship one): herdr-core's binary is named herdr-core.
func isHerdrCore(path string) bool {
	if filepath.Base(path) == "herdr-core" {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return filepath.Base(resolved) == "herdr-core"
}

// call runs hc with args and returns stdout. Failures that hc reports as
// JSON on stderr come back as *HCError.
func (h *HC) call(args ...string) ([]byte, error) {
	stdout, stderr, err := h.Cmd.Output(append([]string{h.Bin}, args...))
	if err == nil {
		return stdout, nil
	}
	if he := parseHCError(stderr); he != nil {
		return nil, he
	}
	if he := parseHCError(stdout); he != nil { // ssh merges streams on failure
		return nil, he
	}
	msg := strings.TrimSpace(string(stderr))
	if msg == "" {
		msg = strings.TrimSpace(string(stdout))
	}
	return nil, fmt.Errorf("hc %s: %v: %s", args[0], err, msg)
}

func parseHCError(data []byte) *HCError {
	var wrapper struct {
		Error *HCError `json:"error"`
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if json.Unmarshal(bytes.TrimSpace(line), &wrapper) == nil && wrapper.Error != nil {
			return wrapper.Error
		}
	}
	if json.Unmarshal(bytes.TrimSpace(data), &wrapper) == nil && wrapper.Error != nil {
		return wrapper.Error
	}
	return nil
}

// callJSON runs hc with --json and decodes stdout into v.
func (h *HC) callJSON(v any, args ...string) error {
	out, err := h.call(append(args, "--json")...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("hc %s: unexpected output: %w", args[0], err)
	}
	return nil
}

// Version returns hc's version, e.g. "0.1.18".
func (h *HC) Version() (string, error) {
	out, err := h.call("--version")
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "hc "), nil
}

// ServerStatus reports whether a core answers (hc starts one on demand).
func (h *HC) ServerStatus() (version string, err error) {
	var st struct {
		Version string `json:"version"`
	}
	if err := h.callJSON(&st, "server", "status"); err != nil {
		return "", err
	}
	return st.Version, nil
}

// StartOptions describe a background terminal to start.
type StartOptions struct {
	Name   string
	Cwd    string
	Labels map[string]string
	Argv   []string
}

// Start runs argv in a new background terminal with no parent, so it
// outlives whoever started it, and returns its address.
func (h *HC) Start(o StartOptions) (string, error) {
	args := []string{"run", "--bg", "--no-parent", "--no-notify"}
	if o.Name != "" {
		args = append(args, "--name", o.Name)
	}
	if o.Cwd != "" {
		args = append(args, "--cwd", o.Cwd)
	}
	for _, k := range sortedKeys(o.Labels) {
		args = append(args, "--label", k+"="+o.Labels[k])
	}
	args = append(args, "--json", "--")
	args = append(args, o.Argv...)
	out, err := h.call(args...)
	if err != nil {
		return "", err
	}
	var t struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(out, &t); err != nil || t.Address == "" {
		return "", fmt.Errorf("hc run: unexpected output %q", strings.TrimSpace(string(out)))
	}
	return t.Address, nil
}

// WaitResult is hc wait's answer.
type WaitResult struct {
	Ended    bool   `json:"ended"`
	ExitCode *int   `json:"exit_code"`
	Signal   *int   `json:"signal"`
	Reason   string `json:"reason"`
}

// Wait blocks until the terminal ends, prints output (when non-empty), or
// timeoutSec passes (Ended false, Reason "timeout").
func (h *HC) Wait(addr string, timeoutSec int, output string) (WaitResult, error) {
	args := []string{"wait", addr, "--timeout", strconv.Itoa(timeoutSec)}
	if output != "" {
		args = append(args, "--output", output)
	}
	var w WaitResult
	err := h.callJSON(&w, args...)
	return w, err
}

// Terminal is the subset of `hc get` lazycron uses.
type Terminal struct {
	Address    string            `json:"address"`
	Name       string            `json:"name"`
	State      string            `json:"state"` // live or finished
	Status     *string           `json:"status"`
	ExitCode   *int              `json:"exit_code"`
	Signal     *int              `json:"signal"`
	Labels     map[string]string `json:"labels"`
	CreatedMS  int64             `json:"created_ms"`
	FinishedMS *int64            `json:"finished_ms"`
}

// Live reports whether the terminal's program is still running.
func (t Terminal) Live() bool { return t.State == "live" }

// Get returns one terminal.
func (h *HC) Get(addr string) (Terminal, error) {
	var t Terminal
	err := h.callJSON(&t, "get", addr)
	return t, err
}

// List returns live and finished terminals carrying all labels.
func (h *HC) List(labels map[string]string) ([]Terminal, error) {
	args := []string{"ls", "--all"}
	for _, k := range sortedKeys(labels) {
		args = append(args, "--label", k+"="+labels[k])
	}
	var out struct {
		Terminals []Terminal `json:"terminals"`
	}
	err := h.callJSON(&out, args...)
	return out.Terminals, err
}

// ReadResult is a terminal's screen and history.
type ReadResult struct {
	Complete bool   `json:"complete"`
	Text     string `json:"text"`
}

// Read returns up to lines of history plus the screen.
func (h *HC) Read(addr string, lines int) (ReadResult, error) {
	var r ReadResult
	err := h.callJSON(&r, "read", addr, "--lines", strconv.Itoa(lines))
	return r, err
}

// Kill ends a terminal's program.
func (h *HC) Kill(addr string) error {
	_, err := h.call("kill", addr, "--json")
	return err
}

// Remove deletes a finished terminal.
func (h *HC) Remove(addr string) error {
	_, err := h.call("rm", addr, "--json")
	return err
}

// AttachArgs returns the argv that attaches the caller's terminal to addr.
func (h *HC) AttachArgs(addr string) []string {
	return []string{h.Bin, "attach", addr}
}

// VersionAtLeast compares dotted versions numerically ("0.1.18" >= "0.1.9").
func VersionAtLeast(have, want string) bool {
	hp, wp := strings.Split(have, "."), strings.Split(want, ".")
	for i := 0; i < len(wp); i++ {
		var h, w int
		if i < len(hp) {
			h = leadingInt(hp[i])
		}
		w = leadingInt(wp[i])
		if h != w {
			return h > w
		}
	}
	return true
}

// leadingInt parses the digits at the start of s ("18-rc1" -> 18).
func leadingInt(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}
