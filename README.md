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

## Sync — Jobs as Code

Define cron jobs as YAML files in your repo and sync them to any crontab. Version-controlled, reviewable, deployable.

### Setup

Create a `.lazycron/jobs/` directory in your project and add YAML files — the filename becomes the job ID:

```yaml
# .lazycron/jobs/db-backup.yaml
name: Database Backup
schedule: "0 3 * * *"
command: pg_dump mydb | gzip > /backups/db_$(date +%F).sql.gz
project: backend
tag: DB
tag_color: "#a6e3a1"
```

```yaml
# .lazycron/jobs/log-rotate.yaml
name: Log Rotation
schedule: "0 0 * * 0"
command: find /var/log/myapp -name '*.log' -mtime +7 -delete
project: backend
```

### Sync

```bash
lazycron sync                        # sync to local crontab
lazycron sync -s CronWorker          # sync to a remote server
lazycron sync --dir /path/to/.lazycron  # custom directory
```

Sync is a **safe merge** — it only adds or updates jobs defined in the YAML files. Existing jobs created through the TUI are never deleted.

Running sync again with no changes is idempotent:

```
$ lazycron sync
Synced: 0 added, 0 updated, 2 unchanged
```

### YAML Fields

| Field      | Required | Description                                |
|------------|----------|--------------------------------------------|
| `name`     | yes      | Display name for the job                   |
| `schedule` | yes      | Cron expression or human-readable schedule |
| `command`  | yes      | Shell command to run                       |
| `project`  | no       | Project group for organizing jobs          |
| `tag`      | no       | Short label displayed next to the name     |
| `tag_color`| no       | Hex color for the tag (e.g. `"#a6e3a1"`)  |
| `enabled`  | no       | `true` (default) or `false`                |
| `once`     | no       | `true` for one-shot jobs                   |

### Job IDs

The filename (minus `.yaml`) is the job ID. IDs must be lowercase, using only `a-z`, `0-9`, `-`, and `_`. Examples: `db-backup`, `salati_cleanup`, `log-rotate-weekly`.

Jobs created through the TUI get auto-generated hex IDs. Both formats coexist in the same crontab.

See [docs/sync.md](docs/sync.md) for the full sync reference.

## CLI

```bash
lazycron                    # launch TUI (default)
lazycron list               # list all cron jobs
lazycron add -n "backup" -s "every day at 3am" -c "pg_dump mydb > /tmp/backup.sql"
lazycron run "backup"       # run a job by name or ID
lazycron sync               # sync jobs from .lazycron/jobs/
lazycron sync -s MyServer   # sync to a remote server
lazycron templates list     # browse templates
lazycron templates apply "Backup Database"  # apply a template
lazycron --version          # show version
```

## Requirements

- macOS or Linux
- `crontab` command available on PATH

## License

MIT
