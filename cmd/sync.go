package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/envsubst"
	"github.com/swalha1999/lazycron/notify"
	"github.com/swalha1999/lazycron/record"
	sshclient "github.com/swalha1999/lazycron/ssh"
	"gopkg.in/yaml.v3"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync job definitions from .lazycron/jobs/ to the crontab",
	Long:  "Reads YAML job files from .lazycron/jobs/ in the current directory and creates or updates matching cron jobs.",
	RunE:  runSync,
}

var (
	syncServer string
	syncDir    string
	syncVars   []string
)

func init() {
	syncCmd.Flags().StringVarP(&syncServer, "server", "s", "", "target server name from config")
	syncCmd.Flags().StringVar(&syncDir, "dir", "", "path to .lazycron directory (default: ./.lazycron)")
	syncCmd.Flags().StringArrayVar(&syncVars, "var", nil, "variable substitution in KEY=VALUE format (can be repeated)")
	rootCmd.AddCommand(syncCmd)
}

// jobFile is the YAML structure for a job definition file.
type jobFile struct {
	Name      string            `yaml:"name"`
	Schedule  string            `yaml:"schedule"`
	Command   string            `yaml:"command"`
	Project   string            `yaml:"project,omitempty"`
	Tag       string             `yaml:"tag,omitempty"`
	TagColor  string            `yaml:"tag_color,omitempty"`
	Enabled   *bool             `yaml:"enabled,omitempty"`
	Once      bool              `yaml:"once,omitempty"`
	OnFailure *[]notify.Action  `yaml:"on_failure,omitempty"`
	OnSuccess *[]notify.Action  `yaml:"on_success,omitempty"`
}

// jobNotifyConfig represents a per-job notification config with flags
// indicating which fields were explicitly set in the YAML.
type jobNotifyConfig struct {
	Config              notify.Config
	OnFailureExplicit   bool
	OnSuccessExplicit   bool
}

func runSync(cmd *cobra.Command, args []string) error {
	// Resolve jobs directory.
	dir := syncDir
	if dir == "" {
		dir = ".lazycron"
	}
	jobsDir := filepath.Join(dir, "jobs")

	info, err := os.Stat(jobsDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("no jobs directory found at %s", jobsDir)
	}

	// Build variable map for substitution.
	envFile := filepath.Join(dir, ".env")
	vars, err := envsubst.BuildVarMap(syncVars, envFile)
	if err != nil {
		return err
	}

	// Read YAML job files.
	incoming, notifyConfigs, err := readJobFiles(jobsDir, vars)
	if err != nil {
		return err
	}
	if len(incoming) == 0 {
		fmt.Printf("No job files found in %s\n", jobsDir)
		return nil
	}

	// Load global notification defaults.
	globalCfg, err := config.Load()
	if err != nil {
		globalCfg = &config.Config{}
	}

	// Resolve backend.
	b, err := resolveBackend(syncServer)
	if err != nil {
		return err
	}
	defer b.Close()

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

	// Write per-job notification configs (local only).
	if syncServer == "" {
		if err := syncNotifyConfigs(incoming, notifyConfigs, globalCfg.Notifications); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write notification configs: %v\n", err)
		}
	}

	fmt.Printf("Synced: %d added, %d updated, %d unchanged\n", added, updated, unchanged)
	return nil
}

