package runner

import (
	"fmt"
	"os"
	"time"

	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/history"
)

// Request asks for one run of a job.
type Request struct {
	JobID   string
	JobName string
	Runner  string // config.RunnerPlain or config.RunnerHerdr
	Trigger string // TriggerCron or TriggerManual
	Once    bool   // disable the job after this run
	RunID   string // optional; generated when empty
}

// Executor runs jobs and records them. Construct it with the paths of the
// machine it runs on; HC is nil when herdr-core is unavailable, with
// HCUnavailable saying why (recorded as the run's fallback reason).
type Executor struct {
	Paths         Paths
	HC            *HC
	HCUnavailable string
	SpaceID       string
	ExtraLabels   map[string]string // added to run terminals; "" removes a default
	KeepRuns      int
	Hold          time.Duration

	// DisableOnce disables a one-shot job after it ran (optional).
	DisableOnce func(jobID string) error
	// NameSpace makes herdr-app show the space as "Cron" (optional, best effort).
	NameSpace func()
	// Started is called once the run is under way (optional).
	Started func(Active)

	Now         func() time.Time
	WaitTimeout int           // seconds per hc wait call
	MinPoll     time.Duration // shortest gap between hc wait calls
	LostAfter   time.Duration // give up on a herdr run after this long without hc
}

func (e *Executor) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// outcome is how a run ended, before it becomes a history entry.
type outcome struct {
	exitCode        *int
	signal          int
	output          string
	outputTruncated bool
	note            string // appended to output when the run's end is unclear
}

func (o outcome) success() bool {
	return o.exitCode != nil && *o.exitCode == 0 && o.signal == 0
}

// Run executes one run of a job, waits for it to end, records the history
// entry and applies --once and retention. The returned error covers only
// lazycron's own failures (e.g. history could not be written); a failing
// job is reported in the entry.
func (e *Executor) Run(req Request) (history.Entry, error) {
	start := e.now()
	if req.RunID == "" {
		req.RunID = NewRunID(req.JobID, start)
	}
	active := Active{
		RunID: req.RunID, JobID: req.JobID, JobName: req.JobName, Runner: req.Runner,
		Trigger: req.Trigger, StartedAt: start.Format(time.RFC3339), ExecPID: os.Getpid(),
	}

	var out outcome
	fallback := ""
	switch {
	case req.Runner != config.RunnerHerdr:
	case e.HC == nil:
		fallback = e.HCUnavailable
	default:
		out, fallback = e.runHerdr(&active)
	}
	if req.Runner != config.RunnerHerdr || fallback != "" {
		out = e.runPlain(&active)
	}

	entry := e.entry(active, out, fallback)
	err := history.WriteRunEntry(e.Paths.History, entry)
	RemoveActive(e.Paths.Runs, active.RunID)
	if err != nil {
		return entry, fmt.Errorf("write history: %w", err)
	}
	if req.Once && e.DisableOnce != nil {
		if derr := e.DisableOnce(req.JobID); derr != nil {
			fmt.Fprintf(os.Stderr, "lazycron: could not disable one-shot job %s: %v\n", req.JobID, derr)
		}
	}
	if active.Runner == config.RunnerHerdr {
		e.prune(req.JobID)
	}
	return entry, nil
}

// markStarted persists the marker and notifies Started.
func (e *Executor) markStarted(a Active) {
	if err := WriteActive(e.Paths.Runs, a); err != nil {
		fmt.Fprintf(os.Stderr, "lazycron: could not write run marker: %v\n", err)
	}
	if e.Started != nil {
		e.Started(a)
	}
}

func (e *Executor) entry(a Active, o outcome, fallback string) history.Entry {
	success := o.success()
	output := o.output
	if o.note != "" {
		if output != "" {
			output += "\n"
		}
		output += o.note
	}
	return history.Entry{
		JobID:           a.JobID,
		JobName:         a.JobName,
		Timestamp:       e.now().Format(time.RFC3339),
		Output:          output,
		Success:         &success,
		RunID:           a.RunID,
		StartedAt:       a.StartedAt,
		ExitCode:        o.exitCode,
		Signal:          o.signal,
		Runner:          a.Runner,
		Trigger:         a.Trigger,
		Terminal:        a.Terminal,
		TerminalName:    a.TerminalName,
		OutputTruncated: o.outputTruncated,
		FallbackReason:  fallback,
	}
}
