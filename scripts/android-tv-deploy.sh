#!/usr/bin/env bash
# Install release Plum TV APK on connected ADB device(s); optionally launch the app.
# Use JAVA_HOME=Android Studio JBR and ANDROID_HOME=SDK (see apps/android-tv/AGENT_DEPLOY.md).
#
# PLUM_TV_ADB_CONNECT — if set, runs `adb connect` first (classic TCP, e.g. 192.168.2.11:5555).
#   Used by android-tv-deploy-desk.sh / android-tv-deploy-lr.sh so TVs do not need USB or wireless
#   pairing ports.
#
# ANDROID_SERIAL — if set, uninstall/install/(launch) only this device. Use when several TVs are
#   connected or when a stale mDNS entry (e.g. adb-…. _adb-tls-connect._tcp) breaks Gradle’s
#   installDebug (ddmlib “device not found”).
#
# PLUM_TV_REINSTALL=1 — adb uninstall the app before install (fixes signature mismatch /
#   INSTALL_FAILED_UPDATE_INCOMPATIBLE; clears app data). Ignores uninstall failure if absent.
#
# PLUM_TV_NO_LAUNCH=1 — install only; do not run `am start` (useful when pushing to all TVs
#   without interrupting what is on screen).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_ID="com.plum.android.tv"
ACTIVITY="plum.tv.app.MainActivity"
APK_RELEASE="${ROOT}/apps/android-tv/app/build/outputs/apk/release/app-release.apk"
APK_RELEASE_UNSIGNED="${ROOT}/apps/android-tv/app/build/outputs/apk/release/app-release-unsigned.apk"

# Match android-tv.sh: prefer Android Studio JBR when JAVA_HOME is unset.
if [[ -z "${JAVA_HOME:-}" ]]; then
  if [[ -x /opt/android-studio/jbr/bin/java ]]; then
    export JAVA_HOME=/opt/android-studio/jbr
  elif [[ -x "${HOME}/android-studio/jbr/bin/java" ]]; then
    export JAVA_HOME="${HOME}/android-studio/jbr"
  elif [[ -d "/Applications/Android Studio.app/Contents/jbr/Contents/Home" ]]; then
    export JAVA_HOME="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
  elif [[ -x /usr/lib/jvm/java-21-openjdk/bin/java ]]; then
    export JAVA_HOME=/usr/lib/jvm/java-21-openjdk
  elif [[ -x /usr/lib/jvm/java-17-openjdk/bin/java ]]; then
    export JAVA_HOME=/usr/lib/jvm/java-17-openjdk
  fi
fi

adb_bin() {
  if [[ -n "${ANDROID_HOME:-}" && -x "${ANDROID_HOME}/platform-tools/adb" ]]; then
    echo "${ANDROID_HOME}/platform-tools/adb"
    return
  fi
  local props="${ROOT}/apps/android-tv/local.properties"
  if [[ -f "$props" ]] && grep -q '^sdk.dir=' "$props"; then
    local sdk
    sdk="$(grep '^sdk.dir=' "$props" | head -1 | cut -d= -f2- | tr -d '\r')"
    if [[ -x "${sdk}/platform-tools/adb" ]]; then
      echo "${sdk}/platform-tools/adb"
      return
    fi
  fi
  command -v adb
}

ADB="$(adb_bin)"
echo "android-tv-deploy: using adb: $ADB"
if [[ -n "${PLUM_TV_ADB_CONNECT:-}" ]]; then
  echo "android-tv-deploy: adb connect ${PLUM_TV_ADB_CONNECT}…"
  set +e
  connect_out="$("$ADB" connect "${PLUM_TV_ADB_CONNECT}" 2>&1)"
  connect_ec=$?
  set -e
  printf '%s\n' "$connect_out"
  if [[ $connect_ec -ne 0 ]] || echo "$connect_out" | grep -qiE 'failed to connect|cannot connect|unable to connect'; then
    echo "android-tv-deploy: adb connect did not succeed for ${PLUM_TV_ADB_CONNECT}." >&2
    echo "android-tv-deploy: \"No route to host\" usually means wrong IP, device offline, or not on your LAN. Check with ping, or fix PLUM_TV_ADB." >&2
    echo "android-tv-deploy: TCP devices already in adb:" >&2
    "$ADB" devices | awk 'NR>1 && $2=="device" && $1 ~ /:[0-9]+$/ {print "  " $1}' >&2 || true
    exit 1
  fi
fi
"$ADB" devices -l

device_serials() {
  "$ADB" devices | awk 'NR>1 && $2=="device" {print $1}'
}

