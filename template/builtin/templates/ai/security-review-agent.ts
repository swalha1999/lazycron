// description: AI agent that scans a repository for security vulnerabilities and opens one GitHub issue per run
export const cron = "0 6 * * 1-5";
export const name = "Security Review Agent";
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

const openIssues = Number(sh("gh issue list -l security --state open --json number -q 'length'"));
if (openIssues >= 3) {
  console.log(`Too many open security issues (${openIssues}), skipping`);
  process.exit(0);
}

// Issue-only agents have gh + GH_TOKEN inside the sandbox so the agent can
// open the issue directly. Set GH_TOKEN in .sandcastle/.env (or pass via the
// sandcastle docker() env: option) so the container has it.
await run({
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker(agentSandboxConfig()),
  branchStrategy: { type: "merge-to-head" },
  prompt: `You are a security review agent. Your job:
1. Run \`gh issue list -l security --state open\` — read existing findings so you never duplicate.
2. Scan the codebase for ONE new security vulnerability (OWASP Top 10, dependency issues, secrets, misconfigs).
3. If you find something new, open exactly ONE GitHub issue with the security label using \`gh issue create --title ... --body ... --label security\`.
4. If everything looks clean, exit without creating an issue.
Rules: Do NOT fix anything. Do NOT open PRs. Do NOT open more than one issue. Be specific — include file paths and line numbers.
IMPORTANT: If you do not find a clear, critical vulnerability that you are confident about, STAND DOWN — exit cleanly without opening any issue. Do not create issues for minor or speculative findings.`,
  maxIterations: 1,
});
