package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/agent"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/runner"
)

var (
	agentProject string
	agentDryRun  bool
)

var agentCmd = &cobra.Command{
	Use:   "agent <job.md>",
	Short: "Run one Claude agent job now, in a herdr terminal (what synced cron entries call)",
	Long: "Runs a .lazycron/jobs/<id>.md agent job: makes a fresh git worktree from origin's default " +
		"branch, starts an interactive Claude Code session in a herdr terminal you can watch or take " +
		"over, and waits until it is done. A session that blocks on a question is left open and the " +
		"run fails with the terminal to attach to.",
	Args: cobra.ExactArgs(1),
	RunE: runAgent,
}

func init() {
	agentCmd.Flags().StringVar(&agentProject, "project", "", "project name (default: .lazycron/config.yaml, else the directory name)")
	agentCmd.Flags().BoolVar(&agentDryRun, "dry-run", false, "print the repo, worktree and prompt without starting anything")
	rootCmd.AddCommand(agentCmd)
}

func runAgent(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	pcfg, err := config.LoadProjectConfig(filepath.Join(cwd, agentsDirName))
	if err != nil {
		return err
	}
	opts := agent.Options{
		ProjectDir:  cwd,
		ProjectName: config.ResolveProjectName(agentProject, pcfg, cwd),
		JobPath:     args[0],
		DryRun:      agentDryRun,
		Log:         func(format string, a ...any) { fmt.Fprintf(humanOut, format+"\n", a...) },
	}
	if !agentDryRun {
		configured := ""
		if cfg, err := config.Load(); err == nil {
			configured = cfg.Herdr.Bin
		}
		hc, err := runner.FindHC(configured)
		if err != nil {
			return errUnavailable("%v", err).withHint("agents run in herdr terminals; run `lazycron setup`")
		}
		opts.HC = agent.ExecHC{Path: hc}
	}
	if err := agent.Run(opts); err != nil {
		if errors.Is(err, agent.ErrBlocked) {
			return errFailure("%v", err)
		}
		return err
	}
	return nil
}
