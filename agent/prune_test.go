package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// addJobWorktrees makes n worktrees of repo in dir named <jobID>-<stamp>,
// with stamps starting at hour (so their age order is known).
func addJobWorktrees(t *testing.T, repo, dir, jobID string, hour, n int) []string {
	t.Helper()
	var paths []string
	for i := 0; i < n; i++ {
		wt := filepath.Join(dir, fmt.Sprintf("%s-20260101-%02d0000", jobID, hour+i))
		if err := AddWorktree(repo, wt, "origin/trunk"); err != nil {
			t.Fatalf("AddWorktree: %v", err)
		}
		paths = append(paths, wt)
	}
	return paths
}

func noLog(string, ...any) {}

func TestPruneWorktrees_LeavesSameNamedProjectAlone(t *testing.T) {
	_, repoA := newOrigin(t)
	_, repoB := newOrigin(t)
	dir := filepath.Join(t.TempDir(), "worktrees", "api")

	a := addJobWorktrees(t, repoA, dir, "audit", 0, 3)
	b := addJobWorktrees(t, repoB, dir, "audit", 10, 3)

	pruneWorktrees(repoA, dir, "audit", 2, noLog)

	for _, wt := range a[:2] {
		if exists(wt) {
			t.Errorf("A's old worktree %s was kept", wt)
		}
	}
	if !exists(a[2]) {
		t.Errorf("A's newest worktree %s was removed", a[2])
	}
	for _, wt := range b {
		if !exists(wt) {
			t.Errorf("B's worktree %s was removed by A's prune", wt)
		}
	}
	if got, _ := ListWorktrees(repoB); len(jobWorktrees(got, dir, "audit")) != 3 {
		t.Errorf("B's worktree records = %v, want its 3 worktrees", got)
	}
}

func TestPruneWorktrees_NeverDeletesUnknownDirectories(t *testing.T) {
	_, repo := newOrigin(t)
	dir := filepath.Join(t.TempDir(), "worktrees", "api")
	stray := filepath.Join(dir, "audit-20250101-000000")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := addJobWorktrees(t, repo, dir, "audit", 0, 2)

	pruneWorktrees(repo, dir, "audit", 1, noLog)

	if !exists(stray) {
		t.Error("a directory that is not a worktree of repo was deleted")
	}
	for _, wt := range mine {
		if exists(wt) {
			t.Errorf("old worktree %s was kept", wt)
		}
	}
}

func TestPruneWorktrees_KeepsOtherJobs(t *testing.T) {
	_, repo := newOrigin(t)
	dir := filepath.Join(t.TempDir(), "worktrees", "api")
	other := addJobWorktrees(t, repo, dir, "audit-deep", 0, 2)
	addJobWorktrees(t, repo, dir, "audit", 5, 2)

	pruneWorktrees(repo, dir, "audit", 1, noLog)

	for _, wt := range other {
		if !exists(wt) {
			t.Errorf("job audit-deep's worktree %s was removed", wt)
		}
	}
}

func TestIsJobWorktree(t *testing.T) {
	cases := []struct {
		name, job string
		want      bool
	}{
		{"audit-20260101-000000", "audit", true},
		{"audit-deep-20260101-000000", "audit", false},
		{"audit-20260101", "audit", false},
		{"other-20260101-000000", "audit", false},
	}
	for _, c := range cases {
		if got := isJobWorktree(c.name, c.job); got != c.want {
			t.Errorf("isJobWorktree(%q, %q) = %v, want %v", c.name, c.job, got, c.want)
		}
	}
}
