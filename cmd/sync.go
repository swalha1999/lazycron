package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/agent"
	"github.com/swalha1999/lazycron/cron"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Install .lazycron/jobs/*.md agent jobs into the crontab (local or a server)",
	Long: "Reads .lazycron/jobs/*.md in the current directory and installs one cron entry per job, " +
		"from each file's `name`/`cron` front matter. Each entry runs `lazycron agent <job>`, which " +
		"starts a real Claude Code session in a herdr terminal on that machine. With --server, also " +
		"ships .lazycron/ to ~/.lazycron/projects/<project>/ there; the agent clones REPO_URL from " +
		".lazycron/.env because the server has no checkout.",
	RunE: runSync,
}

var (
	syncProject     string
	syncNoEnv       bool
	syncNoFiles     bool
	syncSkipDepsChk bool
)

func init() {
	syncCmd.Flags().StringVarP(&globalServer, "server", "s", "", "target server name from config")
	syncCmd.Flags().StringVar(&syncProject, "project", "", "project name (overrides .lazycron/config.yaml; defaults to cwd basename)")
	syncCmd.Flags().BoolVar(&syncNoEnv, "no-env", false, "do NOT sync .lazycron/.env to the remote")
	syncCmd.Flags().BoolVar(&syncNoFiles, "no-files", false, "do NOT transfer .lazycron/ files (crontab only)")
	syncCmd.Flags().BoolVar(&syncSkipDepsChk, "skip-deps-check", false, "skip the claude/git/gh/herdr-core presence check on the target")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	sctx, err := loadSyncContext(globalServer, syncProject)
	if err != nil {
		return err
	}
	if sctx == nil {
		return nil
	}
	defer sctx.Backend.Close()

	// Deps check on the target (where cron will fire).
	if !syncSkipDepsChk {
		missing, err := sctx.Backend.CheckAgentDeps()
		if err != nil {
			return fmt.Errorf("check agent deps: %w", err)
		}
		if len(missing) > 0 {
			target := "this machine"
			if globalServer != "" {
				target = globalServer
			}
			return fmt.Errorf(
				"agents need lazycron, claude, git, gh and herdr-core installed on %s but these are missing: %s. "+
					"`lazycron setup` (with --server for a server) installs lazycron and herdr-core; "+
					"install the rest, or use --skip-deps-check.",
				target, strings.Join(missing, ", "),
			)
		}
	}

	// Transfer files (only if remote and --no-files not set).
	if globalServer != "" && !syncNoFiles {
		var excludes []string
		if syncNoEnv {
			excludes = append(excludes, "./.env")
		}
		fmt.Fprintf(humanOut, "Transferring .lazycron/ to %s:~/.lazycron/projects/%s/.lazycron/\n", globalServer, sctx.ProjectName)
		if err := sctx.Backend.CopyProjectFiles(sctx.LazycronDir, sctx.ProjectName+"/"+agentsDirName, excludes); err != nil {
			return fmt.Errorf("copy .lazycron/: %w", err)
		}
	}

	merged, added, updated, unchanged := mergeJobs(sctx.ExistingJobs, sctx.IncomingJobs)

	if added+updated > 0 {
		if err := sctx.Backend.WriteJobs(merged); err != nil {
			return fmt.Errorf("failed to write jobs: %w", err)
		}
	}

	summary := map[string]any{"project": sctx.ProjectName, "server": serverLabel(), "added": added, "updated": updated, "unchanged": unchanged}
	return render(cmd, summary, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "Synced: %d added, %d updated, %d unchanged\n", added, updated, unchanged)
		return err
	})
}

// agentCommand is the job command for one agent. Job scripts rebuild the
// login-shell PATH first, so the installed lazycron is found the same way
// locally and on a server.
func agentCommand(bareID, projectName string) string {
	return fmt.Sprintf(`lazycron agent %s/jobs/%s.md --project %s`,
		agentsDirName, bareID, cron.ShellQuote(projectName))
}

// readAgentJobs walks .lazycron/jobs/*.md, parses each job's front matter,
// and returns one cron.Job per file. Files that fail to parse are reported
// with their filenames and stop the sync (better than silently shipping a
// partial set).
func readAgentJobs(jobsDir, projectName string) ([]cron.Job, error) {
	files, err := filepath.Glob(filepath.Join(jobsDir, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var jobs []cron.Job
	for _, f := range files {
		bareID := strings.TrimSuffix(filepath.Base(f), ".md")
		if err := cron.ValidateID(bareID); err != nil {
			return nil, fmt.Errorf("invalid agent job filename %s: %w", filepath.Base(f), err)
		}

		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.Base(f), err)
		}
		job, err := agent.ParseJob(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filepath.Base(f), err)
		}

		cronExpr := cron.HumanToCron(job.Cron)
		if err := cron.ValidateCron(cronExpr); err != nil {
			return nil, fmt.Errorf("%s: invalid schedule %q: %w", filepath.Base(f), job.Cron, err)
		}

		// Namespace the ID by project so two repos with the same filename
		// (e.g. both have a worker.md) don't share a script path or crontab
		// entry on a shared machine.
		id := cron.ScopedID(projectName, bareID)
		if err := cron.ValidateID(id); err != nil {
			return nil, fmt.Errorf("scoped job ID %q invalid (project=%q, file=%s): %w", id, projectName, filepath.Base(f), err)
		}

		jobs = append(jobs, cron.Job{
			ID:       id,
			Name:     job.Name,
			Schedule: cronExpr,
			Command:  agentCommand(bareID, projectName),
			Enabled:  true,
			Wrapped:  true,
			Tag:      job.Tag,
			TagColor: job.TagColor,
			Project:  projectName,
		})
	}
	return jobs, nil
}

// mergeJobs merges incoming jobs into existing jobs by ID.
// Jobs in existing that are not in incoming are preserved unchanged.
func mergeJobs(existing, incoming []cron.Job) (merged []cron.Job, added, updated, unchanged int) {
	idxByID := make(map[string]int, len(existing))
	for i, j := range existing {
		if j.ID == "" {
			continue
		}
		idxByID[j.ID] = i
		// Migration: a job synced by an older lazycron has a bare ID
		// (e.g. "worker-agent") rather than the project-scoped ID
		// ("lazycron-worker-agent") this version writes. Index the
		// scoped form too so a re-sync upgrades the entry in place
		// instead of leaving the old one orphaned alongside the new.
		if scoped := cron.ScopedID(j.Project, j.ID); scoped != j.ID {
			idxByID[scoped] = i
		}
	}

	merged = make([]cron.Job, len(existing))
	copy(merged, existing)

	for _, inc := range incoming {
		if idx, ok := idxByID[inc.ID]; ok {
			if cron.JobsDiffer(merged[idx], inc) {
				merged[idx] = inc
				updated++
			} else {
				unchanged++
			}
		} else {
			merged = append(merged, inc)
			added++
		}
	}

	return merged, added, updated, unchanged
}
