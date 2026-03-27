# Phase 1-3 Roadmap Audit

Updated against the current repo on 2026-03-26.

## Unchecked Item Audit

| Roadmap item | Audit | Evidence | Notes |
| --- | --- | --- | --- |
| background job retry system | implemented | `apps/server/internal/http/library_scan_jobs.go`, `apps/server/internal/http/library_automation.go`, `apps/server/internal/db/library_jobs.go` | Retry state is persisted, bounded, classified as retryable vs terminal, and recovered on restart for scan and identify flows. |
| filesystem watcher | implemented | `apps/server/internal/http/library_automation.go` | Native `fsnotify` watcher exists and recursively adds directories. |
| filesystem watcher options | partial | `apps/web/src/pages/Settings.tsx`, `apps/server/internal/http/library_automation.go` | Per-library enable/disable and `auto`/`poll` mode exist. Debounce is implemented as a fixed window in scan jobs, not user-configurable. |
| scan scheduling | implemented | `apps/web/src/pages/Settings.tsx`, `apps/server/internal/http/library_automation.go` | Per-library scheduled scan interval is stored and drives a scheduler. |
| canonical media identity model | partial | `apps/server/internal/db/db.go`, `apps/server/internal/db/metadata_storage.go` | Global media IDs plus `shows`/`seasons` and canonical metadata exist, but there is no explicit multi-version logical item model for movies or alternate physical versions. |
| cast lists | implemented | `apps/web/src/pages/MovieDetail.tsx`, `apps/web/src/pages/ShowDetail.tsx` | Both movie and show detail pages render cast metadata. |
| genres | implemented | `apps/web/src/pages/MovieDetail.tsx`, `apps/web/src/pages/ShowDetail.tsx` | Both movie and show detail pages render genre chips. |
| search index | implemented | `apps/server/internal/db/search_index.go`, `apps/server/internal/http/search_index_jobs.go` | Indexed library search is backed by dedicated search tables and refresh jobs. |
| title search | implemented | `apps/web/src/pages/Search.tsx`, `apps/server/internal/db/search_index.go` | Search UI and backend title matching are live. |
| actor search | implemented | `apps/web/src/pages/Search.tsx`, `apps/server/internal/db/search_index.go` | Actor/cast matching is indexed and surfaced in search results. |
| genre filtering | implemented | `apps/web/src/pages/Search.tsx`, `apps/server/internal/db/search_index.go` | Genre facets and filtering are wired through UI and backend query filters. |
| fuzzy search | implemented | `apps/server/internal/db/search_index.go` | Fuzzy fallback search is implemented server-side. |
| index refresh jobs | implemented | `apps/server/internal/http/search_index_jobs.go`, `apps/server/internal/http/library_scan_jobs.go` | Search index refresh is queued on startup and after completed scans. |

## Milestone Updates Applied

`milestones.md` was updated to check off the items confirmed as implemented:

- background job retry system
- filesystem watcher
- scan scheduling
- cast lists
- genres
- search index
- title search
- actor search
- genre filtering
- fuzzy search
- index refresh jobs

## Settings Coverage

| Feature area | Coverage | Notes |
| --- | --- | --- |
| library settings | already exposed | Per-library playback defaults, watcher enablement, watcher mode, and scheduled scan interval are in `apps/web/src/pages/Settings.tsx`. |
| filesystem watcher enable/mode | already exposed | Matches the implemented portion of the roadmap item. |
| scan scheduling | already exposed | Stored per library and editable in Settings. |
| watcher debounce configuration | missing UI/backend | Debounce exists as a fixed server constant, but there is no settings surface or persisted config. |
| metadata refresh policy | missing UI/API | Policy exists in `app_settings` via `apps/server/internal/db/metadata_storage.go`, but no settings endpoint or admin UI exposes it. |
| retry policy tuning/visibility | missing UI | Retry state is persisted in library job status, but there is no admin settings section for retry policy controls. |
| search and metadata display | does not belong in settings | These are browse/detail capabilities rather than configuration surfaces. |

## Right-Click Coverage

| Surface | Coverage | Notes |
| --- | --- | --- |
| show cards | already exposed | Right-click menu supports `Refresh metadata`, `Open details`, and `Identify…` in `apps/web/src/pages/Home.tsx`. |
| movie cards | now exposed | Right-click menu now supports `Open details`, plus `Retry identify` for failed/unmatched movies. |
| movie metadata refresh | missing backend/UI | There is no movie-specific refresh mutation or endpoint yet. |
| scan library / scan folder from card context | deferred intentionally | No safe per-card mapping exists for folder-targeted scans, so this should wait for explicit backend support. |

## Remaining True Gaps

### Priority 1

- Finish the canonical media identity model so logical titles and physical file versions are represented explicitly across movies and episodic content.
- Decide whether watcher debounce needs to become persisted per-library config or remain intentionally fixed; if it should be configurable, add storage, API, and Settings UI.

### Priority 2

- Add an admin Settings section for metadata refresh policy with API support around the existing `app_settings` storage.
- Decide whether retry policy belongs in Settings or in a dedicated job diagnostics surface; if exposed, prefer admin-only controls and current-status visibility.

### Priority 3

- Add movie metadata refresh support to reach full context-menu parity with show cards.
