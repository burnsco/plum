
## What looks good

You clearly have the right product instincts.

* The monorepo split is sensible: `apps/server`, `apps/web`, `apps/desktop`, `packages/shared`, `packages/contracts`.
* The backend shape is good for this kind of product: Go + SQLite + ffmpeg + websocket hub is a practical stack.
* You already built useful user-facing features instead of getting stuck in scaffolding.
* The contract-sharing approach between frontend and backend is a smart move.
* Playback is not a fake MVP. You actually accounted for sessions, revisions, HLS, embedded subtitles, external subtitles, audio track switching, and progress updates.
* The metadata pipeline is ambitious in a good way.

This has the bones of something real.

## Critical issues I would fix first

### 2. Your websocket origin policy is effectively open

In `apps/server/internal/ws/client.go`, `CheckOrigin` returns `true`.

That means any origin can attempt a websocket connection. On top of that, `/ws` is not protected by `RequireAuth`, so even though `AuthMiddleware` runs globally, websocket access itself is not actually enforced as authenticated.

That is too loose for a media app.

What to do:

* require auth for websocket connections
* validate the origin against the same allowlist concept you use for CORS
* reject unauthenticated socket upgrades

Right now, that endpoint is a soft target.

### 3. Production hardening depends too much on env discipline

Session cookies only become `Secure` if `PLUM_SECURE_COOKIES` is explicitly enabled in `middleware.go`.

That is okay for local dev. It is not okay to leave as a footgun for production.

What to do:

* default to secure cookies when running behind HTTPS
* make insecure cookies an explicit dev-only mode
* document it loudly

### 4. Build reproducibility is shaky

Your Go module is pinned to `go 1.26.0`, and the Dockerfile also targets `golang:1.26.0-alpine3.23`.

In this environment, `go test` immediately tried to fetch that toolchain and failed. Separate from the network restriction here, this tells me your build is more fragile than it should be.

You also depend on an Effect package via a PR URL in multiple packages:

* `effect@8881a9b` from `pkg.pr.new`

That is not a stable dependency story.

What to do:

* pin to a mainstream released Go version unless you truly need 1.26
* avoid PR-hosted dependencies in the main app path
* make “fresh clone builds locally” a non-negotiable requirement

## Architectural problems starting to form

### 1. Some files are already too big

A few files stand out immediately:

* `apps/web/src/components/PlaybackDock.tsx` ~68 KB
* `apps/server/internal/db/db.go` ~74 KB
* `apps/server/internal/http/library_handlers.go` ~26 KB
* `apps/web/src/contexts/PlayerContext.tsx` ~19 KB
* `apps/web/src/pages/Settings.tsx` ~27 KB

That is the classic point where velocity starts feeling good for a week, then gets worse every month.

The danger is not file size by itself. It is mixed responsibility.

For example:

* `PlaybackDock.tsx` appears to contain player UI, track logic, subtitle parsing, buffering logic, HLS handling, progress updates, preferences, and interaction state all together.
* `db.go` is doing too many unrelated jobs.
* `library_handlers.go` is becoming a mini-application by itself.

What to do:

* split by responsibility, not just by file size
* move pure media/player logic into hooks or service modules
* split DB code by domain: libraries, media, subtitles, playback, thumbnails, settings, auth
* split HTTP handlers by resource area

### 2. The frontend styling approach is mixed and drifting

You have Tailwind v4 imports in `index.css`, shadcn-style UI components, and also a very large hand-written `App.css`.

That tells me the styling system is not settled yet. I also saw duplicated theme variables between `index.css` and `App.css`.

That creates long-term pain:

* duplicated tokens
* inconsistent component styling
* harder dark theme maintenance
* harder responsive cleanup

What to do:
pick one direction and commit to it.

Best option here:

* keep the theme tokens
* use Tailwind + component primitives for most layout/state styling
* keep only a small amount of custom CSS for media-player-specific visuals

Right now it feels mid-migration.

### 3. The desktop app is not real yet, but it is already part of the workspace burden

