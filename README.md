# lazycron

A terminal UI for managing cron jobs, inspired by [lazygit](https://github.com/jesseduffield/lazygit). View, create, edit, and run your crontab entries without memorizing cron syntax.

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

### 3-Panel Layout
- **Jobs** — lists all cron jobs with enabled/disabled status and human-readable schedules
- **History** — shows execution history with timestamps, success/failure indicators, and relative times
- **Details** — displays full job info (schedule, command, next 3 runs) or history entry output

Switch panels with `1`/`2`/`3` or arrow keys.

### Job Management
- Create, edit, and delete jobs with a form overlay
- Toggle jobs enabled/disabled with `space`
- Run any job immediately and see its output in a scrollable modal
- Reads and writes directly to your system crontab

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

### Format Migration
Jobs created by older versions can be upgraded to the current recording format with `U`, enabling proper output and exit code capture.

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

## Requirements

- macOS or Linux
- `crontab` command available on PATH

## License

MIT
