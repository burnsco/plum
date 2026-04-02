# Android TV V1 Plan for Plum

## Summary

- Build a new native Android TV client in [apps/android-tv](/home/cburns/apps/plum/apps/android-tv) using Kotlin, Compose for TV, and Media3/ExoPlayer.
- Scope v1 to TV only: server connect, login, home, library browse, movie/show details, search, direct/HLS playback, progress sync, and resume playback.
- Reuse Plum’s existing browse/search/playback APIs where they are already solid, and add a small backend compatibility slice first so the native app does not have to fight browser-only auth and data-shape assumptions.
- Keep v1 single-account only. Do not pull profiles, admin features, discover/downloads, poster management, or handheld Android into the first milestone.

## Public Interfaces

- Add `POST /api/auth/device-login` that returns `DeviceLoginResponse { user, sessionToken, expiresAt }`.
- Update auth handling in [apps/server](/home/cburns/apps/plum/apps/server) so protected HTTP routes accept either the existing session cookie or `Authorization: Bearer <sessionToken>`. Reuse the existing `sessions` table; do not introduce a separate device-token system yet.
- Update `/api/auth/me` and `/api/auth/logout` to work with bearer auth as well as cookies.
- Update `/ws` auth to accept bearer-authenticated native clients and allow missing `Origin` only for that bearer-authenticated path. Keep current origin enforcement for browser/cookie clients.
- Add `GET /api/libraries/{id}/shows/{showKey}/episodes` returning `ShowEpisodesResponse { seasons: ShowSeason[] }`.
- Define `ShowSeason` as `{ seasonNumber, label, episodes }`, with `episodes` containing the fields needed for TV detail/playback flows: id, title, season, episode, duration, overview, artwork/thumbnail URLs, and progress/resume fields already present on media items.
- Keep the existing playback contracts and WebSocket playback-session update contract in [packages/contracts](/home/cburns/apps/plum/packages/contracts). The Android app should send `clientCapabilities` on playback session creation and implement the existing `attach_playback_session` / `detach_playback_session` messages.

## Implementation Phases

- Phase 1: backend native-client slice. Add `device-login`, bearer auth support, native WebSocket auth, and the new show-episodes query/handler/tests. Also switch the existing web show-detail flow away from paging an entire library for episode lists so both clients rely on the same efficient backend path.
- Phase 2: Android project bootstrap. Create a standalone Gradle project under `apps/android-tv` with modules `app`, `core-model`, `core-network`, `core-data`, `core-player`, `feature-auth`, `feature-home`, `feature-library`, `feature-details`, `feature-search`, and `feature-settings`. Use Hilt, Retrofit + OkHttp, Coil, coroutines/StateFlow, and DataStore. Use DataStore only in v1; do not add Room yet.
- Phase 3: auth and app shell. Implement splash/session restore via saved server URL + bearer token, a manual server URL entry screen, email/password login, logout, and a minimal settings screen for server switch + logout. Add Android network-security config so local/LAN `http://` Plum servers work during self-hosted/dev use.
- Phase 4: browse and detail UX. Home should use `/api/home` for continue watching and recently added, plus `/api/libraries` and first-page library queries for preview rails. Library screens should use paged poster grids with deterministic focus entry/restore. Movie details should use the current movie-details endpoint. Show details should use show metadata plus the new episodes endpoint with season switching and resume/play actions.
- Phase 5: playback and sync. Build a `core-player` wrapper around Media3 that supports direct URLs and HLS, injects bearer auth on playlists/segments/subtitles/artwork requests, opens a persistent `/ws` connection after login, attaches to playback sessions for transcode readiness and audio-revision changes, posts progress every 10 seconds plus on pause/stop/completion, and supports audio/subtitle switching and resume playback.
- Phase 6: TV polish and hardware validation. Apply 10-foot UI rules everywhere: larger typography, oversized hit targets, obvious focused state, stable back behavior, and no focus traps. Use emulator-first development, then validate on at least one real Android TV device over ADB before calling v1 done.

## Test Plan

- Preserve current repo gates: `bun lint`, `bun typecheck`, and `go test ./...` in `apps/server`.
- Add Go tests for bearer-auth access to protected routes, `device-login`/logout/session restore, WebSocket auth with bearer token and no `Origin`, and show-episodes ordering/grouping.
- Add Android unit tests for auth/session persistence, repository mapping, capability reporting, and playback state handling.
- Add Compose/UI tests for login flow, D-pad focus entry/restore, back-stack behavior, season switching, and search results.
- Add instrumented playback checks on emulator and one real TV device for direct play, HLS transcode, transcode revision updates after audio-track changes, subtitle selection, 10-second heartbeat sync, and resume after app restart.

## Assumptions And Defaults

- First target is Android TV / Google TV only. Fire TV and handheld Android are later follow-ons.
- Distribution for this milestone is sideload/dev-first, not Play Store launch.
- V1 is single-account only; profile selection and switching stay out of scope.
- Do not attempt shared UI code with the React app. Share backend behavior and contracts, not rendering.
- Do not introduce automated TS-to-Kotlin schema/codegen in v1. Mirror the contract types manually in Kotlin and revisit generation only if Plum adds another native client.
- Do not build a new aggregated “mega home” endpoint in v1. If startup performance is not good enough after the first Android pass, treat that as a focused backend follow-up.
