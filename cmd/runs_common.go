package cmd

import (
	"sort"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/runner"
)

// Run states.
const (
	stateRunning   = "running"
	stateSucceeded = "succeeded"
	stateFailed    = "failed"
	stateLost      = "lost"    // its supervising lazycron process died mid-run
	stateUnknown   = "unknown" // old history entry without a status
)

// runView is the stable JSON shape of a run.
type runView struct {
	ID              string `json:"id"`
	JobID           string `json:"job_id"`
	JobName         string `json:"job_name"`
	State           string `json:"state"`
	Runner          string `json:"runner,omitempty"`
	Trigger         string `json:"trigger,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	FinishedAt      string `json:"finished_at,omitempty"`
	DurationMS      *int64 `json:"duration_ms,omitempty"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	Signal          int    `json:"signal,omitempty"`
	Terminal        string `json:"terminal,omitempty"`
	TerminalName    string `json:"terminal_name,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
	FallbackReason  string `json:"fallback_reason,omitempty"`
	// Agent is the Claude session an agent run started.
	Agent  *history.AgentSession `json:"agent,omitempty"`
	Output *string               `json:"output,omitempty"`
}

// finished reports whether the run has ended (it will not change any more).
func (r runView) finished() bool {
	return r.State != stateRunning
}

func runFromEntry(e history.Entry) runView {
	v := runView{
		ID: e.RunID, JobID: e.JobID, JobName: e.JobName, Runner: e.Runner, Trigger: e.Trigger,
		StartedAt: e.StartedAt, FinishedAt: e.Timestamp, ExitCode: e.ExitCode, Signal: e.Signal,
		Terminal: e.Terminal, TerminalName: e.TerminalName, OutputTruncated: e.OutputTruncated,
		FallbackReason: e.FallbackReason, Agent: e.Agent,
	}
	switch {
	case e.Success == nil:
		v.State = stateUnknown
	case *e.Success:
		v.State = stateSucceeded
	default:
		v.State = stateFailed
	}
	if start, err := time.Parse(time.RFC3339, e.StartedAt); err == nil {
		if end, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			ms := end.Sub(start).Milliseconds()
			v.DurationMS = &ms
		}
	}
	return v
}

func runFromActive(a runner.Active) runView {
	v := runView{
		ID: a.RunID, JobID: a.JobID, JobName: a.JobName, State: stateRunning, Runner: a.Runner,
		Trigger: a.Trigger, StartedAt: a.StartedAt, Terminal: a.Terminal, TerminalName: a.TerminalName,
	}
	if a.Stale {
		v.State = stateLost
	}
	return v
}

// sortKey orders runs by start (or finish, for old entries) time.
func (r runView) sortKey() time.Time {
	for _, s := range []string{r.StartedAt, r.FinishedAt} {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// runRecords bundles everything known about runs on a backend.
type runRecords struct {
	views   []runView
	entries map[string]history.Entry
}

// loadRuns returns in-progress and finished runs on b, newest first,
// optionally limited to one job id.
func loadRuns(b backend.Backend, jobID string) (runRecords, error) {
	active, err := b.ActiveRuns()
	if err != nil {
		return runRecords{}, errUnavailable("read active runs: %v", err)
	}
	entries, err := b.LoadHistory()
	if err != nil {
		return runRecords{}, errUnavailable("read history: %v", err)
	}
	rec := runRecords{entries: make(map[string]history.Entry, len(entries))}
	for _, e := range entries {
		rec.entries[e.RunID] = e
		if jobID == "" || e.JobID == jobID {
			rec.views = append(rec.views, runFromEntry(e))
		}
	}
	for _, a := range active {
		if _, done := rec.entries[a.RunID]; done {
			continue // finished between the two reads
		}
		if jobID == "" || a.JobID == jobID {
			rec.views = append(rec.views, runFromActive(a))
		}
	}
	sort.SliceStable(rec.views, func(i, j int) bool { return rec.views[i].sortKey().After(rec.views[j].sortKey()) })
	return rec, nil
}

// listRuns returns the runs of one job (or all jobs), newest first.
func listRuns(b backend.Backend, jobID string) ([]runView, error) {
	rec, err := loadRuns(b, jobID)
	return rec.views, err
}

// resolveRun finds a run by run id, or by job id/name meaning that job's
// newest run (an in-progress one first).
func resolveRun(b backend.Backend, ref string) (runView, runRecords, error) {
	rec, err := loadRuns(b, "")
	if err != nil {
		return runView{}, rec, err
	}
	for _, r := range rec.views {
		if r.ID == ref {
			return r, rec, nil
		}
	}
	jobs, err := b.ReadJobs()
	if err != nil {
		return runView{}, rec, errUnavailable("read jobs: %v", err)
	}
	jobID := ref
	if i, err := findJob(jobs, ref); err == nil {
		jobID = jobs[i].ID
	}
	var newest *runView
	for i, r := range rec.views {
		if r.JobID != jobID && !strings.EqualFold(r.JobName, ref) {
			continue
		}
		if r.State == stateRunning {
			return r, rec, nil
		}
		if newest == nil {
			newest = &rec.views[i]
		}
	}
	if newest != nil {
		return *newest, rec, nil
	}
	return runView{}, rec, errNotFound("no run %q", ref).withHint("list runs with `lazycron runs`")
}
