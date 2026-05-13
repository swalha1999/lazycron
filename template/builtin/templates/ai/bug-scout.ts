// description: Weekly bug scout — reads recent code and files ONE issue for a real, reproducible bug
export const cron = "0 8 * * 3";
export const name = "Bug Scout";
export const tag = "BG";
export const tagColor = "#fab387";

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

const branch = `bug-scout-${Date.now()}`;

const result = await run({
  name: "bug-scout",
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker(agentSandboxConfig()),
  branchStrategy: { type: "branch", branch },
  prompt: `You are a bug scout. Read the codebase and file ONE issue describing a real, concrete bug. Do NOT modify code. Do NOT open a PR.

You are looking for actual defects, not style. Good candidates:
- Wrong control flow: off-by-one, inverted boolean, missing return, unreachable branch
- Wrong logic or implementation: the code does X but the spec / comment / call site clearly expects Y; an algorithm that returns the wrong result on a realistic input
- Broken or missing authorization: an endpoint / handler / RPC that trusts the caller, skips an ownership check, mixes up "authenticated" with "authorized", IDOR (one user can act on another user's resource), missing role/tenant check, auth check on the client only
- Data leakage to the UI / API response: serializing a whole DB row when only a subset is meant to be public (password hashes, tokens, internal IDs, other users' data, soft-deleted rows, draft/private records), returning stack traces or internal error messages to end users, debug logs that include secrets / PII, over-broad SELECT * crossing a trust boundary
- Unhandled error paths: ignored \`err\`, swallowed exceptions, missing rollback on partial failure
- Concurrency hazards: shared mutable state, missing lock/await, data race, leaked goroutine/promise
- Resource leaks: unclosed file/connection/timer, missing \`defer\` / \`finally\`
- Incorrect input validation: trust boundary violations, integer overflow, path traversal, command injection, SSRF
- Logic that contradicts the surrounding comment, docstring, or test expectation

Skip: refactor opportunities, style nits, "could be simpler", anything you can't pin to a file:line with a concrete failure scenario.

Process:
1. Skim the recent diff: \`git log --oneline -50\` then \`git diff HEAD~20 HEAD -- <changed paths>\` for the hottest files.
2. Read those files end-to-end (not just the diff) — bugs often live in the unchanged neighbor.
3. Trace at least one request/handler from entry point to response: who is the caller, what auth/ownership check runs, what gets returned. Auth and data-leak bugs only surface when you follow the full path.
4. For each candidate, write down: input (and which user/role) that triggers it → expected behaviour → actual behaviour. If you can't fill all three, it isn't ready to file.
4. Pick the single highest-impact bug. Check existing open issues with the \`bug\` label — do not duplicate.
5. File the issue:
   \`gh issue create --label bug --title "<concise symptom>" --body "<location (file:line) / trigger / expected vs actual / why this is wrong / suggested fix direction>\\n\\nAuto by Lazycron agent."\`

If nothing meets the "real defect with a concrete trigger" bar, STAND DOWN and exit cleanly.`,
  maxIterations: 1,
});

console.log(`Bug scout finished: ${result.commits.length === 0 ? "no commits (expected — scout doesn't modify code)" : "unexpected commits, check the run"}.`);
