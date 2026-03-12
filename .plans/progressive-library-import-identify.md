# Progressive Library Import + Identify Plan

## Summary

Change Plum’s library pipeline so import, identify, and enrichment overlap in small bounded batches instead of running as one full-library step at a time.

The user goal is:

- after first login, imported media should appear almost immediately
- TV, movie, and anime libraries should start identifying before the full library scan finishes
- the UI must not falsely show "couldn't match automatically" while auto-identify is still pending
- multi-core machines should be used, but with bounded concurrency that stays safe for SQLite and provider rate limits

## What Is Broken Today

### 1. Automatic identify is effectively browser-owned during onboarding

`apps/web/src/pages/Onboarding.tsx` starts library scans with `identify: false`.

That means first-login auto-identify only happens later from the web app’s client queue, not from the server job system. If the user leaves onboarding early or the browser gets ahead of scan progress, identify behavior becomes inconsistent.

### 2. Scan and identify are serialized at the wrong level

`apps/server/internal/http/library_scan_jobs.go` currently does this:

- one active library scan globally via `activeScanID`
- one global identify slot via `identifySem`
- one global enrichment slot via `enrichSem`
- identify only starts after `HandleScanLibraryWithOptions(...)` finishes the full library

So a large TV library gets completely imported first, and only then starts matching metadata.

### 3. The backend already writes partial rows, but the pipeline does not exploit it

`apps/server/internal/db/db.go` inserts and updates rows during the scan loop, so partial media is already visible before scan completion.

That is good news: we do not need to invent partial visibility from scratch. We mainly need to:

- start identify earlier
- keep the work flowing in batches
- stop the UI from interpreting "not finished yet" as "failed"

### 4. The current UI can show false failure states

Right now the web app mixes:

- library-level identify phase
- item-level `match_status`
- client-managed identify retries

This causes a bad state where TV or movie cards can show as not auto-identified even though the library still has pending work or future batches that have not been attempted yet.

In practice, `local` is currently overloaded:

- sometimes it means "freshly imported and waiting for auto-identify"
- sometimes it means "not matched yet"
- sometimes the UI treats it like a terminal failure once the library-level phase flips

That is the false-negative behavior we need to remove.

## Target UX

When a user adds libraries during onboarding or first login:

- the first library should begin showing imported items within seconds
- identified posters/titles should start appearing while the scan is still running
- if the user opens TV first, they should see shows populate progressively instead of waiting for movies/music/anime to finish
- incomplete cards should read as "Searching..." or similar while auto-identify is still pending
- only rows that have actually exhausted automatic matching should show manual-identify or retry actions

## Proposed Architecture

## 1. Make auto-identify server-owned

Use the server scan job pipeline as the source of truth for automatic identify.

Changes:

- onboarding should start scans with `identify: true`
- automatic identify should no longer depend on `IdentifyQueueContext` kicking off background `/identify` requests from the browser
- the browser can still trigger manual retry/identify actions, but routine auto-identify should be owned by `LibraryScanManager`

Why:

- the server keeps working if the user navigates away
- persisted job state survives reconnects better
- the UI stops guessing when identify should run

## 2. Turn scan into a progressive producer instead of a full-library barrier

Keep the current filesystem walk, but split work into batch-sized units.

Design:

- scan discovers files continuously
- import commits rows in small batches
- each committed batch immediately feeds identify work for just those rows
- prune-missing still runs once at the end of the full walk

Recommended first-pass batch shape:

- 25-50 imported items per batch, or
- flush every 250-500ms, whichever comes first

This gives fast first results without creating SQLite lock storms.

## 3. Use bounded pipeline concurrency instead of single global serialization

Use separate worker pools for three stages:

- scan/import stage
- identify stage
- enrichment stage

Recommended initial limits:

- active scan libraries: `2`
- active identify batches across libraries: `3-4`
- active enrichment batches across libraries: `1-2`

Important constraint:

- do not fan out uncontrolled DB writers
- prefer one batch committer per active library, or another similarly bounded write path
- keep TMDB/TVDB requests behind a shared rate limiter

This uses multiple cores while staying reliable with SQLite WAL.

## 4. Feed identify from committed scan batches

Add a scan callback or batch event so `HandleScanLibraryWithOptions` can tell `LibraryScanManager`:

- which rows were inserted or refreshed
- which of those rows are identify-eligible
- how many items are now pending identify for that library

This should replace the current "scan everything, then list the whole library and identify it" behavior.

Good fits here:

- extend `db.ScanOptions` with a committed-batch callback
- return lightweight row refs/global ids for the batch
- enqueue identify work immediately after the batch is durable

## 5. Make identify incremental and resumable

Instead of one monolithic `identifyLibrary(libraryID)` pass, add an internal batch-oriented path:

- identify a specific batch of refs
- update per-library identify counters as batches complete
- keep running new batches while the scan is still producing more work
- finish the library identify phase only when scan discovery is done and the identify backlog reaches zero

The existing per-item identify worker pool in `apps/server/internal/http/library_handlers.go` can still be reused internally, but it should operate on targeted batches rather than a full-library snapshot.

## 6. Move enrichment behind identify and give it lower priority

Enrichment is important, but it should not delay early browseability.

Priority should be:

1. import rows
2. identify metadata
3. enrichment work like probes, embedded subtitles, sidecars, thumbnails

That means:

- no enrichment slot should block early identify for visible libraries
- enrichment should consume only leftover capacity
- first poster/title wins over first subtitle stream

## State Model Changes

## 1. Stop treating "local" as a failure proxy

