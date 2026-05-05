# lazycron

A terminal UI for managing cron jobs, inspired by [lazygit](https://github.com/jesseduffield/lazygit). View, create, edit, and run your crontab entries without memorizing cron syntax. Comes with built-in job templates and a full CLI.

![lazycron demo](assets/demo.gif)

## Install

```bash
curl -fsSL https://get.lazycron.com | sh
```

**Other methods:**

```bash
go install github.com/swalha1999/lazycron@latest   # Go
```

**macOS Users:** On first run, macOS will ask you to allow your terminal to "administer your computer." Click Allow — this is needed for crontab access. If the prompt keeps appearing, go to System Settings → Privacy & Security → App Management and add your terminal app.

## Features

### 4-Panel Layout
- **Servers** — local + remote SSH servers
- **Jobs** — lists all cron jobs with enabled/disabled status and human-readable schedules
- **History** — shows execution history with timestamps, success/failure indicators, and relative times
- **Details** — displays full job info (schedule, command, next 3 runs) or history entry output with scrolling

Switch panels with `1`/`2`/`3`/`4` or arrow keys.

### Job Management
- Create jobs from scratch or from built-in templates (`n` → `b` blank / `t` template)
- Edit, delete, toggle enabled/disabled, and run jobs immediately
- Reads and writes directly to your system crontab
- Manage cron jobs on remote servers via SSH

### Human-Readable Schedules
The schedule field accepts plain English alongside raw cron expressions:

| Input                    | Cron Expression |
|--------------------------|-----------------|
| `every minute`           | `* * * * *`     |
| `every hour`             | `0 * * * *`     |
| `every 5 minutes`        | `*/5 * * * *`   |
| `every day at 9am`       | `0 9 * * *`     |
| `every monday at 8:30am` | `30 8 * * 1`    |
| `every weekday at 9am`   | `0 9 * * 1-5`   |
| `0 */2 * * *`            | `0 */2 * * *`   |

### Visual Cron Picker
An interactive 5-column picker for building cron expressions field by field. Supports wildcard (`*`), interval (`*/N`), and specific value modes with live preview.

### Directory Completer
The work directory field includes a fuzzy-matching directory browser with drill-in/drill-out navigation, breadcrumbs, and scroll support.

### Execution History
Job runs are recorded to `~/.lazycron/history/` as JSON files, capturing output, exit codes, and timestamps. History refreshes automatically and is viewable in the History panel.

## Keybindings

| Key | Action |
|---|---|
| `n` | New job |
| `enter` / `e` | Edit selected job |
| `d` | Delete selected job |
| `space` | Toggle enable/disable |
| `r` | Run job now |
| `U` | Update job to latest format |
| `R` | Refresh from crontab |
| `1` / `2` / `3` | Switch panel |
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `?` | Show all keybindings |
| `q` | Quit |

## Templates

Press `n` then `t` to create a job from a built-in template, or use the CLI:

```bash
lazycron templates list                    # browse all templates
lazycron templates list --category ai      # filter by category
lazycron templates apply "Claude Code Review"  # apply interactively
```

| Category   | Template            | Description                                      |
|------------|---------------------|--------------------------------------------------|
| DevOps     | Backup Database     | Dump a PostgreSQL database to a timestamped file  |
| DevOps     | Log Rotation        | Compress and rotate logs older than N days        |
| AI / LLM   | Claude Code Review  | Run Claude Code to review recent repo changes     |
| AI / LLM   | Claude Test Runner  | Run Claude Code to execute and analyze tests      |
| Git / CI   | Auto Pull Repos     | Pull latest changes from remote on a schedule     |
| Monitoring | HTTP Health Check   | Ping an HTTP endpoint and log failures            |
| System     | Disk Cleanup        | Remove old temp files to free disk space          |
| Lazycron   | Clean History       | Delete lazycron history entries older than N days  |

You can also [create your own templates](docs/templates.md).

## Sync — Sandcastle Agents as Code

Schedule sandboxed AI agents from your repo. Lazycron pairs with [sandcastle](https://github.com/mattpocock/sandcastle) to run Claude Code in Docker containers with per-run git worktrees, then ships everything to your target machine and installs cron entries.

### Setup

```bash
lazycron init --with-agents
```

This scaffolds `.lazycron/config.yaml` and a sibling `.sandcastle/` (Dockerfile, `.env.example`, `lib/ensureRepo.ts`, plus all bundled agent templates under `jobs/`). Defaults: Claude Code agent, Docker sandbox, GitHub Issues backlog. No prompts, no `npx sandcastle init` call.

Apply an agent template:

```bash
lazycron templates apply fix-agent
# Created .sandcastle/jobs/fix-agent.ts
```

Each `.sandcastle/jobs/*.ts` file is self-describing — it declares its cron schedule and name as `export const`s and contains the agent's prompt + post-run actions.

### Sync

```bash
lazycron sync                        # local crontab
lazycron sync -s CronWorker          # remote server
lazycron sync --no-env               # skip .env transfer
lazycron sync --no-build             # skip docker build
lazycron sync --skip-deps-check      # skip docker/node/npx check
```

`lazycron sync` checks docker/node/npx on the target, tars `.lazycron/` and `.sandcastle/` over SSH, runs `npx @ai-hero/sandcastle docker build-image` on the target, and installs cron entries that `cd` into the synced project directory before launching each agent.

Sync is a **safe merge** — it only adds or updates jobs derived from `.sandcastle/jobs/*.ts`. Existing jobs created through the TUI are never deleted.

Running sync again with no changes is idempotent:

```
$ lazycron sync
Synced: 0 added, 0 updated, 2 unchanged
```

### TS Job Format

Each `.sandcastle/jobs/*.ts` file declares two metadata exports as **double-quoted string literals**:

```typescript
export const cron = "0 9 * * 1-5";
export const name = "Fix Agent";
export const tag = "BP";          // optional
export const tagColor = "#f38ba8"; // optional

// rest of the file: imports, sandcastle.run, post-run hooks
```

Lazycron does **not** execute user TS at sync time — it only reads metadata. See [docs/agents.md](docs/agents.md) for the full agent lifecycle and [docs/sync.md](docs/sync.md) for the sync reference.

### Project Layout

```
my-repo/
├── .lazycron/config.yaml            # project name (and future per-project config)
└── .sandcastle/
    ├── Dockerfile, .env, .env.example
    ├── lib/ensureRepo.ts            # bundled helper for clone + fetch
    └── jobs/
        ├── fix-agent.ts
        └── code-quality-agent.ts
```

## CLI

```bash
lazycron                    # launch TUI (default)
lazycron init               # scaffold .lazycron/ in current dir
lazycron init --with-agents # also scaffold .sandcastle/ (Claude Code + Docker + bundled agent templates)
lazycron list               # list all cron jobs
lazycron add -n "backup" -s "every day at 3am" -c "pg_dump mydb > /tmp/backup.sql"
lazycron run "backup"       # run a job by name or ID
lazycron sync               # sync .sandcastle/jobs/*.ts to local crontab
lazycron sync -s MyServer   # sync to a remote server (ships files + builds image)
lazycron templates list     # browse templates
lazycron templates apply "fix-agent"        # scaffold a sandcastle agent template
lazycron --version          # show version
```

## Requirements

- macOS or Linux
- `crontab` command available on PATH

## License

MIT
