# PlayerContext responsibility audit

This document maps what `PlayerProvider` composes today, grouped for the playback refactor (see `.plans/fix-up.md`). **Server video session** logic lives in `usePlaybackSession.ts`; **queue / navigation** in `usePlaybackQueue.ts`; **library + web playback preferences** in `usePlaybackPreferences.ts` (wired from `useLibraries`); **transport** (volume, media elements, seek) remains in `PlayerContext.tsx`. The provider still exposes **three React contexts** (`PlayerSessionContext`, `PlayerQueueContext`, `PlayerTransportContext`); hooks `usePlayerSession`, `usePlayerQueue`, and `usePlayerTransport` exist—`usePlayer()` merges all three.

---

## 1. Playback session

**Server-aligned video session**

- State: `videoSession` (`VideoSessionState`), refs `videoSessionRef`, `activeVideoItemIdRef`, `lastReadyVideoUrlRef`, `lastReadyVideoItemRef`, `prevVideoSessionIdRef` (ready-URL caching so HLS does not jump to a partial manifest during transcode startup).
- Types/helpers in module: `PlaybackSessionSource`, `toVideoSessionState`, `mergePlaybackTracks`, `resolvePlaybackStreamUrl`, `applyPlaybackSession`, `createClientPlaybackSession`.
- Lifecycle: `closeVideoSession` (HTTP + WS detach), effect that sends `attach_playback_session` when WS connects.
- Real-time updates: `useEffect` on `latestEvent` / `eventSequence` for `playback_session_update` (ready / error / closed, revision gating for `desiredRevision` vs `currentRevision`).

**Derived session-facing fields**

- `videoSourceUrl`, `playbackDurationSeconds` (video: server duration or item duration), `videoDelivery`, `videoAudioIndex`, `burnEmbeddedSubtitleStreamIndex`.

**Cross-cutting with queue**

- Starting or switching video items runs `createPlaybackSession` / `applyPlaybackSession` and may patch the active queue row via `mergePlaybackTracks` (subtitle/audio metadata from API).

**Not in context**

- Current playback time / scrub position: owned by the media element and UI (e.g. `PlaybackDock`), not this context—only duration and stream URL are supplied here.

---

## 2. Queue / navigation

**State**

- `playbackSession` holds: `queue`, `queueIndex`, `shuffle`, `repeatMode`, `activeMode` (`video` | `music`), plus dock-related `isDockOpen` and `viewMode` (see UI below).
- `musicBaseQueue`: pre-shuffle ordered list; `shuffle` rebuilds display queue from this base.

**Entry points**

- `playVideoQueue`, `playVideoQueueIndex` (video): set queue + index, tear down prior video session, warm subtitles, resolve prefs, create server session.
- `playMovie`, `playEpisode` (may `getShowEpisodes` + `sortEpisodes`), `playShowGroup`, `playMusicCollection` (`sortMusicTracks`, optional `shuffleQueue`), `playMedia` (dispatches by item type).

**Navigation / rules**

- `playNextInQueue` / `playPreviousInQueue`: video delegates to `playVideoQueueIndex`; music updates `queueIndex` with `repeatMode === "all"` wrap.
- `toggleShuffle` / `cycleRepeatMode`: music only; shuffle uses `musicBaseQueue` and `indexOfQueueItem` / `shuffleQueue` from `lib/playback/playerQueue`.

**Autoplay**

- Video: implicit “queue” only; no auto-advance loop in this file (end-of-item behavior lives in UI / media callbacks elsewhere).
- Music: repeat modes affect next/prev at list ends.

---

## 3. Preferences

**Hook: `usePlaybackPreferences`**

- Takes `libraries` from `PlayerProvider` (`useLibraries`), exposes `libraryPrefsForItem`, `effectivePreferredAudioLanguage`, `initialAudioStreamIndex`, `initialBurnEmbeddedSubtitleStreamIndex`, `audioIndexForSubtitleBurnChange`.
- The same API is exposed to descendants via `PlaybackPreferencesProvider` / `usePlayerPlaybackPreferences` (`playbackPreferencesContext.tsx`) so `PlaybackDock` does not call `useLibraries` only for prefs.
- Pure helper `resolveInitialBurnSubtitleStreamIndex` lives in `usePlaybackPreferences.ts` (subtitle burn selection from web defaults + library + item tracks).

