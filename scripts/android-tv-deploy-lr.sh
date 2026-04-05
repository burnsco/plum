#!/usr/bin/env bash
# Living-room TV over classic TCP ADB (`make deploy-tv-lr` / `bun run deploy-tv-lr`).
# Override: PLUM_TV_ADB_LR=192.168.2.x:5555 make deploy-tv-lr
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
_target="${PLUM_TV_ADB_LR:-192.168.2.20:5555}"
export PLUM_TV_ADB_CONNECT="$_target"
if [[ -z "${ANDROID_SERIAL:-}" ]]; then
  export ANDROID_SERIAL="$_target"
fi
exec bash "${ROOT}/scripts/android-tv-deploy.sh"
