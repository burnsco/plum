# Cross-platform alignment

This document captures **Milestone E** outcomes: where logic lives, how the web, Android TV, and server stay aligned, and known duplication.

## Ownership rules

| Layer | Owns |
|--------|------|
| **Server (`apps/server`)** | Domain logic, persistence, TMDB/metadata integration, authorization, and API behavior. Single source of truth for home dashboard composition, discover browse results, playback sessions, and library rules. |
| **Contracts (`packages/contracts`)** | Wire types and Effect schemas: request/response shapes, WebSocket enums, and **canonical string unions** (e.g. `DiscoverBrowseCategory`). Runtime constants that define those unions (e.g. `DISCOVER_BROWSE_CATEGORY_ORDER`) live here so clients and server refer to the same identifiers. |
| **Shared (`packages/shared`)** | Cross-client runtime utilities: API client, URL helpers, media URL resolution, and **pure helpers** that must stay bit-for-bit aligned with the server (e.g. `normalizeDiscoverOriginKey`). |
| **Clients (`apps/web`, `apps/android`)** | Presentation, routing, platform UX, and local state. They call the API and render contracts-typed payloads; they **do not** reimplement server-side aggregation or ranking. |

When adding a feature that touches more than one client, prefer: **server behavior + contracts types + thin client UI**. If the same non-trivial validation must exist in multiple languages (e.g. origin country codes), keep one implementation per language and **cross-link** the others in comments; add tests on the Go side that define the contract.

### Package boundaries (`contracts` vs `shared`)

Use this checklist so the TypeScript packages stay small and purposeful (Milestone F audit).

| Put it in… | …when it is |
|------------|-------------|
| **`@plum/contracts`** | A request/response field shape, WebSocket payload shape, or Effect `Schema` that multiple packages must agree on. Canonical string unions and the constants that define ordering (e.g. `DISCOVER_BROWSE_CATEGORY_ORDER`). Anything you would serialize to JSON for the wire. |
| **`@plum/shared`** | The typed HTTP/WebSocket **client** (`createPlumApiClient`, parsers), URL builders (`mediaStreamUrl`, …), media URL resolution, and **pure helpers** that mirror server rules without being part of the wire schema (e.g. `normalizeDiscoverOriginKey`). |
| **Neither** | React components, routes, or app state (stay in `apps/web`). Kotlin UI (stay in `apps/android`). Domain logic that should run only on the server (stay in `apps/server`). |

**Do not:**

- Add Effect `Schema` definitions or large API surface types to `shared` — re-export types from `contracts` and keep `shared` for client/runtime code (`api.ts` imports types from `contracts`).
- Add `fetch`, environment assumptions, or UI code to `contracts` — it stays data-only (plus generated JSON Schema under `packages/contracts/generated/`).
- Duplicate discover/browse constants in the web app when they already exist in `contracts` — import from `@plum/contracts` or `@plum/shared` as appropriate.

Current layout: `packages/contracts/src/index.ts` holds consolidated schemas; `packages/shared/src/` is limited to `api.ts`, `backend.ts`, `media.ts`, `discover.ts`, and `index.ts` re-exports.

## Audit: duplicated or parallel logic

### Centralized in this milestone

1. **Discover TMDB origin filter (`origin_country`)** — ISO 3166-1 alpha-2, two ASCII letters. Implemented in:
   - TypeScript: `normalizeDiscoverOriginKey` in `packages/shared/src/discover.ts` (also used by `createPlumApiClient` discover routes).
   - Go: `parseDiscoverOriginCountry` in `apps/server/internal/http/library_discover_search_handlers.go` and `normalizeDiscoverOrigin` in `apps/server/internal/metadata/tmdb_discover.go`.
   - Kotlin: `DiscoverOrigin.normalizeKey` in `apps/android/core-network/.../DiscoverOrigin.kt` (unit tests in `DiscoverOriginTest.kt`).
2. **Discover browse category ordering** — `DISCOVER_BROWSE_CATEGORY_ORDER` in `packages/contracts/src/index.ts`; the web app builds `DISCOVER_CATEGORY_OPTIONS` from this list so category IDs stay aligned with the server and contracts literals.

### Intentional parallel layers (not duplicated domain logic)

- **TMDB image URLs** — `resolvePosterUrl` / `resolveBackdropUrl` in `packages/shared` (web); Android uses the same path patterns via server-provided URLs or equivalent helpers where needed.
- **Home dashboard** — Server aggregates `HomeDashboard` (`GetHomeDashboard`); clients only display rails and links.
- **Playback session** — Server owns session state; clients send commands and render streams.

### Remaining duplication to watch

- **Discover UI labels** (e.g. “Trending Now”) — English copy in web; Android may use different strings or resources; category **IDs** are shared via contracts.
- **Routing paths** — Web (`/discover/...`) and Android `NavHost` routes are defined per platform; keep behavior aligned by sharing API contracts, not by copying route strings.
- **WebSocket command/event parsing** — Already centralized in `packages/shared` (`parsePlumWebSocketCommand`, etc.).

## Related files

- `packages/shared/src/discover.ts` — origin normalization.
- `packages/contracts/src/index.ts` — `DiscoverBrowseCategory`, `DISCOVER_BROWSE_CATEGORY_ORDER`.
- `apps/web/src/lib/discover.ts` — Discover UX helpers (re-exports `normalizeDiscoverOriginKey` from shared for convenience).
