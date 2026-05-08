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

	b, err := resolveBackend(diffServer)
	if err != nil {
		return err
	}
	defer b.Close()

	// Apply the same project-cd wrap that sync would, so the comparison is fair.
	projectDir, err := b.ProjectDir(projectName)
	if err != nil {
		return fmt.Errorf("resolve project dir: %w", err)
	}
	for i := range incoming {
		incoming[i].Command = fmt.Sprintf("cd %s && %s", shellQuoteSingle(projectDir), incoming[i].Command)
	}

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

// diffEntry holds the diff information for a single job.
type diffEntry struct {
	Kind    diffKind
	Name    string
	Job     cron.Job
	Changes []cron.FieldDiff
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
			if changes := cron.DiffJob(ex, inc); len(changes) > 0 {
				entries = append(entries, diffEntry{
					Kind:    diffUpdated,
					Name:    inc.Name,
					Job:     inc,
					Changes: changes,
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
