// description: AI agent that picks one open refactor issue, implements it on a branch, and opens a PR
export const cron = "0 11 * * 1-5";
export const name = "Refactor Builder Agent";
export const tag = "BP";
export const tagColor = "#f38ba8";

import { run, claudeCode } from "@ai-hero/sandcastle";
import { docker } from "@ai-hero/sandcastle/sandboxes/docker";
import { ensureRepo, requireEnv, agentSandboxConfig } from "../lib/ensureRepo";
import { execSync } from "node:child_process";

const sh = (cmd: string) => execSync(cmd, { encoding: "utf8" }).trim();

requireEnv("REPO_URL", "GH_TOKEN");

const repoDir = ensureRepo({
  url: process.env.REPO_URL!,
  name: process.env.PROJECT_NAME ?? "lazycron",
  defaultBranch: process.env.BASE_BRANCH ?? "main",
});
process.chdir(repoDir);

const openPRs = Number(sh("gh pr list --state open --json number -q 'length'"));
if (openPRs >= 3) {
  console.log(`Too many open PRs (${openPRs}), skipping`);
  process.exit(0);
}

const issue = sh(
  "gh issue list -l refactor --state open --json number,title --jq 'sort_by(.number) | .[0].number'"
);
if (!issue) {
  console.log("No open refactor issues, skipping");
  process.exit(0);
}

const branch = `refactor/issue-${issue}-${Date.now()}`;

const result = await run({
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker(agentSandboxConfig()),
  branchStrategy: { type: "branch", branch },
  prompt: `You are a refactor builder agent. Your job:
1. Read issue #${issue} using \`gh issue view ${issue}\`.
2. Implement the refactor described in the issue. Do not change behavior — only improve structure, readability, and maintainability.
3. Commit your changes with a clear message referencing the issue.
Rules: ONE refactor per run. Do NOT change behavior or add features. Do NOT modify unrelated code.
IMPORTANT: Every commit must be complete and fully working. The build, linting, and tests must all pass before you commit. If your refactor breaks anything, fix it or revert and stand down. Do not commit with failing tests.`,
  maxIterations: 1,
});

if (result.commits.length > 0) {
  sh(`git push origin ${branch}`);
  sh(
    `gh pr create --head ${branch} ` +
      `--title "Refactor #${issue}" ` +
      `--body "Closes #${issue}\n\n## What\nAutomated refactor via lazycron refactor-builder-agent.\n\n## Why\nSee linked issue." ` +
      `--label refactor`
  );
} else {
  console.log("Agent stood down — no commits to push.");
}
