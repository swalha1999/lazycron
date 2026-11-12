package runner

import (
	"sort"
	"testing"
)

func term(addr, run string) Terminal {
	return Terminal{Address: addr, Labels: map[string]string{LabelRun: run}}
}

func TestPruneCandidatesKeepsWholeRuns(t *testing.T) {
	finished := []Terminal{
		term("t1", "2026-11-12T04-00-00_job_aaaa"), term("t2", "2026-11-12T04-00-00_job_aaaa"),
		term("t3", "2026-11-12T08-00-00_job_bbbb"), term("t4", "2026-11-12T08-00-00_job_bbbb"),
		term("t5", "2026-11-12T12-00-00_job_cccc"),
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
