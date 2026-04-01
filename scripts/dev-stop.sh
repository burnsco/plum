#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$ROOT_DIR/tmp/dev"
PID_FILE="$TMP_DIR/mordor-media-stack-tunnel.pid"
LOG_FILE="$TMP_DIR/mordor-media-stack-tunnel.log"

if [[ -f "$PID_FILE" ]]; then
  pid="$(cat "$PID_FILE")"
  if kill -0 "$pid" >/dev/null 2>&1; then
    echo "Stopping mordor media-stack tunnel ($pid)..."
    kill "$pid" >/dev/null 2>&1 || true
    sleep 0.5
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
  fi
  rm -f "$PID_FILE"
else
  echo "No tracked mordor media-stack tunnel is running."
fi

rm -f "$LOG_FILE"
echo "Local dev tunnel state cleaned."
