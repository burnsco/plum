#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APK_PATH="$ROOT_DIR/apps/android-tv/app/build/outputs/apk/debug/app-debug.apk"
COMPONENT="com.plum.android.tv/plum.tv.app.MainActivity"
ADB_BIN="${ADB:-}"

resolve_adb() {
  if [[ -n "$ADB_BIN" ]]; then
    return
  fi

  if command -v adb >/dev/null 2>&1; then
    ADB_BIN="$(command -v adb)"
    return
  fi

  if [[ -n "${ANDROID_HOME:-}" && -x "${ANDROID_HOME}/platform-tools/adb" ]]; then
    ADB_BIN="${ANDROID_HOME}/platform-tools/adb"
    return
  fi

  for sdk_dir in "$HOME/Android/Sdk" /opt/android-sdk /usr/lib/android-sdk; do
    if [[ -x "$sdk_dir/platform-tools/adb" ]]; then
      ADB_BIN="$sdk_dir/platform-tools/adb"
      return
    fi
  done

  echo "android-tv-deploy: unable to find adb; set ADB or ANDROID_HOME" >&2
  exit 1
}

resolve_serial() {
  if [[ -n "${ADB_SERIAL:-}" ]]; then
    echo "$ADB_SERIAL"
    return
  fi

  local serial
  serial="$("$ADB_BIN" devices | awk 'NR > 1 && $2 == "device" { print $1; exit }')"
  if [[ -z "$serial" ]]; then
    echo "android-tv-deploy: no connected adb devices found; set ADB_SERIAL or connect your TV" >&2
    exit 1
  fi

  echo "$serial"
}

resolve_adb

if [[ ! -f "$APK_PATH" ]]; then
  echo "android-tv-deploy: missing APK at $APK_PATH; run make android-tv-build first" >&2
  exit 1
fi

SERIAL="$(resolve_serial)"

echo "Installing Plum on $SERIAL..."
"$ADB_BIN" -s "$SERIAL" install -r "$APK_PATH"

echo "Launching Plum on $SERIAL..."
"$ADB_BIN" -s "$SERIAL" shell am start -n "$COMPONENT" -a android.intent.action.MAIN -c android.intent.category.LAUNCHER