// readJobFiles reads all .yaml files from dir and returns them as cron.Jobs.
// The filename (minus .yaml) is used as the job ID.
// If vars is non-nil, ${VAR} references in file content are substituted.
// It also returns per-job notification configs keyed by job ID.
func readJobFiles(dir string, vars map[string]string) ([]cron.Job, map[string]jobNotifyConfig, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, nil, err
	}

	var jobs []cron.Job
	notifyConfigs := make(map[string]jobNotifyConfig)
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".yaml")
		if err := cron.ValidateID(id); err != nil {
			return nil, nil, fmt.Errorf("invalid job file %s: %w", filepath.Base(f), err)
		}

		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s: %w", filepath.Base(f), err)
		}

		content := string(data)
		if vars != nil {
			content, err = envsubst.Substitute(content, vars)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", filepath.Base(f), err)
			}
		}

		var jf jobFile
		if err := yaml.Unmarshal([]byte(content), &jf); err != nil {
			return nil, nil, fmt.Errorf("parsing %s: %w", filepath.Base(f), err)
		}

		if jf.Name == "" {
			return nil, nil, fmt.Errorf("%s: name is required", filepath.Base(f))
		}
		if jf.Schedule == "" {
			return nil, nil, fmt.Errorf("%s: schedule is required", filepath.Base(f))
		}
		if jf.Command == "" {
			return nil, nil, fmt.Errorf("%s: command is required", filepath.Base(f))
		}

		cronExpr := cron.HumanToCron(jf.Schedule)
		if err := cron.ValidateCron(cronExpr); err != nil {
			return nil, nil, fmt.Errorf("%s: invalid schedule %q: %w", filepath.Base(f), jf.Schedule, err)
		}

		jobs = append(jobs, yamlToJob(id, cronExpr, jf))

		// Store per-job notification config if any field is explicitly set.
		if jf.OnFailure != nil || jf.OnSuccess != nil {
			jnc := jobNotifyConfig{}
			if jf.OnFailure != nil {
				jnc.Config.OnFailure = *jf.OnFailure
				jnc.OnFailureExplicit = true
			}
			if jf.OnSuccess != nil {
				jnc.Config.OnSuccess = *jf.OnSuccess
				jnc.OnSuccessExplicit = true
			}
			notifyConfigs[id] = jnc
		}
	}

	return jobs, notifyConfigs, nil
}

func yamlToJob(id, schedule string, jf jobFile) cron.Job {
	enabled := true
	if jf.Enabled != nil {
		enabled = *jf.Enabled
	}
	return cron.Job{
		ID:       id,
		Name:     jf.Name,
		Schedule: schedule,
		Command:  jf.Command,
		Enabled:  enabled,
		Wrapped:  true,
		Tag:      jf.Tag,
		TagColor: jf.TagColor,
		OneShot:  jf.Once,
		Project:  jf.Project,
	}
}

// mergeJobs merges incoming jobs into existing jobs by ID.
// Jobs in existing that are not in incoming are preserved unchanged.
func mergeJobs(existing, incoming []cron.Job) (merged []cron.Job, added, updated, unchanged int) {
	// Index existing jobs by ID.
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

// syncNotifyConfigs writes per-job notification config files.
// Per-job settings are merged with global defaults field-by-field.
// If a job explicitly sets on_failure or on_success, that field overrides
// the global default for that field only. This allows jobs to override
// individual notification types while preserving others from the global config.
func syncNotifyConfigs(jobs []cron.Job, perJob map[string]jobNotifyConfig, global config.NotificationConfig) error {
	globalNotify := notify.Config{
		OnFailure: configActionsToNotify(global.OnFailure),
		OnSuccess: configActionsToNotify(global.OnSuccess),
	}

	for _, j := range jobs {
		// Start with global defaults.
		cfg := globalNotify

		// Merge per-job config with global defaults field-by-field.
		if jobCfg, ok := perJob[j.ID]; ok {
			// Override only the fields that were explicitly set in the job YAML.
			if jobCfg.OnFailureExplicit {
				cfg.OnFailure = jobCfg.Config.OnFailure
			}
			if jobCfg.OnSuccessExplicit {
				cfg.OnSuccess = jobCfg.Config.OnSuccess
			}
		}

		if err := notify.WriteJobConfig(j.ID, j.Schedule, cfg); err != nil {
			return err
		}
	}
	return nil
}

func configActionsToNotify(actions []config.NotificationAction) []notify.Action {
	result := make([]notify.Action, len(actions))
	for i, a := range actions {
		result[i] = notify.Action{
			Type: a.Type,
			URL:  a.URL,
			Run:  a.Run,
		}
	}
	return result
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
