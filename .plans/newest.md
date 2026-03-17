Good, I have enough context now. Let me also check if there's any existing `direct play` / passthrough logic in the playback handler.Now I have a thorough picture of the codebase. Let me write up a comprehensive answer.Now I have a full picture. Let me write the comprehensive answer.

Here's a thorough breakdown of both features and how to implement them in Plum.

---

## CPU-aware concurrency for scanning & identification

Right now your `NewLibraryScanManager` hardcodes three values:

```go
enrichSem:          make(chan struct{}, 2),
identifySem:        make(chan struct{}, 4),
MaxConcurrentScans: 2,
```

Jellyfin scales these against `Environment.ProcessorCount`. The pattern is straightforward — you detect cores at startup and derive sensible semaphore sizes from that number.

**In `library_scan_jobs.go`:**

```go
import "runtime"

func NewLibraryScanManager(sqlDB *sql.DB, meta metadata.Identifier, hub *ws.Hub) *LibraryScanManager {
    cpus := runtime.NumCPU()

    // Scanning: 1 per 2 cores, min 1, max 4
    maxScans := max(1, min(4, cpus/2))

    // Identify (network-bound, can run more): 1 per core, min 2, max 8
    maxIdentify := max(2, min(8, cpus))

    // Enrich (also network-bound): half of identify
    maxEnrich := max(1, min(4, cpus/2))

    return &LibraryScanManager{
        // ...existing fields...
        enrichSem:          make(chan struct{}, maxEnrich),
        identifySem:        make(chan struct{}, maxIdentify),
        MaxConcurrentScans: maxScans,
    }
}
```

You can also expose these as settings (just like `Threads` in `TranscodingSettings`) so a user can override them. Add to `TranscodingSettings` or a new `LibrarySettings` struct:

```go
type LibrarySettings struct {
    MaxConcurrentScans    int `json:"maxConcurrentScans"`    // 0 = auto
    MaxConcurrentIdentify int `json:"maxConcurrentIdentify"` // 0 = auto
    MaxConcurrentEnrich   int `json:"maxConcurrentEnrich"`   // 0 = auto
}
```

Then in `NewLibraryScanManager`, check if the stored value is `> 0` and use it, otherwise fall back to the CPU-derived value. This is exactly how `Threads: 0` already works in your transcoding settings — zero means "auto."

---

## Jellyfin-style direct play / transcode decision

This is the more significant change. Right now `CreateSession` in `playback_handlers.go` **always** creates an HLS transcode — it never checks whether the file can be played as-is. Jellyfin's logic is essentially:

> **Direct play** → serve the raw file if the client claims it can handle the codec, container, and bitrate.  
> **Direct stream** → remux into a compatible container without re-encoding video (fast, lossless quality).  
> **Transcode** → re-encode video when nothing else works.

Here's how to add that logic to your codebase.

### Step 1: probe more stream info

Your `probeVideoStream` in `plan.go` only grabs `codec_name` and `pix_fmt`. You need container, bitrate, and audio codec too:

```go
type MediaStreamInfo struct {
    VideoCodec    string
    PixelFmt      string
    AudioCodec    string
    AudioChannels int
    Container     string // "matroska", "mp4", "avi", etc.
    BitRate       int64  // bits/sec, 0 = unknown
}

func ProbeMediaInfo(path string) MediaStreamInfo {
    cmd := exec.Command(
        "ffprobe",
        "-v", "error",
        "-show_entries", "stream=codec_name,pix_fmt,channels,codec_type:format=format_name,bit_rate",
        "-of", "json",
        path,
    )
    out, err := cmd.Output()
    if err != nil {
        return MediaStreamInfo{}
    }

    var payload struct {
        Streams []struct {
            CodecName  string `json:"codec_name"`
            PixelFmt   string `json:"pix_fmt"`
            Channels   int    `json:"channels"`
            CodecType  string `json:"codec_type"`
        } `json:"streams"`
        Format struct {
            FormatName string `json:"format_name"`
            BitRate    string `json:"bit_rate"`
        } `json:"format"`
    }
    if err := json.Unmarshal(out, &payload); err != nil {
        return MediaStreamInfo{}
    }

    info := MediaStreamInfo{Container: payload.Format.FormatName}
    if br, err := strconv.ParseInt(payload.Format.BitRate, 10, 64); err == nil {
        info.BitRate = br
    }
    for _, s := range payload.Streams {
        switch s.CodecType {
        case "video":
            info.VideoCodec = s.CodecName
            info.PixelFmt = s.PixelFmt
        case "audio":
            if info.AudioCodec == "" {
                info.AudioCodec = s.CodecName
                info.AudioChannels = s.Channels
            }
        }
    }
    return info
}
```

