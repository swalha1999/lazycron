package agent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Options configures one agent run.
type Options struct {
	ProjectDir  string // where .lazycron/ lives (cron cd's here)
	ProjectName string
	JobPath     string // .lazycron/jobs/<id>.md
	HC          HC
	DryRun      bool // resolve everything and print the prompt; start nothing
	Log         func(format string, args ...any)

	SpaceID  string // herdr-app space when not run by `lazycron exec` (which labels its own)
	RunsDir  string // where the session record for the run's history goes
	KeepRuns int    // worktrees kept per job, the new one included (0: keep all)
}

// ErrBlocked means the agent is waiting on a person; its terminal is still open.
var ErrBlocked = errors.New("agent is waiting for you")

// Run executes one scheduled agent job end to end.
func Run(o Options) error {
	data, err := os.ReadFile(o.JobPath)
	if err != nil {
		return err
	}
	job, err := ParseJob(data)
	if err != nil {
		return fmt.Errorf("%s: %w", o.JobPath, err)
	}
	jobID := strings.TrimSuffix(filepath.Base(o.JobPath), filepath.Ext(o.JobPath))
	timeout, _ := job.TimeoutDuration()

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	os.Setenv("PATH", agentPATH(home, os.Getenv("PATH")))
	claude, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found on PATH: %w", err)
	}

	lcDir := filepath.Join(o.ProjectDir, ".lazycron")
	repo, err := ResolveRepo(o.ProjectDir, envValue(filepath.Join(lcDir, ".env"), "REPO_URL"),
		filepath.Join(home, ".lazycron", "repos", o.ProjectName))
	if err != nil {
		return err
	}
	ref, err := FetchBase(repo, job.Base)
	if err != nil {
		return err
	}

	stamp := time.Now().Format("20060102-150405")
	wt := filepath.Join(home, ".lazycron", "worktrees", o.ProjectName, jobID+"-"+stamp)
	notes, _ := os.ReadFile(filepath.Join(lcDir, "context.md"))
	prompt := BuildPrompt(job, PromptContext{
		JobName: job.Name, Repo: RepoLabel(repo), Base: ref, Worktree: wt, Notes: string(notes),
	})
	sessionID := newSessionID()
	argv := []string{"env", "PATH=" + os.Getenv("PATH"), claude, "--permission-mode", job.PermissionModeOrDefault(),
		"--session-id", sessionID, "--name", job.Name + " " + time.Now().Format("2006-01-02 15:04")}
	if job.Model != "" {
		argv = append(argv, "--model", job.Model)
	}
	argv = append(argv, prompt)

	if o.DryRun {
		o.Log("repo: %s  base: %s  worktree: %s", repo, ref, wt)
		o.Log("claude: %s  (timeout %s)", strings.Join(argv[2:len(argv)-1], " "), timeout)
		o.Log("prompt:\n%s", prompt)
		return nil
	}

	pruneWorktrees(repo, filepath.Dir(wt), jobID, o.KeepRuns, o.Log)
	if err := AddWorktree(repo, wt, ref); err != nil {
		return err
	}
	run := currentRun(o.HC)
	labels := map[string]string{labelProject: o.ProjectName, labelJob: jobID, labelAgent: "claude", labelSpace: o.SpaceID}
	if run.JobID != "" {
		labels[labelJob] = run.JobID
	}
	if run.RunID != "" {
		labels[labelRun] = run.RunID
	}
	if run.SpaceID != "" {
		labels[labelSpace] = run.SpaceID
	}
	for k, v := range labels {
		if v == "" {
			delete(labels, k)
		}
	}
	sess := &Session{
		HC:      o.HC,
		Name:    terminalName(jobID, stamp),
		Dir:     wt,
		Argv:    argv,
		Labels:  labels,
		Timeout: timeout,
		Poll:    5 * time.Minute,
		Log:     o.Log,
	}
	addr, err := sess.Start()
	if err != nil {
		_ = RemoveWorktree(repo, wt)
		return err
	}
	rec := SessionRecord{Terminal: addr, SessionID: sessionID, Dir: wt, Repo: repo, Status: "working"}
	_ = writeRecord(o.RunsDir, run.RunID, rec)
	o.Log("agent %q started in terminal %s (watch: hc attach %s)", job.Name, addr, addr)
	o.Log("claude session %s in %s", sessionID, wt)

	out, err := sess.Wait(addr)
	if err != nil {
		return err
	}
	rec.Status = out.Status
	_ = writeRecord(o.RunsDir, run.RunID, rec)
	continueHint := "continue it with `lazycron resume " + run.RunID + "`"
	if run.RunID == "" {
		continueHint = fmt.Sprintf("continue it with `hc attach %s`, or `claude --resume %s` in %s", addr, sessionID, wt)
	}
	switch out.Status {
	case "blocked":
		o.Log("agent is waiting for input")
		return fmt.Errorf("%w: answer it in herdr-app or with `hc attach %s`", ErrBlocked, addr)
	case "done":
		o.Log("agent finished; its session stays open: %s", continueHint)
	case "exited":
		o.Log("claude exited with code %d; %s", out.ExitCode, continueHint)
	default:
		o.Log("agent stopped: %s; %s", out.Status, continueHint)
	}
	switch {
	case out.Status == "exited" && out.ExitCode != 0:
		return fmt.Errorf("claude exited with code %d", out.ExitCode)
	case out.Status == "error", out.Status == "timeout":
		return fmt.Errorf("agent %s (read the session with `hc read %s`)", out.Status, addr)
	}
	return nil
}

var unsafeName = regexp.MustCompile(`[^a-z0-9_-]+`)

// terminalName is a unique hc name (1-32 of a-z 0-9 - _) for one run.
func terminalName(jobID, stamp string) string {
	id := unsafeName.ReplaceAllString(strings.ToLower(jobID), "-")
	if len(id) > 14 {
		id = id[:14]
	}
	id = strings.Trim(id, "-")
	return "agent-" + id + "-" + strings.ReplaceAll(stamp[4:], "-", "") // agent-<id>-MMDDhhmmss
}

// agentPATH makes sure the tools an agent needs are findable when cron or
// herdr-core started us with a minimal PATH.
func agentPATH(home, current string) string {
	parts := strings.Split(current, ":")
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		seen[p] = true
	}
	for _, p := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".local", "share", "mise", "shims"),
		filepath.Join(home, ".local", "share", "mise", "command-wrappers", "bin"),
		filepath.Join(home, ".claude", "local"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, "go", "bin"),
		"/usr/local/bin", "/usr/bin", "/bin",
	} {
		if !seen[p] {
			parts = append(parts, p)
			seen[p] = true
		}
	}
	return strings.Trim(strings.Join(parts, ":"), ":")
}

// envValue returns key's value from a KEY=VALUE file, or "".
func envValue(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}
