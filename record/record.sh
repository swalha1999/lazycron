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

# disable_job filters a crontab on stdin, prefixing "#DISABLED " to the
# schedule line after the name comment whose @id is exactly $1. Like
# cron.extractID, the id runs from the first " @id:" to the first byte
# outside [a-z0-9_-], so "@id:fix" never matches "@id:fix-agent".
disable_job() {
  LC_ALL=C awk -v id="$1" '
    pending {
      pending = 0
      if ($0 !~ /^#DISABLED /) $0 = "#DISABLED " $0
    }
    /^#/ && !/^#DISABLED / && (i = index($0, " @id:")) {
      rest = substr($0, i + 5)
      after = substr(rest, length(id) + 1, 1)
      if (index(rest, id) == 1 && after !~ /^[a-z0-9_-]$/) pending = 1
    }
    { print }'
}

# One-shot jobs: disable the crontab entry after execution. An unreadable
# crontab is left alone: piping nothing into `crontab -` would wipe it.
if [ "$4" = "--once" ] && [ -n "$JOB_ID" ]; then
  if CURRENT="$(crontab -l 2>/dev/null)"; then
    UPDATED="$(printf '%s\n' "$CURRENT" | disable_job "$JOB_ID")"
    if [ "$UPDATED" != "$CURRENT" ]; then
      printf '%s\n' "$UPDATED" | crontab -
    fi
  fi
fi
