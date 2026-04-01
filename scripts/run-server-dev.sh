#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR/apps/server"

if command -v air >/dev/null 2>&1; then
  exec air
fi

echo "air not found globally; using 'go run github.com/air-verse/air@latest'." >&2
exec go run github.com/air-verse/air@latest
