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
	argv := []string{"env", "PATH=" + os.Getenv("PATH"), claude, "--permission-mode", job.PermissionModeOrDefault()}
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

	if err := AddWorktree(repo, wt, ref); err != nil {
		return err
	}
	sess := &Session{
		HC:      o.HC,
		Name:    terminalName(jobID, stamp),
		Dir:     wt,
		Argv:    argv,
		Labels:  map[string]string{"lazycron.project": o.ProjectName, "lazycron.job": jobID},
		Timeout: timeout,
		Poll:    5 * time.Minute,
		Log:     o.Log,
	}
	return supervise(sess, workspace{repo: repo, dir: wt}, job.Name)
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