**Note**

- Persistence remains in `playbackPreferences.ts` / localStorage; the hook re-reads storage when resolving. Volume/mute are local React state in the provider (see transport).

---

## 4. UI state

**Dock / shell**

- `isDockOpen`, `viewMode` (currently always `"window"`; fullscreen uses the Fullscreen API separately per type comment).
- `lastEvent`: human-readable status line (“Stream ready”, errors, “Switching audio track…”) for dock chrome—not the same as structured playback errors on `videoSession`.

**Aggregated session object**

- `playbackSession` and `activeItem` are exposed for components that need the full snapshot (`PlaybackDock`, tests).

**Ws connectivity**

- `wsConnected` from `useWs()` is passed through session context (UI indicator + attach gating).

---

## 5. Side effects

**HTTP / API**

- `createPlaybackSession`, `closePlaybackSession`, `updatePlaybackSessionAudio`, `warmEmbeddedSubtitleCaches`, `getShowEpisodes`.

**WebSocket**

- `sendCommand` via `sendPlaybackCommand`: `attach_playback_session`, `detach_playback_session`.

**DOM / media elements**

- `registerMediaElement`, refs for audio/video slots; syncing `volume` / `muted` to elements; `pauseAllMediaElements`; `togglePlayPause`, `seekTo` (with `clampVideoSeekSeconds` for video).
- `exitBrowserFullscreen` / `enterFullscreen` (latter ensures dock open; fullscreen exit uses browser API).

**Logging / fire-and-forget**

- `ignorePromise` / `ignorePromiseAlwaysLogUnexpected` around async API calls.

---

## Split boundaries (target hooks)

Rough mapping to planned extractions:

| Planned hook | Take from provider today |
|--------------|----------------------------|
| **`usePlaybackSession`** | `videoSession` + refs for ready URL / revisions; `applyPlaybackSession`, `createClientPlaybackSession`, `closeVideoSession`; WS `playback_session_update` handler; attach effect; derived `videoSourceUrl`, duration, delivery, audio index, burn index; `mergePlaybackTracks` and session-related helpers. *Optional:* keep `lastEvent` for stream lifecycle here or move to a thin “status” slice. |
| **`usePlaybackQueue`** | **Implemented** in `usePlaybackQueue.ts`: `musicBaseQueue`, derived queue fields, all `play*` entry points, next/prev, shuffle/repeat; takes `PlaybackSessionVideoApi` to start video items and tear down prior sessions. |
| **`usePlaybackPreferences`** | **Implemented** in `usePlaybackPreferences.ts`: resolves prefs per `MediaItem` from libraries + `readStoredPlayerWebDefaults` / `resolveEffectiveWebTrackDefaults`; exposes `initialAudioStreamIndex`, `initialBurnEmbeddedSubtitleStreamIndex`, `audioIndexForSubtitleBurnChange`, etc. |
| **Transport (existing name: `usePlayerTransport`)** | `volume`, `muted`, `registerMediaElement`, `togglePlayPause`, `seekTo`, `setVolume`, `setMuted`, `enterFullscreen`, `exitFullscreen`. **`changeAudioTrack` / `changeEmbeddedSubtitleBurn`** are hybrid: they update UI state and call session APIs—either stay with session hook or expose “session commands” from session and invoke from transport UI. |
| **UI-only** | `dismissDock` orchestrates session teardown + queue reset + DOM; likely remains a top-level coordinator or moves with a small “shell” hook that composes session + queue. |

**Dependency direction (recommended)**

1. Preferences hook: no dependency on queue/session state (only library list + storage).
2. Session hook: may call preference helpers and API; should not depend on queue navigation except via callbacks or a narrow “active item id” contract.
3. Queue hook: depends on session for “play this index” (new server session per video item).
4. Transport: depends on media refs and optionally session commands for audio/subtitle.

---

## Consumers (reference)

- `usePlayerQueue`: `Dashboard`, `Home`, `MovieDetail`, `ShowDetail`.
- `usePlayerSession` + `usePlayerQueue` + `usePlayerTransport`: `PlaybackDock`, `MusicNowPlayingBar`.
- `usePlayer()`: tests and any code needing the full merged object.

---

## Changelog

- **2026-04-07**: Initial audit for Milestone B (“Audit PlayerContext responsibilities”).
