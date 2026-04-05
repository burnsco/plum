#!/usr/bin/env bash
# Default TCP ADB target for `make deploy-tv` / `bun run deploy-tv`.
# Override: PLUM_TV_ADB=192.168.1.5:5555 make deploy-tv
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
_target="${PLUM_TV_ADB:-192.168.2.11:5555}"
export PLUM_TV_ADB_CONNECT="$_target"
if [[ -z "${ANDROID_SERIAL:-}" ]]; then
  export ANDROID_SERIAL="$_target"
fi
exec bash "${ROOT}/scripts/android-tv-deploy.sh"
