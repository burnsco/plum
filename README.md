# Plum

<p align="center">
  <img src="apps/web/public/logo.svg" alt="Plum logo" width="128" />
</p>

Plum is an experimental self-hosted media server and client suite. It combines a Go backend, a React web app, and an Android TV client in a single monorepo focused on fast local streaming, responsive playback, and a modern browsing experience.

## Project Status

Plum is actively developed and still evolving. The current codebase is usable for local development and feature work, but it should be treated as an experimental project rather than a polished production server.

Current areas of focus:

- Fast, reliable playback across web and Android TV
- Library browsing and metadata-driven discovery
- Shared contracts and behavior across clients
- Low-overhead local deployment with SQLite and simple tooling

## What Plum Includes

- **Go server** for library management, metadata lookup, playback APIs, WebSockets, and transcoding flows
- **React web client** for browsing, search, discovery, and playback
- **Android TV client** built with Kotlin and Compose for TV
- **Shared TypeScript packages** for API contracts and cross-client runtime logic
- **SQLite-backed local storage** with a lightweight self-hosted footprint

## Monorepo Layout

| Path | Purpose |
| --- | --- |
| `apps/server` | Go backend service |
| `apps/web` | React + Vite web client |
| `apps/android` | Android TV application |
| `packages/contracts` | Shared Effect schemas and API/WebSocket contracts |
| `packages/shared` | Shared TypeScript runtime utilities |

Architecture notes and package ownership guidance live in [docs/cross-platform-alignment.md](docs/cross-platform-alignment.md).

## Tech Stack

- **Backend:** Go, Chi, SQLite
- **Frontend:** React 19, Vite, TypeScript, Tailwind CSS 4
- **Shared contracts:** Effect schemas
- **Tooling:** Bun, Vitest, oxlint
- **Streaming:** HLS-based playback pipeline

## Getting Started

### Prerequisites

- **Bun** `>=1.2.0`
- **Node.js** `^20.19.0 || >=22.12.0`
- **Go** `1.26.1` for `apps/server`

The repo ships with `.nvmrc` set to `22` as a convenient default.

### Setup

1. Clone the repository.
2. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

3. Install dependencies from the repo root:

   ```bash
   bun install
   ```

4. Start the development stack:

   ```bash
   bun run dev
   ```

This runs the web client and Go server together. You can also run them separately with `bun run dev:web` and `bun run dev:server`.

If you use the local helper workflow in the Makefile, `make dev` is also available.

## Environment

Most development only requires a copied `.env`, but metadata and media-stack integrations depend on the values you provide.

### Metadata providers

- `TMDB_API_KEY`
- `TVDB_API_KEY`
- `OMDB_API_KEY`
- `FANART_API_KEY`

### Server and local runtime

- `PLUM_ADDR`
- `PLUM_DATABASE_URL`
- `BACKEND_INTERNAL_URL`
- `VITE_BACKEND_URL`
- `VITE_PLAYBACK_STREAM_BASE`

### Discover / download integrations

- `PLUM_RADARR_BASE_URL`
- `PLUM_RADARR_API_KEY` or `PLUM_RADARR_API_KEY_FILE`
- `PLUM_RADARR_QUALITY_PROFILE_ID`
- `PLUM_RADARR_ROOT_FOLDER_PATH`
- `PLUM_RADARR_SEARCH_ON_ADD`
- `PLUM_SONARR_TV_BASE_URL`
- `PLUM_SONARR_TV_API_KEY` or `PLUM_SONARR_TV_API_KEY_FILE`
- `PLUM_SONARR_TV_QUALITY_PROFILE_ID`
- `PLUM_SONARR_TV_ROOT_FOLDER_PATH`
- `PLUM_SONARR_TV_SEARCH_ON_ADD`

See [.env.example](.env.example) for the current documented defaults and comments.

## Common Commands

Run these from the repository root unless noted otherwise.

| Command | Purpose |
| --- | --- |
| `bun run dev` | Start web and server together |
| `bun run dev:web` | Start only the web client |
| `bun run dev:server` | Start only the Go backend |
| `bun run validate` | Default fast validation path |
| `bun run validate:full` | Full validation including web tests/build and Android debug build |
| `bun run validate:web` | Web-only validation |
| `bun run validate:server` | Go tests and build |
| `bun run validate:android` | Android TV lint and debug assemble |
| `make dev` | Local helper workflow for development |
| `make docker-dev` | Start the Docker-based dev stack |

## Validation

Before considering work complete, the expected default check is:

```bash
bun run validate
```

That fast path runs:

- linting for the web and shared TypeScript packages
- TypeScript typechecks for `apps/web`, `packages/shared`, and `packages/contracts`
- backend tests in `apps/server`

Use the narrower platform-specific validation commands when your changes are clearly scoped to one surface.

## Android TV

The Android TV client lives in `apps/android`.

Useful entry points:

- [ANDROID_TV.md](ANDROID_TV.md) for the main development guide
- [ANDROID_TV_LOCAL.md](ANDROID_TV_LOCAL.md) for local setup notes
- [apps/android/AGENT_DEPLOY.md](apps/android/AGENT_DEPLOY.md) for deployment details

Common Android commands:

- `bun run android:assemble`
- `bun run android:install`
- `bun run android:deploy`

## Docker

A Docker-based development workflow is available through the repo Makefile and `compose.yml`.

```bash
make docker-dev
```

Make sure your `.env` includes any metadata or integration keys you expect the containerized app to use.

## Additional Docs

- [apps/web/README.md](apps/web/README.md) for web-client specifics
- [docs/cross-platform-alignment.md](docs/cross-platform-alignment.md) for shared ownership rules
- [docs/INTRO_FINGERPRINT.md](docs/INTRO_FINGERPRINT.md) for intro fingerprinting notes
- [milestones.md](milestones.md) for roadmap context

## License

No project license has been added yet.
