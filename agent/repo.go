package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// git runs git with args and returns trimmed stdout.
func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveRepo returns the git repository the agent works on: the project
// directory itself when it is a checkout, otherwise a clone of repoURL kept
// under cacheDir (the layout `lazycron sync --server` ships).
func ResolveRepo(projectDir, repoURL, cacheDir string) (string, error) {
	if top, err := git("-C", projectDir, "rev-parse", "--show-toplevel"); err == nil {
		return top, nil
	}
	if repoURL == "" {
		return "", fmt.Errorf("%s is not a git checkout and REPO_URL is not set in .lazycron/.env", projectDir)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); err == nil {
		return cacheDir, nil
	}
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return "", err
	}
	if _, err := git("clone", "--quiet", repoURL, cacheDir); err != nil {
		return "", err
	}
	return cacheDir, nil
}

// FetchBase fetches origin and returns the ref to start from: origin/<base>
// when base is set, otherwise origin's default branch.
func FetchBase(repo, base string) (string, error) {
	if _, err := git("-C", repo, "fetch", "--quiet", "--prune", "origin"); err != nil {
		return "", err
	}
	if base != "" {
		return "origin/" + base, nil
	}
	if ref, err := git("-C", repo, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return ref, nil
	}
	if _, err := git("-C", repo, "remote", "set-head", "origin", "--auto"); err != nil {
		return "", err
	}
	return git("-C", repo, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
}

// AddWorktree creates a detached worktree of repo at ref.
func AddWorktree(repo, path, ref string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err := git("-C", repo, "worktree", "add", "--quiet", "--detach", path, ref)
	return err
}

// RemoveWorktree deletes a worktree made by AddWorktree. Branches the agent
// created stay in the repository.
func RemoveWorktree(repo, path string) error {
	if _, err := git("-C", repo, "worktree", "remove", "--force", path); err != nil {
		return err
	}
	_, err := git("-C", repo, "worktree", "prune")
	return err
}

// RepoLabel returns owner/name for a GitHub origin, else the origin URL.
func RepoLabel(repo string) string {
	url, err := git("-C", repo, "remote", "get-url", "origin")
	if err != nil {
		return repo
	}
	for _, prefix := range []string{"https://github.com/", "git@github.com:", "ssh://git@github.com/"} {
		if rest, ok := strings.CutPrefix(url, prefix); ok {
			return strings.TrimSuffix(rest, ".git")
		}
	}
	return url
}