### Step 2: the compatibility decision

Add a new file `transcoder/compatibility.go`:

```go
package transcoder

import "plum/internal/db"

type PlaybackMethod int

const (
    DirectPlay   PlaybackMethod = iota
    DirectStream                // remux only, no video re-encode
    Transcode
)

// ClientCapabilities describes what the browser/player supports.
// Populated from a query param or the Accept header on CreateSession.
type ClientCapabilities struct {
    // Video codecs the client can decode
    H264  bool
    HEVC  bool
    AV1   bool
    VP9   bool

    // Containers the client can handle natively
    MP4   bool
    WebM  bool
    MKV   bool // browsers generally can't, native players can

    // Max bitrate the client can handle (0 = unlimited)
    MaxBitrateBps int64
}

// DefaultBrowserCapabilities is a safe baseline for web clients.
func DefaultBrowserCapabilities() ClientCapabilities {
    return ClientCapabilities{
        H264: true,
        MP4:  true,
        MaxBitrateBps: 0,
    }
}

func DecidePlaybackMethod(info MediaStreamInfo, caps ClientCapabilities, settings db.TranscodingSettings) PlaybackMethod {
    // 1. Check bitrate ceiling
    if caps.MaxBitrateBps > 0 && info.BitRate > caps.MaxBitrateBps {
        return Transcode
    }

    videoOK := isVideoCompatible(info, caps)
    audioOK := isAudioCompatible(info)
    containerOK := isContainerCompatible(info, caps)

    // 2. Everything compatible → direct play
    if videoOK && audioOK && containerOK {
        return DirectPlay
    }

    // 3. Video is compatible but container/audio isn't → remux (direct stream)
    // Remuxing is nearly instant and lossless — always prefer it over transcode
    if videoOK && containerOK {
        return DirectStream
    }

    // 4. Must re-encode
    return Transcode
}

func isVideoCompatible(info MediaStreamInfo, caps ClientCapabilities) bool {
    switch info.VideoCodec {
    case "h264":
        return caps.H264 && !isTenBitStream(videoStreamInfo{PixelFmt: info.PixelFmt})
    case "hevc":
        return caps.HEVC
    case "av1":
        return caps.AV1
    case "vp9":
        return caps.VP9
    default:
        return false // mpeg2, vc1, etc. — always transcode
    }
}

func isAudioCompatible(info MediaStreamInfo) bool {
    switch info.AudioCodec {
    case "aac", "mp3", "opus", "vorbis":
        return true
    case "ac3", "eac3", "dts", "truehd":
        // These are fine for native apps but not web browsers
        // You could make this a capability flag too
        return false
    default:
        return false
    }
}

func isContainerCompatible(info MediaStreamInfo, caps ClientCapabilities) bool {
    // format_name can be comma-separated ("mov,mp4,m4a,3gp,3g2,mj2")
    for _, f := range strings.Split(info.Container, ",") {
        switch strings.TrimSpace(f) {
        case "mp4", "mov":
            if caps.MP4 { return true }
        case "webm":
            if caps.WebM { return true }
        case "matroska":
            if caps.MKV { return true }
        }
    }
    return false
}
```

### Step 3: wire it into `CreateSession`

Update your handler to accept capabilities from the client and branch on the decision:

```go
func (h *PlaybackHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
    // ... existing media/settings lookup ...

    var payload struct {
        AudioIndex int                           `json:"audioIndex"`
        Caps       *transcoder.ClientCapabilities `json:"capabilities"`
    }
    payload.AudioIndex = -1Good, I have enough context now. Let me also check if there's any existing `direct play` / passthrough logic in the playback handler.Now I have a thorough picture of the codebase. Let me write up a comprehensive answer.Now I have a full picture. Let me write the comprehensive answer.

Here's a thorough breakdown of both features and how to implement them in Plum.

---

## CPU-aware concurrency for scanning & identification

Right now your `NewLibraryScanManager` hardcodes three values:

```go
enrichSem:          make(chan struct{}, 2),
identifySem:        make(chan struct{}, 4),
MaxConcurrentScans: 2,
```

Jellyfin scales these against `Environment.ProcessorCount`. The pattern is straightforward — you detect cores at startup and derive sensible semaphore sizes from that number.

**In `library_scan_jobs.go`:**

```go
import "runtime"

