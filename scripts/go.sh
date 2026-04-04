#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# shellcheck source=./go-temp-env.sh
. "$ROOT_DIR/scripts/go-temp-env.sh"

exec go "$@"
