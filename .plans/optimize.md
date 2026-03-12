# Implement Import/Identification Performance Pass

## Summary

Implement the highest-confidence internal performance wins in the library scan and identify pipeline without changing user-facing behavior or scan progress semantics. The work will target repeated filesystem work, repeated provider calls, and repeated probe subprocesses while preserving current scan results, identify behavior, and API shapes.

## Key Changes

- Consolidate video enrichment probing into a single ffprobe path in `apps/server/internal/db/db.go`.
  - Replace the current three-call pattern for duration, embedded subtitles, and embedded audio with one shared probe result per file.
  - Keep existing stored data shape the same: duration, `embedded_subtitles`, and `embedded_audio_tracks` remain unchanged externally.
  - Preserve current timeouts and best-effort behavior: failed enrichment should still not fail the scan.

- Cache per-directory sidecar subtitle discovery during a scan run in `apps/server/internal/db/db.go`.
  - Build a scan-local cache keyed by directory path so each directory is read once, not once per video.
  - Reuse cached directory entries when matching `base.*.(srt|vtt|ass|ssa)` sidecars.
  - Keep subtitle row semantics unchanged.

- Cache show-level NFO parsing during scan and identify flows.
  - Add a lightweight per-run cache keyed by show root path in the scan path and in the identify path.
  - Avoid repeated `tvshow.nfo` reads/parses for every episode in the same show.
  - Do not change NFO precedence or matching logic; only memoize existing behavior.

- Remove redundant provider searches in `apps/server/internal/metadata/pipeline.go`.
  - Refactor `identifySeries` so a failed TMDB-only pass does not immediately re-run the same TMDB search when broadening provider coverage.
  - Keep provider priority and scoring behavior intact; only dedupe equivalent search work.

- Tighten provider request cancellation behavior where the implementation already intends per-item timeouts.
  - Update TMDB detail/episode lookups to honor the passed context instead of uncancelable `http.Get` calls.
  - Keep response parsing and caches unchanged.

## Public APIs / Interfaces

- No HTTP API, contract, database schema, or websocket payload changes.
- Internal helper signatures may expand to accept scan-local caches or shared probe results.
- Existing scan and identify entrypoints remain the same.

## Test Plan

- Run targeted Go tests around scan/idempotence/identify behavior in `apps/server`.
  - Verify scan results are unchanged for movies, TV, anime, and music.
  - Verify identified metadata output is unchanged for explicit-ID, scored-match, unmatched, and anime fallback cases.
  - Verify repeated episodes in the same show still pick up the same NFO-derived IDs.
  - Verify sidecar subtitle discovery still finds the same subtitle files after directory caching.
  - Verify embedded subtitle/audio rows and duration still populate correctly from the unified probe path.
  - Verify provider timeout/cancellation behavior still returns nil/failure cleanly instead of hanging workers.

- Add focused regression tests for the new internal behavior where practical.
  - A test proving TMDB search is not duplicated in the fallback path.
  - A test or stub-backed assertion that the shared probe helper is called once per enriched video item.
  - A test or stub-backed assertion that show NFO parsing is reused across multiple episodes from one show.

- Before completion, run:
  - `go test ./...` in `apps/server`
  - `bun lint`
  - `bun typecheck`

## Assumptions

- We are not changing the pre-scan estimate walk in this pass; `EstimatedItems` behavior stays exactly as it is.
- We are not batching DB writes or changing transaction visibility semantics during scan.
- Best-effort enrichment remains non-fatal: probe/subtitle/audio failures should log/skip rather than fail the library run.