`apps/desktop` is basically a shell right now. `src/main.ts` and `src/preload.ts` are empty exports, but the root workspace already treats desktop as a first-class target in scripts.

That adds maintenance surface without delivering product value yet.

What to do:

* either make desktop a serious milestone
* or cut it from the root scripts until web/server are tighter

Do not carry dead weight this early.

## Dev workflow and repo hygiene issues

### 1. Root scripts do not represent the real repo health

Your root scripts are incomplete.

`build` only builds the web app.
`typecheck` ignores the server.
`lint` ignores the server.
There is no root-level “prove the repo is healthy” command.

For a monorepo, that is a problem.

What to do:
add one command that validates the whole repo:

* web typecheck/lint/test/build
* server test/build
* packages typecheck/lint
* optional desktop only if enabled

### 2. Docker/dev config is still personal-machine flavored

In `compose.yml`, host media paths are hardcoded to your local home directory layout.

That is fine for your machine. It is not fine as a reusable dev story.

What to do:

* move host media paths to env vars
* provide a generic compose example
* document the required mounts cleanly

### 3. Server env naming is inconsistent

I noticed both `PLUM_DATABASE_URL` and `PLUM_DB_PATH` patterns in different places.

That kind of config drift becomes annoying fast.

Pick one naming scheme and remove the other.

## Product and UX observations from the code

The app has real potential, but I can also see where users will feel roughness.

### Good signs

* onboarding flow exists
* continue watching exists
* library-specific playback preferences exist
* transcoding settings exist
* show detail exists
* thumbnails exist

That is the right stuff.

### Weak spots I would expect in actual use

* likely too much logic tied directly to UI components
* limited global error-handling strategy
* no clear route-level fallback / error boundary story
* the app seems optimized for successful flows more than degraded flows
* websocket/auth edge cases probably need tightening
* a lot of CSS suggests responsive polish may still be inconsistent in practice

I would expect the app to feel promising but uneven, especially around playback edge cases, settings interactions, and state recovery after failures.

## Backend-specific thoughts

Your backend direction is good. The main risks are maintainability and security posture.

Things I like:

* session-based playback revision model
* transcoding settings as explicit domain config
* thumbnail generation on demand
* metadata pipeline separation from handlers
* SQLite is a reasonable choice for this stage

Things I would tighten:

* websocket auth/origin handling
* clearer separation between DB access, domain logic, and HTTP handlers
* more explicit background job ownership and lifecycle
* rate limiting or guardrails around expensive ffmpeg paths
* structured logging instead of mostly plain `log.Printf`

## Frontend-specific thoughts

The frontend already does a lot, which is good, but it is taking on too much responsibility in a few places.

The biggest issue is that the player looks like the gravitational center of the app. That is normal for a media server product, but you need to isolate it before it takes over everything.

I would restructure the player layer roughly like this:

* `usePlaybackSession`
* `useHlsPlayback`
* `useSubtitleTracks`
* `useAudioTracks`
* `usePlaybackProgress`
* `PlaybackDockView`

That kind of split would make the code easier to test and much easier to trust.

## My blunt verdict

**You are past the “can I build this?” stage.**
Now the question is whether you will turn it into a durable product or let it become a clever prototype that gets harder to change every week.

Right now I would score it like this:

* **Product direction:** strong
* **Feature ambition:** strong
* **Architecture maturity:** medium
* **Security posture:** weak
* **Maintainability trend:** starting to slip
* **Release readiness:** not yet

## Priority order I would use

1. Rotate secrets and clean up env handling.
2. Lock down websocket auth and origin checks.
3. Fix build reproducibility and dependency stability.
4. Split the giant player and DB files by domain.
5. Unify styling strategy.
6. Create one real root validation command for the whole repo.
7. Either commit to desktop or remove it from the main maintenance path for now.

## Best single compliment I can give it

This app already feels like it was designed by someone who understands the actual media-server problem space, not just someone copying screens.

## Best single criticism I can give it

You are accumulating complexity faster than you are putting guardrails around it.

If you want, I can turn this into a **very detailed action plan with concrete refactors by file and module**, starting with the highest-impact cleanup pass.
