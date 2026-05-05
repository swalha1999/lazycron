// description: AI agent that finds the highest-impact code quality improvement and opens one issue
export const cron = "0 7 * * 1-5";
export const name = "Code Quality Agent";
export const tag = "BP";
export const tagColor = "#f38ba8";

import { run, claudeCode } from "@ai-hero/sandcastle";
import { docker } from "@ai-hero/sandcastle/sandboxes/docker";
import { ensureRepo } from "../lib/ensureRepo";
import { execSync } from "node:child_process";

const sh = (cmd: string) => execSync(cmd, { encoding: "utf8" }).trim();

const repoDir = ensureRepo({
  url: process.env.REPO_URL!,
  name: process.env.PROJECT_NAME ?? "lazycron",
  defaultBranch: process.env.BASE_BRANCH ?? "main",
});
process.chdir(repoDir);

const open = Number(sh("gh issue list -l refactor --state open --json number -q 'length'"));
if (open >= 5) {
  console.log(`Too many open refactor issues (${open}), skipping`);
  process.exit(0);
}

await run({
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker({
    env: {
      GH_TOKEN: process.env.GH_TOKEN ?? "",
    },
  }),
  branchStrategy: { type: "merge-to-head" },
  prompt: `You are a senior code quality engineer. Think like a staff engineer doing a codebase health check. Your job:
1. Run \`gh issue list -l refactor --state open\` AND \`gh issue list -l refactor --state closed\` — read ALL existing issues so you never duplicate.
2. Analyze the codebase for the ONE highest-impact improvement in any of these areas:
   - DRY violations: duplicated logic that should be extracted into shared functions
   - Large files or functions that should be split (single responsibility)
   - Poor naming that hurts readability
   - Missing or weak error handling
   - Dead code, unused imports, unreachable paths
   - Test gaps for critical paths
   - Inconsistent patterns across similar code
3. Open exactly ONE GitHub issue with the refactor label using \`gh issue create --title ... --body ... --label refactor\`.
4. The issue body must include: what the problem is, which files are affected, what the refactor should look like, and why it improves maintainability.
Rules: Do NOT implement anything. Do NOT open PRs. Do NOT open more than one issue. Focus on changes that make the codebase easier to maintain, extend, and build on.
IMPORTANT: If the codebase is already clean and well-structured, STAND DOWN — exit cleanly without opening any issue. Do not create issues for nitpicks or cosmetic changes.`,
  maxIterations: 1,
});
