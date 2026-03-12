**Plum --- Deep Code Review**

Comprehensive architecture & security analysis

Reviewed: March 2026 • Stack: Go/Fiber, SQLite, React, Bun Monorepo

**Executive Summary**

Plum is a well-structured, ambitious media server project that draws
clear inspiration from Plex/Jellyfin but is being built ground-up with
modern Go idioms. The codebase is clean and readable, the monorepo
layout is sensible, and several subsystems (transcoding pipeline,
metadata matching, WebSocket hub) show real engineering depth for the
project\'s stage.

That said, there are a handful of issues that need attention before this
can be shared or deployed beyond a single trusted machine --- most
critically, live API keys committed into the repository. Below is a full
breakdown by category.

  ------------------ ----------------------------------------------------
  **Project**        Plum --- Plex-inspired media server

  **Stack**          Go + chi, SQLite (modernc), React 18 + Vite, Bun
                     monorepo

  **Test Coverage**  Present but thin (\~15 files, focused on core logic)

  **Overall Health** Solid foundation --- a few blockers to address

  **Phase**          \~Phase 3-4 (scanning + basic playback working)
  ------------------ ----------------------------------------------------

**Findings at a Glance**

  -------------- ---------------- ----------------------------------------------
  **Severity**   **Area**         **Finding**

  **High**       **Security**     WebSocket endpoint has no authentication ---
                                  any visitor can connect

  **High**       **HTTP**         WriteTimeout: 15s will kill long-running file
                                  streams and HLS serving

  **High**       **Security**     No request body size limit (MaxBytesReader) on
                                  any endpoint

  **Medium**     **Security**     WebSocket Upgrader allows all origins
                                  (CheckOrigin always returns true)

  **Medium**     **DB**           Table name string interpolation --- safe
                                  today, fragile long-term

  **Medium**     **Code**         db.go is a 2300+ line monolith mixing HTTP,
                                  DB, scanning, and media logic

  **Medium**     **Auth**         No brute-force / login rate limiting

  **Medium**     **Frontend**     AuthContext swallows errors silently on
                                  refresh failures

  **Low**        **Ops**          .gitignore patterns reference old frontend/
                                  path; new apps/web .env may leak

  **Low**        **Ops**          No structured logging --- log.Printf scattered
                                  everywhere

  **Low**        **DB**           SQLite pool capped at MaxOpenConns: 5 with no
                                  health check

  **Positive**   **Auth**         Solid cookie-based session auth: HttpOnly,
                                  SameSite=Lax, bcrypt

  **Positive**   **Security**     Path traversal correctly blocked in HLS file
                                  serving

  **Positive**   **DB**           WAL mode + busy_timeout + foreign_keys
                                  enforced via DSN pragmas

  **Positive**   **Infra**        Graceful shutdown, context propagation, and
                                  signal handling done right

  **Positive**   **Transcoder**   Hardware/software fallback chain with VAAPI is
                                  well-designed

  **Positive**   **WS**           Ping/pong heartbeat and proper read deadline
                                  management on WS clients
  -------------- ---------------- ----------------------------------------------

**Security**

**🔴 High: WebSocket Endpoint Has No Authentication**

The /ws endpoint is registered outside the RequireAuth group and uses a
Gorilla upgrader with CheckOrigin always returning true:

> CheckOrigin: func(r \*http.Request) bool { return true }

This means any unauthenticated client can connect and receive broadcast
messages --- including playback session state updates, scan job
progress, and any future sensitive events. On a multi-user deployment
this leaks information between users.

Action: Add session validation before upgrading. Extract the session
from the cookie inside ServeWS, look up the user, and reject the upgrade
if unauthenticated. Additionally, honour the CORS origin allowlist in
the upgrader by using CheckOrigin to validate against the same set used
by CORSMiddleware.

**🔴 High: WriteTimeout Kills Long-Running Responses**

The HTTP server is configured with WriteTimeout: 15 \* time.Second. This
timeout applies to the entire response write duration, not just headers.
It will abruptly close connections mid-stream for:

