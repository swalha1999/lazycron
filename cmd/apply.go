package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"gopkg.in/yaml.v3"
)

var applyCmd = &cobra.Command{
	Use:   "apply <file>",
	Short: "Make jobs match a declarative jobs file (YAML or JSON) and report the diff",
	Long: "Creates and updates jobs so they match the file, matched by id (derived from " +
		"the name when omitted). With --prune, jobs in scope that the file does not list " +
		"are deleted; the scope is the file's top-level project, or all jobs without one. " +
		"Running it twice changes nothing the second time. `-` reads stdin.",
	Args: cobra.ExactArgs(1),
	RunE: runApply,
}

var applyPrune bool

func init() {
	applyCmd.Flags().BoolVar(&applyPrune, "prune", false, "delete jobs in scope that the file does not list")
	addDryRunFlag(applyCmd)
	rootCmd.AddCommand(applyCmd)
}

// jobsFile is the declarative format read by apply and written by export.
type jobsFile struct {
	Project string    `yaml:"project,omitempty" json:"project,omitempty"`
	Jobs    []jobSpec `yaml:"jobs" json:"jobs"`
}

// applyResult is the JSON result of apply.
type applyResult struct {
	diffSummary
	DryRun bool   `json:"dry_run"`
	Server string `json:"server"`
}

func runApply(cmd *cobra.Command, args []string) error {
	desired, project, err := loadJobsFile(args[0])
	if err != nil {
		return err
	}
	return withBackend(func(b backend.Backend) error {
		existing, err := b.ReadJobs()
		if err != nil {
			return errUnavailable("read jobs: %v", err)
		}
		merged, entries := planApply(existing, desired, project, applyPrune)
		if hasChanges(entries) && !dryRun {
			if err := b.WriteJobs(merged); err != nil {
				return errFailure("write jobs: %v", err)
			}
		}
		res := applyResult{diffSummary: summarizeDiff(entries), DryRun: dryRun, Server: serverLabel()}
		return render(cmd, res, func(w io.Writer) error {
			if dryRun {
				fmt.Fprintln(w, "[dry-run] nothing was written")
			}
			printDiff(w, entries, false)
			return nil
		})
	})
}

// loadJobsFile reads and validates a jobs file; path "-" means stdin.
func loadJobsFile(path string) ([]cron.Job, string, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, "", errNotFound("read %s: %v", path, err)
	}
	var f jobsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, "", errUsage("parse %s: %v", path, err)
	}
	seen := map[string]bool{}
	jobs := make([]cron.Job, 0, len(f.Jobs))
	for _, spec := range f.Jobs {
		if spec.Project == "" {
			spec.Project = f.Project
		}
		job, err := spec.toJob()
		if err != nil {
			return nil, "", err
		}
		if seen[job.ID] {
			return nil, "", errUsage("job id %q appears twice in %s", job.ID, path)
		}
		seen[job.ID] = true
		jobs = append(jobs, job)
	}
	return jobs, f.Project, nil
}

// planApply computes the job list after applying desired to existing, and
// the per-job diff. Existing order is kept; new jobs are appended.
func planApply(existing, desired []cron.Job, project string, prune bool) ([]cron.Job, []diffEntry) {
	entries := computeDiff(existing, desired)
	want := make(map[string]cron.Job, len(desired))
	for _, j := range desired {
		want[j.ID] = j
	}
	var merged []cron.Job
	for _, j := range existing {
		if d, ok := want[j.ID]; ok {
			merged = append(merged, d)
			delete(want, j.ID)
			continue
		}
		if prune && (project == "" || j.Project == project) {
			entries = append(entries, diffEntry{Kind: diffDeleted, Name: j.Name, Job: j})
			continue
		}
		merged = append(merged, j)
	}
	for _, j := range desired {
		if _, isNew := want[j.ID]; isNew {
			merged = append(merged, j)
		}
	}
	return merged, entries
}
