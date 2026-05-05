// description: AI agent that analyzes a repo and suggests one high-impact UX feature as a GitHub issue
export const cron = "0 8 * * 1-5";
export const name = "Feature Suggestion Agent";
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

const open = Number(sh("gh issue list -l enhancement --state open --json number -q 'length'"));
if (open >= 5) {
  console.log(`Too many open feature suggestions (${open}), skipping`);
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
  prompt: `You are a feature suggestion agent focused on HIGH-IMPACT, USER-FACING improvements. Your job:
1. Run \`gh issue list -l enhancement --state open\` AND \`gh issue list -l enhancement --state closed\` — read ALL existing suggestions so you never duplicate.
2. Analyze the codebase — focus on the user experience: how does the user interact with this tool? What feels clunky, slow, confusing, or missing? Look at UI flows, error messages, onboarding, feedback loops, and accessibility.
3. Prioritize by impact: suggest the ONE thing that would make the biggest difference to the end user. Think like a product manager, not an engineer — care about what the user sees and feels, not internal refactors.
4. Open exactly ONE GitHub issue with the enhancement label using \`gh issue create --title ... --body ... --label enhancement\`.
5. The issue body must include: what the user experience problem is today, how the suggestion improves it, and clear acceptance criteria.
Rules: Do NOT implement anything. Do NOT open PRs. Do NOT open more than one issue. Do NOT suggest internal refactors, code cleanup, or developer-only improvements — every suggestion must directly improve the end-user experience.
IMPORTANT: If you cannot identify a suggestion that meaningfully improves the product for users, STAND DOWN — exit cleanly without opening any issue. No filler, no low-impact ideas.`,
  maxIterations: 1,
});
