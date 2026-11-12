package agent

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Labels on agent terminals. The herdr.space label puts the terminal in the
// herdr-app space lazycron runs show in; lazycron.job and lazycron.run tie it
// to its run so `lazycron attach/resume` find it and retention closes it.
const (
	labelSpace   = "herdr.space"
	labelJob     = "lazycron.job"
	labelRun     = "lazycron.run"
	labelProject = "lazycron.project"
	labelAgent   = "lazycron.agent"
)

// runContext is what the `lazycron exec` run around this agent tells us: the
// labels of the terminal we were started in. Empty when the agent was run by
// hand outside a lazycron run.
type runContext struct {
	RunID   string
	JobID   string
	SpaceID string
}

// currentRun reads the labels of the herdr terminal this process runs in.
func currentRun(hc HC) runContext {
	id := os.Getenv("HC_TERMINAL_ID")
	if id == "" || hc == nil {
		return runContext{}
	}
	out, err := hc.Run("get", id, "--json")
	if err != nil {
		return runContext{}
	}
	var t struct {
		Labels map[string]string `json:"labels"`
	}
	if json.Unmarshal(out, &t) != nil {
		return runContext{}
	}
	return runContext{RunID: t.Labels[labelRun], JobID: t.Labels[labelJob], SpaceID: t.Labels[labelSpace]}
}

// newSessionID returns a random (version 4) UUID for claude --session-id.
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// SessionRecord is what an agent run leaves for its run's history entry. It
// matches history.AgentSession.
type SessionRecord struct {
	Terminal  string `json:"terminal,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Dir       string `json:"dir,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Status    string `json:"status,omitempty"`
}

// writeRecord saves rec where `lazycron exec` picks it up for the history
// entry. It does nothing for an agent run outside a lazycron run.
func writeRecord(runsDir, runID string, rec SessionRecord) error {
	if runsDir == "" || runID == "" {
		return nil
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(runsDir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(runsDir, runID+".agent.json")
	if err := os.WriteFile(path+".tmp", data, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// pruneWorktrees removes this job's oldest worktrees so that, with the one
// about to be made, keep remain. Worktrees are named <job>-<stamp>, so they
// sort by age.
func pruneWorktrees(repo, dir, jobID string, keep int, log func(string, ...any)) {
	if keep <= 0 {
		return
	}
	matches, _ := filepath.Glob(filepath.Join(dir, jobID+"-*"))
	var mine []string
	for _, m := range matches {
		stamp := strings.TrimPrefix(filepath.Base(m), jobID+"-")
		if len(stamp) == len("20060102-150405") && !strings.Contains(stamp[9:], "-") {
			mine = append(mine, m)
		}
	}
	sort.Strings(mine)
	for len(mine) >= keep {
		if err := RemoveWorktree(repo, mine[0]); err != nil {
			log("warning: could not remove old worktree %s: %v", mine[0], err)
			_ = os.RemoveAll(mine[0])
		}
		mine = mine[1:]
	}
}
