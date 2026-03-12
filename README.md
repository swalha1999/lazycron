# lazycron

A TUI cron job manager inspired by [lazygit](https://github.com/jesseduffield/lazygit). Manage your crontab visually from the terminal.

![lazycron screenshot](screenshot.png)

## Install

```bash
go install github.com/swalha1999/lazycron@latest
```

Or build from source:

```bash
git clone https://github.com/swalha1999/lazycron.git
cd lazycron
go build -o lazycron .
```

## Features

- View all cron jobs in a split-panel TUI
- Create, edit, and delete jobs with a form overlay
- Toggle jobs enabled/disabled
- Run jobs immediately
- Human-readable schedule input (`every day at 9am` → `0 9 * * *`)
- Shows next 3 scheduled run times
- Reads/writes directly to your system crontab

## Keybindings

| Key       | Action                |
|-----------|-----------------------|
| `n`       | New job               |
| `enter`   | Edit selected job     |
| `d`       | Delete selected job   |
| `space`   | Toggle enable/disable |
| `r`       | Run job now           |
| `j` / `↓` | Move down            |
| `k` / `↑` | Move up              |
| `?`       | Show all keybindings  |
| `q`       | Quit                  |

## Schedule Input

The schedule field accepts both raw cron expressions and human-readable strings:

| Input                    | Cron Expression |
|--------------------------|-----------------|
| `every minute`           | `* * * * *`     |
| `every hour`             | `0 * * * *`     |
| `every 5 minutes`        | `*/5 * * * *`   |
| `every day at 9am`       | `0 9 * * *`     |
| `every monday at 8:30am` | `30 8 * * 1`    |
| `every weekday at 9am`   | `0 9 * * 1-5`   |
| `0 */2 * * *`            | `0 */2 * * *`   |

## Requirements

- macOS or Linux
- `crontab` command available on PATH

## Roadmap

- **V2**: Claude-powered job scheduling — describe what you want in plain English and let AI generate the cron expression and command.

## License

MIT
