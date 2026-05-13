// description: Picks one open issue, claims it with `Taken`, implements/fixes it, and opens a PR
export const cron = "0 9 * * 1-5";
export const name = "Worker Agent";
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

// Pick the lowest-numbered open issue not yet `Taken`.
const issue = sh(
  `gh issue list --state open --json number,labels ` +
    `--jq '[.[] | select([.labels[].name] | index("Taken") | not)] | sort_by(.number) | .[0].number // empty'`
);
if (!issue) {
  console.log("No untaken issues, skipping");
  process.exit(0);
}

// Claim the issue immediately so a re-run (or a parallel agent) does not double-pick it.
sh(`gh issue edit ${issue} --add-label Taken`);

const branch = `agent/issue-${issue}-${Date.now()}`;

let result;
try {
  result = await run({
    agent: claudeCode("claude-opus-4-7"),
    sandbox: docker(agentSandboxConfig()),
    branchStrategy: { type: "branch", branch },
    prompt: `You are an issue worker agent. Your job:
1. Read issue #${issue} using \`gh issue view ${issue}\`.
2. Implement what the issue describes. Follow existing code patterns. Keep the diff focused.
3. Commit your changes.
4. Open the PR with \`gh pr create --head ${branch} --title "<conventional-commit-style title describing the change>" --body "Closes #${issue}\\n\\nAuto by Lazycron agent."\`. \`gh\` will push the branch automatically.

PR title rules: under 70 chars, present tense, describes WHAT the change does — not "address #X". Match the repo's existing commit style (\`git log --oneline -10\` to check). Examples: "fix: handle empty crontab in Parse", "refactor: extract shared dialog handler".

Rules: One issue per run. No unrelated changes. Build/lint/tests must pass before committing.
If you can't finish end-to-end or aren't confident the change is correct, STAND DOWN — exit cleanly without committing or opening a PR.`,
    maxIterations: 1,
  });
} catch (err) {
  // Release the claim so a future run can retry.
  sh(`gh issue edit ${issue} --remove-label Taken`);
  throw err;
}

if (result.commits.length > 0) {
  console.log(`Agent committed ${result.commits.length} commit(s); push and PR done in-sandbox.`);
} else {
  // Agent stood down — release the claim so the issue is eligible again.
  console.log("Agent stood down — no commits, releasing claim.");
  sh(`gh issue edit ${issue} --remove-label Taken`);
}
