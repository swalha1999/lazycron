package agent

import (
	"fmt"
	"strings"
)

// PromptContext is what the agent is told about where it is running.
type PromptContext struct {
	JobName  string
	Repo     string // owner/name or remote URL
	Base     string // e.g. origin/main
	Worktree string
	Notes    string // optional per-project notes from .lazycron/context.md
}

// BuildPrompt wraps a job's prompt with the facts every scheduled run needs.
func BuildPrompt(job Job, c PromptContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are a scheduled lazycron agent (job: %s). Nobody is watching this session live, "+
		"so don't ask questions: decide and act, or stand down and say why.\n\n", c.JobName)
	fmt.Fprintf(&b, "- Repository: %s\n", c.Repo)
	fmt.Fprintf(&b, "- You are in a fresh git worktree at %s, detached at %s. Create your own branch before committing.\n", c.Worktree, c.Base)
	b.WriteString("- Use `gh` for GitHub. Never push to or merge into the default branch; open pull requests instead.\n")
	b.WriteString("- When the task is done, end with a short summary of what you did and stop. Don't wait for a reply: " +
		"the session stays open afterwards only so a person can read it or continue it later.\n")
	if notes := strings.TrimSpace(c.Notes); notes != "" {
		b.WriteString("\n## Notes about this repository\n\n")
		b.WriteString(notes)
		b.WriteString("\n")
	}
	b.WriteString("\n## Task\n\n")
	b.WriteString(job.Prompt)
	b.WriteString("\n")
	return b.String()
}
