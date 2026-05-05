package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/template"
	"github.com/swalha1999/lazycron/template/builtin"
)

var (
	initWithAgents bool
	initName       string
	initForce      bool
	// scaffoldSandcastleConfig is a swappable seam so tests can stub the
	// .sandcastle/ scaffold step.
	scaffoldSandcastleConfig = realScaffoldSandcastleConfig
	// writeEnsureRepoLib is the file-write step, swapped out in tests.
	writeEnsureRepoLib = realWriteEnsureRepoLib
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a lazycron project (.lazycron/ + optional .sandcastle/)",
	Long: "Scaffolds a .lazycron/config.yaml in the current directory. " +
		"With --with-agents, also scaffolds a sibling .sandcastle/ directory " +
		"with a Claude Code + Docker + GitHub Issues setup ready for `lazycron sync`.",
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initWithAgents, "with-agents", false, "also scaffold .sandcastle/ (Claude Code, Docker, GitHub Issues)")
	initCmd.Flags().StringVar(&initName, "name", "", "project name (defaults to current directory's basename)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite an existing .lazycron/")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	lazycronDir := filepath.Join(cwd, ".lazycron")
	if _, err := os.Stat(lazycronDir); err == nil && !initForce {
		return fmt.Errorf(".lazycron/ already exists in %s — pass --force to overwrite", cwd)
	}

	name := initName
	if name == "" {
		name = filepath.Base(cwd)
	}

	if err := config.SaveProjectConfig(lazycronDir, &config.ProjectConfig{Name: name}); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}
	fmt.Printf("Created %s\n", filepath.Join(".lazycron", "config.yaml"))

	if err := appendGitignore(cwd, ".lazycron/.env"); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	withAgents := initWithAgents
	if !withAgents && isInteractive() {
		withAgents = promptYesNo("Set up sandcastle agents (.sandcastle/) as a sibling? [y/N] ")
	}

	if withAgents {
		if err := scaffoldSandcastle(cwd); err != nil {
			return err
		}
	}

	printNextSteps(withAgents)
	return nil
}

// scaffoldSandcastle writes the full .sandcastle/ tree directly — no
// `npx sandcastle init` invocation. This avoids sandcastle's interactive
// prompts (sandbox provider, backlog manager, label, build-image) and
// guarantees the same defaults every time: Claude Code agent, Docker
// sandbox, GitHub Issues backlog. The Dockerfile, .env.example, and
// in-folder .gitignore are pinned copies of @ai-hero/sandcastle@0.5.7's
// blank-template output for that combination.
func scaffoldSandcastle(cwd string) error {
	if err := scaffoldSandcastleConfig(cwd); err != nil {
		return fmt.Errorf("scaffold .sandcastle/: %w", err)
	}

	if err := writeEnsureRepoLib(cwd); err != nil {
		return fmt.Errorf("write ensureRepo.ts: %w", err)
	}
	fmt.Printf("Created %s\n", filepath.Join(".sandcastle", "lib", "ensureRepo.ts"))

	if err := appendGitignore(cwd, ".sandcastle/.env"); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	if err := applyAllSandcastleTemplates(cwd); err != nil {
		return fmt.Errorf("apply sandcastle templates: %w", err)
	}
	return nil
}