func NewLibraryScanManager(sqlDB *sql.DB, meta metadata.Identifier, hub *ws.Hub) *LibraryScanManager {
    cpus := runtime.NumCPU()

    // Scanning: 1 per 2 cores, min 1, max 4
    maxScans := max(1, min(4, cpus/2))

    // Identify (network-bound, can run more): 1 per core, min 2, max 8
    maxIdentify := max(2, min(8, cpus))

    // Enrich (also network-bound): half of identify
    maxEnrich := max(1, min(4, cpus/2))

    return &LibraryScanManager{
        // ...existing fields...
        enrichSem:          make(chan struct{}, maxEnrich),
        identifySem:        make(chan struct{}, maxIdentify),
        MaxConcurrentScans: maxScans,
    }
}
```

You can also expose these as settings (just like `Threads` in `TranscodingSettings`) so a user can override them. Add to `TranscodingSettings` or a new `LibrarySettings` struct:

```go
type LibrarySettings struct {
    MaxConcurrentScans    int `json:"maxConcurrentScans"`    // 0 = auto
    MaxConcurrentIdentify int `json:"maxConcurrentIdentify"` // 0 = auto
    MaxConcurrentEnrich   int `json:"maxConcurrentEnrich"`   // 0 = auto
}
```

Then in `NewLibraryScanManager`, check if the stored value is `> 0` and use it, otherwise fall back to the CPU-derived value. This is exactly how `Threads: 0` already works in your transcoding settings — zero means "auto."

---

## Jellyfin-style direct play / transcode decision

This is the more significant change. Right now `CreateSession` in `playback_handlers.go` **always** creates an HLS transcode — it never checks whether the file can be played as-is. Jellyfin's logic is essentially:

> **Direct play** → serve the raw file if the client claims it can handle the codec, container, and bitrate.  
> **Direct stream** → remux into a compatible container without re-encoding video (fast, lossless quality).  
> **Transcode** → re-encode video when nothing else works.

Here's how to add that logic to your codebase.

### Step 1: probe more stream info

Your `probeVideoStream` in `plan.go` only grabs `codec_name` and `pix_fmt`. You need container, bitrate, and audio codec too:

```go
type MediaStreamInfo struct {
    VideoCodec    string
    PixelFmt      string
    AudioCodec    string
    AudioChannels int
    Container     string // "matroska", "mp4", "avi", etc.
    BitRate       int64  // bits/sec, 0 = unknown
}

func ProbeMediaInfo(path string) MediaStreamInfo {
    cmd := exec.Command(
        "ffprobe",
        "-v", "error",
        "-show_entries", "stream=codec_name,pix_fmt,channels,codec_type:format=format_name,bit_rate",
        "-of", "json",
        path,
    )
    out, err := cmd.Output()
    if err != nil {
        return MediaStreamInfo{}
    }

    var payload struct {
        Streams []struct {
            CodecName  string `json:"codec_name"`
            PixelFmt   string `json:"pix_fmt"`
            Channels   int    `json:"channels"`
            CodecType  string `json:"codec_type"`
        } `json:"streams"`
        Format struct {
            FormatName string `json:"format_name"`
            BitRate    string `json:"bit_rate"`
        } `json:"format"`
    }
    if err := json.Unmarshal(out, &payload); err != nil {
        return MediaStreamInfo{}
    }

    info := MediaStreamInfo{Container: payload.Format.FormatName}
    if br, err := strconv.ParseInt(payload.Format.BitRate, 10, 64); err == nil {
        info.BitRate = br
    }
    for _, s := range payload.Streams {
        switch s.CodecType {
        case "video":
            info.VideoCodec = s.CodecName
            info.PixelFmt = s.PixelFmt
        case "audio":
            if info.AudioCodec == "" {
                info.AudioCodec = s.CodecName
                info.AudioChannels = s.Channels
            }
        }
    }
    return info
}
```

### Step 2: the compatibility decision

Add a new file `transcoder/compatibility.go`:

```go
package transcoder

import "plum/internal/db"

type PlaybackMethod int

const (
    DirectPlay   PlaybackMethod = iota
    DirectStream                // remux only, no video re-encode
    Transcode
)

// ClientCapabilities describes what the browser/player supports.
// Populated from a query param or the Accept header on CreateSession.
type ClientCapabilities struct {
    // Video codecs the client can decode
    H264  bool
    HEVC  bool
    AV1   bool
    VP9   bool

    // Containers the client can handle natively
    MP4   bool
    WebM  bool
    MKV   bool // browsers generally can't, native players can

    // Max bitrate the client can handle (0 = unlimited)
    MaxBitrateBps int64
}

// DefaultBrowserCapabilities is a safe baseline for web clients.
func DefaultBrowserCapabilities() ClientCapabilities {
    return ClientCapabilities{
        H264: true,
        MP4:  true,
        MaxBitrateBps: 0,
    }
}

