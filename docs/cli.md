# CLI reference

Every TUI action has a CLI command, so scripts and AI agents can do
everything: create, edit, enable/disable, delete, run, wait, read output,
attach, manage servers and diagnose. The complete contract (JSON shapes,
jobs-file format, recipes) is printed by `lazycron agent-help`; its source is
[`cmd/agenthelp.md`](../cmd/agenthelp.md). `lazycron skill install` installs
it as a Claude Code skill (`~/.claude/skills/lazycron/SKILL.md`).

## Conventions

- **Output**: JSON when stdout is not a terminal; `--json` forces JSON,
  `--json=false` forces text. Progress messages go to stderr in JSON mode.
  `exec` and `export` print YAML/nothing unless `--json` is given.
- **Errors**: `{"error":{"kind","message","hint"}}` on stderr (the same shape
  as herdr-core's `hc`).
- **Exit codes**: `0` ok · `1` failure (`failure`; `job_failed` when a run
  failed) · `2` usage · `3` not_found · `4` conflict · `5` unavailable
  (crontab, herdr-core or a server unreachable) · `6` timeout.
- **Ids**: jobs have stable ids (`--id`, `[a-z0-9_-]`); runs have ids like
  `2026-11-06T02-00-00_backup_3f2a`; servers have names. A job can be given by
  id or exact name; where a run is expected, a job means its newest run.
- **Idempotent**: repeating a change reports `"action":"unchanged"`;
  conflicting changes exit 4. Every mutating command takes `--dry-run`.
- **Targets**: `--server <name>` on any command; `--cron-file <file>` uses a
  plain file instead of the system crontab (testing; scripts and runs are kept
  beside the file).

## Commands

| Area | Command | Notes |
|---|---|---|
| Jobs | `ls` (`list`) `[-p project]` | |
| | `get <job>` | job + `last_run` |
| | `add -n NAME -s SCHEDULE -c CMD [--id] [-w DIR] [-p PROJECT] [--tag] [--tag-color] [--runner] [--once] [--disabled]` | idempotent by id (or name) |
| | `edit <job> [flags as add]` | only passed flags change |
| | `enable <job>`, `disable <job>`, `rm <job>` (`delete`) | |
| | `export [-p project]` | YAML jobs file (`--json` for JSON) |
| | `apply <file\|-> [--prune]` | create/update to match; `--prune` deletes unlisted jobs in scope |
| Runs | `run <job> [--no-wait\|--bg]` | waits by default; exit 1 if the job failed |
| | `runs [job] [-n N]` | newest first, in progress included |
| | `logs <run\|job> [-o] [-n N]` | a run's output (live while running) or a job's runs |
| | `wait <run\|job> [--timeout D]` | exit 0/1 by result, 6 on timeout |
| | `kill <run\|job>` | ends a run (process group or herdr terminal) or closes a held terminal |
| | `attach <run\|job> [--observe]` | `hc attach` to the run's terminal; needs a TTY |
| | `events [--job J] [--count N] [--for D]` | JSON lines `run.started` / `run.finished` / `run.failed` |
| | `exec <job-id>` | what herdr-mode crontab lines call |
| Servers | `server add --name --host --user [--port] [--key] [--agent]`, `server ls`, `server rm <name>`, `server test <name>` | |
| Setup | `setup [--dry-run] [--no-herdr] [--no-service] [--no-linger] [--runner]` | see [herdr.md](herdr.md) |
| | `doctor` | exit 1 when a check failed |
| Agents | `agent-help`, `skill show`, `skill install [--dir]` | |
| Templates | `templates ls [-c category]`, `templates apply <name>` | |
| Claude agents | `init`, `agent`, `sync`, `diff`, `pull` | see [agents.md](agents.md), [sync.md](sync.md) |

## Jobs file

```yaml
project: ops              # optional: --prune scope and default project
jobs:
  - id: backup            # optional, derived from the name
    name: Nightly backup
    schedule: every day at 2am
    command: restic backup ~/data
    workdir: /srv         # optional
    runner: herdr         # optional
    enabled: true         # optional
    once: false           # optional
```

Names, tags, tag colors and projects must not contain newlines, other control
characters or `%` (cron ends a command at `%`); such jobs are rejected with a
usage error.

`lazycron export | lazycron apply -` is a no-op; the pair is the way to keep
jobs in git and converge a machine to them.
