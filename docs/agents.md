# Claude Agents

Lazycron schedules real Claude Code agents. Each run is an interactive Claude Code session on the machine cron fires on, inside a herdr terminal you can watch live, read later, or take over. Every run gets a fresh git worktree, so runs never touch your checkout or each other. There is no container: agents use the machine's own `claude` login, `git`, `gh` and toolchains, so run them on a machine you dedicate to this.

## Lifecycle

```
lazycron init --with-agents
        │
        ▼
.lazycron/config.yaml, context.md, jobs/{audit,improve,deps,worker,upstream-sync}.md
        │
        │  edit context.md and the jobs; preview with `lazycron agent <job> --dry-run`
        │
        │  lazycron sync [--server X]
        ▼
crontab entry per job: cd <project> && lazycron agent .lazycron/jobs/<job>.md --project <name>
        │
        │  cron fires (inside a herdr run terminal)
        ▼
1. git fetch origin; worktree at ~/.lazycron/worktrees/<project>/<job>-<time> from origin's default branch
2. hc run: claude --permission-mode <mode> --session-id <uuid> [--model M] "<prompt + context.md>" in terminal agent-<job>-<time>
3. first-run prompts for the new folder (trust, permission mode) are answered
4. hc wait follows the session:
     done     → session stays open, run succeeds
     blocked  → session stays open, run fails with the terminal to attach to
     error    → session stays open, run fails
     timeout  → session stopped, run fails
     hc wait keeps failing (4 in a row, retried with backoff within the timeout)
              → session stopped, run fails naming the terminal (status "lost")
   the worktree is kept in every case
```

Each run's history entry records its session under `agent`: terminal, session id, worktree and the status above. `lazycron resume <run-or-job>` continues it: in its terminal while that is open, otherwise in a new terminal running `claude --resume` in the run's worktree. So a session that was stopped (timeout, or lazycron lost track of it) loses nothing the agent had written. When a lost session can't be stopped either, the run's error says how to stop it by hand (`hc kill <terminal>`). Sessions and worktrees of the newest `herdr.keep_runs` runs (default 5) per job are kept; older ones are closed and removed.

The agent does the GitHub work itself with `gh` (issues, branches, pushes, PRs). The wrapping prompt tells it never to push to or merge the default branch.

## Requirements on the machine that runs agents

- **Claude Code** logged in (`claude auth status`), and `lazycron doctor` reporting ok
- **lazycron** and **herdr-core** installed: `lazycron setup` (or `lazycron setup --server X`) installs both; on a server lazycron is installed globally with `curl -fsSL https://get.lazycron.com | sh`
- **git** and **gh** logged in with push access to the repos
- The toolchains your repos build with (node/pnpm, go, cargo, zig, ...). Agents get your PATH plus `~/.local/bin` and mise shims.

## Job file

`.lazycron/jobs/<id>.md`. The file name is the job id (`a-z 0-9 _ -`); sync scopes it by project, e.g. `my-repo-worker`.

```markdown
---
name: Worker                   # required
cron: every 4 hours            # required: cron expression or phrase
description: Implements issues # optional, shown in `templates ls`
model: opus                    # optional: claude --model
permission_mode: auto          # optional (default auto): auto, acceptEdits, bypassPermissions, ...
base: main                     # optional: branch to start from (default: origin's default branch)
timeout: 3h                    # optional (default 2h)
tag: WK                        # optional TUI tag
tag_color: "#f38ba8"           # optional
---
The prompt. Everything after the front matter is sent to Claude as the task.
```

`auto` is the default permission mode: the agent runs without prompting and risky actions are checked by Claude Code's classifier. Use `bypassPermissions` only on a machine where that is acceptable.

## context.md

Per-repo notes added to every job's prompt: what the project is, how to install, build and test it, what to focus on, what never to touch, and for forks which upstream to sync from. Short and concrete beats long.

## Bundled jobs and labels

| Job | Output |
|---|---|
| `audit` | Up to 3 issues labeled `lazycron` + a kind (`security`, `bug`, `performance`, `tech-debt`); skips when 8 are open |
| `improve` | One PR labeled `lazycron:improve` at a time |
| `deps` | One PR from `lazycron/deps`, refreshed each run |
| `worker` | PR closing the issue it took; `lazycron:ready` issues go first, claimed with `lazycron:taken`, `lazycron:blocked` when it needs you |
| `upstream-sync` | PR from `lazycron/sync-upstream` for forks; stands down when the repo isn't a fork |

Label an issue `lazycron:ready` to hand it to the worker; remove `lazycron:blocked` to let it try again.

## Watching and taking over

```sh
hc ls                          # live terminals, agents included (agent-<job>-<time>)
hc attach agent-worker-1107040000   # watch or type; ctrl+\ detaches
lazycron runs my-repo-worker   # run history; a blocked run says which terminal to attach to
lazycron logs my-repo-worker -o
```

herdr-app shows the same terminals.

## Running one now

```sh
lazycron agent .lazycron/jobs/audit.md --dry-run   # resolve repo/base/worktree, print the full prompt
lazycron agent .lazycron/jobs/audit.md             # start it and wait
lazycron run my-repo-audit                         # the synced cron job, through the lazycron runner
```

## Writing your own

Copy a bundled job and change the prompt. Keep it to one outcome per run, say where results go (an issue, a PR branch name, a label), and say when to stand down so idle runs are cheap.
