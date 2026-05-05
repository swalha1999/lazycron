package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/record"
	sshclient "github.com/swalha1999/lazycron/ssh"
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
	syncServer      string
	syncProject     string
	syncNoEnv       bool
	syncNoFiles     bool
	syncNoBuild     bool
	syncNoInstall   bool
	syncSkipDepsChk bool
)

func init() {
	syncCmd.Flags().StringVarP(&syncServer, "server", "s", "", "target server name from config")
	syncCmd.Flags().StringVar(&syncProject, "project", "", "project name (overrides .lazycron/config.yaml; defaults to cwd basename)")
	syncCmd.Flags().BoolVar(&syncNoEnv, "no-env", false, "do NOT sync .lazycron/.env or .sandcastle/.env to the remote")
	syncCmd.Flags().BoolVar(&syncNoFiles, "no-files", false, "do NOT transfer .lazycron/ or .sandcastle/ files (crontab only)")
	syncCmd.Flags().BoolVar(&syncNoBuild, "no-build", false, "do NOT run `npx @ai-hero/sandcastle docker build-image` after transferring files")
	syncCmd.Flags().BoolVar(&syncNoInstall, "no-install", false, "do NOT run `npm install --prefix .sandcastle` before building the image")
	syncCmd.Flags().BoolVar(&syncSkipDepsChk, "skip-deps-check", false, "skip the docker/node/npx presence check on the target")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	// Resolve project layout.
	lazycronDir := filepath.Join(cwd, ".lazycron")
	sandcastleDir := filepath.Join(cwd, ".sandcastle")
	jobsDir := filepath.Join(sandcastleDir, "jobs")

	if _, err := os.Stat(jobsDir); err != nil {
		if _, scErr := os.Stat(sandcastleDir); scErr != nil {
			return fmt.Errorf("no .sandcastle/ directory found at %s — run `lazycron init --with-agents` first", sandcastleDir)
		}
		return fmt.Errorf("no .sandcastle/jobs/ directory found at %s — run `lazycron templates list` to browse agents and `lazycron templates apply <name>` to scaffold one", jobsDir)
	}

	// Resolve project name (--project > config.yaml > cwd basename).
	pcfg, err := config.LoadProjectConfig(lazycronDir)
	if err != nil {
		return err
	}
	projectName := config.ResolveProjectName(syncProject, pcfg, cwd)

	// Read TS jobs.
	incoming, err := readSandcastleJobs(jobsDir, projectName)
	if err != nil {
		return err
	}
	if len(incoming) == 0 {
		fmt.Printf("No .ts files found in %s\n", jobsDir)
		return nil
	}

	// Resolve backend.
	b, err := resolveBackend(syncServer)
	if err != nil {
		return err
	}
	defer b.Close()

	// Deps check on the target (where cron will fire).
	if !syncSkipDepsChk {
		missing, err := b.CheckAgentDeps()
		if err != nil {
			return fmt.Errorf("check agent deps: %w", err)
		}
		if len(missing) > 0 {
			target := "this machine"
			if syncServer != "" {
				target = syncServer
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
	if syncServer != "" && !syncNoFiles {
		excludesLazycron := []string{}
		excludesSandcastle := []string{"./node_modules", "./logs"}
		if syncNoEnv {
			excludesLazycron = append(excludesLazycron, "./.env")
			excludesSandcastle = append(excludesSandcastle, "./.env")
		}

		if _, err := os.Stat(lazycronDir); err == nil {
			fmt.Printf("Transferring .lazycron/ to %s:~/.lazycron/projects/%s/.lazycron/\n", syncServer, projectName)
			if err := b.CopyProjectFiles(lazycronDir, projectName+"/.lazycron", excludesLazycron); err != nil {
				return fmt.Errorf("copy .lazycron/: %w", err)
			}
		}
		fmt.Printf("Transferring .sandcastle/ to %s:~/.lazycron/projects/%s/.sandcastle/\n", syncServer, projectName)
		if err := b.CopyProjectFiles(sandcastleDir, projectName+"/.sandcastle", excludesSandcastle); err != nil {
			return fmt.Errorf("copy .sandcastle/: %w", err)
		}
	}

	// Install .sandcastle/ npm deps (sandcastle + tsx). Cron commands invoke
	// .sandcastle/node_modules/.bin/tsx directly, so this must complete
	// before any cron firing.
	if !syncNoInstall {
		fmt.Println("Installing .sandcastle/ dependencies (npm install)...")
		if err := b.RunInProject(projectName, "npm install --prefix .sandcastle --no-audit --no-fund --silent", os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("npm install in .sandcastle/: %w", err)
		}
	}

	// Build the sandcastle image (cheap when layers cached; explicit so cron firings can't fail on missing image).
	if !syncNoBuild {
		fmt.Println("Building sandcastle image...")
		if err := b.RunInProject(projectName, "npx -y @ai-hero/sandcastle@0.5.7 docker build-image", os.Stdout, os.Stderr); err != nil {
			return fmt.Errorf("build sandcastle image: %w", err)
		}
	}

	// Inject `cd <project-dir>` into each command (lands inside the wrapped script body).
	projectDir := remoteProjectPath(syncServer, projectName, cwd)
	for i := range incoming {
		incoming[i].Command = fmt.Sprintf("cd %s && %s", shellQuoteSingle(projectDir), incoming[i].Command)
	}

	// Read existing jobs.
	existing, err := b.ReadJobs()
	if err != nil {
		return fmt.Errorf("failed to read jobs: %w", err)
	}

	// Merge.
	merged, added, updated, unchanged := mergeJobs(existing, incoming)

	if added+updated > 0 {
		if err := b.WriteJobs(merged); err != nil {
			return fmt.Errorf("failed to write jobs: %w", err)
		}
	}

	fmt.Printf("Synced: %d added, %d updated, %d unchanged\n", added, updated, unchanged)
	return nil
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
		id := strings.TrimSuffix(filepath.Base(f), ".ts")
		if err := cron.ValidateID(id); err != nil {
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

		jobs = append(jobs, cron.Job{
			ID:       id,
			Name:     meta.Name,
			Schedule: cronExpr,
			Command:  fmt.Sprintf(".sandcastle/node_modules/.bin/tsx .sandcastle/jobs/%s.ts", id),
			Enabled:  true,
			Wrapped:  true,
			Tag:      meta.Tag,
			TagColor: meta.TagColor,
			Project:  projectName,
		})
	}
	return jobs, nil
}

// remoteProjectPath returns the directory the cron command should `cd` into
// before invoking the agent. Local mode uses the current working directory;
// remote mode uses ~/.lazycron/projects/<name>/.
func remoteProjectPath(server, projectName, cwd string) string {
	if server == "" {
		return cwd
	}
	return "~/.lazycron/projects/" + projectName
}

// shellQuoteSingle wraps s in single quotes, escaping any embedded singles.
// Local copy here so cmd doesn't depend on ssh-internal helpers.
func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// mergeJobs merges incoming jobs into existing jobs by ID.
// Jobs in existing that are not in incoming are preserved unchanged.
func mergeJobs(existing, incoming []cron.Job) (merged []cron.Job, added, updated, unchanged int) {
	idxByID := make(map[string]int, len(existing))
	for i, j := range existing {
		if j.ID != "" {
			idxByID[j.ID] = i
		}
	}

	merged = make([]cron.Job, len(existing))
	copy(merged, existing)

	for _, inc := range incoming {
		if idx, ok := idxByID[inc.ID]; ok {
			if jobNeedsUpdate(merged[idx], inc) {
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

func jobNeedsUpdate(existing, incoming cron.Job) bool {
	return existing.Name != incoming.Name ||
		existing.Schedule != incoming.Schedule ||
		existing.Command != incoming.Command ||
		existing.Enabled != incoming.Enabled ||
		existing.Wrapped != incoming.Wrapped ||
		existing.OneShot != incoming.OneShot ||
		existing.Tag != incoming.Tag ||
		existing.TagColor != incoming.TagColor ||
		existing.Project != incoming.Project
}

// resolveBackend creates the appropriate backend for the sync target.
func resolveBackend(serverName string) (backend.Backend, error) {
	if serverName == "" {
		if err := cron.CheckCrontabAvailable(); err != nil {
			return nil, err
		}
		if err := record.InstallRecord(); err != nil {
			return nil, fmt.Errorf("install record script: %w", err)
		}
		return backend.NewLocalBackend(), nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	var srv *config.ServerConfig
	var names []string
	for i := range cfg.Servers {
		names = append(names, cfg.Servers[i].Name)
		if cfg.Servers[i].Name == serverName {
			srv = &cfg.Servers[i]
		}
	}
	if srv == nil {
		if len(names) == 0 {
			return nil, fmt.Errorf("server %q not found (no servers configured)", serverName)
		}
		return nil, fmt.Errorf("server %q not found (available: %s)", serverName, strings.Join(names, ", "))
	}

	port := srv.Port
	if port == 0 {
		port = 22
	}

	client := sshclient.NewClient(srv.Host, port, srv.User, "", config.ExpandHome(srv.KeyPath), srv.UseAgent)
	remote := backend.NewRemoteBackend(srv.Name, client)

	if err := remote.EnsureRecordScript(); err != nil {
		remote.Close()
		return nil, fmt.Errorf("connect to %s: %w", serverName, err)
	}

	return remote, nil
}
