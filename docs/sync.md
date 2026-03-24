# Sync — Jobs as Code

Define cron jobs as YAML files in your repository and sync them to any crontab with a single command. This makes cron jobs version-controlled, code-reviewable, and deployable alongside your application.

## Quick Start

1. Create a `.lazycron/jobs/` directory in your project root:

```
my-project/
├── .lazycron/
│   └── jobs/
│       ├── db-backup.yaml
│       └── log-rotate.yaml
├── src/
└── ...
```

2. Add a job file:

```yaml
# .lazycron/jobs/db-backup.yaml
name: Database Backup
schedule: "0 3 * * *"
command: pg_dump mydb | gzip > /backups/db_$(date +%F).sql.gz
project: backend
```

3. Run sync from the project root:

```bash
lazycron sync
# Synced: 1 added, 0 updated, 0 unchanged
```

## Sync Targets

Sync to different targets from the same YAML files:

```bash
lazycron sync                        # local crontab
lazycron sync -s CronWorker          # remote server (short flag)
lazycron sync --server CronWorker    # remote server (long flag)
lazycron sync --dir /path/to/.lazycron  # custom directory
```

Remote servers must be configured in `~/.lazycron/config.yml`. See the TUI's server management for setup.

## Sync Behavior

Sync performs a **safe merge**:

- **New job** (ID not in crontab) → added
- **Existing job** (ID matches, fields changed) → updated
- **Unchanged job** (ID matches, all fields identical) → skipped
- **Jobs not in YAML** (created via TUI or other tools) → left untouched

Sync never deletes jobs. It only adds or updates jobs that have matching YAML files.

Running sync is **idempotent** — running it twice with no YAML changes produces no crontab changes:

```
$ lazycron sync
Synced: 0 added, 0 updated, 2 unchanged
```

## YAML Format

### Required Fields

| Field      | Description                                |
|------------|--------------------------------------------|
| `name`     | Display name shown in the TUI and CLI      |
| `schedule` | Cron expression or human-readable schedule |
| `command`  | Shell command to execute                   |

### Optional Fields

| Field       | Default | Description                                |
|-------------|---------|-------------------------------------------|
| `project`   |         | Project group for organizing jobs in the TUI |
| `tag`       |         | Short label displayed next to the job name |
| `tag_color` |         | Hex color for the tag (e.g. `"#a6e3a1"`)  |
| `enabled`   | `true`  | Set to `false` to create a disabled job    |
| `once`      | `false` | Set to `true` for one-shot jobs that auto-disable after running |

### Schedule Format

The `schedule` field accepts both cron expressions and human-readable strings:

```yaml
schedule: "0 3 * * *"              # cron expression
schedule: "every day at 3am"       # human-readable
schedule: "every weekday at 9am"   # human-readable
schedule: "*/30 * * * *"           # every 30 minutes
```

## Job IDs

The filename (minus the `.yaml` extension) becomes the job's unique ID. IDs must follow these rules:

- **Allowed characters:** `a-z`, `0-9`, `-`, `_`
- **Cannot** start or end with `-` or `_`
- **Maximum length:** 64 characters
- **Case:** lowercase only

Good examples:
```
db-backup.yaml          → ID: db-backup
log-rotate-weekly.yaml  → ID: log-rotate-weekly
salati_cleanup.yaml     → ID: salati_cleanup
```

Invalid examples:
```
DB-Backup.yaml          → error: uppercase not allowed
my job.yaml             → error: spaces not allowed
-leading-dash.yaml      → error: cannot start with dash
```

Jobs created through the TUI get auto-generated 8-character hex IDs (e.g. `a1b2c3d4`). Both slug IDs and hex IDs coexist in the same crontab.

## Running Synced Jobs

You can run any synced job immediately by its ID:

```bash
lazycron run db-backup
```

The `run` command accepts both job names and IDs.

## Examples

### AI Agent Jobs

Schedule AI agents that open issues and PRs for your project:

```yaml
# .lazycron/jobs/code-quality-agent.yaml
name: Code Quality Agent
schedule: "0 7 * * *"
command: >-
  cd /home/user/my-project &&
  claude -p "Analyze the codebase for the ONE highest-impact
  code quality improvement and open a GitHub issue."
project: ai-agents
tag: BP
tag_color: "#f38ba8"
```

### Multi-Command Jobs

Use YAML block scalars for complex commands:

```yaml
# .lazycron/jobs/deploy-check.yaml
name: Deploy Health Check
schedule: "*/5 * * * *"
command: >-
  STATUS=$(curl -s -o /dev/null -w '%{http_code}' https://myapp.com/health) &&
  if [ "$STATUS" != "200" ]; then
    echo "Health check failed: $STATUS" | mail -s "Deploy Alert" team@example.com;
  fi
project: monitoring
tag: SYS
tag_color: "#89b4fa"
```

### Disabled Jobs

Create a job in disabled state — useful for preparing jobs before enabling them:

```yaml
# .lazycron/jobs/migration-job.yaml
name: Database Migration
schedule: "0 2 * * 0"
command: cd /app && ./migrate.sh
enabled: false
project: backend
```

## Tips

- **Commit `.lazycron/jobs/`** to your repository so jobs are tracked alongside your code.
- **Use `lazycron list`** after syncing to verify jobs look correct.
- **One file per job** — each `.yaml` file defines exactly one cron job.
- **Use projects** to group related jobs together in the TUI.
- **Schedule offsets** — stagger job times to avoid running multiple jobs simultaneously (e.g. `:00`, `:10`, `:20`).
