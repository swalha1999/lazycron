package cmd

import (
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new cron job",
	Long: "Add a new cron job non-interactively with flags. Idempotent: adding a job " +
		"identical to an existing one (same id, or same name without --id) changes " +
		"nothing; a different job under the same id or name is a conflict (exit 4).",
	Args: cobra.NoArgs,
	RunE: runAdd,
}

var addSpec struct {
	jobSpec
	disabled bool
}

func init() {
	f := addCmd.Flags()
	f.StringVar(&addSpec.ID, "id", "", "job id (default: random; [a-z0-9_-], stable across edits)")
	f.StringVarP(&addSpec.Name, "name", "n", "", "Job name (required)")
	f.StringVarP(&addSpec.Schedule, "schedule", "s", "", "Cron schedule or human-readable (required)")
	f.StringVarP(&addSpec.Command, "command", "c", "", "Command to run (required)")
	f.StringVarP(&addSpec.Workdir, "workdir", "w", "", "Working directory (optional)")
	f.StringVarP(&addSpec.Project, "project", "p", "", "Project group (optional)")
	f.StringVar(&addSpec.Tag, "tag", "", "short colored tag shown after the name")
	f.StringVar(&addSpec.TagColor, "tag-color", "", "tag color, e.g. #f38ba8")
	f.StringVar(&addSpec.Runner, "runner", "", "plain or herdr (default: config runner)")
	f.BoolVar(&addSpec.disabled, "disabled", false, "Create the job in disabled state")
	f.BoolVar(&addSpec.Once, "once", false, "Run once at the specified datetime and then disable")
	addDryRunFlag(addCmd)

	addCmd.MarkFlagRequired("name")
	addCmd.MarkFlagRequired("schedule")
	addCmd.MarkFlagRequired("command")

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	spec := addSpec.jobSpec
	enabled := !addSpec.disabled
	spec.Enabled = &enabled
	if spec.ID == "" {
		spec.ID = cron.GenerateID()
	}
	job, err := spec.toJob()
	if err != nil {
		return err
	}
	explicitID := addSpec.ID != ""
	return mutateJobs(cmd, func(jobs []cron.Job) ([]cron.Job, mutationResult, error) {
		if existing, ok := sameJob(jobs, job, explicitID); ok {
			if cron.JobsDiffer(existing, withID(job, existing.ID)) {
				return nil, mutationResult{}, errConflict("job %q (%s) already exists with different settings", existing.Name, existing.ID).
					withHint("change it with `lazycron edit %s`, or pick another --id/--name", existing.ID)
			}
			return jobs, mutationResult{Action: actionUnchanged, Job: toJobView(existing)}, nil
		}
		return append(jobs, job), mutationResult{Action: actionCreated, Job: toJobView(job)}, nil
	})
}

// sameJob finds the job an add would duplicate: by id when one was given,
// otherwise by name.
func sameJob(jobs []cron.Job, job cron.Job, byID bool) (cron.Job, bool) {
	for _, j := range jobs {
		if (byID && j.ID == job.ID) || (!byID && j.Name == job.Name) {
			return j, true
		}
	}
	return cron.Job{}, false
}

func withID(j cron.Job, id string) cron.Job {
	j.ID = id
	return j
}
