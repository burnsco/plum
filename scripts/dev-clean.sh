#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

archive_unwritable_target() {
  local target="$1"
  local parent
  local base
  local archived
  parent="$(dirname "$target")"
  base="$(basename "$target")"
  archived="$parent/${base}-stale-$(date +%Y%m%d-%H%M%S)-$$"
  if mv "$target" "$archived" 2>/dev/null; then
    echo "Moved unwritable $target to $archived for later cleanup." >&2
    return 0
  fi
  return 1
}

best_effort_remove() {
  local target="$1"
  if [[ ! -e "$target" ]]; then
    return
  fi
  if rm -rf "$target" 2>/dev/null; then
    return
  fi
  if archive_unwritable_target "$target"; then
    return
  fi
  echo "Skipping cleanup for $target because it is not writable." >&2
}

"$ROOT_DIR/scripts/dev-stop.sh"

echo "Removing local dev database and transient caches..."
rm -f \
  "$ROOT_DIR/apps/server/data/plum.db" \
  "$ROOT_DIR/apps/server/data/plum.db-shm" \
  "$ROOT_DIR/apps/server/data/plum.db-wal"
best_effort_remove "$ROOT_DIR/apps/server/tmp"
best_effort_remove "$ROOT_DIR/apps/web/.vite"
best_effort_remove "$ROOT_DIR/tmp/dev"

echo "Restarting local dev..."
exec "$ROOT_DIR/scripts/dev.sh"
