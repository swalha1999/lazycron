package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Preview what sync would change without writing to the crontab",
	Long:  "Shows a dry-run diff of what `lazycron sync` would add or update in the crontab.",
	RunE:  runDiff,
}

var (
	diffProject  string
	diffQuiet    bool
	diffExitCode bool
)

func init() {
	diffCmd.Flags().StringVarP(&globalServer, "server", "s", "", "target server name from config")
	diffCmd.Flags().StringVar(&diffProject, "project", "", "project name (overrides .lazycron/config.yaml)")
	diffCmd.Flags().BoolVarP(&diffQuiet, "quiet", "q", false, "only show changes, hide unchanged jobs")
	diffCmd.Flags().BoolVar(&diffExitCode, "exit-code", false, "exit with code 1 if there are changes")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	sctx, err := loadSyncContext(globalServer, diffProject)
	if err != nil {
		return err
	}
	if sctx == nil {
		return nil
	}
	defer sctx.Backend.Close()

	changes := computeDiff(sctx.ExistingJobs, sctx.IncomingJobs)
	if err := renderDiff(cmd, changes, diffQuiet); err != nil {
		return err
	}

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
	diffDeleted
)

// String returns the kind's JSON name.
func (k diffKind) String() string {
	return [...]string{"new", "updated", "unchanged", "deleted"}[k]
}

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

// diffEntryView is the JSON shape of one diff entry.
type diffEntryView struct {
	Kind    string           `json:"kind"`
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Changes []cron.FieldDiff `json:"changes,omitempty"`
}

// diffSummary is the JSON shape of a whole diff.
type diffSummary struct {
	Entries   []diffEntryView `json:"entries"`
	New       int             `json:"new"`
	Updated   int             `json:"updated"`
	Deleted   int             `json:"deleted"`
	Unchanged int             `json:"unchanged"`
	Changed   bool            `json:"changed"`
}

func summarizeDiff(entries []diffEntry) diffSummary {
	sum := diffSummary{Entries: make([]diffEntryView, 0, len(entries))}
	for _, e := range entries {
		sum.Entries = append(sum.Entries, diffEntryView{Kind: e.Kind.String(), ID: e.Job.ID, Name: e.Name, Changes: e.Changes})
		switch e.Kind {
		case diffNew:
			sum.New++
		case diffUpdated:
			sum.Updated++
		case diffDeleted:
			sum.Deleted++
		case diffUnchanged:
			sum.Unchanged++
		}
	}
	sum.Changed = sum.New+sum.Updated+sum.Deleted > 0
	return sum
}

// renderDiff prints a diff as JSON or as the +/~/- text listing.
func renderDiff(cmd *cobra.Command, entries []diffEntry, quiet bool) error {
	return render(cmd, summarizeDiff(entries), func(w io.Writer) error {
		printDiff(w, entries, quiet)
		return nil
	})
}

func printDiff(w io.Writer, entries []diffEntry, quiet bool) {
	for _, e := range entries {
		switch e.Kind {
		case diffNew:
			fmt.Fprintf(w, "+ %-20s %q   %s\n", e.Name, e.Job.Schedule, e.Job.Command)
		case diffUpdated:
			parts := make([]string, 0, len(e.Changes))
			for _, c := range e.Changes {
				parts = append(parts, fmt.Sprintf("%s: %s → %s", c.Field, c.Old, c.New))
			}
			fmt.Fprintf(w, "~ %-20s %s\n", e.Name, strings.Join(parts, ", "))
		case diffDeleted:
			fmt.Fprintf(w, "- %-20s %q   %s\n", e.Name, e.Job.Schedule, e.Job.Command)
		case diffUnchanged:
			if !quiet {
				fmt.Fprintf(w, "  %-20s (unchanged)\n", e.Name)
			}
		}
	}

	sum := summarizeDiff(entries)
	total := sum.New + sum.Updated + sum.Deleted
	if total == 0 {
		fmt.Fprintf(w, "\nNo changes (%d unchanged)\n", sum.Unchanged)
		return
	}
	fmt.Fprintf(w, "\n%d changes (%d new, %d updated, %d deleted, %d unchanged)\n", total, sum.New, sum.Updated, sum.Deleted, sum.Unchanged)
}

func hasChanges(entries []diffEntry) bool {
	for _, e := range entries {
		if e.Kind != diffUnchanged {
			return true
		}
	}
	return false
}
