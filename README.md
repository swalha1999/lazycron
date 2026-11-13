# lazycron

A terminal UI for managing cron jobs, inspired by [lazygit](https://github.com/jesseduffield/lazygit). View, create, edit, and run your crontab entries without memorizing cron syntax. Comes with built-in job templates and a full, agent-friendly CLI.

Optionally, every run can execute inside a [herdr-core](https://herdr.dev) terminal: cron still decides *when*, but you can watch a run live, open it after the fact or take it over (answer a blocked agent) from the TUI, `hc`, herdr-app or the hirte iPhone app.

![lazycron demo](assets/demo.gif)

## Install

```bash
curl -fsSL https://get.lazycron.com | sh
```

**Other methods:**

```bash
go install github.com/swalha1999/lazycron@latest   # Go
```

The install script also installs **herdr-core** with herdr's official core-only installer if it is missing. herdr is a separate project; skip it with `sh -s -- --no-herdr` or `LAZYCRON_NO_HERDR=1`. Then run:

```bash
lazycron setup --dry-run   # see what it would change
lazycron setup             # herdr-core at boot, linger, herdr runner, crontab rewrite
```

Without herdr-core everything works as plain cron jobs.

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
Job runs are recorded to `~/.lazycron/history/` as JSON files (one per run, named by its run id), capturing output, exit code or signal, timestamps, the runner and the herdr terminal. Runs in progress are tracked in `~/.lazycron/runs/`. History refreshes automatically and is viewable in the History panel; jobs with a run in progress show `⟳`, and history entries whose herdr terminal is still kept show `⧉`.

### Runs in herdr terminals
With the herdr runner (`lazycron setup`, or `--runner herdr` per job), each run starts as a labeled herdr-core terminal:

- **Watch or take over**: press `a` on a job (its running run) or a history entry, or run `lazycron attach <run>`; detach with `ctrl+\`.
- **Seen later**: after the job ends the terminal stays open for `herdr.hold` (default 72h) in `~/.lazycron/Cron`, so herdr-app, which only shows live terminals, still picks up a 02:00 run when you open it in the morning, in a space named "Cron". Close the pane when done.
- **Never lost**: if herdr-core is missing or can't start the run, the job runs plainly and the history entry says why. If the lazycron binary is gone, the crontab line runs the job through the plain record wrapper.
- **Tidy**: only the newest `herdr.keep_runs` (default 5) finished run terminals per job are kept.

See [docs/herdr.md](docs/herdr.md) for details and configuration.

## Keybindings

| Key | Action |
|---|---|
| `n` | New job |
| `enter` / `e` | Edit selected job |
| `d` | Duplicate selected job |
| `D` | Delete selected job (or history entry) |
| `space` | Toggle enable/disable |
| `r` | Run job now |
| `a` | Attach to the run's herdr terminal (jobs / history panel) |
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
lazycron templates ls                      # browse all templates
lazycron templates ls --category ai        # filter by category
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

## Claude Agents

Schedule real Claude Code agents per repo. Each run is an interactive Claude Code session **on this machine**, in a herdr terminal you can watch live or take over, working in a fresh git worktree. No Docker, no sandbox image: the agent uses the machine's own `claude` login, `git`, `gh` and toolchains.

### Setup

```bash
cd ~/code/my-repo
lazycron init --with-agents
```

This writes the following, meant to be committed with the repo (only `.env` and `_pulled/` are ignored, via `.lazycron/.gitignore`):

```
my-repo/
└── .lazycron/
    ├── .gitignore         # .env, _pulled/
    ├── config.yaml        # project name
    ├── context.md         # notes every agent gets: how to build/test, focus, no-go areas
    └── jobs/
        ├── audit.md
        ├── improve.md
        ├── deps.md
        ├── worker.md
        └── upstream-sync.md
```

Edit or delete jobs, fill in `context.md`, preview, then install:

```bash
lazycron agent .lazycron/jobs/audit.md --dry-run   # repo, worktree and the full prompt
lazycron agent .lazycron/jobs/audit.md             # run it now and wait
lazycron sync                                      # one cron entry per jobs/*.md
```

### Bundled agents

| Job | Default schedule | What it does |
|---|---|---|
| `audit` | Sundays 3am | Files up to 3 verified issues (security, bugs, performance, debt), labeled `lazycron`; pauses at 8 open |
| `improve` | Wednesdays 3am | One focused improvement as a PR (`lazycron:improve`), one open at a time |
| `deps` | Daily 4am | CVE audit + patch/minor dependency bumps, checks run, one PR on `lazycron/deps` kept fresh |
| `worker` | Every 4 hours | Takes the oldest `lazycron:ready` (you approved) then `lazycron` issue, claims it with `lazycron:taken`, opens a PR that closes it, or comments and marks it `lazycron:blocked` |
| `upstream-sync` | Daily 5am | Forks only: merges upstream into a `lazycron/sync-upstream` PR, resolving conflicts while keeping fork changes |

`audit` fills the backlog, `worker` drains it. Agents never push to or merge the default branch; you review and merge.

### Job format

A job is Markdown: YAML front matter, then the prompt.

```markdown
---
name: Audit
cron: every sunday at 3am      # cron expression or phrase
model: opus                    # optional: claude --model
permission_mode: auto          # optional (default auto): claude --permission-mode
base: main                     # optional: default is origin's default branch
timeout: 90m                   # optional (default 2h): the session is stopped after it
tag: AU                        # optional, shown in the TUI
---
Audit this repository and file GitHub issues for ...
```

### How a run works

`lazycron agent <job>` (what the cron entry calls) fetches origin, adds a worktree under `~/.lazycron/worktrees/<project>/`, and starts `claude` in a herdr terminal named `agent-<job>-<time>`, with the prompt plus `context.md`. It answers Claude's first-run prompts for that fresh folder (trust, permission mode) and follows the session with `hc wait`:

- **done**: the run succeeds and the Claude session stays open in herdr-app's Cron space, so you can read it or keep talking to it.
- **blocked on a question**: the run fails and the session stays open for you to answer.
- **error or timeout**: the run fails; on timeout the session is stopped.
- **lost track of it** (`hc wait` keeps failing, e.g. herdr-core restarted): brief failures are retried with backoff within the timeout; after 4 in a row the session is stopped, so it can't run on unsupervised, and the run fails naming the terminal. If it can't be stopped, the error says how to stop it by hand.

The worktree is kept in every case, and every run starts Claude with its own `--session-id`, recorded with its last status in the run's history entry (`lazycron logs <run> --json` shows `agent`). `lazycron resume <run-or-job>` puts you in that session: in its terminal while it is open, otherwise (after a timeout or a lost session, say) in a new terminal running `claude --resume` in the run's worktree. Sessions and worktrees of the newest `herdr.keep_runs` runs (default 5) per job are kept; older ones are closed and removed.

### Sync

```bash
lazycron sync                        # local crontab
lazycron sync -s CronWorker          # remote server
lazycron sync --no-env               # don't ship .lazycron/.env
lazycron sync --skip-deps-check      # skip the claude/git/gh/herdr-core check
```

Sync checks the target for `claude`, `git`, `gh` and `herdr-core`, and installs one cron entry per `.lazycron/jobs/*.md`. With `--server` it also copies `.lazycron/` to `~/.lazycron/projects/<project>/` there; set `REPO_URL` in `.lazycron/.env` so the agent can clone the repo on that machine.

Sync is a **safe merge**: it only adds or updates jobs derived from `.lazycron/jobs/*.md`. Jobs created through the TUI are never deleted. Running it again with no changes reports `0 added, 0 updated, N unchanged`. See [docs/agents.md](docs/agents.md) and [docs/sync.md](docs/sync.md).

## CLI

Everything the TUI does is available on the command line, built for scripts and AI agents: JSON output when stdout is not a terminal (`--json` / `--json=false` to force), errors as `{"error":{"kind","message","hint"}}` with meaningful exit codes, stable ids, idempotent commands and `--dry-run` on every change. `--server <name>` targets a remote server on any command.

```bash
lazycron                                 # launch TUI (default)

# jobs
lazycron ls                              # list jobs (alias: list)
lazycron get backup                      # one job + its last run
lazycron add --id backup -n "Backup" -s "every day at 3am" -c "pg_dump mydb > /tmp/backup.sql"
lazycron edit backup --schedule "0 4 * * *"
lazycron disable backup | enable backup | rm backup
lazycron export > jobs.yml               # jobs as a declarative file
lazycron apply jobs.yml [--prune]        # make the machine match it, report the diff

# runs
lazycron run backup                      # run now (waits; exit 1 if it fails)
lazycron run backup --no-wait            # start in the background, print the run id
lazycron wait <run> --timeout 10m
lazycron runs [backup]                   # in progress + finished
lazycron logs <run>                      # output (live while running); logs <job> lists runs
lazycron attach <run>                    # take over the herdr terminal
lazycron kill <run>
lazycron events                          # JSON lines: run.started / run.finished / run.failed

# machine
lazycron setup [--server X] [--dry-run]  # herdr-core, boot service, linger, crontab format
lazycron doctor                          # health check
lazycron server ls | add | rm | test
lazycron agent-help                      # one-read guide for agents
lazycron skill install                   # install it as a Claude Code skill

# Claude agents
lazycron init [--with-agents]
lazycron agent .lazycron/jobs/worker.md [--dry-run]
lazycron sync [-s MyServer] | diff | pull
lazycron templates ls | apply Worker
```

Exit codes: `0` ok, `1` failure (including a failed run), `2` usage, `3` not found, `4` conflict, `5` unavailable, `6` timeout. See [docs/cli.md](docs/cli.md).

## Requirements

- macOS or Linux
- `crontab` command available on PATH and a running cron daemon (`lazycron doctor` checks both)
- Optional: [herdr-core](https://herdr.dev) 0.1.18+ for herdr runs (required for Claude agents)
- For Claude agents: [Claude Code](https://claude.com/claude-code) logged in, `git`, and `gh` logged in

## License

MIT
