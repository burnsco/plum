#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$ROOT_DIR/tmp/dev"
PID_FILE="$TMP_DIR/mordor-media-stack-tunnel.pid"
LOG_FILE="$TMP_DIR/mordor-media-stack-tunnel.log"
BUN_BIN="${BUN:-bun}"
DEV_COMMAND="${PLUM_DEV_COMMAND:-$BUN_BIN run dev}"

mkdir -p "$TMP_DIR"

load_env() {
  if [[ -f "$ROOT_DIR/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    . "$ROOT_DIR/.env"
    set +a
  fi
}

port_ready() {
  local port="$1"
  curl -sS -I --max-time 2 "http://127.0.0.1:${port}" >/dev/null 2>&1
}

clear_stale_pid() {
  if [[ -f "$PID_FILE" ]]; then
    local pid
    pid="$(cat "$PID_FILE")"
    if ! kill -0 "$pid" >/dev/null 2>&1; then
      rm -f "$PID_FILE"
    fi
  fi
}

stop_tracked_tunnel() {
  if [[ ! -f "$PID_FILE" ]]; then
    return
  fi

  local pid
  pid="$(cat "$PID_FILE")"
  if kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid" >/dev/null 2>&1 || true
    sleep 0.5
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill -9 "$pid" >/dev/null 2>&1 || true
    fi
  fi

  rm -f "$PID_FILE"
}

start_tunnel() {
  clear_stale_pid

  if port_ready 7878 && port_ready 8989; then
    echo "Using existing media-stack endpoints on localhost."
    return 0
  fi

  if [[ -f "$PID_FILE" ]]; then
    echo "Waiting for existing mordor media-stack tunnel..."
  else
    echo "Starting mordor media-stack tunnel..."
    "$ROOT_DIR/scripts/mordor-media-stack-tunnel.sh" >"$LOG_FILE" 2>&1 &
    echo "$!" >"$PID_FILE"
  fi

  local attempt
  for attempt in {1..20}; do
    if port_ready 7878 && port_ready 8989; then
      echo "Media-stack tunnel ready on 127.0.0.1:7878 and 127.0.0.1:8989."
      return
    fi
    sleep 0.5
  done

  echo "Warning: failed to establish the mordor media-stack tunnel." >&2
  if [[ -f "$LOG_FILE" ]]; then
    echo "Tunnel log:" >&2
    tail -n 20 "$LOG_FILE" >&2 || true
  fi
  stop_tracked_tunnel
  return 1
}

load_env

# Older local `.env` files may still set VITE_BACKEND_URL to the old
# localhost default. Treat that legacy value as unset so local dev uses Vite's
# same-origin proxy unless the user explicitly picks another browser-facing URL.
if [[ "${VITE_BACKEND_URL:-}" == "http://localhost:8080" ]]; then
  unset VITE_BACKEND_URL
fi

# Local dev should use the Vite proxy by default so browser requests and
# WebSocket upgrades stay same-origin with the dev server. Keep
# VITE_BACKEND_URL as an explicit opt-in for direct browser-to-backend traffic.
export BACKEND_INTERNAL_URL="${BACKEND_INTERNAL_URL:-http://localhost:8080}"

if start_tunnel; then
  export PLUM_RADARR_BASE_URL="${PLUM_RADARR_BASE_URL:-http://127.0.0.1:7878}"
  export PLUM_SONARR_TV_BASE_URL="${PLUM_SONARR_TV_BASE_URL:-http://127.0.0.1:8989}"
else
  if [[ -n "${PLUM_RADARR_BASE_URL:-}" || -n "${PLUM_SONARR_TV_BASE_URL:-}" ]]; then
    echo "Continuing local dev without the tracked mordor tunnel; using configured media-stack endpoints."
  else
    echo "Continuing local dev without media-stack defaults; Discover add/download integration will remain unavailable until Radarr/Sonarr endpoints are reachable."
  fi
fi

echo "Starting Plum local dev. Run 'make dev-clean' to stop the tracked tunnel."
cd "$ROOT_DIR"
exec bash -lc "$DEV_COMMAND"
