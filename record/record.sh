#!/bin/sh
# record — lazycron history recorder
# Captures stdin and writes a JSON history entry.
# Usage: <command> | record <job-id> <job-name> [exit-code] [--once]

if [ $# -lt 2 ]; then
  echo "usage: record <job-id> <job-name> [exit-code]" >&2
  exit 1
fi

JOB_ID="$1"
JOB="$2"
EXIT="${3:-0}"
OUTPUT="$(cat)"

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

# Send notifications (if configured for this job).
# Pass history file path instead of full output to avoid ARG_MAX limits.
NOTIFY_BIN="$HOME/.lazycron/bin/notify"
HISTORY_FILE="$DIR/${STAMP}_${JOB_ID}.json"
if [ -x "$NOTIFY_BIN" ]; then
  "$NOTIFY_BIN" "$JOB_ID" "$JOB" "$EXIT" "$HISTORY_FILE" >/dev/null 2>&1 || true
fi

# One-shot jobs: disable the crontab entry after execution
if [ "$4" = "--once" ]; then
  crontab -l 2>/dev/null | sed "/@id:${JOB_ID}/{ n; s/^/#DISABLED /; }" | crontab -
fi
