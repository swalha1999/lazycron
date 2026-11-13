package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/agent"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/runner"
)

var resumeCmd = &cobra.Command{
	Use:   "resume <run-or-job>",
	Short: "Continue an agent run's Claude session (detach with ctrl+\\)",
	Long: "Puts this terminal on the Claude session an agent run started. When that session's " +
		"terminal is still open you land in it; when it was closed, a new herdr terminal " +
		"runs `claude --resume <session>` in the run's worktree (recreated if retention removed " +
		"it) and shows up in herdr-app's Cron space. With a job id or name, its newest run. " +
		"--no-attach only prints the terminal, for agents and scripts.",
	Args: cobra.ExactArgs(1),
	RunE: runResume,
}

var resumeNoAttach bool

func init() {
	resumeCmd.Flags().BoolVar(&resumeNoAttach, "no-attach", false, "start or find the session and print its terminal; do not attach")
	rootCmd.AddCommand(resumeCmd)
}

// agentLabel marks the terminal of an agent run's Claude session.
const agentLabel = "lazycron.agent"

// agentTerminal returns the live terminal of run's Claude session, if any.
func agentTerminal(hc *runner.HC, run runView) (runner.Terminal, bool) {
	if run.Agent != nil && run.Agent.Terminal != "" {
		if t, err := hc.Get(run.Agent.Terminal); err == nil && t.Live() {
			return t, true
		}
	}
	terms, err := hc.List(map[string]string{runner.LabelRun: run.ID, agentLabel: "claude"})
	if err != nil {
		return runner.Terminal{}, false
	}
	for _, t := range terms {
		if t.Live() {
			return t, true
		}
	}
	return runner.Terminal{}, false
}

func runResume(cmd *cobra.Command, args []string) error {
	if !resumeNoAttach && !isatty.IsTerminal(os.Stdin.Fd()) {
		return errUsage("resume needs an interactive terminal").withHint("pass --no-attach to only start the session")
	}
	return withBackend(func(b backend.Backend) error {
		run, _, err := resolveRun(b, args[0])
		if err != nil {
			return err
		}
		hc, err := b.Herdr()
		if err != nil {
			return errUnavailable("%v", err)
		}
		addr := ""
		if t, ok := agentTerminal(hc, run); ok {
			addr = t.Address
		} else {
			if addr, err = restartSession(b, hc, run); err != nil {
				return err
			}
		}
		if resumeNoAttach {
			fmt.Fprintln(cmd.OutOrStdout(), addr)
			return nil
		}
		return attachTo(b, hc, addr)
	})
}

// restartSession runs `claude --resume` for run's session in a new terminal.
func restartSession(b backend.Backend, hc *runner.HC, run runView) (string, error) {
	a := run.Agent
	if a == nil || a.SessionID == "" || a.Dir == "" {
		return "", errConflict("run %s has no Claude session to resume", run.ID).
			withHint("only agent runs recorded by this lazycron version have one; see `lazycron logs %s`", run.ID)
	}
	_, local := b.(*backend.LocalBackend)
	if local {
		if _, err := os.Stat(a.Dir); err != nil {
			if a.Repo == "" {
				return "", errNotFound("worktree %s of run %s is gone", a.Dir, run.ID)
			}
			// Claude finds a session by its folder: put a worktree back at the same path.
			ref, err := agent.FetchBase(a.Repo, "")
			if err == nil {
				err = agent.AddWorktree(a.Repo, a.Dir, ref)
			}
			if err != nil {
				return "", errFailure("recreate worktree %s: %v", a.Dir, err)
			}
			fmt.Fprintf(humanOut, "recreated the worktree %s (the run's own changes are on its branch, if it pushed one)\n", a.Dir)
		}
	}
	labels := map[string]string{runner.LabelJob: run.JobID, runner.LabelRun: run.ID, agentLabel: "claude"}
	if cfg, err := config.Load(); err == nil && local && cfg.Herdr.SpaceID != "" {
		labels[runner.LabelSpace] = cfg.Herdr.SpaceID
	}
	addr, err := hc.Start(runner.StartOptions{
		Cwd:    a.Dir,
		Labels: labels,
		// A login shell, so claude is on PATH however herdr-core was started.
		Argv: []string{"bash", "-lc", `exec claude --resume "$0"`, a.SessionID},
	})
	if err != nil {
		return "", errFailure("start claude --resume: %v", err)
	}
	return addr, nil
}
