package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/template"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync sandcastle agent jobs to crontab and (optionally) a remote machine",
	Long: "Reads .sandcastle/jobs/*.ts in the current directory, derives cron entries " +
		"from each file's `export const cron`/`export const name` metadata, and installs " +
		"them on the local crontab or — with --server — on a remote machine. When --server " +
		"is set, also tars and ships .lazycron/ and .sandcastle/ to the remote, builds the " +
		"sandcastle Docker image there, and rewrites each job's command to run inside the " +
		"project's remote directory.",
	RunE: runSync,
}

var (
	syncProject     string
	syncNoEnv       bool
	syncNoFiles     bool
	syncNoBuild     bool
	syncNoInstall   bool
	syncSkipDepsChk bool
)

func init() {
	syncCmd.Flags().StringVarP(&globalServer, "server", "s", "", "target server name from config")
	syncCmd.Flags().StringVar(&syncProject, "project", "", "project name (overrides .lazycron/config.yaml; defaults to cwd basename)")
	syncCmd.Flags().BoolVar(&syncNoEnv, "no-env", false, "do NOT sync .lazycron/.env or .sandcastle/.env to the remote")
	syncCmd.Flags().BoolVar(&syncNoFiles, "no-files", false, "do NOT transfer .lazycron/ or .sandcastle/ files (crontab only)")
	syncCmd.Flags().BoolVar(&syncNoBuild, "no-build", false, "do NOT run `npx @ai-hero/sandcastle docker build-image` after transferring files")
	syncCmd.Flags().BoolVar(&syncNoInstall, "no-install", false, "do NOT run `npm install --prefix .sandcastle` before building the image")
	syncCmd.Flags().BoolVar(&syncSkipDepsChk, "skip-deps-check", false, "skip the docker/node/npx presence check on the target")
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

	// Ensure .sandcastle/.env has PROJECT_NAME. The agent reads this to align
	// its cache dir (~/.lazycron-cache/repos/<name>) and sandcastle Docker image
	// tag with the lazycron project — missing it lands on a hard-coded default
	// and silently runs the wrong cached repo / image.
	if err := ensureSandcastleEnvProjectName(sctx.SandcastleDir, sctx.ProjectName); err != nil {
		return fmt.Errorf("update .sandcastle/.env: %w", err)
	}

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
				"sandcastle agents need {docker, node, npx} on %s but the following are missing: %s. "+
					"Install Docker (https://docker.com) and Node 20+ (https://nodejs.org). "+
					"Use --skip-deps-check to override.",
				target, strings.Join(missing, ", "),
			)
		}
	}

	// Transfer files (only if remote and --no-files not set).
	if globalServer != "" && !syncNoFiles {
		excludesLazycron := []string{}
		excludesSandcastle := []string{"./node_modules", "./logs"}
		if syncNoEnv {
			excludesLazycron = append(excludesLazycron, "./.env")
			excludesSandcastle = append(excludesSandcastle, "./.env")
		}

		if _, err := os.Stat(sctx.LazycronDir); err == nil {
			fmt.Fprintf(humanOut, "Transferring .lazycron/ to %s:~/.lazycron/projects/%s/.lazycron/\n", globalServer, sctx.ProjectName)
			if err := sctx.Backend.CopyProjectFiles(sctx.LazycronDir, sctx.ProjectName+"/.lazycron", excludesLazycron); err != nil {
				return fmt.Errorf("copy .lazycron/: %w", err)
			}
		}
		fmt.Fprintf(humanOut, "Transferring .sandcastle/ to %s:~/.lazycron/projects/%s/.sandcastle/\n", globalServer, sctx.ProjectName)
		if err := sctx.Backend.CopyProjectFiles(sctx.SandcastleDir, sctx.ProjectName+"/.sandcastle", excludesSandcastle); err != nil {
			return fmt.Errorf("copy .sandcastle/: %w", err)
		}
	}

	// Install .sandcastle/ npm deps (sandcastle + tsx). Cron commands invoke
	// .sandcastle/node_modules/.bin/tsx directly, so this must complete
	// before any cron firing.
	if !syncNoInstall {
		fmt.Fprintln(humanOut, "Installing .sandcastle/ dependencies (npm install)...")
		if err := sctx.Backend.RunInProject(sctx.ProjectName, "npm install --prefix .sandcastle --no-audit --no-fund --silent", humanOut, humanOut); err != nil {
			return fmt.Errorf("npm install in .sandcastle/: %w", err)
		}
	}

	// Build the sandcastle image (cheap when layers cached; explicit so cron firings can't fail on missing image).
	if !syncNoBuild {
		fmt.Fprintln(humanOut, "Building sandcastle image...")
		if err := sctx.Backend.RunInProject(sctx.ProjectName, "npx -y @ai-hero/sandcastle@0.5.7 docker build-image", humanOut, humanOut); err != nil {
			return fmt.Errorf("build sandcastle image: %w", err)
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

// readSandcastleJobs walks .sandcastle/jobs/*.ts, parses metadata from each
// file, and returns one cron.Job per file. Files that fail to parse are
// reported with their filenames and stop the sync (better than silently
// shipping a partial set).
func readSandcastleJobs(jobsDir, projectName string) ([]cron.Job, error) {
	files, err := filepath.Glob(filepath.Join(jobsDir, "*.ts"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	var jobs []cron.Job
	for _, f := range files {
		bareID := strings.TrimSuffix(filepath.Base(f), ".ts")
		if err := cron.ValidateID(bareID); err != nil {
			return nil, fmt.Errorf("invalid sandcastle job filename %s: %w", filepath.Base(f), err)
		}

		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.Base(f), err)
		}
		meta, err := template.ParseSandcastleMeta(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filepath.Base(f), err)
		}

		cronExpr := cron.HumanToCron(meta.Cron)
		if err := cron.ValidateCron(cronExpr); err != nil {
			return nil, fmt.Errorf("%s: invalid schedule %q: %w", filepath.Base(f), meta.Cron, err)
		}

		// Namespace the ID by project so two repos with the same filename
		// (e.g. both have a worker-agent.ts) don't share a script path or
		// crontab entry on a shared remote.
		id := cron.ScopedID(projectName, bareID)
		if err := cron.ValidateID(id); err != nil {
			return nil, fmt.Errorf("scoped job ID %q invalid (project=%q, file=%s): %w", id, projectName, filepath.Base(f), err)
		}

		jobs = append(jobs, cron.Job{
			ID:       id,
			Name:     meta.Name,
			Schedule: cronExpr,
			Command:  fmt.Sprintf(".sandcastle/node_modules/.bin/tsx --env-file-if-exists=.sandcastle/.env .sandcastle/jobs/%s.ts", bareID),
			Enabled:  true,
			Wrapped:  true,
			Tag:      meta.Tag,
			TagColor: meta.TagColor,
			Project:  projectName,
		})
	}
	return jobs, nil
}

// ensureSandcastleEnvProjectName makes sure .sandcastle/.env contains a
// PROJECT_NAME entry pointing at projectName. Existing values are left
// untouched; a missing key (or missing file) is added. Idempotent.
func ensureSandcastleEnvProjectName(sandcastleDir, projectName string) error {
	if projectName == "" {
		return nil
	}
	envPath := filepath.Join(sandcastleDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "PROJECT_NAME=") {
			return nil
		}
	}

	if err := os.MkdirAll(sandcastleDir, 0o755); err != nil {
		return err
	}

	prefix := ""
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		prefix = "\n"
	}
	addition := prefix + "PROJECT_NAME=" + projectName + "\n"
	f, err := os.OpenFile(envPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(addition); err != nil {
		return err
	}
	fmt.Fprintf(humanOut, "Added PROJECT_NAME=%s to %s\n", projectName, filepath.Join(".sandcastle", ".env"))
	return nil
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
