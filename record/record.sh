#!/bin/sh
# record — lazycron history recorder
# Captures stdin and writes a JSON history entry.
# Usage: <command> | record <job-name> [exit-code]

if [ $# -lt 1 ]; then
  echo "usage: record <job-name> [exit-code]" >&2
  exit 1
fi

JOB="$1"
EXIT="${2:-0}"
OUTPUT="$(cat)"

DIR="$HOME/.lazycron/history"
mkdir -p -m 0700 "$DIR"
umask 077

STAMP="$(date +%Y-%m-%dT%H-%M-%S)"
SAFE="$(printf '%s' "$JOB" | tr '/ ' '__')"

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
} > "$DIR/${STAMP}_${SAFE}.json"

# One-shot jobs: disable the crontab entry after execution
if [ "$3" = "--once" ]; then
  SAFE_NAME="$(printf '%s' "$JOB" | sed 's/[.*[\^$]/\\&/g')"
  crontab -l 2>/dev/null | sed "/^# ${SAFE_NAME}/{ n; s/^/#DISABLED /; }" | crontab -
fi
