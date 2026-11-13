package cmd

import (
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
)

var editCmd = &cobra.Command{
	Use:   "edit <job>",
	Short: "Change fields of an existing job (only the flags you pass)",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

var editSpec jobSpec

func init() {
	f := editCmd.Flags()
	f.StringVarP(&editSpec.Name, "name", "n", "", "new name")
	f.StringVarP(&editSpec.Schedule, "schedule", "s", "", "new schedule (cron or human-readable)")
	f.StringVarP(&editSpec.Command, "command", "c", "", "new command")
	f.StringVarP(&editSpec.Workdir, "workdir", "w", "", "working directory for the new --command")
	f.StringVarP(&editSpec.Project, "project", "p", "", "project group (\"\" clears)")
	f.StringVar(&editSpec.Tag, "tag", "", "tag (\"\" clears)")
	f.StringVar(&editSpec.TagColor, "tag-color", "", "tag color")
	f.StringVar(&editSpec.Runner, "runner", "", "plain, herdr, or \"\" for the config default")
	f.BoolVar(&editSpec.Once, "once", false, "make it a one-shot job (--once=false: recurring)")
	addDryRunFlag(editCmd)
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	changed := cmd.Flags().Changed
	if changed("workdir") && !changed("command") {
		return errUsage("--workdir needs --command").withHint("pass the full command again with --command")
	}
	if changed("runner") {
		if err := validateRunner(editSpec.Runner); err != nil {
			return err
		}
	}
	return mutateJobs(cmd, updateJob(args[0], func(j *cron.Job) error {
		return applyEdits(j, changed)
	}))
}

// applyEdits copies the flags that were passed onto j.
func applyEdits(j *cron.Job, changed func(string) bool) error {
	if changed("once") {
		j.OneShot = editSpec.Once
	}
	if changed("schedule") {
		expr, err := parseSchedule(editSpec.Schedule, j.OneShot)
		if err != nil {
			return err
		}
		j.Schedule = expr
	}
	if changed("name") {
		if editSpec.Name == "" {
			return errUsage("--name cannot be empty")
		}
		j.Name = editSpec.Name
	}
	if changed("command") {
		if editSpec.Command == "" {
			return errUsage("--command cannot be empty")
		}
		j.Command = composeCommand(editSpec.Command, editSpec.Workdir)
	}
	stringFields := []struct {
		flag  string
		value string
		field *string
	}{
		{"project", editSpec.Project, &j.Project},
		{"tag", editSpec.Tag, &j.Tag},
		{"tag-color", editSpec.TagColor, &j.TagColor},
		{"runner", editSpec.Runner, &j.Runner},
	}
	for _, f := range stringFields {
		if changed(f.flag) {
			*f.field = f.value
		}
	}
	j.Wrapped = true
	return validateLabels(*j)
}
