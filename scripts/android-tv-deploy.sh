#!/usr/bin/env bash
# Install debug Plum TV APK on a connected ADB device and bring the app to the foreground.
# Use JAVA_HOME=Android Studio JBR and ANDROID_HOME=SDK (see apps/android-tv/AGENT_DEPLOY.md).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_ID="com.plum.android.tv"
ACTIVITY="plum.tv.app.MainActivity"

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
"$ADB" devices -l

echo "android-tv-deploy: installDebug (Gradle)…"
bash "${ROOT}/scripts/android-tv.sh" :app:installDebug

echo "android-tv-deploy: launching ${APP_ID}/${ACTIVITY}…"
"$ADB" shell am start -a android.intent.action.MAIN \
  -c android.intent.category.LEANBACK_LAUNCHER \
  -n "${APP_ID}/${ACTIVITY}" \
  || "$ADB" shell am start -n "${APP_ID}/${ACTIVITY}"

echo "android-tv-deploy: done."
