package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
)

// syncContext holds everything `sync` and `diff` need after the shared
// pre-processing pipeline has run: paths resolved, project name picked,
// TS-derived jobs loaded with their `cd <project-dir> && ` prefix applied,
// and the existing crontab read from the target.
type syncContext struct {
	Backend       backend.Backend
	ProjectName   string
	LazycronDir   string
	SandcastleDir string
	ProjectDir    string
	IncomingJobs  []cron.Job // commands already cd-prefixed for the target
	ExistingJobs  []cron.Job
}

// loadSyncContext runs the pipeline shared by `sync` and `diff`: resolve
// project paths, verify .sandcastle/jobs/ exists, load project config, parse
// TS jobs, open the backend, and read the existing crontab. If there are no
// .ts files to install it prints a notice and returns (nil, nil) so callers
// can exit cleanly. On any error after the backend is opened the backend is
// closed for the caller; on success the caller owns it and must defer Close.
func loadSyncContext(serverFlag, projectFlag string) (*syncContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get cwd: %w", err)
	}

	lazycronDir := filepath.Join(cwd, ".lazycron")
	sandcastleDir := filepath.Join(cwd, ".sandcastle")
	jobsDir := filepath.Join(sandcastleDir, "jobs")

	// `!info.IsDir()` catches the case where someone has a regular file named
	// `.sandcastle/jobs` — `os.Stat` would not error but the directory walk
	// later would, with a less helpful message.
	if info, err := os.Stat(jobsDir); err != nil || !info.IsDir() {
		if _, scErr := os.Stat(sandcastleDir); scErr != nil {
			return nil, fmt.Errorf("no .sandcastle/ directory found at %s — run `lazycron init --with-agents` first", sandcastleDir)
		}
		return nil, fmt.Errorf("no .sandcastle/jobs/ directory found at %s — run `lazycron templates list` to browse agents and `lazycron templates apply <name>` to scaffold one", jobsDir)
	}

	pcfg, err := config.LoadProjectConfig(lazycronDir)
	if err != nil {
		return nil, err
	}
	projectName := config.ResolveProjectName(projectFlag, pcfg, cwd)

	incoming, err := readSandcastleJobs(jobsDir, projectName)
	if err != nil {
		return nil, err
	}
	if len(incoming) == 0 {
		fmt.Printf("No .ts files found in %s\n", jobsDir)
		return nil, nil
	}

	b, err := resolveBackend(serverFlag)
	if err != nil {
		return nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			b.Close()
		}
	}()

	// Resolve via the backend so the path is absolute on the target — a literal
	// `~` would not expand inside the single-quoted shell argument cron sees.
	projectDir, err := b.ProjectDir(projectName)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}
	for i := range incoming {
		incoming[i].Command = fmt.Sprintf("cd %s && %s", shellQuoteSingle(projectDir), incoming[i].Command)
	}

	existing, err := b.ReadJobs()
	if err != nil {
		return nil, fmt.Errorf("failed to read jobs: %w", err)
	}

	closeOnError = false
	return &syncContext{
		Backend:       b,
		ProjectName:   projectName,
		LazycronDir:   lazycronDir,
		SandcastleDir: sandcastleDir,
		ProjectDir:    projectDir,
		IncomingJobs:  incoming,
		ExistingJobs:  existing,
	}, nil
}