if [[ -z "${ANDROID_SERIAL:-}" ]]; then
  n="$(device_serials | wc -l)"
  n="${n//[[:space:]]/}"
  if [[ "${n:-0}" -gt 1 ]]; then
    echo "android-tv-deploy: note: multiple devices connected; set ANDROID_SERIAL to target one and avoid stale mDNS duplicates." >&2
  fi
elif ! device_serials | grep -qxF "$ANDROID_SERIAL"; then
  echo "android-tv-deploy: ANDROID_SERIAL=${ANDROID_SERIAL} is not connected (no row in state 'device')." >&2
  if [[ -n "${PLUM_TV_ADB_CONNECT:-}" ]]; then
    echo "android-tv-deploy: If you overrode PLUM_TV_ADB, try the IP shown under \"List of devices\" for your TV (e.g. 192.168.x.x:5555), or drop PLUM_TV_ADB to use the repo default." >&2
  else
    echo "android-tv-deploy: Run adb devices -l, adb connect <ip>:5555, USB, or wireless debugging on the TV, then retry." >&2
  fi
  exit 1
fi

if [[ "${PLUM_TV_REINSTALL:-}" == "1" ]]; then
  if [[ -n "${ANDROID_SERIAL:-}" ]]; then
    echo "android-tv-deploy: uninstalling ${APP_ID} from ${ANDROID_SERIAL} (PLUM_TV_REINSTALL=1)…"
    "$ADB" -s "$ANDROID_SERIAL" uninstall "$APP_ID" || true
  else
    echo "android-tv-deploy: uninstalling ${APP_ID} from all devices (PLUM_TV_REINSTALL=1)…"
    while IFS= read -r serial; do
      echo "android-tv-deploy: uninstalling from ${serial}…"
      "$ADB" -s "$serial" uninstall "$APP_ID" || true
    done < <(device_serials)
  fi
fi

echo "android-tv-deploy: assembleRelease (Gradle)…"
bash "${ROOT}/scripts/android-tv.sh" ':app:assembleRelease'

APK_TO_INSTALL=""
if [[ -f "$APK_RELEASE" ]]; then
  APK_TO_INSTALL="$APK_RELEASE"
elif [[ -f "$APK_RELEASE_UNSIGNED" ]]; then
  APK_TO_INSTALL="$APK_RELEASE_UNSIGNED"
  echo "android-tv-deploy: note: installing unsigned release APK (configure plumTv.release* in local.properties for a signed build)." >&2
fi
if [[ -z "$APK_TO_INSTALL" ]]; then
  echo "android-tv-deploy: release APK not found at $APK_RELEASE (or $APK_RELEASE_UNSIGNED)" >&2
  exit 1
fi

install_apk() {
  local serial="$1"
  echo "android-tv-deploy: installing on ${serial}…"
  "$ADB" -s "$serial" install -r "$APK_TO_INSTALL"
}

if [[ -n "${ANDROID_SERIAL:-}" ]]; then
  install_apk "$ANDROID_SERIAL"
else
  mapfile -t _serials < <(device_serials)
  if [[ "${#_serials[@]}" -eq 0 ]]; then
    echo "android-tv-deploy: no device in state 'device'; connect a TV or set ANDROID_SERIAL." >&2
    exit 1
  fi
  for serial in "${_serials[@]}"; do
    install_apk "$serial"
  done
fi

if [[ "${PLUM_TV_NO_LAUNCH:-}" == "1" ]]; then
  echo "android-tv-deploy: skipping launch (PLUM_TV_NO_LAUNCH=1)."
else
  echo "android-tv-deploy: launching ${APP_ID}/${ACTIVITY}…"
  if [[ -n "${ANDROID_SERIAL:-}" ]]; then
    "$ADB" -s "$ANDROID_SERIAL" shell am start -a android.intent.action.MAIN \
      -c android.intent.category.LEANBACK_LAUNCHER \
      -n "${APP_ID}/${ACTIVITY}" \
      || "$ADB" -s "$ANDROID_SERIAL" shell am start -n "${APP_ID}/${ACTIVITY}" || true
  else
    while IFS= read -r serial; do
      "$ADB" -s "$serial" shell am start -a android.intent.action.MAIN \
        -c android.intent.category.LEANBACK_LAUNCHER \
        -n "${APP_ID}/${ACTIVITY}" \
        || "$ADB" -s "$serial" shell am start -n "${APP_ID}/${ACTIVITY}" || true
    done < <(device_serials)
  fi
fi

echo "android-tv-deploy: done."
