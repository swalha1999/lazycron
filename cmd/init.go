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
	"github.com/swalha1999/lazycron/template"
	"github.com/swalha1999/lazycron/template/builtin"
)

var (
	initWithAgents bool
	initName       string
	initForce      bool
	// execNpxInit is a swappable seam so tests can stub the npx call.
	execNpxInit = realNpxInit
	// writeEnsureRepoLib is the file-write step, swapped out in tests.
	writeEnsureRepoLib = realWriteEnsureRepoLib
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a lazycron project (.lazycron/ + optional .sandcastle/)",
	Long: "Scaffolds a .lazycron/config.yaml in the current directory. " +
		"With --with-agents, also runs `npx @ai-hero/sandcastle init` to set up agent " +
		"sandboxing as a sibling .sandcastle/ directory.",
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initWithAgents, "with-agents", false, "also scaffold .sandcastle/ via `npx @ai-hero/sandcastle init`")
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

func scaffoldSandcastle(cwd string) error {
	if _, err := exec.LookPath("npx"); err != nil {
		return errors.New("npx is required for --with-agents but was not found in PATH; install Node 20+ and Docker, then re-run with --with-agents")
	}
	if err := execNpxInit(cwd); err != nil {
		return fmt.Errorf("`npx @ai-hero/sandcastle init` failed: %w", err)
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

func realNpxInit(cwd string) error {
	// -y skips the npx "Ok to proceed?" confirmation; the user already opted in
	// via --with-agents or the interactive prompt. The scoped @ai-hero/sandcastle
	// package is the actual CLI — the unscoped `sandcastle` on npm is an
	// unrelated JS sandbox library and silently exits 0 on `init`.
	//
	// `--template blank --agent claude-code` skips the agent and template
	// pickers; lazycron scaffolds its own .sandcastle/jobs/*.ts on top, so the
	// blank base layout is what we want.
	c := exec.Command("npx", "-y", "@ai-hero/sandcastle", "init", "--template", "blank", "--agent", "claude-code")
	c.Dir = cwd
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
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
		fmt.Println("  1. Edit .sandcastle/.env (set ANTHROPIC_API_KEY, REPO_URL, GH_TOKEN, …)")
		fmt.Println("  2. Review/trim .sandcastle/jobs/*.ts — delete agents you don't want scheduled")
		fmt.Println("  3. lazycron sync             # local")
		fmt.Println("     lazycron sync --server X  # remote, ships .sandcastle/ + builds image")
	} else {
		fmt.Println("  1. lazycron init --with-agents   # to add sandcastle agents later")
		fmt.Println("  2. lazycron templates list")
	}
}
