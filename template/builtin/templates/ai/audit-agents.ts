// description: Weekly audit — files ONE meaningful code-cleanup issue (worker-agent picks it up to implement)
export const cron = "0 8 * * 0";
export const name = "Cleanup Scout";
export const tag = "AA";
export const tagColor = "#94e2d5";

import { run, claudeCode } from "@ai-hero/sandcastle";
import { docker } from "@ai-hero/sandcastle/sandboxes/docker";
import { ensureRepo, requireEnv, agentSandboxConfig } from "../lib/ensureRepo";

requireEnv("REPO_URL", "GH_TOKEN");

const repoDir = ensureRepo({
  url: process.env.REPO_URL!,
  name: process.env.PROJECT_NAME ?? "lazycron",
  defaultBranch: process.env.BASE_BRANCH ?? "main",
});
process.chdir(repoDir);

const branch = `cleanup-scout-${Date.now()}`;

const result = await run({
  name: "cleanup-scout",
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker(agentSandboxConfig()),
  branchStrategy: { type: "branch", branch },
  prompt: `You are a code-cleanup scout. Read the codebase and file ONE meaningful, high-impact cleanup issue. Do NOT modify code. Do NOT open a PR.

Look for changes that genuinely improve the project — not nitpicks. Good candidates:
- Significant duplication that's been copy-pasted across files
- A function/file that's grown too large and is hard to reason about
- A whole abstraction that's leaking complexity (wrong layer, wrong boundary)
- Dead code paths or unused exports surface area
- A package whose responsibilities have drifted and now does too many things

Skip: minor style, naming preferences, "could be simpler" without a concrete win, anything where the impact is "marginal".

Process:
1. Read \`git log --oneline -30\` and the top-level layout to understand the project.
2. Pick ONE cleanup that would clearly make the codebase easier to maintain or reason about.
3. Check existing open issues — do not duplicate.
4. File the issue:
   \`gh issue create --label refactor --title "<concise title>" --body "<what / where (file:line) / why this matters / suggested approach>\\n\\nAuto by Lazycron agent."\`

If nothing rises to "this would clearly help," STAND DOWN and exit cleanly.`,
  maxIterations: 1,
});

console.log(`Cleanup scout finished: ${result.commits.length === 0 ? "no commits (expected — scout doesn't modify code)" : "unexpected commits, check the run"}.`);
