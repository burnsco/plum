# DB package domain boundaries

This document maps `internal/db` to maintenance domains. Shared row/value types and scan options live in `db.go`; logic is split across focused files.

## Core files

| File | Responsibility |
|------|----------------|
| `db.go` | `SkipFFprobeInScan`, extension maps, regexes for show keys, `nullStr` / `nullFloat64`, `readVideoMetadata` / `computeMediaHash` wiring |
| `types.go` | Shared structs (`MediaItem`, `Library`, `ScanOptions`, `User`, …) and match/library constants |
| `schema.go` | `InitDB`, `createSchema`, migration registry, `RetryOnBusy`, `IsSQLiteBusy`, column helpers |
| `library_queries.go` | Cross-library media listing, duplicates, `MediaTableForKind` |
| `library_identification.go` | Identification queues and episode identify rows |
| `library_scan.go` | Filesystem scan, discovery, enrichment, hashing, insert/update/prune |
| `library_jobs.go`, `library_missing_paths.go` | Library jobs and missing-path repair |
| `metadata_media_updates.go` | `UpdateMediaMetadata*` and episodic show-key helpers, `ListShowEpisodeRefs` |
| `media_read_probe.go` | `GetMediaByID`, paging, subtitles/embedded batches, ffprobe / sidecar scan |
| `playback.go` | Continue watching, progress, home rails |
| `sessions.go` | Expired HTTP `sessions` row cleanup |
| `search_index.go` | Library search index |
| `settings.go`, `intro_*.go`, `media_files.go`, … | As before (settings, intro detection, `media_files`, artwork, etc.) |

## HTTP layer (`internal/http`)

| File | Responsibility |
|------|----------------|
| `library_handlers.go` | `LibraryHandler` struct, library CRUD, scan HTTP entrypoints, shared DTO/helpers |
| `library_identify_handlers.go` | Bulk identify / provider matching |
| `library_collections_handlers.go` | Series details, library media page, home dashboard |
| `library_discover_search_handlers.go` | Discover rails, browse/search, TMDB search, library search |
| `library_items_handlers.go` | Movie/show details, identify movie, show episodes JSON |
| `library_playback_refresh_handlers.go` | Playback track refresh, intro-only, chromaprint, show refresh/confirm, intro status APIs |
| `metadata_artwork_handlers.go` | Artwork/posters (existing) |
| `respond.go` | `writeJSON`, `writeJSONError` |
| `request_parse.go` | `parsePathInt` (playback-style IDs), `chiURLIntParam`, `chiURLIntParamInvalidID` |
| `json_decode.go` | Strict JSON body decode |

## Milestone C status

The former monolithic `db.go` scan/metadata/media paths and the oversized `library_handlers.go` are split as above. Further cleanups (optional): `types.go` for structs only, or migrating more handlers to `writeJSON` / `chiURLIntParam`.
