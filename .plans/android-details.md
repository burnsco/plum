# High-level shape

```text
Android TV App
  ├── UI layer
  ├── App/state layer
  ├── Data layer
  ├── Playback layer
  └── Local cache/storage

          ↓ HTTPS

Your Plum API
  ├── Auth
  ├── Users / profiles
  ├── Libraries
  ├── Metadata
  ├── Search
  ├── Continue watching
  ├── Playback session
  ├── Stream decision
  ├── Transcode/direct play
  └── Subtitles / audio tracks

          ↓

Media files + ffmpeg/transcoding pipeline
```

For Android TV, Google’s current direction is: build the app as a normal Android app, use **Compose for TV** for the big-screen UI, and use **Media3 / ExoPlayer** for playback. ([Android Developers][1])

## Recommended TV stack

I’d use:

- **Kotlin**
- **Android Studio**
- **Jetpack Compose**
- **Compose for TV**
- **Media3 / ExoPlayer**
- **Retrofit + OkHttp**
- **Coil**
- **Room or DataStore**
- **Hilt**
- **ViewModel + StateFlow**

Why this stack:

- Jetpack Compose is Android’s recommended modern UI toolkit. ([Android Developers][2])
- Compose for TV is the TV-specific UI path for Android TV apps. ([Android Developers][1])
- Media3 is Google’s current media stack, and ExoPlayer lives there. ([Android Developers][3])

## App modules I’d split out

```text
app/
core/
  network/
  model/
  common/
feature-auth/
feature-home/
feature-library/
feature-details/
feature-search/
feature-player/
feature-settings/
data/
playback/
```

That gives you clean separation so the TV app doesn’t become one giant mess.

## Screen flow

This is the typical media-server TV flow:

```text
Splash
  → Server select / login
  → Profile select
  → Home

Home
  → Continue Watching
  → Recently Added
  → Movies
  → Shows
  → Collections
  → Search
  → Settings

Item Details
  → Play
  → Resume
  → Episodes / Seasons
  → Versions
  → Audio/Subtitles
  → Related / Recommended

Player
  → Pause/seek
  → Subtitle select
  → Audio track select
  → Playback speed
  → Next episode
  → Back to details
```

On TV, this needs to be designed around **D-pad navigation, focus states, and 10-foot readability**, not desktop/web interaction. Android’s TV guidance explicitly calls that out. ([Android Developers][1])

## The 5 layers that matter

### 1. UI layer

Use Compose for TV for:

- home rails
- poster rows
- hero banners
- details pages
- search UI
- settings
- playback overlays

Important TV-specific concerns:

- focus ring / focused card scaling
- remote-friendly nav
- very little dense text
- oversized hit areas
- predictable back behavior

This is where a lot of TV apps fail. Web UI habits do not transfer well.

---

### 2. Data/API layer

Your TV app should mostly be a **consumer** of your backend.

Example API groups:

```text
POST   /auth/login
GET    /me
GET    /home
GET    /libraries
GET    /libraries/:id/items
GET    /items/:id
GET    /items/:id/playback-info
POST   /sessions/start
POST   /sessions/:id/progress
POST   /sessions/:id/stop
GET    /search?q=
GET    /users/:id/continue-watching
POST   /users/:id/watch-state
GET    /items/:id/subtitles
GET    /items/:id/streams
```

The TV app should not do heavy media logic on its own unless needed. Let the backend decide:

- direct play vs transcode
- stream URL
- subtitle strategy
- bitrate profile
- container/codec compatibility

That keeps your clients thinner and easier to maintain.

---

### 3. Playback layer

This is the heart of it.

Use **Media3 / ExoPlayer** for:

- HLS playback
- progressive file playback
- subtitles
- multiple audio tracks
- seeking
- buffering
- playback state
- track selection

Media3’s player model is built around the `Player` interface and ExoPlayer is the recommended app-level playback engine. Media3 also has Compose UI support if you want to build player UI in Compose. ([Android Developers][4])

For your backend, the most practical stream outputs are:

```text
1. Direct play URL
2. HLS transcoded stream (.m3u8)
3. Optional progressive fallback
```

For a Jellyfin-style setup, your backend usually returns something like:

```json
{
  "sessionId": "abc123",
  "streamType": "hls",
  "streamUrl": "https://api.example.com/stream/abc123/master.m3u8",
  "canSeek": true,
  "audioTracks": [...],
  "subtitleTracks": [...],
  "resumePositionMs": 1923000
}
```

That lets the TV app stay simple.

---

### 4. Session/reporting layer

You need this or your app will feel incomplete.

While playback is happening, report:

- session started
- current position every few seconds
- paused / resumed
- completed / stopped
- subtitle/audio changes if you care
- device info / client name

This powers:

- continue watching
- resume playback
- multi-device session view
- “now playing”
- next episode logic

A media app without solid playback reporting feels broken fast.

---

### 5. Local storage/cache layer

Use **Room** or **DataStore** for:

- auth token
- selected server
- user/profile
- playback preferences
- subtitle defaults
- local cached home rails
- search history
- continue-watching snapshot

