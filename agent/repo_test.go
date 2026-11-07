package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newOrigin makes a bare repo with one commit on branch trunk and returns a
// clone of it, so tests exercise a real origin and default branch.
func newOrigin(t *testing.T) (origin, clone string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	origin = filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	clone = filepath.Join(root, "clone")
	run := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(root, "init", "--quiet", "--bare", "--initial-branch=trunk", origin)
	run(root, "init", "--quiet", "--initial-branch=trunk", seed)
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(seed, "add", ".")
	run(seed, "commit", "--quiet", "-m", "init")
	run(seed, "push", "--quiet", origin, "trunk")
	run(root, "clone", "--quiet", origin, clone)
	return origin, clone
}

func TestRepoLifecycle(t *testing.T) {
	_, clone := newOrigin(t)

	repo, err := ResolveRepo(filepath.Join(clone), "", "")
	if err != nil {
		t.Fatalf("ResolveRepo: %v", err)
	}
	ref, err := FetchBase(repo, "")
	if err != nil || ref != "origin/trunk" {
		t.Fatalf("FetchBase = %q, %v; want origin/trunk", ref, err)
	}
	wt := filepath.Join(t.TempDir(), "wt", "job-1")
	if err := AddWorktree(repo, wt, ref); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt, "README")); err != nil {
		t.Fatalf("worktree not checked out: %v", err)
	}
	if err := RemoveWorktree(repo, wt); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree still there: %v", err)
	}
}

func TestResolveRepo_ClonesWhenProjectDirIsNotACheckout(t *testing.T) {
	origin, _ := newOrigin(t)
	cache := filepath.Join(t.TempDir(), "repos", "proj")

	repo, err := ResolveRepo(t.TempDir(), origin, cache)
	if err != nil || repo != cache {
		t.Fatalf("ResolveRepo = %q, %v", repo, err)
	}
	if again, err := ResolveRepo(t.TempDir(), origin, cache); err != nil || again != cache {
		t.Fatalf("second ResolveRepo = %q, %v", again, err)
	}
	if _, err := ResolveRepo(t.TempDir(), "", cache+"-none"); err == nil {
		t.Error("expected an error with no checkout and no REPO_URL")
	}
}

func TestFetchBase_ExplicitBranch(t *testing.T) {
	_, clone := newOrigin(t)
	if ref, err := FetchBase(clone, "release"); err != nil || ref != "origin/release" {
		t.Errorf("FetchBase = %q, %v", ref, err)
	}
}