func DecidePlaybackMethod(info MediaStreamInfo, caps ClientCapabilities, settings db.TranscodingSettings) PlaybackMethod {
    // 1. Check bitrate ceiling
    if caps.MaxBitrateBps > 0 && info.BitRate > caps.MaxBitrateBps {
        return Transcode
    }

    videoOK := isVideoCompatible(info, caps)
    audioOK := isAudioCompatible(info)
    containerOK := isContainerCompatible(info, caps)

    // 2. Everything compatible → direct play
    if videoOK && audioOK && containerOK {
        return DirectPlay
    }

    // 3. Video is compatible but container/audio isn't → remux (direct stream)
    // Remuxing is nearly instant and lossless — always prefer it over transcode
    if videoOK && containerOK {
        return DirectStream
    }

    // 4. Must re-encode
    return Transcode
}

func isVideoCompatible(info MediaStreamInfo, caps ClientCapabilities) bool {
    switch info.VideoCodec {
    case "h264":
        return caps.H264 && !isTenBitStream(videoStreamInfo{PixelFmt: info.PixelFmt})
    case "hevc":
        return caps.HEVC
    case "av1":
        return caps.AV1
    case "vp9":
        return caps.VP9
    default:
        return false // mpeg2, vc1, etc. — always transcode
    }
}

func isAudioCompatible(info MediaStreamInfo) bool {
    switch info.AudioCodec {
    case "aac", "mp3", "opus", "vorbis":
        return true
    case "ac3", "eac3", "dts", "truehd":
        // These are fine for native apps but not web browsers
        // You could make this a capability flag too
        return false
    default:
        return false
    }
}

func isContainerCompatible(info MediaStreamInfo, caps ClientCapabilities) bool {
    // format_name can be comma-separated ("mov,mp4,m4a,3gp,3g2,mj2")
    for _, f := range strings.Split(info.Container, ",") {
        switch strings.TrimSpace(f) {
        case "mp4", "mov":
            if caps.MP4 { return true }
        case "webm":
            if caps.WebM { return true }
        case "matroska":
            if caps.MKV { return true }
        }
    }
    return false
}
```

### Step 3: wire it into `CreateSession`

Update your handler to accept capabilities from the client and branch on the decision:

```go
func (h *PlaybackHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
    // ... existing media/settings lookup ...

    var payload struct {
        AudioIndex int                           `json:"audioIndex"`
        Caps       *transcoder.ClientCapabilities `json:"capabilities"`
    }
    payload.AudioIndex = -1
    payload.Caps = nil
    if r.ContentLength != 0 {
        _ = json.NewDecoder(r.Body).Decode(&payload)
    }

    caps := transcoder.DefaultBrowserCapabilities()
    if payload.Caps != nil {
        caps = *payload.Caps
    }

    info := transcoder.ProbeMediaInfo(media.Path)
    method := transcoder.DecidePlaybackMethod(info, caps, settings)

    switch method {
    case transcoder.DirectPlay:
        // Return the raw file URL — no session needed
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "method": "direct_play",
            "url":    fmt.Sprintf("/api/media/%d/stream", media.ID),
        })
        return

    case transcoder.DirectStream:
        // TODO: implement remux path (ffmpeg -c:v copy -c:a aac ...)
        // For now, fall through to transcode
        fallthrough

    case transcoder.Transcode:
        state, err := h.Sessions.Create(*media, settings, payload.AudioIndex, user.ID)
        if err != nil {
            http.Error(w, "internal error", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "method": "transcode",
            "session": state,
        })
    }
}
```

Your `/api/media/:id/stream` route already exists (`StreamMedia` in `playback_handlers.go`) — direct play just points there instead of starting HLS.

### Step 4: direct stream (remux) path

For the remux case, add a new session type that runs ffmpeg with `-c:v copy` instead of re-encoding:

```go
// In sessions.go or a new file
func buildRemuxPlan(itemPath, outDir string, audioIndex int) transcodePlan {
    args := []string{
        "-y",
        "-i", itemPath,
        "-map", "0:v:0",
    }
    if audioIndex >= 0 {
        args = append(args, "-map", fmt.Sprintf("0:%d", audioIndex))
    } else {
        args = append(args, "-map", "0:a:0?")
    }
    args = append(args,
        "-c:v", "copy",      // no video re-encode
        "-c:a", "aac",       // normalize audio
        "-b:a", "192k",
    )
    args = appendHLSOutputArgs(args, outDir)

    return transcodePlan{
        Args:         args,
        Mode:         "remux",
        EncodeFormat: "copy",
    }
}
```

This starts HLS output in seconds rather than waiting for the video encoder to spin up.

---

Here's a visual of the full decision flow:---

## Summary of what to do

**CPU-aware concurrency** — a one-liner change. Import `runtime`, call `runtime.NumCPU()` in `NewLibraryScanManager`, and derive semaphore sizes from it. Optionally expose them as `LibrarySettings` so users can override, the same way `Threads: 0` already works for ffmpeg.

**Transcode-only-if-necessary** — four changes across existing files:
1. Expand `probeVideoStream` to also return container, audio codec, and bitrate → rename it `ProbeMediaInfo`.
2. Add a `compatibility.go` with `DecidePlaybackMethod` that runs the three checks (bitrate cap → video codec → audio+container).
3. Update `CreateSession` to call `ProbeMediaInfo` + `DecidePlaybackMethod` before touching `Sessions.Create`. If `DirectPlay`, return the `/stream` URL directly without creating any session.
4. Add a `buildRemuxPlan` (ffmpeg `-c:v copy`) for the direct stream case — this is the fastest path for files that are in the right codec but the wrong container (very common with MKV/H.264).

The client should also send its capabilities in the `POST /api/playback/sessions/:id` body — at minimum `{"capabilities": {"h264": true, "mp4": true}}` from the web player, so the server doesn't have to guess.
    payload.Caps = nil
    if r.ContentLength != 0 {
        _ = json.NewDecoder(r.Body).Decode(&payload)
    }

    caps := transcoder.DefaultBrowserCapabilities()
    if payload.Caps != nil {
        caps = *payload.Caps
    }

    info := transcoder.ProbeMediaInfo(media.Path)
    method := transcoder.DecidePlaybackMethod(info, caps, settings)

    switch method {
    case transcoder.DirectPlay:
        // Return the raw file URL — no session needed
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "method": "direct_play",
            "url":    fmt.Sprintf("/api/media/%d/stream", media.ID),
        })
        return

    case transcoder.DirectStream:
        // TODO: implement remux path (ffmpeg -c:v copy -c:a aac ...)
        // For now, fall through to transcode
        fallthrough

    case transcoder.Transcode:
        state, err := h.Sessions.Create(*media, settings, payload.AudioIndex, user.ID)
        if err != nil {
            http.Error(w, "internal error", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "method": "transcode",
            "session": state,
        })
    }
}
```

