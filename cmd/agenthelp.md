# lazycron for agents

lazycron manages cron jobs. Cron decides *when*; each run executes either
plainly or, in **herdr mode**, inside a herdr-core (`hc`) terminal that people
can watch live, open later or take over (herdr-app shows them in a space
named "Cron"). Everything the TUI does is available here.

## Rules of the CLI

- Output is JSON when stdout is not a terminal (always, for you). `--json`
  forces it, `--json=false` forces text.
- Errors go to stderr as `{"error":{"kind","message","hint"}}`, with exit
  codes: `0` ok, `1` failure (`failure`, or `job_failed` when a run failed),
  `2` usage, `3` not_found, `4` conflict, `5` unavailable (crontab, herdr-core
  or a server cannot be reached), `6` timeout (`wait --timeout`).
- Stable ids: jobs have an `id` (`[a-z0-9_-]`, set it with `--id`), runs have
  an `id` like `2026-11-06T02-00-00_backup_3f2a`, servers have a `name`.
  Anywhere a job is expected, its id or exact name works; anywhere a run is
  expected, a job means "its newest run".
- Mutating commands take `--dry-run` and are idempotent: repeating one
  reports `"action":"unchanged"`. A conflicting change exits 4.
- `--server <name>` runs any command against a configured remote server.

## Jobs

```sh
lazycron ls                                   # {"jobs":[...]}
lazycron get <job>                            # job + last_run
lazycron add --id backup --name "Nightly backup" \
  --schedule "every day at 2am" --command "restic backup ~/data" \
  [--workdir DIR] [--project P] [--runner herdr|plain] [--once] [--disabled]
lazycron edit <job> --schedule "0 3 * * *"    # only the flags you pass change
lazycron enable <job> | disable <job> | rm <job>
lazycron export > jobs.yml                    # current jobs as a jobs file
lazycron apply jobs.yml [--prune] [--dry-run] # make the machine match it
```

Schedules are cron expressions or phrases: `every minute`, `every hour`,
`every 15 minutes`, `every 2 hours`, `every day at 2am`, `every weekday at
9:30`, `every monday at 8am`. With `--once` the schedule is a date/time
(`2026-12-01 09:00`, `tomorrow 9am`) and the job disables itself after running.

A job's JSON: `id, name, schedule, schedule_human, command, enabled, once,
tag, project, runner, runner_override, current_format, next_runs`.

Jobs file (YAML or JSON) for `apply`:

```yaml
project: ops              # optional: scope for --prune, default project of jobs
jobs:
  - id: backup            # optional, derived from name
    name: Nightly backup
    schedule: every day at 2am
    command: restic backup ~/data
    workdir: /srv         # optional
    runner: herdr         # optional: herdr | plain (default from config)
    enabled: true         # optional
    once: false           # optional
```

`apply` reports `{"entries":[{"kind":"new|updated|unchanged|deleted",...}],
"new","updated","deleted","unchanged","changed"}`.

## Runs

```sh
lazycron run <job>              # run now and wait; exit 1 if the job failed
lazycron run <job> --no-wait    # start in the background, print the run
lazycron wait <run|job> [--timeout 10m]
lazycron runs [job] [-n 20]     # newest first, in progress included
lazycron logs <run>             # output (live from the terminal while running)
lazycron logs <job>             # recent runs; -o for the newest run's output
lazycron kill <run|job>         # stop a run, or close a finished run's held terminal
lazycron events [--job J] [--count N] [--for 1h]   # JSON lines as runs start/end
```

A run's JSON: `id, job_id, job_name, state (running|succeeded|failed|lost|
unknown), runner (herdr|plain), trigger (cron|manual), started_at,
finished_at, duration_ms, exit_code, signal, terminal, terminal_name,
output_truncated, fallback_reason` (+ `output` from `run` and `logs`).
Events: `{"type":"run.started|run.finished|run.failed","at":..,"run":{..}}`.

In herdr mode the run's terminal stays open after the job ends (default
72h, `herdr.hold` in config) so it can be opened later; `terminal` is its
hc address (`hc read <t>`, `hc attach <t>` work too). Scrollback is capped by
herdr-core: very long output is stored with `output_truncated: true`. If
herdr-core is missing or cannot start the run, the job still runs plainly and
`fallback_reason` says why.

Do not use `lazycron attach` (it needs a person at a TTY): read with `logs`,
wait with `wait`. When a herdr run is blocked on a question, tell your user
the run id; they can `lazycron attach <run>` or open it in herdr-app.

## Claude agents

Scheduled agents are real interactive Claude Code sessions that run on this
machine in a herdr terminal (no Docker). Each repo keeps its own in
`.lazycron/`, committed with the repo (`init` ignores only `.env` and `_pulled/`):

```sh
lazycron init --with-agents          # .lazycron/jobs/{audit,improve,deps,worker,upstream-sync}.md + context.md
lazycron agent .lazycron/jobs/audit.md --dry-run   # repo, worktree and full prompt; starts nothing
lazycron agent .lazycron/jobs/audit.md             # run one now and wait
lazycron sync [--server N]           # one cron entry per jobs/*.md
```

A job file is YAML front matter + the prompt as the body:

```markdown
---
name: Audit
cron: every sunday at 3am      # cron expression or phrase
model: opus                    # optional, claude --model
permission_mode: auto          # optional (default auto), claude --permission-mode
base: main                     # optional, default origin's default branch
timeout: 90m                   # optional (default 2h), then the session is stopped
---
Audit this repository and file issues for ...
```

Each run fetches origin, makes a fresh worktree under
`~/.lazycron/worktrees/<project>/`, and starts `claude` with the prompt (plus
`.lazycron/context.md`, the per-repo notes) in a terminal named
`agent-<job>-<time>`. When the agent finishes its turn the terminal is closed
and the worktree removed. If it blocks on a question, the run fails, the
terminal stays open and the worktree is kept: tell your user the terminal so
they can `hc attach <t>` (or open it in herdr-app) and answer.

## Machine

```sh
lazycron doctor                 # cron, herdr-core, boot service, linger, claude
lazycron setup [--dry-run]      # install/enable herdr mode (idempotent)
lazycron server ls | add --name N --host H --user U | rm N | test N
lazycron templates ls | apply <name>
```

`setup` installs herdr-core (a separate project) only if missing, never
updates or restarts a running core, adds a user service that starts the core
at boot plus linger, makes herdr the default runner and rewrites the crontab.
`--runner plain` switches back. Config lives in `~/.lazycron/config.yml`
(`runner`, `herdr.keep_runs`, `herdr.hold`, `herdr.space_id`, `servers`).

## Recipes

Create or update a job and check it works:

```sh
lazycron add --id report --name Report --schedule "every weekday at 9am" \
  --command 'claude -p "Summarize the files in ~/logs" > ~/report.md'
lazycron run report --no-wait | jq -r .id      # -> RUN
lazycron wait RUN --timeout 30m; lazycron logs RUN
```

Find out why a job failed: `lazycron runs <job> -n 5`, then
`lazycron logs <run>`; `lazycron doctor` if runs are missing or `lost`.
