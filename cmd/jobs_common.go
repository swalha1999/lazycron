package cmd

import (
	"strings"
	"time"

	"github.com/swalha1999/lazycron/cron"
)

// jobView is the stable JSON shape of a job.
type jobView struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Schedule      string   `json:"schedule"`
	ScheduleHuman string   `json:"schedule_human"`
	Command       string   `json:"command"`
	Enabled       bool     `json:"enabled"`
	Once          bool     `json:"once"`
	Tag           string   `json:"tag,omitempty"`
	TagColor      string   `json:"tag_color,omitempty"`
	Project       string   `json:"project,omitempty"`
	Runner        string   `json:"runner"`
	RunnerSet     bool     `json:"runner_override"`
	CurrentFormat bool     `json:"current_format"`
	NextRuns      []string `json:"next_runs,omitempty"`
}

func toJobView(j cron.Job) jobView {
	v := jobView{
		ID:            j.ID,
		Name:          j.Name,
		Schedule:      j.Schedule,
		ScheduleHuman: cron.CronToHuman(j.Schedule),
		Command:       j.Command,
		Enabled:       j.Enabled,
		Once:          j.OneShot,
		Tag:           j.Tag,
		TagColor:      j.TagColor,
		Project:       j.Project,
		Runner:        j.EffectiveRunner(),
		RunnerSet:     j.Runner != "",
		CurrentFormat: j.Wrapped,
	}
	if j.Enabled {
		for _, t := range cron.NextRuns(j.Schedule, 3) {
			v.NextRuns = append(v.NextRuns, t.Format(time.RFC3339))
		}
	}
	return v
}

func toJobViews(jobs []cron.Job) []jobView {
	views := make([]jobView, 0, len(jobs))
	for _, j := range jobs {
		views = append(views, toJobView(j))
	}
	return views
}

// findJob resolves ref to a job index: exact ID first, then exact name, then
// case-insensitive name. Ambiguous names are a conflict.
func findJob(jobs []cron.Job, ref string) (int, error) {
	for i := range jobs {
		if jobs[i].ID == ref {
			return i, nil
		}
	}
	for _, match := range []func(string) bool{
		func(name string) bool { return name == ref },
		func(name string) bool { return strings.EqualFold(name, ref) },
	} {
		var hits []int
		for i := range jobs {
			if match(jobs[i].Name) {
				hits = append(hits, i)
			}
		}
		switch len(hits) {
		case 0:
			continue
		case 1:
			return hits[0], nil
		default:
			ids := make([]string, len(hits))
			for k, i := range hits {
				ids[k] = jobs[i].ID
			}
			return -1, errConflict("%d jobs are named %q", len(hits), ref).withHint("use a job id: %s", strings.Join(ids, ", "))
		}
	}
	return -1, errNotFound("job %q not found", ref).withHint("list jobs with `lazycron ls`")
}
