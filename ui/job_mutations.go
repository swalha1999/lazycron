package ui

import (
	"errors"
	"fmt"
	"sync"

	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

// jobMutation edits a freshly read job list. It finds jobs by ID, never by
// an index into the TUI's (possibly stale) m.jobs, so jobs added or changed
// outside the TUI survive the write.
type jobMutation func(jobs []cron.Job) ([]cron.Job, error)

// errNoChange tells applyMutation that nothing needs to be written.
var errNoChange = errors.New("no change")

// writeMu serialises the TUI's read-modify-write cycles so two quick edits
// (each run in its own goroutine) can't overwrite one another.
var writeMu sync.Mutex

// applyMutation reads b's current jobs, applies fn and writes the result,
// returning the jobs as written. When fn fails it returns the jobs as read,
// and when reading or writing fails it returns nil jobs.
func applyMutation(b backend.Backend, fn jobMutation) ([]cron.Job, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	jobs, err := b.ReadJobs()
	if err != nil {
		return nil, err
	}
	updated, err := fn(jobs)
	if err != nil {
		return jobs, err
	}
	if err := b.WriteJobs(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func indexOfJob(jobs []cron.Job, id string) (int, error) {
	for i := range jobs {
		if jobs[i].ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf("job %s no longer exists (changed outside lazycron?)", id)
}

// editJob applies change to the job with the given ID.
func editJob(id string, change func(j *cron.Job)) jobMutation {
	return func(jobs []cron.Job) ([]cron.Job, error) {
		i, err := indexOfJob(jobs, id)
		if err != nil {
			return nil, err
		}
		change(&jobs[i])
		return jobs, nil
	}
}

func setJobEnabled(id string, enabled bool) jobMutation {
	return editJob(id, func(j *cron.Job) { j.Enabled = enabled })
}

func setJobProject(id, project string) jobMutation {
	return editJob(id, func(j *cron.Job) { j.Project = project })
}

func markJobWrapped(id string) jobMutation {
	return editJob(id, func(j *cron.Job) { j.Wrapped = true })
}

// replaceJob swaps in an edited job, keeping its current enabled state.
func replaceJob(job cron.Job) jobMutation {
	return editJob(job.ID, func(j *cron.Job) {
		enabled := j.Enabled
		*j = job
		j.Enabled = enabled
	})
}

func appendJob(job cron.Job) jobMutation {
	return func(jobs []cron.Job) ([]cron.Job, error) {
		return append(jobs, job), nil
	}
}

func deleteJob(id string) jobMutation {
	return func(jobs []cron.Job) ([]cron.Job, error) {
		i, err := indexOfJob(jobs, id)
		if err != nil {
			return nil, err
		}
		return append(jobs[:i], jobs[i+1:]...), nil
	}
}

// swapJobs exchanges the positions of two jobs (used to reorder).
func swapJobs(idA, idB string) jobMutation {
	return func(jobs []cron.Job) ([]cron.Job, error) {
		a, err := indexOfJob(jobs, idA)
		if err != nil {
			return nil, err
		}
		b, err := indexOfJob(jobs, idB)
		if err != nil {
			return nil, err
		}
		jobs[a], jobs[b] = jobs[b], jobs[a]
		return jobs, nil
	}
}

// disableRanOneShots disables enabled one-shot jobs that already have a
// history entry, reporting how many through count. It returns errNoChange
// when there is nothing to disable.
func disableRanOneShots(hist []history.Entry, count *int) jobMutation {
	ran := ranJobKeys(hist)
	return func(jobs []cron.Job) ([]cron.Job, error) {
		for i := range jobs {
			if jobs[i].OneShot && jobs[i].Enabled && ran[jobs[i].ID] {
				jobs[i].Enabled = false
				*count++
			}
		}
		if *count == 0 {
			return nil, errNoChange
		}
		return jobs, nil
	}
}

// ranJobKeys returns the job IDs (or names, for old entries) found in hist.
func ranJobKeys(hist []history.Entry) map[string]bool {
	ran := make(map[string]bool, len(hist))
	for _, e := range hist {
		if e.JobID != "" {
			ran[e.JobID] = true
		} else {
			ran[e.JobName] = true
		}
	}
	return ran
}