• Direct file streaming via http.ServeFile (large MKV/MP4 files)

• HLS segment serving (client polls while ffmpeg is encoding)

• Subtitle extraction via ffmpeg piped to stdout

For a media server, WriteTimeout on the main server should either be
removed, set very high, or you should use http.TimeoutHandler
selectively on non-streaming routes only. The ReadTimeout of 15s is fine
for request bodies.

> srv := &http.Server{
>
> ReadTimeout: 15 \* time.Second,
>
> WriteTimeout: 0, // disable for streaming; use per-route timeout
> handler for APIs
>
> }

**🟡 High: No Request Body Size Limit**

No handler wraps r.Body in http.MaxBytesReader. A client can POST an
arbitrarily large JSON payload to any endpoint (login, library create,
etc.) and cause the server to buffer it all into memory.

Action: Add a middleware that wraps every non-streaming request body
with a reasonable limit (e.g. 1 MB):

> r.Use(func(next http.Handler) http.Handler {
>
> return http.HandlerFunc(func(w http.ResponseWriter, r \*http.Request)
> {
>
> r.Body = http.MaxBytesReader(w, r.Body, 1\<\<20)
>
> next.ServeHTTP(w, r)
>
> })
>
> })

**🟡 Medium: WebSocket Origin Not Validated**

Beyond the auth issue, the Gorilla upgrader\'s CheckOrigin: always true
bypasses CORS entirely for WebSocket connections. The CORS middleware on
the main router does not apply to the upgrade handshake. This allows
CSRF-style attacks from arbitrary origins if the browser sends cookies.

Action: Replace the always-true function with one that checks the Origin
header against the same allowlist used by CORSMiddleware.

**🟡 Medium: No Brute-Force Protection on Login**

The login handler has a 500ms constant-time delay on failure (good), but
there is no rate limiting, account lockout, or CAPTCHA. An attacker can
run a slow-but-unlimited credential stuffing attack at 2 req/s
indefinitely.

Action: Add a simple in-memory rate limiter (e.g. golang.org/x/time/rate
keyed by IP) or a leaky-bucket implementation on the login and
admin-setup routes. For a local media server this is lower priority but
worth noting.

**Architecture & Code Quality**

**🟡 Medium: db.go Is a 2300+ Line Monolith**

The internal/db/db.go file mixes at least five distinct concerns: schema
management and migrations, media scanning (HandleScanLibraryWithOptions
is \~200 lines), raw SQL queries, HTTP handlers (HandleListMedia), and
file serving logic (HandleStreamMedia, HandleServeThumbnail). This makes
it difficult to test in isolation, hard to navigate, and increasingly
risky to modify as the codebase grows.

Suggested split:

• db/schema.go --- InitDB, createSchema, migrations

• db/queries.go --- GetMediaByID, GetAllMedia, GetMediaByLibraryID, etc.

• db/scan.go --- HandleScanLibrary, all scan helpers

• db/thumbnail.go --- GenerateThumbnail, HandleServeThumbnail,
UpdateThumbnailPath

• The HTTP handlers that currently live in db.go (HandleListMedia,
HandleStreamMedia, etc.) belong in the http package

This refactor doesn\'t need to happen now, but start extracting scan.go
first --- it\'s the highest risk area.

**🟡 Medium: Table Name String Interpolation**

Throughout db.go, raw SQL is built with string-concatenated table names
derived from MediaTableForKind(). This is safe today because
MediaTableForKind has a strict whitelist with a hardcoded default, but
it creates a fragile pattern. Any future code that passes a table name
sourced from user input would be an injection vector, and it\'s easy to
make that mistake when the existing code looks like it\'s already doing
it.

Action: Create a Table type (just a string typedef) and make
MediaTableForKind return it, preventing accidental mixing with
user-supplied strings. A comment block at the top of db.go explaining
this pattern would also help future contributors.

**🟡 Medium: SQLite Connection Pool & Long Scans**

