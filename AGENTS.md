# AGENTS.md

## Toolchain

- **Node.js**: Root `package.json` sets `engines.node` to `^20.19.0 || >=22.12.0` so it matches **Vite 8**, **@vitejs/plugin-react**, and **oxlint**. Odd majors (e.g. 21.x) are outside that range; use **20.19+**, **22.12+**, or current **24** as appropriate. [`.nvmrc`](.nvmrc) is **`22`** as a default for `nvm use` (pick a 22.x release **≥ 22.12** if you pin an exact patch).
- **Bun**: `engines.bun` is **`>=1.2.0`**. Use Bun at the repo root for installs and scripts (`bun install`, `bun run …`).
- **Go** (backend): Follow the `go` version in [`apps/server/go.mod`](apps/server/go.mod) for `apps/server` builds and tests.
- **`effect`**: **`effect@4.0.0-beta.43`** from the **npm** registry (Effect v4 beta line). Keep the same version in `apps/web`, `packages/contracts`, and `packages/shared`.

## Task Completion Requirements

- `bun run validate` (same as `validate:fast`) must pass before considering tasks completed: root lint and typecheck across `apps/web`, `packages/shared`, and `packages/contracts`, plus `bun run server:test` in `apps/server`.
- For changes scoped to one surface, you may run the matching per-platform command (see **Validation**) instead of the full monorepo fast path, but still cover anything your edits could break.
- Backend-only work can use `go test ./...` in `apps/server` (or `bun run validate:server`) instead of the full JS toolchain.

## Validation

Root scripts in `package.json`:

| Command | Purpose |
| --- | --- |
| `bun run validate` / `validate:fast` | Frequent check: lint + typecheck (web, shared, contracts) + server tests. Default pre-commit style bar. |
| `validate:full` | Merge gate: everything in `validate:fast`, plus web unit tests, web production build, Go build, Android `lintDebug` + `assembleDebug`. |
| `validate:web` | Web stack only: lint/typecheck for web + shared + contracts, web tests, web build. |
| `validate:server` | Server only: `go test ./...` + Go build via `apps/server` scripts. |
| `validate:android` | Android TV only: `lintDebug` + `assembleDebug` (requires SDK; see **Android TV development**). |

`validate:full` currently runs `bun run --cwd apps/web test` (full Vitest suite). For the subset that excludes `App.test.tsx`, use `bun run --cwd apps/web test:stable`; for that file alone, `bun run --cwd apps/web test:app`.

Per [.plans/fix-up.md](.plans/fix-up.md) **Milestone A**, the fast path, full gate, per-platform scripts, SQLite WAL/SHM ignores, and toolchain pinning (**Toolchain** above) are in place; see the plan file for remaining milestone items.

## Project Snapshot

Plum is a lightweight, experimental media server and player suite inspired by platforms like Plex and Jellyfin.

## Core Priorities

1. Performance first.
2. Reliability first.
3. Media playback consistency across devices.

## HTTP streaming routes

Handlers that stream large or long-lived bodies (playback, transcodes, downloads) should clear the per-request write deadline (e.g. `httputil.ClearStreamWriteDeadline`) so the server default `WriteTimeout` does not abort mid-stream.

## Maintainability

Long term maintainability is a core priority. If you add new functionality, first check if there are shared logic that can be extracted to a separate module. Duplicate logic across multiple files is a code smell and should be avoided.

Shared TypeScript utilities that multiple clients need should live in `@plum/shared` and be consumed from there rather than duplicated.

## Package Roles

- `apps/server`: Go backend server. Manages media library, transcoding, and SQLite database.
- `apps/web`: React/Vite UI. Modern media player frontend.
- `packages/contracts`: Shared effect/Schema schemas and TypeScript contracts for API and WebSocket protocol.
- `packages/shared`: Shared runtime utilities consumed by the web app (and other TypeScript clients as needed).
- `apps/android-tv`: Kotlin Android TV app (Gradle). Not part of root `bun lint` / `bun typecheck`; use `bun run validate:android` or Gradle via `bun run android:assemble` / `android:lint` when working on TV.

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
