package cmd

import (
	"fmt"

	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
)

// jobSpec is a job as a user or agent declares it: in `add`/`edit` flags or
// an `apply` file. Schedules may be cron expressions or human-readable.
type jobSpec struct {
	ID       string `yaml:"id,omitempty" json:"id,omitempty"`
	Name     string `yaml:"name" json:"name"`
	Schedule string `yaml:"schedule" json:"schedule"`
	Command  string `yaml:"command" json:"command"`
	Workdir  string `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	Enabled  *bool  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Once     bool   `yaml:"once,omitempty" json:"once,omitempty"`
	Tag      string `yaml:"tag,omitempty" json:"tag,omitempty"`
	TagColor string `yaml:"tag_color,omitempty" json:"tag_color,omitempty"`
	Project  string `yaml:"project,omitempty" json:"project,omitempty"`
	Runner   string `yaml:"runner,omitempty" json:"runner,omitempty"`
}

// parseSchedule turns a human or cron schedule into a validated cron
// expression. With once, the schedule is a datetime for a one-shot job.
func parseSchedule(schedule string, once bool) (string, error) {
	if once {
		if cron.ValidateCron(schedule) == nil {
			return schedule, nil // already pinned to a date (e.g. from export)
		}
		expr, _, err := cron.DatetimeToCron(schedule)
		if err != nil {
			return "", errUsage("invalid datetime %q: %v", schedule, err).withHint(`use e.g. "2026-12-01 09:00" or "tomorrow 9am"`)
		}
		return expr, nil
	}
	expr := cron.HumanToCron(schedule)
	if err := cron.ValidateCron(expr); err != nil {
		return "", errUsage("invalid schedule %q: %v", schedule, err).withHint(`use a cron expression ("0 2 * * *") or e.g. "every day at 2am"`)
	}
	return expr, nil
}

// composeCommand prefixes command with `cd <workdir> && ` when workdir is set.
func composeCommand(command, workdir string) string {
	if workdir == "" {
		return command
	}
	return fmt.Sprintf("cd %s && %s", cron.ShellQuote(workdir), command)
}

// validateRunner rejects unknown runner names.
func validateRunner(runner string) error {
	if !config.ValidRunner(runner) {
		return errUsage("unknown runner %q", runner).withHint("use %q or %q (or omit for the default)", config.RunnerPlain, config.RunnerHerdr)
	}
	return nil
}

// toJob validates the spec and builds the crontab job it describes.
func (s jobSpec) toJob() (cron.Job, error) {
	if s.Name == "" || s.Schedule == "" || s.Command == "" {
		return cron.Job{}, errUsage("job %q needs name, schedule and command", s.ID)
	}
	id := s.ID
	if id == "" {
		id = cron.SlugID(s.Name)
	}
	if err := cron.ValidateID(id); err != nil {
		return cron.Job{}, errUsage("%v", err).withHint("set an explicit id")
	}
	if err := validateRunner(s.Runner); err != nil {
		return cron.Job{}, err
	}
	expr, err := parseSchedule(s.Schedule, s.Once)
	if err != nil {
		return cron.Job{}, err
	}
	enabled := s.Enabled == nil || *s.Enabled
	return cron.Job{
		ID:       id,
		Name:     s.Name,
		Schedule: expr,
		Command:  composeCommand(s.Command, s.Workdir),
		Enabled:  enabled,
		Wrapped:  true,
		OneShot:  s.Once,
		Tag:      s.Tag,
		TagColor: s.TagColor,
		Project:  s.Project,
		Runner:   s.Runner,
	}, nil
}

// specFromJob is the inverse of toJob, used by `export`.
func specFromJob(j cron.Job) jobSpec {
	enabled := j.Enabled
	command, workdir := cron.StripProjectCd(j.Command)
	return jobSpec{
		ID:       j.ID,
		Name:     j.Name,
		Schedule: j.Schedule,
		Command:  command,
		Workdir:  workdir,
		Enabled:  &enabled,
		Once:     j.OneShot,
		Tag:      j.Tag,
		TagColor: j.TagColor,
		Project:  j.Project,
		Runner:   j.Runner,
	}
}