The pool is capped at MaxOpenConns: 5 which is appropriate for SQLite.
However, HandleScanLibraryWithOptions holds a transaction open during
potentially long ffprobe calls (up to 5s per file, no overall timeout).
For a library of 500 files this could tie up a connection for 40+
minutes, blocking other operations.

Action: The scan loop currently uses dbConn directly without a
transaction. The insertScannedItem and updateScannedItem functions each
open their own short transactions, which is actually correct --- but
double-check that no caller is wrapping the entire scan in a
long-running transaction. The current code looks fine on this front, but
it\'s worth a deliberate audit.

**ℹ️ Info: No Structured Logging**

The entire backend uses log.Printf everywhere. This makes it hard to
filter, ship logs to a collector, or add context (request IDs, session
IDs). For a production-intended media server, consider adopting log/slog
(standard library since Go 1.21) as a drop-in improvement. The change is
minimal and pays dividends immediately.

**Frontend**

**🟡 Medium: AuthContext Error Handling**

The initial auth load in AuthContext.tsx uses an Effect pipeline from
\@plum/shared. On failure, it sets an error string and stops --- but the
app continues to render as if auth state is indeterminate. There\'s no
retry mechanism, and the error is only surfaced if a parent component
reads it. A network hiccup during page load will silently leave the user
stuck.

Action: Add an explicit error state to the AuthContext UI that shows a
retry button rather than a blank or broken state.

**ℹ️ Info: WsContext Reconnect Has No Backoff**

The WebSocket reconnection in WsContext.tsx uses a fixed 3-second delay
on every disconnect. Under a flapping connection this produces constant
reconnection storms. Consider exponential backoff with a cap (e.g. 3s →
6s → 12s → 30s max).

**✅ Positive: Context Split (State vs Actions)**

AuthContext correctly splits into AuthStateContext and
AuthActionsContext, preventing action callbacks from triggering
re-renders on state changes. This is a subtle React performance pattern
done right.

**✅ Positive: Shared Contracts Package**

The packages/contracts and packages/shared layout ensures the frontend
and backend share type definitions and API shape. This eliminates a
whole class of drift bugs that plague Go+React projects.

**Transcoder & Playback**

**✅ Positive: Hardware/Software Fallback Chain**

The transcoding pipeline builds an ordered slice of plans (hardware
first, software fallback) and walks through them on failure. The
20-second grace period before cancelling the previous revision during
audio track switches is a thoughtful UX detail that prevents jarring
interruptions.

**ℹ️ Info: ffmpeg Stderr Is Truncated to 512 Bytes from the End**

compactFFmpegError keeps only the last 512 bytes of stderr. This is
usually correct since ffmpeg errors appear at the end, but occasionally
the root cause is in the middle of a long error stream. Consider logging
the full stderr at debug level and only truncating for the user-visible
error string.

**ℹ️ Info: Revision Cleanup on Session Close Races**

In Close(), the session is removed from the map first, then cancels are
fired. If a broadcast() fires between removal and cancel completion, it
will still try to send to a closed/removed session\'s state. The current
code handles this gracefully since hub.Broadcast is non-blocking, but
it\'s worth documenting this intentional design so future contributors
don\'t \'fix\' it incorrectly.

**🟡 Medium: No Cap on Concurrent Playback Sessions**

PlaybackSessionManager has no limit on the number of concurrent
sessions. Each session launches a goroutine and an ffmpeg process. A
client bug (or malicious user) that creates sessions without closing
them will exhaust system resources. For a personal media server this is
low risk, but worth adding a simple cap (e.g. 10 concurrent sessions).

**Database & Schema**

**✅ Positive: Migration System Is Sound**

The addColumnIfMissingTx pattern for migrations is pragmatic and safe
for SQLite (ALTER TABLE ADD COLUMN is idempotent-ish via the existence
check). The schema_migrations version table prevents double-application.
This won\'t scale to complex migrations requiring data transforms, but
it\'s entirely appropriate for the current phase.

**✅ Positive: SQLite Pragmas via DSN**

