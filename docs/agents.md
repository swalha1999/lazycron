# Sandcastle Agents

Lazycron schedules sandboxed AI agents using [sandcastle](https://github.com/mattpocock/sandcastle). Each agent runs Claude Code in a Docker container with a per-run git worktree, so concurrent runs don't conflict and a confused agent can't reach outside its sandbox.

## Lifecycle

```
lazycron init --with-agents
        │
        ▼
.lazycron/config.yaml + .sandcastle/{Dockerfile,.env,lib/ensureRepo.ts}
        │
        │  lazycron templates apply <name>
        ▼
.sandcastle/jobs/<name>.ts
        │
        │  edit .sandcastle/.env (REPO_URL, ANTHROPIC_API_KEY, GH_TOKEN, …)
        │
        │  lazycron sync [--server X]
        ▼
1. Deps check (docker, node, npx)
2. Tar + ship .lazycron/ and .sandcastle/ to remote (or skip if local)
3. npx @ai-hero/sandcastle docker build-image
4. Install crontab entry: `cd <project-dir> && npx tsx .sandcastle/jobs/<name>.ts`
        │
        │  cron fires
        ▼
5. ensureRepo clones / fetches origin/main into ~/.lazycron-cache/repos/<name>/
6. sandcastle.run() spawns Docker container with a unique-branch worktree
7. Agent runs the prompt
8. (For PR agents) host pushes branch and runs gh pr create
```

## Required Environment

Both the local machine running `lazycron sync` and the target machine where cron fires need:

- **Docker** — runs the agent sandbox
- **Node 20+** — runs `npx @ai-hero/sandcastle ...` and the agent TS file
- **`gh` CLI** + a GitHub token (in `.sandcastle/.env` as `GH_TOKEN`) for any agent that calls `gh issue create` or `gh pr create`
- **Git** — for the repo cache and per-run worktrees

`lazycron sync` checks `docker`/`node`/`npx` before doing anything destructive. Missing tools produce an explicit error pointing at install pages.

## TS File Anatomy

```typescript
// description: One-line summary shown in `lazycron templates list`
export const cron = "0 9 * * 1-5";
export const name = "Worker Agent";
export const tag = "BP";          // optional
export const tagColor = "#f38ba8"; // optional

import { run, claudeCode } from "@ai-hero/sandcastle";
import { docker } from "@ai-hero/sandcastle/sandboxes/docker";
import { ensureRepo } from "../lib/ensureRepo";
import { execSync } from "node:child_process";

const sh = (cmd: string) => execSync(cmd, { encoding: "utf8" }).trim();

// 1. Make sure a fresh copy of main is locally available.
const repoDir = ensureRepo({
  url: process.env.REPO_URL!,
  name: process.env.PROJECT_NAME ?? "myproject",
  defaultBranch: process.env.BASE_BRANCH ?? "main",
});
process.chdir(repoDir);

// 2. Cheap preconditions: bail before paying for an agent run.
if (Number(sh("gh pr list --state open --json number -q 'length'")) >= 3) {
  console.log("Too many open PRs, skipping");
  process.exit(0);
}

// 3. Build a unique branch name so concurrent runs do not collide.
const issue = sh("gh issue list -l bug --state open --json number --jq '.[0].number'");
const branch = `fix/issue-${issue}-${Date.now()}`;

// 4. Run the agent inside the sandbox.
const result = await run({
  agent: claudeCode("claude-opus-4-7"),
  sandbox: docker(),
  branchStrategy: { type: "branch", branch },
  prompt: `Read issue #${issue} via 'gh issue view ${issue}'. Fix it. Commit.`,
  maxIterations: 1,
});

// 5. Host-side push and PR — keeps GitHub auth out of the sandbox.
if (result.commits.length > 0) {
  sh(`git push origin ${branch}`);
  sh(`gh pr create --head ${branch} --title "Fix #${issue}" --body "Closes #${issue}"`);
}
```

### Two patterns for the host/sandbox split

- **PR agents** (e.g. `worker-agent`): the agent works on a fresh branch inside the sandbox and opens the PR from there. The sandbox needs `GH_TOKEN` (passed via `agentSandboxConfig()`) so it can run `gh pr create`.
- **Issue-only agents** (e.g. `audit-agents`): the agent calls `gh issue create` directly inside the sandbox. The container needs `GH_TOKEN`, supplied via `docker({ env: { GH_TOKEN: process.env.GH_TOKEN ?? "" } })`.

## Concurrency

Sandcastle's `branchStrategy: { type: "branch", branch }` creates a separate `git worktree` on every call. Two cron firings of the same agent at overlapping times get separate working directories and separate branches, as long as the branch names are unique. The bundled templates use `Date.now()` in the branch name to guarantee uniqueness.

For agents that don't push branches (issue-only), `branchStrategy: { type: "merge-to-head" }` lets sandcastle invent a unique temp branch per run; you never pick the name yourself.

## Project Layout (after init + apply)

```
my-repo/
├── .lazycron/
│   └── config.yaml          # name: my-repo
├── .sandcastle/
│   ├── Dockerfile           # built by `npx @ai-hero/sandcastle docker build-image`
│   ├── .env                 # ANTHROPIC_API_KEY, REPO_URL, GH_TOKEN, …
│   ├── .env.example
│   ├── lib/
│   │   └── ensureRepo.ts    # bundled by `lazycron init --with-agents`
│   └── jobs/
│       ├── audit-agents.ts
│       └── worker-agent.ts
└── …rest of your repo
```

After `lazycron sync --server X`, the same layout (minus your source code) lives at `~/.lazycron/projects/<name>/` on the remote machine. The repo cache (where `ensureRepo` clones to) lives at `~/.lazycron-cache/repos/<name>/` and is **not** synced — it's created on first run.

## Caveats

- **`lazycron pull` is one-way for sandcastle jobs.** The metadata parser cannot reverse a TS file from a crontab line, so `pull` writes read-only `.lazycron/_pulled/<id>.crontab` stubs for inspection — not source files.
- **Docker layer cache lives on the target.** First sync to a fresh remote takes minutes; later syncs are seconds.
- **`.env` is synced by default.** If you have multiple environments (staging vs prod) with different secrets, use `--no-env` and ssh in to manage env files server-side.
- **Single project per repo.** The project name is derived once and used for the remote dir + repo cache. Multiple projects in one repo is not supported in v1.
