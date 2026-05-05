// description: AI agent that picks one open enhancement issue, implements it on a branch, and opens a PR
export const cron = "0 10 * * 1-5";
export const name = "Feature Builder Agent";
export const tag = "BP";
export const tagColor = "#f38ba8";

import { run, claudeCode } from "@ai-hero/sandcastle";
import { docker } from "@ai-hero/sandcastle/sandboxes/docker";
import { ensureRepo, requireEnv } from "../lib/ensureRepo";
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
  "gh issue list -l enhancement --state open --json number,title --jq 'sort_by(.number) | .[0].number'"
);
if (!issue) {
  console.log("No open enhancement issues, skipping");
  process.exit(0);
}

const branch = `feat/issue-${issue}-${Date.now()}`;

const result = await run({
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker(),
  branchStrategy: { type: "branch", branch },
  prompt: `You are an issue worker agent. Your job:
1. Read issue #${issue} using \`gh issue view ${issue}\`.
2. Implement the feature described in the issue. Follow existing code patterns and conventions.
3. Commit your changes with a clear message referencing the issue.
Rules: Implement only ONE issue. Do NOT modify unrelated code. Keep the diff focused and reviewable.
IMPORTANT: Every commit must be complete and fully working. No half-done implementations, no TODOs, no placeholder code. The build, linting, and tests must all pass before you commit. If you cannot finish the feature end-to-end, do not commit.`,
  maxIterations: 1,
});

if (result.commits.length > 0) {
  sh(`git push origin ${branch}`);
  sh(
    `gh pr create --head ${branch} ` +
      `--title "Feat #${issue}: enhancement" ` +
      `--body "Closes #${issue}\n\nAutomated implementation via lazycron feature-builder-agent." ` +
      `--label enhancement`
  );
} else {
  console.log("Agent stood down — no commits to push.");
}
