# Plan: agent-first lazycron on herdr-core

Status: **done** (2026-11-06, branch `agent-first-herdr`). Owner: Muhammad (swalha1999).
See "Outcome" at the end for what was built, decided and left open.

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
- **herdr-app adoption (read from github.com/herdrdev/herdr-app, master:
  `src/window.rs`, `src/model.rs`, `defaults/AGENTS.md`, `docs/commands.md`).**
  - The app adopts *live* terminals labeled `herdr.space=<id>` on every
    listing (`reconcile`), including other machines' terminals (the id is
    qualified as `<machine>/<id>`). `adoptable()` requires
    `state == "live"`; the app's own test
    `a_finished_terminal_is_not_put_back_in_its_tab` asserts a finished one
    is skipped. Also: "a program exiting closes its pane, even if Herdr was
    closed". **So a 02:00 job that finishes before the app is open would
    never be seen.**
  - An unknown space id becomes a new space **unnamed**; an unnamed space is
    "called after where its first terminal is": `folder_name()` of the first
    terminal's live folder (`foreground.cwd` from the core, its git checkout
    top if inside one). There is no label for a space name. The app has
    `herdr-app do space.rename --id <id> --name <name>` (and `space.new
    --name`), but that only reaches a window running on the same machine;
    Muhammad views ROG14 from the macOS app, so lazycron cannot rely on it.

## Decisions

- **Runs stay live until seen ("hold").** In herdr mode the run terminal runs
  a small wrapper: `cd $HOME; bash <script>`, write the exit code to
  `~/.lazycron/runs/<run>.exit`, print `[lazycron] finished with exit N`,
  then `cd ~/.lazycron/Cron` and sleep for the hold time (config
  `herdr.hold`, default 72h; `0` ends the terminal with the job). The
  terminal is therefore live when herdr-app next connects and gets adopted
  into the Cron space, with the full screen to read; closing its pane ends
  it. `lazycron exec` takes the job's result from the exit file (falling back
  to the terminal's own exit code/signal if it was killed mid-run), so the
  hold never delays history. Retention (`herdr.keep_runs`, default 5 per
  job) kills and removes older *finished* run terminals, held or not;
  running jobs are never touched. Verified: during the hold `hc get` reports
  `foreground.cwd = .../Cron`.
- **Space name "Cron".** Because held terminals sit in `~/.lazycron/Cron`
  (not a git checkout), the space the app creates for the label is called
  "Cron" without any app-side action. When `herdr-app` is installed on the
  same machine and its window is running, `lazycron setup` and each herdr
  run also call `herdr-app do space.rename --id <id> --name Cron`
  (best effort), which makes the name stick even while a running job's
  folder is elsewhere. Caveat: while the space's *first* terminal is a job
  still running in another folder, the automatic name follows that folder
  until it finishes.
- The space id is `sp` + 32 hex, generated once and stored in
  `~/.lazycron/config.yml` (`herdr.space_id`).
- Open: herdr's OSC 7501 wire format is not documented in `hc --skill`, so
  plain (non-agent) scripts do not report `done`; Claude Code jobs do.

## Outcome

All seven steps of the order of work are implemented on `agent-first-herdr`.

1. **JSON / errors / exit codes.** Global `--json` (default when stdout is not
   a TTY, `--json=false` forces text), `--server` and `--cron-file` on every
   command. Errors `{"error":{kind,message,hint}}`; exit codes 1 failure
   (`job_failed` for failed runs), 2 usage, 3 not_found, 4 conflict,
   5 unavailable, 6 timeout (added for `wait --timeout`).
