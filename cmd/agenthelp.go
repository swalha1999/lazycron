package cmd

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// agentGuide is the one-read guide for agents; the Claude Code skill is the
// same text with skill frontmatter.
//
//go:embed agenthelp.md
var agentGuide string

const skillFrontmatter = `---
name: lazycron
description: Manage scheduled jobs with lazycron (cron + herdr-core terminals): create, edit, enable/disable, delete, run, wait for, read output of and diagnose cron jobs and their runs. Use when the user asks about cron jobs, scheduled tasks, recurring agents or lazycron.
---

`

var agentHelpCmd = &cobra.Command{
	Use:   "agent-help",
	Short: "Print the guide for AI agents using lazycron (one read)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := io.WriteString(cmd.OutOrStdout(), agentGuide)
		return err
	},
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "The lazycron Claude Code skill",
}

var skillShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the skill's SKILL.md",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := io.WriteString(cmd.OutOrStdout(), skillContent())
		return err
	},
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the skill for Claude Code (~/.claude/skills/lazycron/SKILL.md)",
	Args:  cobra.NoArgs,
	RunE:  runSkillInstall,
}

var skillDir string

func init() {
	skillInstallCmd.Flags().StringVar(&skillDir, "dir", "", "skills directory (default ~/.claude/skills; use .claude/skills for a project)")
	addDryRunFlag(skillInstallCmd)
	skillCmd.AddCommand(skillShowCmd, skillInstallCmd)
	rootCmd.AddCommand(agentHelpCmd, skillCmd)
}

func skillContent() string {
	return skillFrontmatter + agentGuide
}

func runSkillInstall(cmd *cobra.Command, args []string) error {
	dir := skillDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return errFailure("find home: %v", err)
		}
		dir = filepath.Join(home, ".claude", "skills")
	}
	path := filepath.Join(dir, "lazycron", "SKILL.md")
	action := actionCreated
	if current, err := os.ReadFile(path); err == nil {
		action = actionUpdated
		if strings.TrimSpace(string(current)) == strings.TrimSpace(skillContent()) {
			action = actionUnchanged
		}
	}
	if action != actionUnchanged && !dryRun {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return errFailure("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(skillContent()), 0o644); err != nil {
			return errFailure("write %s: %v", path, err)
		}
	}
	res := map[string]any{"action": action, "dry_run": dryRun, "path": path}
	return render(cmd, res, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "%s %s\n", action, path)
		return err
	})
}
