package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
)

// Mutation actions reported by job commands.
const (
	actionCreated   = "created"
	actionUpdated   = "updated"
	actionDeleted   = "deleted"
	actionUnchanged = "unchanged"
)

// dryRun backs the --dry-run flag of every mutating command.
var dryRun bool

func addDryRunFlag(c *cobra.Command) {
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without writing anything")
}

// mutationResult is the JSON result of a single-job mutation.
type mutationResult struct {
	Action  string           `json:"action"`
	DryRun  bool             `json:"dry_run"`
	Server  string           `json:"server"`
	Job     jobView          `json:"job"`
	Changes []cron.FieldDiff `json:"changes,omitempty"`
}

// jobMutation edits the job list in place and describes what it did. It
// returns the new list (which may be the same slice).
type jobMutation func(jobs []cron.Job) ([]cron.Job, mutationResult, error)

// mutateJobs reads the selected backend's jobs, applies fn, writes the
// result unless nothing changed or --dry-run is set, and prints the result.
func mutateJobs(cmd *cobra.Command, fn jobMutation) error {
	return withBackend(func(b backend.Backend) error {
		jobs, err := b.ReadJobs()
		if err != nil {
			return errUnavailable("read jobs: %v", err)
		}
		updated, res, err := fn(jobs)
		if err != nil {
			return err
		}
		res.DryRun = dryRun
		res.Server = serverLabel()
		if res.Action != actionUnchanged && !dryRun {
			if err := b.WriteJobs(updated); err != nil {
				return errFailure("write jobs: %v", err)
			}
		}
		return render(cmd, res, func(w io.Writer) error {
			printMutation(w, res)
			return nil
		})
	})
}

func printMutation(w io.Writer, res mutationResult) {
	prefix := ""
	if res.DryRun {
		prefix = "[dry-run] would have "
	}
	fmt.Fprintf(w, "%s%s job %q (%s)\n", prefix, res.Action, res.Job.Name, res.Job.ID)
	for _, c := range res.Changes {
		fmt.Fprintf(w, "  %s: %s -> %s\n", c.Field, c.Old, c.New)
	}
}

// updateJob returns a mutation that changes the job ref resolves to with
// change, reporting the field-level diff (or unchanged).
func updateJob(ref string, change func(j *cron.Job) error) jobMutation {
	return func(jobs []cron.Job) ([]cron.Job, mutationResult, error) {
		i, err := findJob(jobs, ref)
		if err != nil {
			return nil, mutationResult{}, err
		}
		before := jobs[i]
		after := before
		if err := change(&after); err != nil {
			return nil, mutationResult{}, err
		}
		jobs[i] = after
		res := mutationResult{Action: actionUnchanged, Job: toJobView(after)}
		if diffs := cron.DiffJob(before, after); len(diffs) > 0 {
			res.Action = actionUpdated
			res.Changes = diffs
		}
		return jobs, res, nil
	}
}
