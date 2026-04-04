#!/usr/bin/env bash
# Run Gradle for apps/android-tv. Requires Android SDK (ANDROID_HOME or apps/android-tv/local.properties).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT/apps/android-tv"

if [[ -z "${JAVA_HOME:-}" ]]; then
  if [[ -x /opt/android-studio/jbr/bin/java ]]; then
    export JAVA_HOME=/opt/android-studio/jbr
  elif [[ -x /usr/lib/jvm/java-21-openjdk/bin/java ]]; then
    export JAVA_HOME=/usr/lib/jvm/java-21-openjdk
  elif [[ -x /usr/lib/jvm/java-17-openjdk/bin/java ]]; then
    export JAVA_HOME=/usr/lib/jvm/java-17-openjdk
  fi
fi

# Kotlin stores recoverable compilation state under $TMPDIR (e.g. kotlin-backups*).
# Default to an on-disk temp directory so /tmp quota pressure does not break builds.
if [[ -z "${PLUM_ANDROID_USE_SYSTEM_TMPDIR:-}" ]]; then
  TMPDIR_ROOT="${PLUM_ANDROID_TV_TMPDIR:-$ROOT/tmp/android-tv}"
  mkdir -p "$TMPDIR_ROOT"
  export TMPDIR="$TMPDIR_ROOT"
  export TEMP="$TMPDIR_ROOT"
  export TMP="$TMPDIR_ROOT"
fi

if [[ ! -f local.properties ]]; then
  if [[ -z "${ANDROID_HOME:-}" ]]; then
    for sdk_dir in "$HOME/Android/Sdk" /opt/android-sdk /usr/lib/android-sdk; do
      if [[ -d "$sdk_dir" ]]; then
        export ANDROID_HOME="$sdk_dir"
        break
      fi
    done
  fi
  if [[ -z "${ANDROID_HOME:-}" ]]; then
    echo "android-tv: set ANDROID_HOME or copy apps/android-tv/local.properties.example to local.properties with sdk.dir=" >&2
    exit 1
  fi
  echo "sdk.dir=$ANDROID_HOME" > local.properties
fi

exec ./gradlew "$@"
