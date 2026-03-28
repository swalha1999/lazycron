#!/bin/sh
# notify — lazycron notification sender
# Called by the record script after writing a history entry.
# Usage: notify <job-id> <job-name> <exit-code> <history-file-path>

if [ $# -lt 4 ]; then
  exit 0
fi

JOB_ID="$1"
JOB_NAME="$2"
EXIT_CODE="$3"
HISTORY_FILE="$4"

CONF="$HOME/.lazycron/notify/${JOB_ID}.conf"
[ -f "$CONF" ] || exit 0

SERVER="$(hostname 2>/dev/null || echo 'unknown')"
TIMESTAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date +%Y-%m-%dT%H:%M:%S)"

# Read output from history file to avoid ARG_MAX limits.
# Extract the "output" field from JSON (handles escaped quotes and newlines).
OUTPUT=""
if [ -f "$HISTORY_FILE" ]; then
  OUTPUT="$(sed -n 's/.*"output": "\(.*\)",$/\1/p' "$HISTORY_FILE" | head -1)"
fi

# Export environment variables for user commands.
export LC_JOB_NAME="$JOB_NAME"
export LC_EXIT_CODE="$EXIT_CODE"
export LC_OUTPUT="$OUTPUT"
export LC_SERVER="$SERVER"
export LC_TIMESTAMP="$TIMESTAMP"

# JSON-escape a string (backslashes, double quotes, newlines, tabs).
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

# Template substitution using awk for safety.
substitute() {
  printf '%s' "$1" | awk \
    -v jn="$JOB_NAME" \
    -v ec="$EXIT_CODE" \
    -v srv="$SERVER" \
    -v sched="$LC_SCHEDULE" \
    -v ts="$TIMESTAMP" '{
    gsub(/\{\{\.JobName\}\}/, jn)
    gsub(/\{\{\.ExitCode\}\}/, ec)
    gsub(/\{\{\.Server\}\}/, srv)
    gsub(/\{\{\.Schedule\}\}/, sched)
    gsub(/\{\{\.Timestamp\}\}/, ts)
    print
  }'
}

# Unescape TSV-escaped strings (reverse of escapeTSV in notify.go).
unescape_tsv() {
  printf '%s' "$1" | sed -e 's/\\r/\r/g' -e 's/\\n/\n/g' -e 's/\\t/\t/g' -e 's/\\\\/\\/g'
}

SCHEDULE=""

while IFS='	' read -r EVENT TYPE VALUE; do
  # Skip empty lines and comments.
  case "$EVENT" in
    ''|'#'*) continue ;;
  esac

  # Parse metadata.
  if [ "$EVENT" = "meta" ]; then
    case "$TYPE" in
      schedule) SCHEDULE="$VALUE"; export LC_SCHEDULE="$SCHEDULE" ;;
    esac
    continue
  fi

  # Check if this event should fire based on exit code.
  case "$EVENT" in
    on_failure) [ "$EXIT_CODE" = "0" ] && continue ;;
    on_success) [ "$EXIT_CODE" != "0" ] && continue ;;
    *) continue ;;
  esac

  case "$TYPE" in
    webhook)
      # Unescape the URL from TSV format.
      URL="$(unescape_tsv "$VALUE")"
      # Truncate output for payload (first 1000 chars).
      TRUNC_OUTPUT="$(printf '%.1000s' "$OUTPUT")"
      ESC_NAME="$(json_escape "$JOB_NAME")"
      ESC_OUTPUT="$(json_escape "$TRUNC_OUTPUT")"
      ESC_SCHEDULE="$(json_escape "$SCHEDULE")"
      PAYLOAD="$(printf '{"job_name":"%s","schedule":"%s","exit_code":%s,"output":"%s","server":"%s","timestamp":"%s"}' \
        "$ESC_NAME" "$ESC_SCHEDULE" "$EXIT_CODE" "$ESC_OUTPUT" "$SERVER" "$TIMESTAMP")"
      # Use -- to prevent URL from being interpreted as curl option.
      curl -s -m 10 -X POST -H 'Content-Type: application/json' -d "$PAYLOAD" -- "$URL" >/dev/null 2>&1 || true
      ;;
    command)
      # Unescape the command from TSV format.
      UNESC_CMD="$(unescape_tsv "$VALUE")"
      CMD="$(substitute "$UNESC_CMD")"
      sh -c "$CMD" >/dev/null 2>&1 || true
      ;;
    desktop)
      if [ -n "$VALUE" ]; then
        # Unescape the template from TSV format.
        UNESC_VALUE="$(unescape_tsv "$VALUE")"
        MSG="$(substitute "$UNESC_VALUE")"
      elif [ "$EXIT_CODE" = "0" ]; then
        MSG="$JOB_NAME completed successfully"
      else
        MSG="$JOB_NAME failed (exit $EXIT_CODE)"
      fi
      if command -v notify-send >/dev/null 2>&1; then
        notify-send "lazycron" "$MSG" >/dev/null 2>&1 || true
      elif command -v osascript >/dev/null 2>&1; then
        # Escape quotes and backslashes for AppleScript.
        ESC_MSG="${MSG//\\/\\\\}"
        ESC_MSG="${ESC_MSG//\"/\\\"}"
        osascript -e "display notification \"$ESC_MSG\" with title \"lazycron\"" >/dev/null 2>&1 || true
      fi
      ;;
  esac
done < "$CONF"
