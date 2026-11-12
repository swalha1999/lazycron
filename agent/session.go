package agent

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// HC runs herdr-core (hc) commands. It is an interface so tests can fake it.
type HC interface {
	Run(args ...string) ([]byte, error)
}

// ExecHC runs the hc binary at Path.
type ExecHC struct{ Path string }

func (h ExecHC) Run(args ...string) ([]byte, error) {
	out, err := exec.Command(h.Path, args...).Output()
	if ee, ok := err.(*exec.ExitError); ok {
		return out, fmt.Errorf("hc %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
	}
	return out, err
}

// waitResult is the JSON `hc wait --json` prints.
type waitResult struct {
	Ended    bool   `json:"ended"`
	ExitCode *int   `json:"exit_code"`
	Reason   string `json:"reason"`
	Status   string `json:"status"`
	Turn     int    `json:"turn"`
}

// state is what a wait result means for the run.
type state int

const (
	stateWorking  state = iota // keep waiting
	stateStarting              // idle before the first turn: maybe a startup prompt
	stateDone                  // finished its turn
	stateBlocked               // waiting on a person
	stateError                 // the agent reported an error
	stateExited                // the claude process ended
)

// classify maps an hc wait result to a run state.
func classify(w waitResult) state {
	switch {
	case w.ExitCode != nil || w.Reason == "exit":
		return stateExited
	case w.Reason == "timeout":
		return stateWorking
	case w.Status == "blocked":
		return stateBlocked
	case w.Status == "error":
		return stateError
	case w.Status == "done", w.Status == "idle" && w.Turn > 0:
		return stateDone
	case w.Status == "idle":
		return stateStarting
	default:
		return stateWorking
	}
}

// startupPrompts are Claude Code's first-run questions in a new folder. The
// worktree is one lazycron just made, so accepting them is safe; anything
// else on screen is a real question and is left for a person.
var startupPrompts = []string{
	"Yes, I trust this folder",
	"Yes, I accept",
}

// Outcome is how a session ended.
type Outcome struct {
	Terminal string
	Status   string // done, blocked, error, exited, timeout
	ExitCode int    // claude's exit code when Status is exited
}

// Session is one agent run in a herdr terminal.
type Session struct {
	HC       HC
	Name     string            // hc terminal name
	Dir      string            // working directory (the worktree)
	Argv     []string          // program and arguments
	Labels   map[string]string // hc labels
	Timeout  time.Duration
	Poll     time.Duration // hc wait --timeout per call
	Log      func(format string, args ...any)
	sleep    func(time.Duration)
	deadline time.Time
}

// Start launches the agent in a herdr terminal and returns its address. The
// terminal outlives this process so a person can watch, answer or continue
// the session after the run is over; run retention closes it later.
func (s *Session) Start() (string, error) {
	args := []string{"run", "--bg", "--json", "--outlive-parent", "--name", s.Name, "--cwd", s.Dir}
	for _, k := range sortedKeys(s.Labels) {
		args = append(args, "--label", k+"="+s.Labels[k])
	}
	args = append(args, s.Argv...)
	out, err := s.HC.Run(args...)
	if err != nil {
		return "", err
	}
	var started struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(out, &started); err != nil || started.Address == "" {
		return "", fmt.Errorf("hc run: unexpected output %q", strings.TrimSpace(string(out)))
	}
	return started.Address, nil
}

// Wait follows the agent until it is done, blocked, errors, exits or runs out of time.
func (s *Session) Wait(addr string) (Outcome, error) {
	if s.sleep == nil {
		s.sleep = time.Sleep
	}
	s.deadline = time.Now().Add(s.Timeout)
	turn := 0
	for {
		if time.Now().After(s.deadline) {
			_, _ = s.HC.Run("kill", addr)
			return Outcome{Terminal: addr, Status: "timeout"}, nil
		}
		out, err := s.HC.Run("wait", addr, "--json", "--timeout", s.pollSeconds(turn))
		if err != nil {
			return Outcome{Terminal: addr}, err
		}
		var w waitResult
		if err := json.Unmarshal(out, &w); err != nil {
			return Outcome{Terminal: addr}, fmt.Errorf("hc wait: %w", err)
		}
		turn = w.Turn
		st := classify(w)
		// Before the first turn a startup prompt can look like work, idle or blocked.
		if turn == 0 && st != stateExited && s.answerStartupPrompt(addr) {
			continue
		}
		switch st {
		case stateWorking:
			continue
		case stateStarting:
			s.sleep(2 * time.Second) // still starting up; don't spin
		case stateDone:
			// Left open: the session can be read or continued later.
			return Outcome{Terminal: addr, Status: "done"}, nil
		case stateBlocked:
			return Outcome{Terminal: addr, Status: "blocked"}, nil
		case stateError:
			return Outcome{Terminal: addr, Status: "error"}, nil
		case stateExited:
			code := 0
			if w.ExitCode != nil {
				code = *w.ExitCode
			}
			return Outcome{Terminal: addr, Status: "exited", ExitCode: code}, nil
		}
	}
}

// startupPoll is how often a session that hasn't finished a turn is checked
// for startup prompts.
const startupPoll = 15 * time.Second

func (s *Session) pollSeconds(turn int) string {
	p := s.Poll
	if turn == 0 && p > startupPoll {
		p = startupPoll
	}
	return fmt.Sprint(int(p.Seconds()))
}

// answerStartupPrompt accepts a known first-run prompt if one is on screen.
func (s *Session) answerStartupPrompt(addr string) bool {
	screen, err := s.HC.Run("read", addr)
	if err != nil {
		return false
	}
	for _, p := range startupPrompts {
		if strings.Contains(string(screen), p) {
			s.Log("answering startup prompt: %s", p)
			_, _ = s.HC.Run("keys", addr, "down", "enter")
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
