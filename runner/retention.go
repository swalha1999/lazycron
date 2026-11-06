package runner

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/swalha1999/lazycron/history"
)

// prune keeps the newest KeepRuns finished run terminals of a job and ends
// and removes the rest. Runs still in progress (no history entry yet) are
// never touched. Errors are ignored: retention is housekeeping.
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

// PruneCandidates returns the finished run terminals beyond the newest keep.
func PruneCandidates(finished []Terminal, keep int) []Terminal {
	sort.Slice(finished, func(i, j int) bool { return finished[i].CreatedMS > finished[j].CreatedMS })
	if len(finished) <= keep {
		return nil
	}
	return finished[keep:]
}

func (e *Executor) hasHistory(runID string) bool {
	_, err := os.Stat(filepath.Join(e.Paths.History, history.EntryFileName(runID)))
	return err == nil
}
