# Plan: agent-first lazycron on herdr-core

Status: in progress (started 2026-11-06). Owner: Muhammad (swalha1999).

## Why

lazycron replaces Hermes as Muhammad's scheduler for recurring jobs (some are
`claude -p` agents, some plain scripts). Two goals:

1. **Every run is a herdr terminal.** Cron still decides *when*; herdr-core
   runs the job as a labeled terminal so it can be watched live, opened after
   the fact, or taken over (answer a blocked agent) from herdr-app on macOS or
   the hirte iPhone app. Claude Code's native OSC 7501 status means "needs you"
   / "done" notifications come for free.
2. **Agent first.** An agent (Claude) must be able to do everything through the
   CLI: create, edit, enable/disable, delete, run, wait, read output, attach,
   manage servers, diagnose. The TUI becomes a viewer over the same operations.

## Design

### herdr backend
- Crontab line today: `record` wrapper -> `bash ~/.lazycron/scripts/<id>.sh`
  (`cron/parser.go`). In herdr mode the inner command becomes a call to a new
  `lazycron exec <job-id>` which runs the script through herdr-core:
  `hc run --no-parent --name cron-<job>-<ts> --label herdr.space=<cron space id>
  --label lazycron.job=<id> --label lazycron.run=<run-id> -- bash <script>`.
- herdr-app adopts a live terminal labeled with a `herdr.space` id into that
  space (new tab unless `herdr.tab` is also set) and creates the space if the id
  is unknown. Space ids are `sp` + 32 lowercase hex. Use one stable "Cron" space
  id persisted in lazycron config. Verify how herdr-app names a space it creates
  from a label and set the name to "Cron" if there's a way.
- `hc run` stops waiting after `--timeout` (default 180 s) and leaves the job
  running: handle long jobs (e.g. `--bg` + `hc wait`, or a large timeout), and
  verify the exit code is propagated in all cases.
- Cron has no TTY: capture full output via `hc read <t>` (screen + history)
  instead of relying on stdout. Store exit code, output and terminal address in
  the history entry.
- Fallback: if herdr-core is missing or the daemon unreachable, run the job
  plainly exactly like today. A job must never silently not run.
- Retention: keep the last N finished run terminals per job (configurable,
  default 5), `hc rm` older ones.
