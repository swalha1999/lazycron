package runner

import (
	"sort"
	"testing"
)

func term(addr, run string, created int64) Terminal {
	return Terminal{Address: addr, CreatedMS: created, Labels: map[string]string{LabelRun: run}}
}

func TestPruneCandidatesKeepsWholeRuns(t *testing.T) {
	finished := []Terminal{
		// Same-second run ids: order comes from when the terminals were made.
		term("t1", "2026-11-12T04-00-00_job_ffff", 100), term("t2", "2026-11-12T04-00-00_job_ffff", 101),
		term("t3", "2026-11-12T04-00-00_job_0000", 200), term("t4", "2026-11-12T04-00-00_job_0000", 201),
		term("t5", "2026-11-12T04-00-00_job_8888", 300),
	}
	got := PruneCandidates(finished, 2)
	var addrs []string
	for _, t := range got {
		addrs = append(addrs, t.Address)
	}
	sort.Strings(addrs)
	if len(addrs) != 2 || addrs[0] != "t1" || addrs[1] != "t2" {
		t.Fatalf("pruned %v, want the oldest run's terminals t1 t2", addrs)
	}
	if got := PruneCandidates(finished, 3); got != nil {
		t.Fatalf("keep 3 of 3 runs pruned %v", got)
	}
}
