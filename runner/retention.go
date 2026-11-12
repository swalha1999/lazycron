package runner

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/swalha1999/lazycron/history"
)

// prune keeps the terminals of the newest KeepRuns finished runs of a job
// and ends and removes the rest. A run can own several terminals (the run
// itself and the Claude session an agent job starts), all labeled with its
// run id. Runs still in progress (no history entry yet) are never touched.
// Errors are ignored: retention is housekeeping.
func (e *Executor) prune(jobID string) {
	if e.HC == nil || e.KeepRuns <= 0 {
		return
	}
	terms, err := e.HC.List(map[string]string{LabelJob: jobID})
	if err != nil {
		return
	}
	var finished []Terminal
	for _, t := range terms {
		full, err := e.HC.Get(t.Address)
		if err != nil {
			continue
		}
		if run := full.Labels[LabelRun]; run != "" && e.hasHistory(run) {
			finished = append(finished, full)
		}
	}
	for _, t := range PruneCandidates(finished, e.KeepRuns) {
		if t.Live() {
			_ = e.HC.Kill(t.Address)
		}
		_ = e.HC.Remove(t.Address)
	}
}

// PruneCandidates returns the terminals of finished runs beyond the newest
// keep runs. Runs are ordered by run id, which starts with the start time.
func PruneCandidates(finished []Terminal, keep int) []Terminal {
	var runs []string
	byRun := map[string][]Terminal{}
	for _, t := range finished {
		run := t.Labels[LabelRun]
		if _, ok := byRun[run]; !ok {
			runs = append(runs, run)
		}
		byRun[run] = append(byRun[run], t)
	}
	if len(runs) <= keep {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(runs)))
	var out []Terminal
	for _, run := range runs[keep:] {
		out = append(out, byRun[run]...)
	}
	return out
}

func (e *Executor) hasHistory(runID string) bool {
	_, err := os.Stat(filepath.Join(e.Paths.History, history.EntryFileName(runID)))
	return err == nil
}
