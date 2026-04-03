# Android TV — build, install, and launch (for agents)

Use this when automating **compile → install on a physical TV (or emulator) → open the app**. The repo already wraps Gradle in [`scripts/android-tv.sh`](../../scripts/android-tv.sh).

## Quick commands (repo root)

| Step | Command |
|------|---------|
| Debug APK only | `bun run android:assemble` |
| Install debug on connected device | `bun run android:install` |
| Install + launch Plum TV | `bun run android:deploy` |

`android:deploy` runs [`scripts/android-tv-deploy.sh`](../../scripts/android-tv-deploy.sh): `installDebug`, then starts the app via `adb`.

## JDK (use Android Studio’s JBR)

Gradle for this project expects **JDK 17 or 21**. **Avoid JDK 26+** (Kotlin/Gradle DSL often fails with a cryptic `IllegalArgumentException`).

1. **Preferred**: let the Gradle wrapper or helper scripts auto-pick Android Studio’s bundled JBR when it is installed.

   Typical locations:

   | OS | `JAVA_HOME` candidate |
   |---|------------------------|
   | Linux | `/opt/android-studio/jbr` |
   | Linux (user install) | `$HOME/android-studio/jbr` |
   | macOS | `/Applications/Android Studio.app/Contents/jbr/Contents/Home` |
   | Windows | `C:\Program Files\Android\Android Studio\jbr` |

2. If you want to override the auto-pick, set `JAVA_HOME` manually to a JDK 17 or 21 install.

3. **Verify**: `"$JAVA_HOME/bin/java" -version` should show 17 or 21.

## Android SDK / `adb`

- Set **`ANDROID_HOME`** to the Android SDK root (same SDK Android Studio uses: **Settings → Languages & Frameworks → Android SDK → Android SDK Location**).
- Or ensure **`apps/android-tv/local.properties`** contains `sdk.dir=/absolute/path/to/sdk` (copy from [`local.properties.example`](./local.properties.example)).
- **`android-tv.sh`** creates `local.properties` from `ANDROID_HOME` if the file is missing.
- **`adb`** must be on `PATH` or available at `$ANDROID_HOME/platform-tools/adb`.

Check device:

```bash
adb devices
```

If multiple devices are connected, set **`ANDROID_SERIAL`** to the target’s serial (from `adb devices`).

## App identity (for `adb` / intents)

| Property | Value |
|----------|--------|
| Application ID | `com.plum.android.tv` |
| Launcher activity | `plum.tv.app.MainActivity` |

Launch after install (any one of these is fine):

```bash
adb shell am start -n com.plum.android.tv/plum.tv.app.MainActivity
```

```bash
adb shell monkey -p com.plum.android.tv -c android.intent.category.LEANBACK_LAUNCHER 1
```

## Android Studio CLI / already paired TV

If the machine is already set up in Android Studio (device authorized, same SDK):

1. Ensure **`adb devices`** shows the TV as `device` (not `unauthorized`).
2. From repo root, with **`JAVA_HOME`** and **`ANDROID_HOME`** set (or Studio’s embedded terminal where env is configured):

   ```bash
   bun run android:deploy
   ```

Equivalent manual steps:

```bash
bash ./scripts/android-tv.sh :app:installDebug
adb shell am start -n com.plum.android.tv/plum.tv.app.MainActivity
```

## Raw Gradle (no Bun)

From repo root:

```bash
bash ./scripts/android-tv.sh :app:assembleDebug
bash ./scripts/android-tv.sh :app:installDebug
```

## Troubleshooting

| Symptom | Check |
|--------|--------|
| Gradle fails with version/`IllegalArgumentException` | Use JDK 17 or 21 (`JAVA_HOME`). |
| `SDK location not found` | `ANDROID_HOME` or `local.properties` `sdk.dir`. |
| `adb: device unauthorized` | Accept RSA on the TV; replug USB or re-enable wireless debugging. |
| Wrong device receives install | `export ANDROID_SERIAL=...` |
| Install succeeds but app doesn’t appear | Use **Leanback/TV** image or hardware; package is `com.plum.android.tv`. |

## Logging

- Android TV app logs use the `PlumTV` tag.
- Server startup and request logs are JSON lines with `"component":"server"`.

Useful filters:

```bash
adb logcat -s PlumTV
adb logcat | grep -E "PlumTV|AndroidRuntime|FATAL"
bun run dev:server 2>&1 | grep '"component":"server"'
bun run dev:server 2>&1 | grep '"event":"startup"'
bun run dev:server 2>&1 | grep '"event":"request"'
bun run dev:server 2>&1 | grep '"component":"server"' | jq -r '.event' | sort | uniq -c
bun run dev:server 2>&1 | grep '"event":"request"' | jq -r '.status / 100 | floor' | sort | uniq -c
bun run dev:server 2>&1 | grep '"event":"request"' | jq -c 'select(.status >= 400)'
```

## Backend (physical TV)

The app talks to the Plum server. On an emulator, default is often `10.0.2.2:8080`. On a **real TV**, configure the server URL in-app to your **LAN IP** (see root [`AGENTS.md`](../../AGENTS.md) Android TV section).
