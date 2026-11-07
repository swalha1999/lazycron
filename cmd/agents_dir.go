package cmd

import (
	"os"
	"path/filepath"
)

// agentsDirName is the per-project directory lazycron manages in a repo:
// config.yaml, .env, context.md, _pulled/ and the agent jobs in jobs/*.md.
const agentsDirName = ".lazycron"

// legacyAgentsDirName is where TypeScript sandcastle agents lived before
// agents became Markdown jobs run locally in herdr.
const legacyAgentsDirName = ".sandcastle"

// legacyDirHint returns a migration message when cwd still has the old
// .sandcastle/ layout and no jobs in .lazycron/, or "" otherwise.
func legacyDirHint(cwd string) string {
	if _, err := os.Stat(filepath.Join(cwd, legacyAgentsDirName, "jobs")); err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(cwd, agentsDirName, "jobs")); err == nil {
		return ""
	}
	return "agents are now Markdown jobs in .lazycron/jobs/ that run locally in herdr (no Docker, no .sandcastle/).\n" +
		"Run `lazycron init --with-agents` to scaffold them, port any custom prompts from .sandcastle/jobs/*.ts,\n" +
		"then remove .sandcastle/ and `lazycron sync` again"
}
