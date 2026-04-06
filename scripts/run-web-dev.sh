#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT_DIR/apps/web"
CACHE_DIR="$WEB_DIR/.vite"
DEPS_DIR="$CACHE_DIR/deps"
STALE_ROOT="$ROOT_DIR/tmp/dev"

load_env() {
  if [[ -f "$ROOT_DIR/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    . "$ROOT_DIR/.env"
    set +a
  fi
}

# Avoid Vite proxying to :8080 before air/go has finished building and listening
# (otherwise the dev console fills with ECONNREFUSED on /api/* and /ws).
wait_for_backend() {
  if [[ -n "${PLUM_SKIP_BACKEND_WAIT:-}" ]]; then
    return 0
  fi

  load_env
  local base="${BACKEND_INTERNAL_URL:-http://localhost:8080}"
  base="${base%/}"
  local attempt=0
  local max=240

  echo "Waiting for backend at ${base} (set PLUM_SKIP_BACKEND_WAIT=1 to skip)..." >&2
  while (( attempt < max )); do
    if curl -sf -o /dev/null --max-time 1 "${base}/health" 2>/dev/null; then
      echo "Backend ready; starting Vite." >&2
      return 0
    fi
    sleep 0.25
    ((attempt++)) || true
  done

  echo "Warning: backend did not respond at ${base}/health within 60s; starting Vite anyway." >&2
}

prepare_vite_cache() {
  mkdir -p "$STALE_ROOT"

  if [[ -d "$DEPS_DIR" && ! -w "$DEPS_DIR" ]]; then
    local stale_dir
    stale_dir="$STALE_ROOT/web-vite-stale-$(date +%Y%m%d-%H%M%S)"
    echo "Moving unwritable Vite cache to $stale_dir" >&2
    mv "$CACHE_DIR" "$stale_dir"
  fi

  mkdir -p "$CACHE_DIR"
}

prepare_vite_cache

wait_for_backend

cd "$WEB_DIR"

# Vite's dev proxy currently depends on Node socket APIs like `destroySoon()`
# that Bun does not fully implement, which can crash local `/ws` proxying.
if [[ -n "${FORCE_COLOR:-}" && -n "${NO_COLOR:-}" ]]; then
  unset NO_COLOR
fi

exec node ./node_modules/vite/bin/vite.js
