#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$ROOT_DIR/apps/web"
CACHE_DIR="$WEB_DIR/.vite"
DEPS_DIR="$CACHE_DIR/deps"
STALE_ROOT="$ROOT_DIR/tmp/dev"

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

cd "$WEB_DIR"

# Vite's dev proxy currently depends on Node socket APIs like `destroySoon()`
# that Bun does not fully implement, which can crash local `/ws` proxying.
if [[ -n "${FORCE_COLOR:-}" && -n "${NO_COLOR:-}" ]]; then
  unset NO_COLOR
fi

exec node ./node_modules/vite/bin/vite.js