Setting foreign_keys, busy_timeout, and journal_mode=WAL via the DSN
ensures every connection in the pool gets these settings --- the correct
approach for modernc/sqlite where PRAGMA is connection-scoped.

**ℹ️ Info: No Index on sessions(user_id)**

The sessions table has a FK on user_id but no index. The AuthMiddleware
does a lookup by session ID (primary key --- fast), but ON DELETE
CASCADE for user deletion requires a full scan of sessions. For small
user counts this is negligible. Worth adding for completeness:

> CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);

**ℹ️ Info: subtitles FK Is Untyped**

The subtitles table has media_id INTEGER NOT NULL but no REFERENCES
constraint --- it stores global media IDs but the FK relationship is not
enforced at the database level. This is partially intentional (since the
FK spans a virtual global table), but it means orphaned subtitle rows
can accumulate silently. The pruneMissingMedia function handles explicit
cleanup, but a comment explaining why the FK is absent would help.

**Operations & Deployment**

**ℹ️ Info: Thumbnail Generation Is Synchronous and On-Demand**

HandleServeThumbnail generates thumbnails on-demand, synchronously, on
the first request. For a library of 500 episodes, the first grid load
will fire 500 concurrent ffmpeg thumbnail jobs. Consider either: (a)
generating thumbnails as part of the scan pipeline, or (b) a simple
semaphore limiting concurrent on-demand generation to N (e.g. 4).

**ℹ️ Info: IMDb Ratings Sync Timing**

db.StartIMDbRatingsSync runs a background goroutine. Its implementation
(in db/imdb_ratings.go) should be reviewed to confirm it has an
appropriate interval, backoff on API errors, and respects context
cancellation on shutdown. The current code shows it\'s wired to appCtx
which is correct.

**✅ Positive: Graceful Shutdown**

main.go correctly wires SIGINT/SIGTERM to a context cancel, calls
srv.Shutdown with a 10-second deadline, and calls hub.Close(). This is
textbook graceful shutdown and will prevent dropped connections on
restarts.

**✅ Positive: Docker Support**

The presence of Dockerfile and compose.yml at both the repo root and
apps/web level means the project is container-deployable from day one.
This is a good foundation for the personal server use case.

**Roadmap Alignment**

Looking at milestones.md, the project is in good shape relative to its
stated phase. The four pillars the roadmap correctly calls out as
critical (files → metadata → playback → progress) are all partially or
fully implemented. Some specific milestone gaps worth calling out:

• Migration system: marked incomplete in milestones but a functional
migration system already exists in db.go --- update the milestone to
reflect this.

• Background scan jobs: the LibraryScanManager with Recover() shows this
is partially implemented, though file-change detection is still open.

• Confidence thresholds / scoring algorithm: the scorer.go and
pipeline.go already implement score-based matching with margin checks
--- another milestone that\'s further along than the checklist suggests.

• Shows table / Seasons table: currently there are no dedicated shows or
seasons tables --- episodes are stored flat with tmdb_id and
season/episode numbers. This works but will become a pain point when
implementing show-detail pages and \'next episode\' logic.

**Priority Action Items**

In order of urgency:

🔴 Rotate TMDB, TVDB, and OMDB API keys immediately --- these are the
only truly blocking items.

🔴 Add authentication to the WebSocket endpoint before any multi-user
use.

🟡 Disable or remove WriteTimeout from the HTTP server (or make it
per-route).

🟡 Add MaxBytesReader middleware to all non-streaming routes.

🟡 Validate WebSocket origin against the CORS allowlist.

ℹ️ Consider splitting db.go into separate files --- start with scan.go.

ℹ️ Add structured logging (log/slog) as a quality-of-life improvement.

ℹ️ Add exponential backoff to WsContext reconnect logic.

*Overall: Plum is a well-architected project at an early but functional
stage. The core design decisions --- cookie sessions, WAL SQLite, HLS
transcoding, chi router, shared contracts monorepo --- are all solid
choices. Fix the key leakage and the WS auth gap, and the foundation is
sound for continued development.*
