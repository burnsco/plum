# Plum

<p align="center">
  <img src="apps/web/public/logo.svg" alt="Plum" width="128" />
</p>

**Plum** is a modern, lightweight, and high-performance media server and player suite. Built for those who want full control over their media collection without the overhead or costs of commercial solutions.

## Why Plum?

The media server landscape is dominated by a few big names, but each comes with its own set of trade-offs. Plum was born out of a desire for a better middle ground:

- **Free and Open:** Plex is powerful, but locking core features (like mobile playback or hardware transcoding) behind a paid "Plex Pass" subscription can be frustrating. Plum is committed to keeping full power in the user's hands without a paywall.
- **Modern UI/UX:** Jellyfin is a fantastic open-source project, but its user interface can often feel dated or inconsistent. Plum focuses on a clean, responsive, and intuitive interface that rivals commercial alternatives.
- **Built for Performance:** Leveraging a Go backend and a modern React frontend, Plum is designed to be lightweight enough for low-power devices while staying fast and responsive.

## Key Features

- **Multi-Library Support:** Organize your Movies, TV Shows, and Music with ease.
- **Smart Metadata Matching:** Automatic identification using TMDB, TVDB, OMDb, and Fanart.tv artwork.
- **High-Performance Streaming:** Server-driven HLS transcoding flow for seamless playback across devices.
- **Real-time Synchronization:** Stay updated with instant library changes and playback progress via WebSockets.
- **User Management:** Secure authentication and multi-user support built-in.
- **Monorepo Architecture:** A unified codebase for Server, Web, and Desktop clients.

## Tech Stack

Plum is built with modern, efficient technologies:

- **Backend:** [Go](https://go.dev/) (High-performance API and media processing)
- **Frontend:** [React](https://reactjs.org/) + [Vite](https://vitejs.dev/) + [TypeScript](https://www.typescriptlang.org/)
- **Runtime:** [Bun](https://bun.sh/) (Fast JavaScript/TypeScript toolchain)
- **Database:** [SQLite](https://sqlite.org/) (Zero-config, high-performance local storage)
- **Desktop:** [Electron](https://www.electronjs.org/) (Cross-platform desktop client)

## Repository Layout

- `apps/server`: The core Go backend service.
- `apps/web`: The React-based web interface.
- `apps/desktop`: The cross-platform desktop application.
- `apps/android-tv`: The Android TV client.
- `packages/contracts`: Shared TypeScript types and API definitions.
- `packages/shared`: Common utilities used across the monorepo.

## Quick Start

### Prerequisites

- [Bun](https://bun.sh/) installed.
- [Go](https://go.dev/) installed (for backend development).

### Setup

1. Clone the repository.
2. Create your local env file:

   ```bash
   cp .env.example .env
   ```

3. Install dependencies:

   ```bash
   bun install
   ```

4. Start the development environment:

   ```bash
   make dev
   ```

### Environment

The backend reads these metadata-related env vars at runtime:

- `TMDB_API_KEY`
- `TVDB_API_KEY`
- `OMDB_API_KEY`
- `FANART_API_KEY`

The Docker Compose setup forwards the same variables into the `app` container, so the same `.env` file works for local development and Docker.

Discover add/download integration can also be bootstrapped from env when the
media-stack settings table is still empty:

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

If your Radarr/Sonarr services only listen on `127.0.0.1` on another machine,
forward them locally first and then point Plum at the forwarded ports. This repo
includes `scripts/mordor-media-stack-tunnel.sh` for the `mordor` host.

If Plum is running on a different machine, VM, or Docker Compose than your
media stack, the Plum backend only needs network reachability to the Radarr and
Sonarr HTTP APIs. qBittorrent can stay private behind Radarr/Sonarr unless you
want to access its web UI directly.

Common setups:

- If Radarr/Sonarr are reachable over your LAN or Tailscale, set
  `PLUM_RADARR_BASE_URL` and `PLUM_SONARR_TV_BASE_URL` to those URLs directly.
- If Radarr/Sonarr only listen on `127.0.0.1` on the remote host, create an SSH
  tunnel from the machine running Plum and keep the Plum env pointed at the
  forwarded local ports.

Example SSH tunnel for a remote host:

```bash
ssh -N \
  -L 7878:127.0.0.1:7878 \
  -L 8989:127.0.0.1:8989 \
  user@your-server
```

Then use:

```bash
PLUM_RADARR_BASE_URL=http://127.0.0.1:7878
PLUM_SONARR_TV_BASE_URL=http://127.0.0.1:8989
```

Other useful runtime env vars:

- `PLUM_ADDR`
- `PLUM_DATABASE_URL` or `PLUM_DB_PATH`
- `PLUM_ALLOWED_ORIGINS`
- `PLUM_THUMBNAILS_DIR`
- `PLUM_ARTWORK_DIR`
- `MUSICBRAINZ_CONTACT_URL`
- `PLUM_MEDIA_TV_PATH`
- `PLUM_MEDIA_MOVIES_PATH`
- `PLUM_MEDIA_ANIME_PATH`
- `PLUM_MEDIA_MUSIC_PATH`

### Useful Commands

- `make dev` — Start local web + server dev, export `.env`, and open the `mordor` Radarr/Sonarr tunnel automatically.
- `make dev-clean` — Reset the local dev DB/cache state, then start local dev with live console output.
- `make dev-stop` — Stop the tracked `mordor` media-stack tunnel used by local dev.
- `make docker-dev` — Run the full Docker dev stack.
- `make docker-dev-clean` — Recreate the Docker dev stack from scratch.
- `bun run dev:web` — Start only the web client.
- `bun run dev:server` — Start only the Go backend.
- `bun run typecheck` — Run TypeScript type checking across the project.
- `bun run lint` — Run the workspace lint checks.
- `bun run validate` — Run linting, type checking, and backend tests.

### Docker

To run the dev stack in Docker:

```bash
make docker-dev
```

Make sure `.env` includes the metadata keys you want enabled, including `FANART_API_KEY` for Fanart.tv artwork.

## Android TV

Plum has an Android TV client in `apps/android-tv`. See [ANDROID_TV.md](ANDROID_TV.md) for a full guide on building and sideloading it onto your device without Android Studio.

## Roadmap

Plum is actively developed. Check out our [Milestones](milestones.md) for a detailed look at the current roadmap and upcoming features.