- Remote servers (lazycron's SSH backend): same behavior on the remote host when
  herdr-core is there.
- Optional, not a hard dependency: plain cron mode keeps working for users
  without herdr. Herdr mode can be a global default with per-job override.

### Install / setup
- `install.sh`: if `herdr-core` is not on PATH or in `~/.local/bin`, install it
  using herdr's official installer. Verify first what
  `https://releases.herdr.dev/app/install.sh` installs (core only on headless
  Linux? does it pull the desktop app?). Opt-out: `--no-herdr` /
  `LAZYCRON_NO_HERDR=1`. Print clearly that herdr is a separate project.
- `lazycron setup`: same check for `go install`/release-download users; also
  makes sure the daemon is *running* at cron time with nobody logged in:
  systemd user service + `loginctl enable-linger` on Linux, launchd agent on
  macOS. Idempotent. Remote: `lazycron setup --server X`.
- Enforce a minimum herdr-core version (`hc update --version X` hint).
- TUI: one-time prompt when herdr-core is missing.

### Agent-first CLI
Rules:
1. Parity: every TUI action has a CLI command.
2. JSON when stdout is not a TTY, `--json` to force; stable ids for jobs, runs,
   servers.
3. Errors as `{"error":{"kind","message","hint"}}` (same shape as `hc`), with
   meaningful exit codes (usage, not found, conflict, failure).
4. Idempotent operations; `lazycron apply <file>` makes the machine match a
   declarative jobs file and reports the diff.
5. `--dry-run` on every mutating command.
6. Self-describing: `lazycron agent-help` (one-read guide for agents) and an
   installable Claude Code skill.

Commands (keep existing names working; add aliases rather than break):

| Area | Commands |
|---|---|
| Jobs | `ls`, `get`, `add`, `edit`, `rm`, `enable`, `disable`, `apply <file>` |
| Runs | `run <job>` (now, via herdr), `runs <job>`, `logs <run>`, `attach <run>`, `kill <run>`, `wait <run>` |
| Servers | `server add/ls/rm/test`, `--server` on every command |
| Setup | `setup`, `doctor` (cron running, herdr-core up, PATH, claude logged in) |
| Watch | `events --json` (run started / finished / failed) |
| Templates | `templates ls/apply` (exists) |

TUI: History panel gets an attach key (`hc attach <terminal>`); show which runs
have a live/kept terminal.

## Order of work
1. JSON output + error format + exit codes across the CLI.
2. Missing job commands (`get/edit/rm/enable/disable/apply`), `--dry-run`.
3. herdr backend (`exec`), history fields, fallback, retention.
4. `setup` + install.sh herdr install + service files.
5. `wait`, `events`, `doctor`, `attach`, `agent-help` + skill.
6. TUI attach key and herdr indicators.
7. Docs (README, docs/agents.md, this plan updated with what was decided).

## Out of scope for this branch
- Migrating Hermes jobs (separate step once this lands).
- Changes to herdr-core or herdr-app.
- The hirte iPhone fork.

## Findings (experiments against herdr-core 0.1.18, 2026-11-06)

Settled with throwaway terminals labeled `lazycron-test` on the live core
(all removed afterwards) and one isolated `HC_SESSION`.

- **Exit codes through `hc run`.** Foreground `hc run` exits with the
  program's exit code (7 -> 7, 3 -> 3) and streams the program's output
  (stdout+stderr, it is a pty) followed by `[hc] t14 exited (exit 7)`.
- **`--timeout`.** When the timeout passes, `hc run` prints
  `[hc] still running as t16: hc wait t16, hc read t16` and exits **124**,
  leaving the program running. 124 is indistinguishable from a program
  that itself exits 124, so lazycron never relies on the foreground exit
  code: it uses `hc run --bg --json` (returns `{"address":"t17",...}` at
  once) and then loops `hc wait <t> --timeout N --json` until
  `"ended":true`. `wait` exits 0 either way; the result is in JSON:
  `exit_code` (null when killed) and `signal` (e.g. 1 after `hc kill`).
  `reason:"timeout"` with `ended:false` is a check-in, not a failure.
  Waiting on an already-finished terminal returns its result immediately.
- **No TTY / cron environment.** `env -i HOME=... PATH=/usr/bin:/bin hc run
  ...` works: hc finds the core socket (`/run/user/<uid>/herdr-core/`)
  without `XDG_RUNTIME_DIR`, and the program gets the *login session's*
  environment (full PATH, DISPLAY, ...), not cron's stripped one.
- **Output capture.** `hc read <t> --json` returns `{complete, how, text}`.
  With `--lines N` it includes history. Scrollback is capped: a 60 000-line
  job returned the last ~1 120 lines with `complete:false`. lazycron stores
  what `hc read --lines 100000` returns and records `output_truncated` when
  `complete` is false; the kept terminal can still be attached. lazycron
  does not `tee` (that would take the TTY away from interactive agents).
- **Core auto-start.** Any `hc` command starts a core when none is running
  (verified with an isolated `HC_SESSION`); on Linux the core runs as a
  transient `systemd-run --user` unit (`hc-serve-*.service`). So at cron
  time the core comes up on demand as long as the user's systemd manager
  exists, which with nobody logged in requires linger.
- **Errors.** `{"error":{"kind","message","hint"}}` on stderr, exit 3 for
  `not_found` (e.g. `hc get nosuch`). lazycron adopts the same shape.
- **Labels / listing.** `hc ls --all --label k=v --json` filters by label
  but does not print labels or creation time; `hc get` does
  (`labels`, `created_ms`, `finished_ms`, `exit_code`, `signal`, `state`).
- **Events.** `hc events --json` prints a snapshot line, then JSON-RPC
  `event` lines with `type` `created` / `finished` (`exit_code`, `reason`).
- **Installers (read, not run).**
  `https://releases.herdr.dev/app/install.sh` installs the **desktop app**
  (Linux: `~/.local/share/herdr-app`, a `.desktop` entry and icon; macOS:
  `~/Applications/Herdr.app`) and the core only if none is present. It is
  not suitable for headless boxes. `https://releases.herdr.dev/core/install.sh`
  installs **only herdr-core** into `~/.local/bin/herdr-core` (+ `hc` link),
  checksum-verified, `HC_VERSION=x` pins a version; if a core is already
  installed it *updates* it (`update --from`, restarting the core on macOS).
  Decision: lazycron uses the core installer, and only when herdr-core is
  missing, so it never updates or restarts an existing core.
- **`herdr.space`.** herdr-core itself has no notion of spaces; the label is
  a herdr-app convention (no herdr-app on this Linux box, no official doc
  found). lazycron sets `herdr.space=<sp + 32 hex>` from config. How the app
  names a space it creates from a label could not be verified here (open).