// realScaffoldSandcastleConfig writes the embedded Dockerfile, .env.example,
// and .gitignore into .sandcastle/. Existing files are not overwritten so
// re-running with --force preserves user edits.
func realScaffoldSandcastleConfig(cwd string) error {
	sandcastleDir := filepath.Join(cwd, ".sandcastle")
	if err := os.MkdirAll(sandcastleDir, 0o755); err != nil {
		return err
	}

	// Auto-detect REPO_URL from the git remote so the user doesn't have to
	// fill it in by hand on first run. Empty string if cwd isn't a git repo
	// or has no origin — .env still gets the empty `REPO_URL=` line.
	repoURL := detectRepoURL(cwd)

	files := []struct {
		embed     string
		dest      string
		transform func([]byte) []byte
	}{
		{"sandcastle_init/Dockerfile", "Dockerfile", nil},
		{"sandcastle_init/env.example", ".env.example", nil},
		// Seed .env from .env.example so the template's requireEnv check
		// surfaces "set REPO_URL in .sandcastle/.env" instead of "no such
		// file" the first time the user runs an agent. Skipped if .env
		// already exists. If we found a git remote, pre-fill REPO_URL.
		{"sandcastle_init/env.example", ".env", fillRepoURL(repoURL)},
		{"sandcastle_init/gitignore", ".gitignore", nil},
		{"sandcastle_init/package.json", "package.json", nil},
	}
	for _, f := range files {
		dest := filepath.Join(sandcastleDir, f.dest)
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		data, err := builtin.SandcastleInitFS.ReadFile(f.embed)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", f.embed, err)
		}
		if f.transform != nil {
			data = f.transform(data)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		fmt.Printf("Created %s\n", filepath.Join(".sandcastle", f.dest))
	}
	if repoURL != "" {
		fmt.Printf("Detected REPO_URL=%s from git remote (pre-filled in .sandcastle/.env)\n", repoURL)
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

// fillRepoURL returns a transform that replaces the `REPO_URL=` line in
// .env contents with `REPO_URL=<url>`. No-op if url is empty.
func fillRepoURL(url string) func([]byte) []byte {
	if url == "" {
		return nil
	}
	return func(data []byte) []byte {
		// Only replace when the line is empty (REPO_URL=\n), so we don't
		// accidentally clobber a value the user already set in env.example.
		return bytes.Replace(data, []byte("REPO_URL=\n"), []byte("REPO_URL="+url+"\n"), 1)
	}
}

// applyAllSandcastleTemplates copies every bundled sandcastle agent template
// (.ts files under template/builtin/templates/) into .sandcastle/jobs/.
// Existing files are left untouched so re-running with --force does not
// clobber user edits.
func applyAllSandcastleTemplates(cwd string) error {
	jobsDir := filepath.Join(cwd, ".sandcastle", "jobs")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		return err
	}

	templates, err := template.LoadBuiltin()
	if err != nil {
		return err
	}

	created := 0
	for _, t := range templates {
		if t.Type != template.TypeSandcastle {
			continue
		}
		dest := filepath.Join(jobsDir, t.Filename+".ts")
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		data, err := builtin.FS.ReadFile(t.SandcastleSource)
		if err != nil {
			return fmt.Errorf("read embedded template %s: %w", t.Filename, err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		fmt.Printf("Created %s\n", filepath.Join(".sandcastle", "jobs", t.Filename+".ts"))
		created++
	}
	if created > 0 {
		fmt.Printf("Applied %d sandcastle agent template(s) to .sandcastle/jobs/\n", created)
	}
	return nil
}

func realWriteEnsureRepoLib(cwd string) error {
	libDir := filepath.Join(cwd, ".sandcastle", "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return err
	}
	data, err := builtin.SandcastleLibFS.ReadFile("sandcastle_lib/ensureRepo.ts")
	if err != nil {
		return fmt.Errorf("read embedded ensureRepo.ts: %w", err)
	}
	return os.WriteFile(filepath.Join(libDir, "ensureRepo.ts"), data, 0o644)
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
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func printNextSteps(withAgents bool) {
	fmt.Println()
	fmt.Println("Next steps:")
	if withAgents {
		fmt.Println("  1. Edit .sandcastle/.env (set ANTHROPIC_API_KEY, REPO_URL, GH_TOKEN)")
		fmt.Println("  2. Review/trim .sandcastle/jobs/*.ts — delete agents you don't want scheduled")
		fmt.Println("  3. lazycron sync             # local (builds Docker image, installs cron)")
		fmt.Println("     lazycron sync --server X  # remote, ships .sandcastle/ + builds image")
	} else {
		fmt.Println("  1. lazycron init --with-agents   # to add sandcastle agents later")
		fmt.Println("  2. lazycron templates list")
	}
}
