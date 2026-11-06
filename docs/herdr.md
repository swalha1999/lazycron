# Runs in herdr terminals

lazycron can run each scheduled job inside a [herdr-core](https://herdr.dev)
(`hc`) terminal. Cron still decides *when*; herdr-core runs the job in a
terminal that stays alive, so a person can watch it live, read it afterwards
or take it over (for example to answer a Claude Code agent that is blocked on
a question) from the lazycron TUI, `hc`, herdr-app or the hirte iPhone app.
Claude Code reports its status itself (OSC 7501), so "needs you" and "done"
notifications in herdr-app work for agent jobs out of the box.

herdr is a separate project. lazycron works without it: jobs then run as
plain cron jobs, exactly as before.

## Getting it

```bash
lazycron setup --dry-run   # what would change
lazycron setup             # do it (idempotent)
lazycron doctor            # check everything a scheduled run needs
```

`setup` (also `setup --server X` for a remote machine):

1. installs herdr-core with herdr's **core-only** installer
   (`https://releases.herdr.dev/core/install.sh`, into `~/.local/bin`) when no
   herdr-core is present. It never updates or restarts an existing core. The
   app installer (`/app/install.sh`) is not used: it installs the desktop app.
   Opt out with `--no-herdr` or `LAZYCRON_NO_HERDR=1`;
2. warns when herdr-core is older than 0.1.18;
3. links `~/.lazycron/bin/lazycron` to the lazycron binary (crontab lines call
   this path, so they keep working wherever lazycron is installed);
4. installs a user service that runs `hc server start` at boot:
   `~/.config/systemd/user/lazycron-herdr.service` on Linux,
   `~/Library/LaunchAgents/dev.lazycron.herdr-core.plist` on macOS
   (`--no-service` skips it);
5. on Linux enables linger (`loginctl enable-linger`) so user services, and
   therefore herdr-core, run while nobody is logged in (`--no-linger` skips);
6. makes `herdr` the default runner (`--runner plain` switches back) and
   gives the machine a stable herdr space id;
7. rewrites crontab lines whose format does not match their runner.

Any `hc` command starts a core on demand, so even without the service a
scheduled run usually finds one; the service and linger make sure a user
systemd instance exists at 02:00 with nobody logged in. On macOS a launchd
agent needs a login session.

## What a run looks like

A herdr-mode crontab line:

```
# Nightly backup @id:backup
0 2 * * * '/home/u/.lazycron/bin/lazycron' exec 'backup' --name 'Nightly backup' || { <plain record line>; }
```

`lazycron exec` then:

1. starts `hc run --bg --no-parent --no-notify --name cron-backup-3f2a
   --label herdr.space=<space id> --label lazycron.job=backup
   --label lazycron.run=<run id> -- bash -c <wrapper> …`;
2. the wrapper runs the job's script from `$HOME` (like cron), writes its exit
   code to `~/.lazycron/runs/<run id>.exit`, prints
   `[lazycron] finished: exit N …`, then **holds** the terminal open in
   `~/.lazycron/Cron` for `herdr.hold`;
3. `lazycron exec` waits with `hc wait --output` (never trusting `hc run`'s
   exit code, which is 124 on its own timeout), reads the output with
   `hc read`, records the history entry and, for `--once` jobs, disables the
   job;
4. keeps the newest `herdr.keep_runs` finished run terminals of the job and
   ends and removes the older ones. Runs still in progress are never touched.

`exec` prints nothing (no cron mail) and exits non-zero only when it could not
start the job. The crontab line then falls back to the plain record wrapper,
so a missing, crashing or incompatible lazycron binary still runs the job.
If herdr-core is missing or cannot start the terminal, `exec` runs the job
plainly itself and records `fallback_reason`.

### Why runs are held open

herdr-app only adopts **live** terminals labeled `herdr.space` (a finished
terminal is never put back, and a pane closes when its program exits, even
while the app is closed). Without the hold, a job that runs at 02:00 while the
app is closed would never appear. Holding the terminal keeps it live until you
look at it in the morning; closing its pane, `lazycron kill <run>`, the hold
running out or retention ends it. The result is already recorded when the job
ends, so the hold never delays `run`, `wait` or history.

### The "Cron" space

herdr-app creates an unknown space id as a new, unnamed space, and names an
unnamed space after its first terminal's folder. Held run terminals sit in
`~/.lazycron/Cron`, so the space shows as **Cron** with no app-side action.
When herdr-app runs on the same machine, `setup` and each run also call
`herdr-app do space.rename --id <id> --name Cron`, which makes the name stick.
While the space's first terminal is a job still running in another folder,
the automatic name follows that folder until it finishes.

### Output

Output is read from the terminal. herdr-core caps scrollback (about 1 100
lines in testing), so very long output is stored truncated with
`output_truncated: true`; the kept terminal and the job's own log files are
the place for the full text. lazycron does not `tee` output, which would take
the TTY away from interactive agents.

## Configuration (`~/.lazycron/config.yml`)

```yaml
runner: herdr            # default runner: plain (default) or herdr
herdr:
  space_id: sp0123…      # herdr-app space for runs (sp + 32 hex), set by setup
  keep_runs: 5           # finished run terminals kept per job
  hold: 72h              # how long a finished run stays open; "0" ends it with the job
  bin: /path/herdr-core  # optional: herdr-core binary (default: PATH, then ~/.local/bin)
  labels:                # optional extra labels; "" removes a default one
    herdr.tab: tb0123…
servers:
  - name: vps
    host: 10.0.0.2
    user: me
    runner: herdr        # runner for jobs written to this server (set by setup --server)
```

A job can override the runner: `lazycron add --runner plain …`,
`lazycron edit <job> --runner herdr`, or `runner:` in an `apply` file. The
override is stored in the crontab comment as `@runner:herdr`.

## Remote servers

`lazycron setup --server X` does the same on the server over SSH (it needs
lazycron installed there for herdr runs: the link step finds `lazycron` on the
server's PATH). Commands with `--server X` run there: `run` calls the
server's `lazycron exec`, `attach` uses `ssh -t … hc attach`, `logs`, `kill`,
`runs` and `events` read the server's state.

## Troubleshooting

- `lazycron doctor` lists what is missing, with hints.
- A run shows `fallback_reason`: herdr-core was missing or refused the run;
  the job ran plainly.
- A run is `lost`: its `lazycron exec` process died (machine restarted
  mid-run). Its terminal may still be readable: `lazycron logs <run>`.
- To stop using herdr: `lazycron setup --runner plain`.
