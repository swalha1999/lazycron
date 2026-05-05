// description: AI agent that picks one open security issue, creates a fix branch, and opens a PR
export const cron = "0 9 * * 1-5";
export const name = "Security Fix Agent";
export const tag = "BP";
export const tagColor = "#f38ba8";

import { run, claudeCode } from "@ai-hero/sandcastle";
import { docker } from "@ai-hero/sandcastle/sandboxes/docker";
import { ensureRepo, requireEnv } from "../lib/ensureRepo";
import { execSync } from "node:child_process";

const sh = (cmd: string) => execSync(cmd, { encoding: "utf8" }).trim();

requireEnv("REPO_URL", "ANTHROPIC_API_KEY", "GH_TOKEN");

const repoDir = ensureRepo({
  url: process.env.REPO_URL!,
  name: process.env.PROJECT_NAME ?? "lazycron",
  defaultBranch: process.env.BASE_BRANCH ?? "main",
});
process.chdir(repoDir);

// Preconditions: bail out cheaply if we already have too many open PRs or no issues to work on.
const openPRs = Number(sh("gh pr list --state open --json number -q 'length'"));
if (openPRs >= 3) {
  console.log(`Too many open PRs (${openPRs}), skipping`);
  process.exit(0);
}

const issue = sh(
  "gh issue list -l security --state open --json number,title --jq 'sort_by(.number) | .[0].number'"
);
if (!issue) {
  console.log("No open security issues, skipping");
  process.exit(0);
}

// Date.now() suffix avoids collisions when concurrent agent runs target the same issue.
const branch = `fix/issue-${issue}-${Date.now()}`;

const result = await run({
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker(),
  branchStrategy: { type: "branch", branch },
  prompt: `You are a fix agent. Your job:
1. Read issue #${issue} using \`gh issue view ${issue}\`.
2. Fix the bug described in the issue. Make minimal, focused changes.
3. Commit your changes with a clear message referencing the issue.
Rules: Fix only ONE issue. Do NOT modify unrelated code. Keep the diff small and reviewable.
IMPORTANT: If you are not confident you fully understand the bug or your fix is correct, STAND DOWN — exit cleanly without committing. Do not submit uncertain or partial fixes.
The build, linting, and tests must all pass before you commit.`,
  maxIterations: 1,
});

// Host-side push + PR — keeps GitHub auth out of the sandbox.
if (result.commits.length > 0) {
  sh(`git push origin ${branch}`);
  sh(
    `gh pr create --head ${branch} ` +
      `--title "Fix #${issue}: security issue" ` +
      `--body "Closes #${issue}\n\nAutomated fix via lazycron security-fix-agent." ` +
      `--label bug`
  );
} else {
  console.log("Agent stood down — no commits to push.");
}
