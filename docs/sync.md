# Sync — Sandcastle Agents as Code

Lazycron `sync` reads sandcastle agent files from `.sandcastle/jobs/*.ts`, ships them (and `.lazycron/`) to your target machine, builds the sandcastle Docker image, and installs cron entries that run each agent on schedule. This makes scheduled AI agents version-controlled, code-reviewable, and deployable alongside your application — with sandbox isolation and per-run git worktrees so concurrent runs do not conflict.

## Quick Start

1. Initialize the project:

```bash
lazycron init --with-agents
```

This creates `.lazycron/config.yaml` (project metadata) and runs `npx @ai-hero/sandcastle init` to scaffold `.sandcastle/` (Dockerfile, env, lib helpers).

2. Apply an agent template:

```bash
lazycron templates apply fix-agent
# Created .sandcastle/jobs/fix-agent.ts
```

The TS file declares its cron schedule and name as `export const`s and contains the agent's prompt + post-run actions (e.g. `gh pr create`).

3. Edit `.sandcastle/.env` to set `ANTHROPIC_API_KEY`, `REPO_URL`, and any agent-specific env vars.

4. Sync:

```bash
lazycron sync                        # local crontab
lazycron sync -s CronWorker          # remote server
```

## Sync Phases

In order, every sync run does:

1. **Resolve project name** from `--project` flag, then `.lazycron/config.yaml`, then the cwd basename.
2. **Read TS jobs** from `.sandcastle/jobs/*.ts`. Each file's metadata (`export const cron`, `name`, optional `tag` and `tagColor`) becomes a cron entry. The command is auto-generated as `npx tsx .sandcastle/jobs/<name>.ts`.
3. **Deps check** the target machine — refuses to continue if `docker`, `node`, or `npx` is missing. Override with `--skip-deps-check`.
4. **Transfer files** (remote sync only): tar `.lazycron/` and `.sandcastle/` over SSH stdin into `~/.lazycron/projects/<name>/` on the remote. `.env` files are included by default; `--no-env` opts out.
5. **Build the sandcastle image** (`npx @ai-hero/sandcastle docker build-image`) on the target. Cheap on rerun (Docker layer cache). `--no-build` opts out.
6. **Inject `cd`**: each job's command is wrapped as `cd <project-dir> && npx tsx ...` so it runs inside the synced project directory. Local syncs `cd` into `cwd`; remote syncs `cd ~/.lazycron/projects/<name>/`.
7. **Merge crontab**: existing entries with matching IDs are updated; non-agent entries (created via TUI or hand-written) are left untouched.

## Sync Targets

```bash
lazycron sync                        # local crontab
lazycron sync -s CronWorker          # remote (configured in ~/.lazycron/config.yml)
lazycron sync --no-env               # skip .env transfer
lazycron sync --no-files             # only update crontab; do not transfer
lazycron sync --no-build             # do not build sandcastle image
lazycron sync --skip-deps-check      # skip docker/node/npx check
lazycron sync --project foo          # override project name
```

## Sync Behavior

Sync performs a **safe merge**:

- **New job** (ID not in crontab) → added
- **Existing job** (ID matches, fields changed) → updated
- **Unchanged job** (ID matches, all fields identical) → skipped
- **Jobs not in `.sandcastle/jobs/`** (created via TUI or hand-written) → left untouched

Sync never deletes jobs. Running it twice with no `.ts` changes produces no crontab changes (idempotent).

## TS Job Format

Every file in `.sandcastle/jobs/*.ts` must declare two metadata exports as **double-quoted string literals**:

```typescript
export const cron = "0 9 * * 1-5";
export const name = "Fix Agent";
```

Optional metadata:

```typescript
export const tag = "BP";
export const tagColor = "#f38ba8";
```

The metadata parser is intentionally restrictive: only double-quoted literals are recognised. Backticks, single quotes, and computed expressions are not metadata. This keeps lazycron from executing user TS code at sync time.

After the metadata, the file is regular TypeScript: imports, sandcastle.run(), post-run hooks, anything you like. Lazycron does not run the TS at sync time — it just reads the metadata.

## Job IDs

The filename (minus `.ts`) becomes the job's unique ID. Same rules as before:

- **Allowed characters:** `a-z`, `0-9`, `-`, `_`
- **Cannot** start or end with `-` or `_`
- **Maximum length:** 64 characters
- **Case:** lowercase only

## Repo Provisioning at Run Time

`lazycron sync` does **not** ship your source code — it ships `.lazycron/` and `.sandcastle/`. The agent's TS file is responsible for fetching the actual repo when it runs. Templates use the bundled `ensureRepo()` helper:

```typescript
import { ensureRepo } from "../lib/ensureRepo";

const repoDir = ensureRepo({
  url: process.env.REPO_URL!,
  name: "myproject",
  defaultBranch: "main",
});
process.chdir(repoDir);
```

`ensureRepo` clones once to `~/.lazycron-cache/repos/<name>/`, then `git fetch --prune && git reset --hard origin/main` on every run — so each run starts from a known-clean state, and concurrent runs share the cache. Sandcastle's `branchStrategy` creates a per-run worktree off this base.

## Running Synced Jobs

You can run any synced job immediately by its ID:

```bash
lazycron run fix-agent
```

## Tips

- **Commit `.sandcastle/jobs/`** to your repository so agents are tracked alongside your code.
- **Don't commit `.sandcastle/.env`** — `lazycron init --with-agents` adds it to `.gitignore`.
- **Use `lazycron list`** after syncing to verify jobs look correct.
- **One file per agent** — each `.ts` file defines exactly one cron job.
- **Schedule offsets** — stagger agent times so concurrent docker builds don't fight (e.g. `0 6`, `0 7`, `0 8`).
- **`lazycron diff`** previews crontab changes before sync writes them.