Do not rely entirely on live API calls for every movement. TV apps feel sluggish when every screen is cold-fetched.

## Suggested domain models

Keep these models clear:

```text
User
Profile
Library
MediaItem
Movie
Series
Season
Episode
PlaybackInfo
StreamVariant
SubtitleTrack
AudioTrack
PlaybackSession
WatchProgress
SearchResult
```

For a media app, bad models cause pain everywhere.

## A strong playback pipeline

This is the flow I’d implement:

```text
User selects item
  → TV app requests playback info
  → Backend inspects file + device profile
  → Backend decides direct play / remux / transcode
  → Backend returns stream URL + track info + resume time
  → ExoPlayer starts playback
  → TV app sends progress updates
  → Backend updates watch state
```

And the backend decision logic should roughly be:

```text
If codec/container/audio/subtitle all supported
  → direct play
Else if container mismatch only
  → remux
Else
  → transcode
```

That is basically the same class of problem Jellyfin/Plex solve.

## Device profiles matter

You’ll want per-device or per-client playback profiles, such as:

```text
Android TV profile
  supported video codecs
  supported audio codecs
  preferred containers
  max bitrate
  subtitle capabilities
```

Because playback success depends on what the client can actually handle. Media3 supports broad Android coverage, but actual device capability still varies. ([Android Developers][3])

## Search architecture

Split search into:

- **server search** for library items
- maybe **global discovery** later
- recent searches cached locally

TV search should be dead simple:

- open search screen
- on-screen keyboard
- optional voice later
- big result tiles

## Images and metadata

Use image endpoints for:

- posters
- backdrops
- logos
- thumbnails
- episode stills

Use **Coil** to cache aggressively. TV UIs lean heavily on image grids and rows, so weak image caching makes the whole app feel cheap.

## Remote navigation rules

This part matters more than most people think.

Every screen needs:

- deterministic focus entry point
- deterministic focus restore
- no focus traps
- visible selected state
- good back-stack behavior
- no weird modal dead ends

Android TV apps are built around controller handling and TV navigation patterns, so this is not optional polish; it’s core functionality. ([Android Developers][1])

## Dev workflow without a TV first

Yes, you can do a lot before touching a real TV:

- Android TV emulator for install/debug
- Compose previews for UI iteration
- local backend running on your machine
- test player against sample HLS/direct streams

Android’s TV docs support emulator-based development, and the emulator is the right place to start. ([Android Developers][1])

But for a media app, do not trust emulator-only testing for final playback. Real hardware catches codec, buffering, remote-feel, and decoder issues sooner. Media3 also documents device variation and known device-specific behavior. ([Android Developers][5])

## What I would build first

Order matters. Do this:

### Phase 1

- login to server
- home screen with poster rows
- movie details page
- basic playback with one stream type
- progress reporting

### Phase 2

- shows/seasons/episodes
- resume playback
- subtitles/audio track switching
- continue watching

### Phase 3

- search
- settings
- profile switching
- better caching
- polished player overlay

### Phase 4

- recommendations
- next episode autoplay
- quality selection
- downloads only if you really need them

## My honest recommendation for your stack

For your project, I would not try to over-share UI code from the React frontend to TV.

Do this instead:

- **Go backend** stays shared
- **shared API schema** stays shared conceptually
- **Android TV app** is native Kotlin
- **web app** stays React
- reuse backend contracts, not frontend rendering

That gives you the cleanest result.

## The minimum real library set

If you want the practical “what do I install” list:

```text
androidx.tv:tv-foundation
androidx.tv:tv-material
androidx.media3:media3-exoplayer
androidx.media3:media3-ui
androidx.media3:media3-ui-compose
retrofit2
okhttp3
coil-compose
androidx.room
androidx.datastore
hilt-android
androidx.lifecycle-viewmodel-compose
kotlinx-coroutines
```

Compose for TV artifacts live under AndroidX TV, and Media3 is the supported media stack today. ([Android Developers][6])

## Best next step

Before writing much TV code, define these 3 backend contracts first:

1. **Home payload**
2. **Item details payload**
3. **Playback-info payload**

If those are well designed, the TV client becomes much easier.

I can sketch those three JSON contracts next, and that would probably save you a lot of rework.

[1]: https://developer.android.com/training/tv/get-started?utm_source=chatgpt.com "Get started with TV apps - Android Developers"
[2]: https://developer.android.com/compose?utm_source=chatgpt.com "Jetpack Compose UI App Development Toolkit"
[3]: https://developer.android.com/media/media3?utm_source=chatgpt.com "Introduction to Jetpack Media3 | Android media"
[4]: https://developer.android.com/media/media3/exoplayer?utm_source=chatgpt.com "Media3 ExoPlayer | Android media"
[5]: https://developer.android.com/media/media3/exoplayer/demo-application?utm_source=chatgpt.com "ExoPlayer demo application | Android media"
[6]: https://developer.android.com/jetpack/androidx/releases/tv?utm_source=chatgpt.com "tv | Jetpack - Android Developers"
