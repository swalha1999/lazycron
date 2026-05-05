package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Preview what sync would change without writing to the crontab",
	Long:  "Shows a dry-run diff of what `lazycron sync` would add or update in the crontab.",
	RunE:  runDiff,
}

var (
	diffServer   string
	diffProject  string
	diffQuiet    bool
	diffExitCode bool
)

func init() {
	diffCmd.Flags().StringVarP(&diffServer, "server", "s", "", "target server name from config")
	diffCmd.Flags().StringVar(&diffProject, "project", "", "project name (overrides .lazycron/config.yaml)")
	diffCmd.Flags().BoolVarP(&diffQuiet, "quiet", "q", false, "only show changes, hide unchanged jobs")
	diffCmd.Flags().BoolVar(&diffExitCode, "exit-code", false, "exit with code 1 if there are changes")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	sandcastleDir := filepath.Join(cwd, ".sandcastle")
	jobsDir := filepath.Join(sandcastleDir, "jobs")
	if info, err := os.Stat(jobsDir); err != nil || !info.IsDir() {
		if _, scErr := os.Stat(sandcastleDir); scErr != nil {
			return fmt.Errorf("no .sandcastle/ directory found at %s — run `lazycron init --with-agents` first", sandcastleDir)
		}
		return fmt.Errorf("no .sandcastle/jobs/ directory found at %s — run `lazycron templates apply <name>` to scaffold an agent", jobsDir)
	}

	pcfg, err := config.LoadProjectConfig(filepath.Join(cwd, ".lazycron"))
	if err != nil {
		return err
	}
	projectName := config.ResolveProjectName(diffProject, pcfg, cwd)

	incoming, err := readSandcastleJobs(jobsDir, projectName)
	if err != nil {
		return err
	}
	if len(incoming) == 0 {
		fmt.Printf("No .ts files found in %s\n", jobsDir)
		return nil
	}

	// Apply the same project-cd wrap that sync would, so the comparison is fair.
	projectDir := remoteProjectPath(diffServer, projectName, cwd)
	for i := range incoming {
		incoming[i].Command = fmt.Sprintf("cd %s && %s", shellQuoteSingle(projectDir), incoming[i].Command)
	}

	b, err := resolveBackend(diffServer)
	if err != nil {
		return err
	}
	defer b.Close()

	existing, err := b.ReadJobs()
	if err != nil {
		return fmt.Errorf("failed to read jobs: %w", err)
	}

	changes := computeDiff(existing, incoming)
	printDiff(changes, diffQuiet)

	if diffExitCode && hasChanges(changes) {
		os.Exit(1)
	}

	return nil
}

// diffKind represents the type of change for a job.
type diffKind int

const (
	diffNew diffKind = iota
	diffUpdated
	diffUnchanged
)

// fieldChange describes a single field that changed between existing and incoming.
type fieldChange struct {
	Field string
	Old   string
	New   string
}

// diffEntry holds the diff information for a single job.
type diffEntry struct {
	Kind    diffKind
	Name    string
	Job     cron.Job
	Changes []fieldChange
}

// computeDiff compares existing crontab jobs with incoming TS-derived jobs.
func computeDiff(existing, incoming []cron.Job) []diffEntry {
	existingByID := make(map[string]cron.Job, len(existing))
	for _, j := range existing {
		if j.ID != "" {
			existingByID[j.ID] = j
		}
	}

	var entries []diffEntry
	for _, inc := range incoming {
		if ex, ok := existingByID[inc.ID]; ok {
			if jobNeedsUpdate(ex, inc) {
				entries = append(entries, diffEntry{
					Kind:    diffUpdated,
					Name:    inc.Name,
					Job:     inc,
					Changes: diffFields(ex, inc),
				})
			} else {
				entries = append(entries, diffEntry{
					Kind: diffUnchanged,
					Name: inc.Name,
					Job:  inc,
				})
			}
		} else {
			entries = append(entries, diffEntry{
				Kind: diffNew,
				Name: inc.Name,
				Job:  inc,
			})
		}
	}

	return entries
}

// diffFields returns the list of fields that differ between two jobs.
func diffFields(old, new cron.Job) []fieldChange {
	var changes []fieldChange
	if old.Name != new.Name {
		changes = append(changes, fieldChange{"name", old.Name, new.Name})
	}
	if old.Schedule != new.Schedule {
		changes = append(changes, fieldChange{"schedule", fmt.Sprintf("%q", old.Schedule), fmt.Sprintf("%q", new.Schedule)})
	}
	if old.Command != new.Command {
		changes = append(changes, fieldChange{"command", old.Command, new.Command})
	}
	if old.Enabled != new.Enabled {
		changes = append(changes, fieldChange{"enabled", fmt.Sprintf("%v", old.Enabled), fmt.Sprintf("%v", new.Enabled)})
	}
	if old.Tag != new.Tag {
		changes = append(changes, fieldChange{"tag", old.Tag, new.Tag})
	}
	if old.TagColor != new.TagColor {
		changes = append(changes, fieldChange{"tag_color", old.TagColor, new.TagColor})
	}
	if old.Project != new.Project {
		changes = append(changes, fieldChange{"project", old.Project, new.Project})
	}
	if old.OneShot != new.OneShot {
		changes = append(changes, fieldChange{"once", fmt.Sprintf("%v", old.OneShot), fmt.Sprintf("%v", new.OneShot)})
	}
	return changes
}

func printDiff(entries []diffEntry, quiet bool) {
	var added, updated, unchanged int
	var lines []string

	for _, e := range entries {
		switch e.Kind {
		case diffNew:
			added++
			lines = append(lines, fmt.Sprintf("+ %-20s %q   %s", e.Name, e.Job.Schedule, e.Job.Command))
		case diffUpdated:
			updated++
			parts := make([]string, 0, len(e.Changes))
			for _, c := range e.Changes {
				parts = append(parts, fmt.Sprintf("%s: %s → %s", c.Field, c.Old, c.New))
			}
			lines = append(lines, fmt.Sprintf("~ %-20s %s", e.Name, strings.Join(parts, ", ")))
		case diffUnchanged:
			unchanged++
			if !quiet {
				lines = append(lines, fmt.Sprintf("  %-20s (unchanged)", e.Name))
			}
		}
	}

	for _, l := range lines {
		fmt.Println(l)
	}

	total := added + updated
	if total == 0 && unchanged > 0 {
		fmt.Printf("\nNo changes (%d unchanged)\n", unchanged)
	} else {
		fmt.Printf("\n%d changes (%d new, %d updated, %d unchanged)\n", total, added, updated, unchanged)
	}
}

func hasChanges(entries []diffEntry) bool {
	for _, e := range entries {
		if e.Kind != diffUnchanged {
			return true
		}
	}
	return false
}
