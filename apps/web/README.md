# Plum Web Client

React + Vite + TypeScript SPA for browsing libraries, discover, and playback. It talks to the Go server over HTTP and WebSockets using the shared API client in `@plum/shared` and wire types from `@plum/contracts`.

## Stack

- **UI:** React 19, React Router 7, TanStack Query
- **Video:** `hls.js`, JASSUB for ASS subtitles
- **Styling:** Tailwind CSS 4
- **Tooling:** Vite, Vitest, oxlint

## Scripts

Run from **this directory** (`apps/web`) or via `bun run --cwd apps/web <script>` from the repo root.

| Script | Description |
|--------|-------------|
| `dev` | Vite dev server with HMR |
| `build` | `tsc -b` then production bundle |
| `preview` | Serve the production build locally |
| `lint` | oxlint |
| `typecheck` | Project references typecheck |
| `test` | Vitest (all tests) |
| `test:stable` | Excludes `App.test.tsx` (matches root `validate:full` web tests) |
| `test:app` | Only `App.test.tsx` |
| `test:watch` | Vitest watch mode |

From the **monorepo root**, `bun run dev:web` runs this app’s dev server; `bun run build` builds the web app.

## Layout (high level)

| Path | Role |
|------|------|
| `src/pages/` | Route-level screens |
| `src/components/` | Reusable UI, including `playback/` for the player shell |
| `src/contexts/` | Player and app providers |
| `src/queries/` | TanStack Query modules |
| `src/lib/` | App-specific helpers (playback, routing, etc.) |

Shared API types and the HTTP/WebSocket client live in `packages/contracts` and `packages/shared`, not under this app.

## Configuration

- Vite: `vite.config.ts`
- TypeScript: `tsconfig.json`, `tsconfig.app.json`
- Tests: `vitest.config.ts`

## Environment and backend

The UI expects the Plum server (default `http://localhost:8080` in dev, depending on Vite proxy / env). See the [root README](../../README.md) for `.env` setup, metadata API keys, and Docker. Player and library behavior are documented in the repo-wide [AGENTS.md](../../AGENTS.md) for contributors.
