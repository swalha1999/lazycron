// Command fakehc imitates the parts of herdr-core's `hc` CLI that lazycron
// uses, for tests. Terminals are real background processes; state lives in
// $FAKEHC_STATE. Every invocation is appended to $FAKEHC_STATE/calls.log.
//
// Knobs: FAKEHC_FAIL_RUN=1 makes `run` fail; FAKEHC_TRUNCATE=1 makes `read`
// report complete=false.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type terminal struct {
	Address   string            `json:"address"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	CreatedMS int64             `json:"created_ms"`
	PID       int               `json:"pid"`
	Argv      []string          `json:"argv"`
}

var state = os.Getenv("FAKEHC_STATE")

func main() {
	logCall()
	args := os.Args[1:]
	if len(args) == 0 {
		fail("usage", "no command", 2)
	}
	switch args[0] {
	case "--version":
		fmt.Println("hc 0.1.18")
	case "server":
		emit(map[string]any{"version": "0.1.18"})
	case "run":
		cmdRun(args[1:])
	case "wait":
		cmdWait(args[1:])
	case "get":
		emit(view(load(positional(args[1:]))))
	case "ls":
		cmdList(args[1:])
	case "read":
		cmdRead(args[1:])
	case "kill":
		cmdKill(args[1:])
	case "rm":
		cmdRemove(args[1:])
	default:
		fail("usage", "unknown command "+args[0], 2)
	}
}

func logCall() {
	f, err := os.OpenFile(filepath.Join(state, "calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err == nil {
		fmt.Fprintln(f, strings.Join(os.Args[1:], " "))
		f.Close()
	}
}

func emit(v any) {
	_ = json.NewEncoder(os.Stdout).Encode(v)
}

func fail(kind, msg string, code int) {
	_ = json.NewEncoder(os.Stderr).Encode(map[string]any{"error": map[string]string{"kind": kind, "message": msg, "hint": ""}})
	os.Exit(code)
}

func path(addr, ext string) string { return filepath.Join(state, addr+ext) }

// flags splits args into options (--k v / --flag) and positionals; values
// after "--" are returned as rest.
func parse(args []string, boolFlags map[string]bool) (opts map[string][]string, pos, rest []string) {
	opts = map[string][]string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return opts, pos, args[i+1:]
		case strings.HasPrefix(a, "--"):
			key := strings.TrimPrefix(a, "--")
			if boolFlags[key] {
				opts[key] = append(opts[key], "true")
			} else if i+1 < len(args) {
				opts[key] = append(opts[key], args[i+1])
				i++
			}
		default:
			pos = append(pos, a)
		}
	}
	return opts, pos, nil
}

var bools = map[string]bool{"bg": true, "no-parent": true, "no-notify": true, "json": true, "all": true}

func positional(args []string) string {
	_, pos, _ := parse(args, bools)
	if len(pos) == 0 {
		fail("needs_target", "no terminal", 7)
	}
	return pos[0]
}

func cmdRun(args []string) {
	if os.Getenv("FAKEHC_FAIL_RUN") == "1" {
		fail("spawn_failed", "core unreachable", 1)
	}
	opts, _, argv := parse(args, bools)
	n := nextID()
	addr := "t" + strconv.Itoa(n)
	labels := map[string]string{}
	for _, l := range opts["label"] {
		k, v, _ := strings.Cut(l, "=")
		labels[k] = v
	}
	shArgs := append([]string{"-c", `"$@" > "$0.out" 2>&1; echo $? > "$0.rc"`, path(addr, "")}, argv...)
	c := exec.Command("sh", shArgs...)
	if cwd := first(opts["cwd"]); cwd != "" {
		c.Dir = cwd
	}
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		fail("spawn_failed", err.Error(), 1)
	}
	t := terminal{Address: addr, Name: first(opts["name"]), Labels: labels, CreatedMS: time.Now().UnixMilli() + int64(n), PID: c.Process.Pid, Argv: argv}
	save(t)
	emit(map[string]any{"address": addr, "name": t.Name, "state": "live"})
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func nextID() int {
	p := filepath.Join(state, "counter")
	data, _ := os.ReadFile(p)
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	n++
	_ = os.WriteFile(p, []byte(strconv.Itoa(n)), 0o600)
	return n
}

func save(t terminal) {
	data, _ := json.Marshal(t)
	_ = os.WriteFile(path(t.Address, ".json"), data, 0o600)
}

func load(ref string) terminal {
	files, _ := filepath.Glob(filepath.Join(state, "t*.json"))
	for _, f := range files {
		var t terminal
		data, _ := os.ReadFile(f)
		if json.Unmarshal(data, &t) == nil && (t.Address == ref || t.Name == ref) {
			return t
		}
	}
	fail("not_found", fmt.Sprintf("no terminal %q", ref), 3)
	return terminal{}
}

// status returns finished?, exit code, signal.
func status(t terminal) (bool, *int, *int) {
	if _, err := os.Stat(path(t.Address, ".killed")); err == nil {
		sig := 1
		return true, nil, &sig
	}
	data, err := os.ReadFile(path(t.Address, ".rc"))
	if err != nil {
		return false, nil, nil
	}
	code, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return true, &code, nil
}

func view(t terminal) map[string]any {
	done, code, sig := status(t)
	st := "live"
	if done {
		st = "finished"
	}
	return map[string]any{"address": t.Address, "name": t.Name, "labels": t.Labels, "state": st,
		"exit_code": code, "signal": sig, "created_ms": t.CreatedMS, "argv": t.Argv}
}

func cmdWait(args []string) {
	opts, _, _ := parse(args, bools)
	t := load(positional(args))
	timeout, _ := strconv.Atoi(first(opts["timeout"]))
	want := first(opts["output"])
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		done, code, sig := status(t)
		if done {
			emit(map[string]any{"ended": true, "exit_code": code, "signal": sig, "reason": "exit", "terminal": t.Address})
			return
		}
		if want != "" {
			if out, _ := os.ReadFile(path(t.Address, ".out")); strings.Contains(string(out), want) {
				emit(map[string]any{"ended": false, "reason": "output", "terminal": t.Address})
				return
			}
		}
		if time.Now().After(deadline) {
			emit(map[string]any{"ended": false, "reason": "timeout", "terminal": t.Address})
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func cmdList(args []string) {
	opts, _, _ := parse(args, bools)
	files, _ := filepath.Glob(filepath.Join(state, "t*.json"))
	sort.Strings(files)
	list := []map[string]any{}
	for _, f := range files {
		var t terminal
		data, _ := os.ReadFile(f)
		if json.Unmarshal(data, &t) != nil {
			continue
		}
		match := true
		for _, l := range opts["label"] {
			k, v, _ := strings.Cut(l, "=")
			if t.Labels[k] != v {
				match = false
			}
		}
		if match {
			v := view(t)
			delete(v, "labels") // the real `hc ls` does not print labels
			list = append(list, v)
		}
	}
	emit(map[string]any{"terminals": list})
}

func cmdRead(args []string) {
	t := load(positional(args))
	out, _ := os.ReadFile(path(t.Address, ".out"))
	emit(map[string]any{"complete": os.Getenv("FAKEHC_TRUNCATE") != "1", "how": "history", "text": strings.TrimRight(string(out), "\n")})
}

func cmdKill(args []string) {
	t := load(positional(args))
	_ = os.WriteFile(path(t.Address, ".killed"), nil, 0o600)
	_ = syscall.Kill(-t.PID, syscall.SIGHUP)
	emit(view(t))
}

func cmdRemove(args []string) {
	t := load(positional(args))
	if done, _, _ := status(t); !done {
		fail("status_mismatch", "terminal is live", 10)
	}
	for _, ext := range []string{".json", ".out", ".rc", ".killed"} {
		_ = os.Remove(path(t.Address, ext))
	}
	emit(map[string]any{"removed": t.Address})
}
