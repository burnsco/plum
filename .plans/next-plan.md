# Implement Persistent Identify Cache Pass

## Summary

Build the next optimization milestone around **persistent match caching and request deduplication** for library scan and identify flows. The goal is to cut repeated TMDB/TVDB/OMDb work for the same movie or show without changing match behavior, scan progress semantics, or public HTTP/contracts. This stays aligned with `more-optimize-tips.md` while keeping the scope much smaller and safer than a full local IMDb/Wikidata title index.

## Key Changes

- Add persistent cache tables in `apps/server/internal/db/db.go` for provider-backed identification:
  - `identify_movie_cache`: key on normalized title + year + explicit IDs; store match status, resolved provider IDs, and serialized match payload.
  - `identify_series_cache`: key on normalized show title + year + explicit IDs; store resolved series identity and series-level metadata.
  - `identify_episode_cache`: key on provider + external series ID + season + episode; store serialized episode match payload.
  - Cache both `identified` and `unmatched` results.
  - Default TTLs: `identified` entries expire after 30 days; `unmatched` entries expire after 24 hours.

- Insert a cache-aware identify layer ahead of remote provider calls in `apps/server/internal/metadata/pipeline.go`.
  - Movies: check `identify_movie_cache` before `SearchMovie`; write back the final resolved movie match or unmatched result.
  - TV/anime: check `identify_series_cache` before `SearchTV`; once a series is resolved, check `identify_episode_cache` before `GetEpisode`.
  - Explicit `TMDBID`/`TVDBID` inputs still take precedence over title search, but successful lookups should populate the matching cache rows.

- Deduplicate work within a single identify/scan run in `apps/server/internal/http/library_handlers.go`.
  - Group episodic rows by a stable show cache key derived from parsed title/year/NFO/explicit IDs.
  - Resolve a show once per batch, then fan out episode lookups using the resolved provider series ID.
  - Coalesce concurrent requests for the same movie/show/episode so workers do not race the same provider call.

- Keep current identification behavior intact.
  - Preserve filename/path parsing, NFO precedence, scoring thresholds, retry behavior, anime fallback grouping, and provider priority.
  - Do not change scan result counters, websocket payloads, or existing category table schemas.

## Public APIs / Interfaces

- No HTTP API, websocket contract, or frontend API changes.
- Add internal DB schema migrations for the three cache tables above.
- Internal identify helpers should accept a cache store dependency and a clock function so TTL behavior is testable.

## Test Plan

- Add regression tests proving repeated episodes from the same show perform one series search per batch and reuse cached episode details.
- Add tests proving a second identify/scan pass with unchanged inputs reuses persistent cache rows and avoids provider search calls.
- Verify explicit `tmdbid-*` and `tvdbid-*` filenames still bypass title search and seed the cache.
- Verify unmatched results are cached and retried only after TTL expiry.
- Verify anime fallback behavior still works when the primary identify pass consults caches first.
- Before implementation is considered complete, run:
  - `go test ./...` in `apps/server`
  - `bun lint`
  - `bun typecheck`

## Assumptions

- This pass is intentionally the bridge between today’s remote-first pipeline and the larger local-title-index architecture from `more-optimize-tips.md`.
- Cache keys use existing normalized parse output; no new GuessIt/IMDb/Wikidata dependency is introduced yet.
- Cache misses and cache read/write failures remain best-effort and must never fail a scan or identify job.
