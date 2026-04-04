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

# Keep Go build trees away from /tmp so air/go run can start reliably even
# when the system temp partition is full or quota-limited.
# shellcheck source=./go-temp-env.sh
. "$ROOT_DIR/scripts/go-temp-env.sh"

cd "$ROOT_DIR/apps/server"

if command -v air >/dev/null 2>&1; then
  exec air
fi

echo "air not found globally; using 'go run github.com/air-verse/air@latest'." >&2
exec "$ROOT_DIR/scripts/go.sh" run github.com/air-verse/air@latest
