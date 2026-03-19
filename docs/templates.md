# Custom Templates

Add your own templates as YAML files in `~/.lazycron/templates/`. They appear alongside built-in templates in both the TUI and CLI.

A template with the same name as a built-in overrides it.

## Template Format

```yaml
name: My Template
category: devops          # devops, ai, git, monitoring, system, lazycron
description: Short description of what this template does
schedule: "0 3 */$INTERVAL * *"
command: "cd $REPO_PATH && run-backup --db $DB_NAME"
variables:
  - name: INTERVAL
    prompt: "Run every N days"
    default: "1"
  - name: REPO_PATH
    prompt: "Repository path"
    default: /home/user/project
  - name: DB_NAME
    prompt: "Database name"
    default: mydb
```

## Fields

| Field         | Required | Description                                             |
|---------------|----------|---------------------------------------------------------|
| `name`        | yes      | Display name, used as the default job name              |
| `category`    | no       | Groups templates in the picker and `--category` filter  |
| `description` | no       | Shown below the template name in the picker             |
| `schedule`    | yes      | Cron expression — can contain `$VARIABLES`              |
| `command`     | yes      | Shell command — can contain `$VARIABLES`                |
| `variables`   | no       | List of customizable parameters                         |

## Variable Fields

| Field     | Required | Description                                    |
|-----------|----------|------------------------------------------------|
| `name`    | yes      | Variable name, referenced as `$NAME` in schedule/command |
| `prompt`  | no       | Label shown to the user when filling in the value |
| `default` | no       | Pre-filled value if the user leaves it blank    |

Variables named with `PATH`, `DIR`, `DIRECTORY`, or `FOLDER` automatically get directory autocomplete in the TUI.

The `schedule` field supports variables just like `command` — use this to let users configure how often the job runs (e.g., `*/$INTERVAL * * * *`).

If the resolved command starts with `cd <path> && ...`, the path is automatically extracted into the Work Dir field in the job form.
