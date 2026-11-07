# Sync — Agent Jobs as Code

`lazycron sync` installs a repo's Claude agent jobs (`.lazycron/jobs/*.md`) into the crontab of this machine or a server. See [agents.md](agents.md) for what a run does.

## Quick Start

```bash
cd ~/code/my-repo
lazycron init --with-agents      # .lazycron/{config.yaml,context.md,jobs/*.md}
lazycron diff                    # what sync would change
lazycron sync                    # install
lazycron ls                      # my-repo-audit, my-repo-worker, ...
```

Add a single bundled job instead of all of them:

```bash
lazycron templates apply Worker
# Created .lazycron/jobs/worker.md
```

## Sync Phases

1. **Resolve project name** from `--project`, then `.lazycron/config.yaml`, then the cwd basename.
2. **Read jobs** from `.lazycron/jobs/*.md`. Each file's front matter (`name`, `cron`, optional `tag`/`tag_color`) becomes a cron entry whose command is `"$HOME/.lazycron/bin/lazycron" agent .lazycron/jobs/<id>.md --project <name>`. A file that doesn't parse stops the sync and names the file.
3. **Deps check** on the target: `claude`, `git`, `gh` and `herdr-core` must be on PATH or in `~/.local/bin`. `--skip-deps-check` overrides.
4. **Transfer files** (remote only): `.lazycron/` is copied to `~/.lazycron/projects/<name>/.lazycron/` on the server, `.env` included unless `--no-env`.
5. **Inject `cd`**: each command runs from the project directory: the repo itself locally, `~/.lazycron/projects/<name>/` on a server.
6. **Merge crontab**: entries with matching IDs are updated; everything else is left alone.

## Sync Targets

```bash
lazycron sync                        # local crontab
lazycron sync -s CronWorker          # remote (configured in ~/.lazycron/config.yml)
lazycron sync --no-env               # don't ship .lazycron/.env
lazycron sync --no-files             # only update the crontab
lazycron sync --skip-deps-check      # skip the claude/git/gh/herdr-core check
lazycron sync --project foo          # override the project name
```

## Sync Behavior

Sync is a **safe merge**:

- **New job** (ID not in crontab) → added
- **Existing job** (ID matches, fields changed) → updated
- **Unchanged job** → skipped
- **Jobs not from `.lazycron/jobs/`** (TUI, hand-written, other projects) → untouched

Sync never deletes jobs; remove a job you dropped with `lazycron rm <id>`. Running it twice with no changes changes nothing.

## Job IDs

The file name minus `.md` is the job id, scoped by project (`worker.md` in project `my-repo` → `my-repo-worker`) so two repos can both have a `worker.md`.

- **Allowed characters:** `a-z`, `0-9`, `-`, `_`
- **Cannot** start or end with `-` or `_`
- **Maximum length:** 64 characters, project prefix included

## Repo at Run Time

Locally the project directory is the checkout, so the agent works on worktrees of your repo (fetched fresh from origin each run). On a server there is no checkout: put the clone URL in `.lazycron/.env`

```
REPO_URL=https://github.com/me/my-repo.git
```

and the first run clones it to `~/.lazycron/repos/<name>/`; later runs fetch.

## Tips

- **`lazycron diff`** previews crontab changes before sync writes them.
- **Stagger schedules** across repos so agents don't all start at once.
- **`lazycron run <id>`** fires a synced job now, through the same runner cron uses.
- **Commit `.lazycron/`** so job and prompt changes are reviewed like code. `init` writes `.lazycron/.gitignore` to keep `.env` and `_pulled/` out.
