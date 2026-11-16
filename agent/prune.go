package agent

import (
	"path/filepath"
	"sort"
	"strings"
)

// stampLayout is the time format of the <stamp> in <job>-<stamp> worktree names.
const stampLayout = "20060102-150405"

// pruneWorktrees removes this job's oldest worktrees so that, with the one
// about to be made, keep remain. Only worktrees git records for repo count:
// dir can be shared with a same-named project, whose worktrees are left alone.
func pruneWorktrees(repo, dir, jobID string, keep int, log func(string, ...any)) {
	if keep <= 0 {
		return
	}
	all, err := ListWorktrees(repo)
	if err != nil {
		log("warning: could not list worktrees of %s: %v", repo, err)
		return
	}
	mine := jobWorktrees(all, dir, jobID)
	for len(mine) >= keep {
		if err := RemoveWorktree(repo, mine[0]); err != nil {
			log("warning: could not remove old worktree %s: %v", mine[0], err)
		}
		mine = mine[1:]
	}
}

// jobWorktrees returns the worktrees in dir named <jobID>-<stamp>, oldest
// first (the stamp sorts by age).
func jobWorktrees(worktrees []string, dir, jobID string) []string {
	dir = realPath(dir)
	var mine []string
	for _, wt := range worktrees {
		if realPath(filepath.Dir(wt)) == dir && isJobWorktree(filepath.Base(wt), jobID) {
			mine = append(mine, wt)
		}
	}
	sort.Slice(mine, func(i, j int) bool { return filepath.Base(mine[i]) < filepath.Base(mine[j]) })
	return mine
}

// isJobWorktree reports whether name is <jobID>-<stamp>. Checking the stamp's
// shape keeps job "a" from matching job "a-b"'s worktrees.
func isJobWorktree(name, jobID string) bool {
	stamp, ok := strings.CutPrefix(name, jobID+"-")
	return ok && len(stamp) == len(stampLayout) && !strings.Contains(stamp[9:], "-")
}

// realPath resolves symlinks so paths git reports compare equal to ours. A
// path that can't be resolved (it no longer exists) is only cleaned.
func realPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}
