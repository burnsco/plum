# AGENTS.md

## Task Completion Requirements

- Both `bun lint` and `bun typecheck` must pass before considering tasks completed.
- Backend tests should be run using `go test ./...` in `apps/server`.

## Project Snapshot

Plum is a lightweight, experimental media server and player suite inspired by platforms like Plex and Jellyfin.

## Core Priorities

1. Performance first.
2. Reliability first.
3. Media playback consistency across devices.

## Maintainability

Long term maintainability is a core priority. If you add new functionality, first check if there are shared logic that can be extracted to a separate module. Duplicate logic across multiple files is a code smell and should be avoided.

Shared TypeScript utilities that multiple clients need should live in `@plum/shared` and be consumed from there rather than duplicated.

## Package Roles

- `apps/server`: Go backend server. Manages media library, transcoding, and SQLite database.
- `apps/web`: React/Vite UI. Modern media player frontend.
- `packages/contracts`: Shared effect/Schema schemas and TypeScript contracts for API and WebSocket protocol.
- `packages/shared`: Shared runtime utilities consumed by the web app (and other TypeScript clients as needed).
- `apps/android-tv`: Kotlin Android TV app (Gradle). Not part of `bun lint` / `bun typecheck`; build with Gradle when working on TV.

### Android TV development

Prerequisites: [Android Studio](https://developer.android.com/studio) with Android SDK, or a standalone SDK with `ANDROID_HOME` set. Use **JDK 17 or 21** for Gradle (e.g. Android Studio’s bundled JBR): some Gradle/Kotlin DSL versions do not run on **JDK 26+**, which surfaces as a cryptic `IllegalArgumentException` with a version number during settings script compilation.

1. **SDK path**: Copy [`apps/android-tv/local.properties.example`](apps/android-tv/local.properties.example) to `apps/android-tv/local.properties` and set `sdk.dir=...`, or export `ANDROID_HOME` (the helper script will write `local.properties` from it on first run).
2. **Build debug APK**: From repo root, `bun run android:assemble` (runs `./scripts/android-tv.sh :app:assembleDebug`).
3. **Install on a connected device/emulator**: `bun run android:install` (device must be running; use an Android TV system image for TV behavior).
4. **Install and launch on device**: `bun run android:deploy` (default desk TV at `192.168.2.11:5555`: `adb connect`, release APK, `adb install -r`, then starts the TV app). Agents: full JDK/SDK/`adb` notes and intent details are in [`apps/android-tv/AGENT_DEPLOY.md`](apps/android-tv/AGENT_DEPLOY.md).
5. **IDE**: Open the **`apps/android-tv`** directory in Android Studio (File → Open) for Gradle sync, Run/Debug, Logcat, and Kotlin editing. Compose Previews for TV are limited; use a TV emulator or hardware for real UI.
6. **Backend**: Run Plum server (`bun run dev:server` or your usual command); the app defaults to `http://10.0.2.2:8080` on the emulator (host loopback). Physical devices need your LAN IP and cleartext is allowed via `network_security_config`.

## Effects & State

The project aims to use `Effect` for managing side effects and domain logic, ensuring robust error handling and composability.

## Future Plans

- Android TV: remaining v1 work is real search UI, extra screens polish, and automated tests plus device smoke runs (see [.plans/ANDROID_PLAN.md](.plans/ANDROID_PLAN.md)). Playback includes Media3 + bearer `OkHttpDataSource`, `/ws` attach + `playback_session_update`, ~10s progress + pause/end sync, transcoding audio cycling via `PATCH` session audio, client-side subtitle cycling, and focused-scale TV controls.
- Enhanced transcoding pipeline.
- Multi-user support.