Your `/api/media/:id/stream` route already exists (`StreamMedia` in `playback_handlers.go`) — direct play just points there instead of starting HLS.

### Step 4: direct stream (remux) path

For the remux case, add a new session type that runs ffmpeg with `-c:v copy` instead of re-encoding:

```go
// In sessions.go or a new file
func buildRemuxPlan(itemPath, outDir string, audioIndex int) transcodePlan {
    args := []string{
        "-y",
        "-i", itemPath,
        "-map", "0:v:0",
    }
    if audioIndex >= 0 {
        args = append(args, "-map", fmt.Sprintf("0:%d", audioIndex))
    } else {
        args = append(args, "-map", "0:a:0?")
    }
    args = append(args,
        "-c:v", "copy",      // no video re-encode
        "-c:a", "aac",       // normalize audio
        "-b:a", "192k",
    )
    args = appendHLSOutputArgs(args, outDir)

    return transcodePlan{
        Args:         args,
        Mode:         "remux",
        EncodeFormat: "copy",
    }
}
```

This starts HLS output in seconds rather than waiting for the video encoder to spin up.

---

Here's a visual of the full decision flow:---

## Summary of what to do

**CPU-aware concurrency** — a one-liner change. Import `runtime`, call `runtime.NumCPU()` in `NewLibraryScanManager`, and derive semaphore sizes from it. Optionally expose them as `LibrarySettings` so users can override, the same way `Threads: 0` already works for ffmpeg.

**Transcode-only-if-necessary** — four changes across existing files:

1. Expand `probeVideoStream` to also return container, audio codec, and bitrate → rename it `ProbeMediaInfo`.
2. Add a `compatibility.go` with `DecidePlaybackMethod` that runs the three checks (bitrate cap → video codec → audio+container).
3. Update `CreateSession` to call `ProbeMediaInfo` + `DecidePlaybackMethod` before touching `Sessions.Create`. If `DirectPlay`, return the `/stream` URL directly without creating any session.
4. Add a `buildRemuxPlan` (ffmpeg `-c:v copy`) for the direct stream case — this is the fastest path for files that are in the right codec but the wrong container (very common with MKV/H.264).

The client should also send its capabilities in the `POST /api/playback/sessions/:id` body — at minimum `{"capabilities": {"h264": true, "mp4": true}}` from the web player, so the server doesn't have to guess.