2. **Job commands.** `ls get add edit rm enable disable apply export`, all
   mutating ones with `--dry-run`; `add` is idempotent (by `--id`, else by
   name) and conflicts exit 4; `apply` is declarative (`--prune`, scoped by
   the file's project) and `export | apply -` is a no-op.
3. **herdr backend.** New `runner` package; history entries gain `run_id,
   started_at, exit_code, signal, runner, trigger, terminal, terminal_name,
   output_truncated, fallback_reason` (old entries get a run id from their file
   name); in-progress markers in `~/.lazycron/runs/`; fallback to plain runs;
   retention (`herdr.keep_runs`, default 5); remote servers via the server's
   own `lazycron exec`, `hc` over SSH and `ssh -t` for attach.
4. **Setup.** `lazycron setup [--server X]`, boot service files, linger,
   crontab migration, install.sh herdr-core install (`--no-herdr`,
   `LAZYCRON_NO_HERDR=1`), minimum version 0.1.18 (warning with the
   `hc update --version` hint; setup never runs it).
5. **Agent commands.** `run --no-wait`, `runs`, `logs` (live for running herdr
   runs), `wait`, `kill`, `attach`, `events` (JSON lines), `doctor`,
   `server add/ls/rm/test`, `agent-help`, `skill show/install`. A test keeps
   `agent-help` in parity with the command tree.
6. **TUI.** `a` attaches (jobs panel: the running run; history panel: the
   run's kept terminal); `⟳` on jobs with a run in progress, `⧉` on history
   entries whose terminal is kept; details show runner, exit code, fallback
   and terminal state; one-time offer to install herdr-core when missing.
7. **Docs.** README, `docs/herdr.md`, `docs/cli.md`, `docs/agents.md`.

### Decided along the way

- **Crontab format.** Plain runner: unchanged record line (no migration for
  users without herdr). herdr runner:
  `'<~/.lazycron/bin/lazycron>' exec '<id>' --name '<name>' || { <record line>; }`.
  `exec` exits non-zero only if it could not start the job, so a missing or
  broken lazycron binary still runs and records the job; it prints nothing
  (no cron mail), needs neither crontab on PATH nor a valid config. Per-job
  override `@runner:herdr|plain` in the name comment. `setup` rewrites lines
  whose format does not match their runner; any write migrates legacy lines.
- **Default runner** stays `plain` until `lazycron setup` switches it, so
  existing installs behave exactly as before. Remote servers get their own
  `servers[].runner`.
- **Long jobs / `--timeout`.** `hc run --bg` + `hc wait --output` loop; the
  result comes from the wrapper's exit file, never from `hc run`'s 124.
- **Hold.** Finished runs stay live for `herdr.hold` (72h) in
  `~/.lazycron/Cron` so herdr-app adopts them later and names the space
  "Cron"; see Decisions above. `herdr.labels` adds labels or removes defaults
  (`herdr.space: ""` keeps runs out of herdr-app).
- **Service.** A oneshot unit that runs `hc server start` (the core then runs
  itself as a transient systemd-run unit) rather than running the core
  directly, so lazycron does not own the core's lifecycle.
- **Events** are computed by polling run state (markers + history), so they
  work for plain and herdr runs, locally and over SSH, without depending on
  `hc events`.
- **cron.RunJobNow** and the backends' `RunJob/WriteHistory` were replaced by
  the run engine (`ExecJob`); manual runs from CLI and TUI are recorded
  exactly like scheduled ones.

### Verified

- Unit and CLI tests for every new command and the herdr backend (fake `hc`
  binary in `internal/fakehc`): exit codes, labels, hold, kill, fallback,
  truncation, retention, `--once`, migration, idempotency, dry-runs.
- End-to-end on ROG14 against the live herdr-core with a temp HOME and
  `lazycron-test` labels (all terminals removed afterwards): manual runs,
  a cron-fired line run under `env -i` the way cron runs it, `--no-wait` +
  `kill`, retention, fallback with a bad `hc` path, `doctor`, `setup`
  (`--dry-run`, and a real run limited to the temp HOME), and the TUI's
  badges and attach/detach inside a test terminal.

### Left open

- **No cron on ROG14.** `crontab`/cronie is not installed, so no real crontab
  round-trip was possible (a fake `crontab` + `env -i sh -c '<line>'` stood
  in). Install and enable cronie before moving Hermes jobs
  (`lazycron doctor` reports it).
- **herdr-app naming on macOS** could only be verified from the source, not in
  the app: check that the space appears as "Cron" for ROG14's runs.
- **OSC 7501 for plain scripts.** The wire format is not documented in
  `hc --skill`, so plain jobs report no `done` status; Claude Code jobs do.
- **Remote lazycron install.** `setup --server` links an existing `lazycron`
  on the server but does not install it there.
- **macOS boot.** launchd agents start at login, not at boot.
