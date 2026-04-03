#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

load_env() {
  if [[ -f "$ROOT_DIR/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    . "$ROOT_DIR/.env"
    set +a
  fi
}

load_env

cd "$ROOT_DIR/apps/server"

if command -v air >/dev/null 2>&1; then
  exec air
fi

echo "air not found globally; using 'go run github.com/air-verse/air@latest'." >&2
exec go run github.com/air-verse/air@latest