We need a clean distinction between:

- pending auto-identify
- actively identifying
- terminal automatic failure
- successfully identified

The lowest-risk way to do that is to extend library job state first, then optionally add item-level workflow state if needed.

### Library job additions

Extend `LibraryScanStatus` with counters like:

- `identifyPending`
- `identifyInFlight`
- `identifyCompletedBatches`

These counters let the UI know whether matching is still underway even if some rows are still `local`.

### UI rule

The UI should only show a terminal auto-identify failure when all of these are true:

- library scan is no longer producing new identify work for that library
- `identifyPending == 0`
- `identifyInFlight == 0`
- the row/group is still incomplete after the automatic pass

Until then, the card should remain in a non-terminal state like `Searching...`.

### Optional follow-up if needed

If library-level counters are not enough, add a dedicated per-item workflow field such as `identify_status` later. Do not overload `match_status` further unless we are willing to update all contracts and grouping logic in one pass.

## Backend Changes

## 1. `apps/server/internal/http/library_scan_jobs.go`

Refactor `LibraryScanManager` into a progressive coordinator:

- replace `activeScanID` FIFO serialization with a bounded active-scan pool
- replace full-library `startIdentify` behavior with per-library identify backlogs
- keep persisted scan status as the source of truth
- advance `identifyPhase` based on backlog state, not only on one blocking library-wide call
- preserve recovery on restart by rebuilding pending scan/identify/enrichment work from persisted job status

## 2. `apps/server/internal/db/db.go`

Extend scan support for progressive commits:

- batch imported rows instead of treating the whole library as one identify boundary
- emit committed-batch notifications
- keep prune-missing at the end of the full run
- preserve current "rows visible before completion" behavior

## 3. `apps/server/internal/http/library_handlers.go`

Split automatic identify internals into reusable batch APIs:

- keep the existing matching heuristics
- keep retry and anime fallback behavior
- add an internal "identify refs/batch" entrypoint used by scan jobs
- reserve the existing `/api/libraries/:id/identify` route for manual full-library retry if we still want it

## 4. Contracts

Update `packages/contracts/src/index.ts` and consumers with any new job counters added to `LibraryScanStatus`.

Prefer additive fields so older code paths stay straightforward.

## Web/UI Changes

## 1. `apps/web/src/pages/Onboarding.tsx`

Start onboarding scans with server-owned auto-identify enabled once the new backend pipeline exists.

That means removing the explicit `identify: false` behavior for the first-login flow.

## 2. `apps/web/src/contexts/IdentifyQueueContext.tsx`

Reduce this context to one of these roles:

- manual retry/manual identify only, or
- UI-only soft state layered on top of backend progress

Do not keep it as the primary automatic identify orchestrator once the server owns the pipeline.

## 3. `apps/web/src/pages/Home.tsx`

Change the card-state rules so incomplete items are not shown as failures while the backend still has identify work pending.

Desired behavior:

- pending/in-flight auto-identify => `Searching...`
- terminal auto-identify exhausted => `Couldn't match automatically`
- explicit unmatched after final pass => manual identify/retry action

## 4. `apps/web/src/contexts/ScanQueueContext.tsx`

Use websocket-driven scan updates as the fast path, with polling as fallback.

The backend already broadcasts `library_scan_update`, so wiring that into the scan status context should make new imports and identify progress appear faster than the current 2-second poll loop.

## Rollout Phases

## Phase 1: Truthful status and server ownership

- make onboarding scans request identify from the server
- stop browser auto-identify from being the primary path
- add `identifyPending` and `identifyInFlight` to scan status
- update Home card-state logic so pending work never appears as terminal failure

This phase fixes the false "not identified automatically" messaging even before full progressive batching lands.

## Phase 2: Progressive scan -> identify pipeline

- batch scan commits
- enqueue identify from those batches immediately
- allow identify to run while the same library is still scanning
- keep enrichment lower priority

This phase delivers the main UX win: early posters and titles during initial import.

## Phase 3: Concurrency tuning and polish

- allow 2 active scan libraries
- tune identify/enrichment worker counts
- optionally prioritize the currently viewed library’s identify backlog without starving others
- shift scan status updates to websocket-first delivery

## Success Criteria

- first imported rows appear in the selected library within a few seconds of scan start
- identified posters/titles start appearing before the full library scan completes
- TV/movie cards no longer show terminal failure while auto-identify backlog still exists
- auto-identify continues even if the user leaves onboarding or refreshes the page
- multi-library onboarding no longer forces one huge full-library barrier before users see meaningful results

## Test Plan

- Add Go tests for progressive scan batches becoming visible before full scan completion.
- Add Go tests proving identify work can start before the library scan fully completes.
- Add Go tests that library identify does not transition to terminal completion/failure while pending batches still exist.
- Add recovery tests showing pending progressive identify resumes after process restart.
- Add web tests for Home card state:
  - incomplete rows + pending backlog => `Searching...`
  - incomplete rows + zero backlog + terminal phase => failure/retry UI
  - no false failure during onboarding-first-login scan
- Add onboarding tests verifying scans are started with server-owned identify enabled.

Before completion, run:

- `go test ./...` in `apps/server`
- `bun lint`
- `bun typecheck`

## Assumptions

- SQLite remains the primary database, so concurrency must stay intentionally bounded.
- Music keeps its current non-remote-identify behavior.
- Provider matching logic should remain functionally the same in this pass; the main change is scheduling and state truthfulness.
- We are not changing the media data model more than necessary in phase 1 unless the library-level counters prove insufficient.
