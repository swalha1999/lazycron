package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/config"
)

var (
	initWithAgents bool
	initName       string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a lazycron project (.lazycron/ + .gitignore)",
	Long: "Adds .lazycron to .gitignore in the current directory and records the project name in " +
		".lazycron/config.yaml. With --with-agents, also writes the built-in Claude agent jobs " +
		"(audit, improve, deps, worker, upstream sync) to .lazycron/jobs/*.md, ready for `lazycron sync`. " +
		"Agents run locally as real Claude Code sessions in herdr terminals.",
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initWithAgents, "with-agents", false, "also write the built-in Claude agent jobs to .lazycron/jobs/")
	initCmd.Flags().StringVar(&initName, "name", "", "project name for .lazycron/config.yaml (defaults to the current directory's basename)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	name := initName
	if name == "" {
		name = filepath.Base(cwd)
	}

	if err := appendGitignore(cwd, agentsDirName); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}
	if err := ensureProjectConfig(cwd, name); err != nil {
		return err
	}

	withAgents := initWithAgents
	if !withAgents && isInteractive() {
		withAgents = promptYesNo("Add the Claude agent jobs to .lazycron/jobs/? [y/N] ")
	}
	if withAgents {
		if err := scaffoldAgents(cwd); err != nil {
			return err
		}
	}

	printNextSteps(withAgents)
	if cmd != nil && wantJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), map[string]any{"project": name, "with_agents": withAgents})
	}
	return nil
}

// ensureProjectConfig writes .lazycron/config.yaml with the project name
// unless the file already exists.
func ensureProjectConfig(cwd, name string) error {
	dir := filepath.Join(cwd, agentsDirName)
	if _, err := os.Stat(config.ProjectConfigPath(dir)); err == nil {
		return nil
	}
	if err := config.SaveProjectConfig(dir, &config.ProjectConfig{Name: name}); err != nil {
		return err
	}
	fmt.Fprintf(humanOut, "Created %s\n", filepath.Join(agentsDirName, "config.yaml"))
	return nil
}

// scaffoldAgents writes every built-in agent job plus a context.md for
// per-repo notes. Existing files are left alone so re-running keeps edits.
func scaffoldAgents(cwd string) error {
	templates, err := builtinAgentTemplates()
	if err != nil {
		return err
	}
	for i := range templates {
		if err := writeAgentTemplate(cwd, &templates[i]); err != nil && !errors.Is(err, errJobExists) {
			return err
		}
	}
	return writeIfMissing(filepath.Join(cwd, agentsDirName, "context.md"), contextTemplate(detectRepoURL(cwd)))
}

// contextTemplate is the starter context.md: notes every agent run gets.
func contextTemplate(repoURL string) string {
	var b strings.Builder
	b.WriteString("<!-- Every lazycron agent in this repo reads this. Fill in what helps; delete the rest. -->\n")
	if repoURL != "" {
		fmt.Fprintf(&b, "<!-- origin: %s -->\n", repoURL)
	}
	b.WriteString("\n- What it is / who uses it:\n- Install deps:\n- Build / typecheck / lint / test:\n" +
		"- Focus on:\n- Never touch:\n- Upstream (forks only, e.g. owner/repo@main):\n")
	return b.String()
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	if rel, err := filepath.Rel(filepath.Dir(filepath.Dir(path)), path); err == nil {
		fmt.Fprintf(humanOut, "Created %s\n", rel)
	}
	return nil
}

// detectRepoURL returns the URL of the git remote `origin` for cwd, or
// empty string if cwd isn't a git working tree, has no origin, or git
// isn't installed. We never fail the init on this — it's a convenience.
func detectRepoURL(cwd string) string {
	c := exec.Command("git", "-C", cwd, "remote", "get-url", "origin")
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// appendGitignore adds line to .gitignore if not already present, creating the
// file if missing. Idempotent.
func appendGitignore(cwd, line string) error {
	path := filepath.Join(cwd, ".gitignore")
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	for _, l := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := ""
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + line + "\n")
	return err
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func promptYesNo(prompt string) bool {
	fmt.Fprint(humanOut, prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func printNextSteps(withAgents bool) {
	fmt.Fprintln(humanOut)
	fmt.Fprintln(humanOut, "Next steps:")
	if withAgents {
		fmt.Fprintln(humanOut, "  1. Fill in .lazycron/context.md (how to build/test, what to focus on)")
		fmt.Fprintln(humanOut, "  2. Edit or delete .lazycron/jobs/*.md — schedule, model and prompt per job")
		fmt.Fprintln(humanOut, "  3. lazycron agent .lazycron/jobs/audit.md --dry-run   # see what an agent gets")
		fmt.Fprintln(humanOut, "  4. lazycron sync                                       # install the cron entries")
	} else {
		fmt.Fprintln(humanOut, "  1. lazycron init --with-agents   # add the Claude agent jobs")
		fmt.Fprintln(humanOut, "  2. lazycron templates ls")
	}
}
