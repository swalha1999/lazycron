#!/bin/sh
# record — lazycron history recorder
# Captures stdin and writes a JSON history entry, then optionally fires an
# OS notification on failure.
# Usage: <command> | record <job-id> <job-name> [exit-code] [--once] [--no-notify]

if [ $# -lt 2 ]; then
  echo "usage: record <job-id> <job-name> [exit-code] [--once] [--no-notify]" >&2
  exit 1
fi

JOB_ID="$1"
JOB="$2"
EXIT="${3:-0}"
OUTPUT="$(cat)"

# Parse trailing flags ($4..) — order independent. shift only when the
# exit-code arg is actually present, so callers omitting it (default 0)
# don't trip POSIX sh's "shift: count out of range".
ONCE=0
NO_NOTIFY=0
if [ $# -ge 3 ]; then
  shift 3
  for arg in "$@"; do
    case "$arg" in
      --once) ONCE=1 ;;
      --no-notify) NO_NOTIFY=1 ;;
    esac
  done
fi

DIR="$HOME/.lazycron/history"
mkdir -p -m 0700 "$DIR"
umask 077

STAMP="$(date +%Y-%m-%dT%H-%M-%S)"

if [ "$EXIT" = "0" ]; then
  SUCCESS="true"
else
  SUCCESS="false"
fi

# JSON-escape: backslashes, quotes, carriage returns, tabs, then newlines
json_escape() {
  RS="$(printf '\036')"
  printf '%s' "$1" | \
    sed -e 's/\\/\\\\/g' \
        -e 's/"/\\"/g' \
        -e "s/$(printf '\r')/\\\\r/g" \
        -e "s/$(printf '\t')/\\\\t/g" | \
    tr '\n' "$RS" | \
    sed "s/$RS/\\\\n/g"
}

ESC_JOB="$(json_escape "$JOB")"
ESC_OUTPUT="$(json_escape "$OUTPUT")"

{
  printf '{\n'
  printf '  "job_id": "%s",\n' "$JOB_ID"
  printf '  "job_name": "'
  printf '%s' "$ESC_JOB"
  printf '",\n'
  # Insert colon in timezone offset for RFC3339 compliance (+0200 -> +02:00)
  __lc_tz="$(date +%z)"
  __lc_ts="$(date +%Y-%m-%dT%H:%M:%S)$(echo "$__lc_tz" | sed 's/\(..\)$/:\1/')"
  printf '  "timestamp": "%s",\n' "$__lc_ts"
  printf '  "output": "'
  printf '%s' "$ESC_OUTPUT"
  printf '",\n'
  printf '  "success": %s\n' "$SUCCESS"
  printf '}\n'
} > "$DIR/${STAMP}_${JOB_ID}.json"

# Fire a desktop notification on failure (best-effort, never blocks the
# recording above). Skip when:
#  - the job succeeded
#  - this job has --no-notify (per-job opt-out)
#  - the global config disables it (notify_on_failure: false)
notify_global_enabled() {
  cfg="$HOME/.lazycron/config.yml"
  [ -f "$cfg" ] || return 0
  val="$(sed -n 's/^notify_on_failure:[[:space:]]*//p' "$cfg" | head -1 | tr -d '[:space:]')"
  case "$val" in
    false|False|FALSE|no|No|NO|0|off|Off|OFF) return 1 ;;
  esac
  return 0
}

if [ "$SUCCESS" = "false" ] && [ "$NO_NOTIFY" -eq 0 ] && notify_global_enabled; then
  TITLE="Job failed: $JOB"
  BODY="Exit $EXIT · just now"
  case "$(uname 2>/dev/null)" in
    Darwin)
      if command -v terminal-notifier >/dev/null 2>&1; then
        terminal-notifier -title "$TITLE" -message "$BODY" >/dev/null 2>&1 || true
      elif command -v osascript >/dev/null 2>&1; then
        # Escape embedded double quotes for AppleScript string literals.
        AS_TITLE="$(printf '%s' "$TITLE" | sed 's/"/\\"/g')"
        AS_BODY="$(printf '%s' "$BODY" | sed 's/"/\\"/g')"
        osascript -e "display notification \"$AS_BODY\" with title \"$AS_TITLE\"" >/dev/null 2>&1 || true
      fi
      ;;
    Linux)
      if command -v notify-send >/dev/null 2>&1; then
        notify-send --urgency=critical "$TITLE" "$BODY" >/dev/null 2>&1 || true
      fi
      ;;
  esac
fi

# One-shot jobs: disable the crontab entry after execution. Best-effort —
# if `crontab` is missing we still want recording to have succeeded.
if [ "$ONCE" -eq 1 ] && command -v crontab >/dev/null 2>&1; then
  crontab -l 2>/dev/null | sed "/@id:${JOB_ID}/{ n; s/^/#DISABLED /; }" | crontab - 2>/dev/null || true
fi
