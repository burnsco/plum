# Plum

Plum is an experimental media server and player suite inspired by Plex and Jellyfin. It pairs a Go backend with web and desktop clients inside a Bun workspace monorepo.

## Repo layout

- `apps/server`: Go backend
- `apps/web`: React web client
- `apps/desktop`: desktop app work
- `packages/*`: shared contracts and utilities

## Highlights

- Media library browsing
- Server-driven transcoding flow
- Realtime updates between backend and client
- Lightweight monorepo setup for cross-app iteration

## Quick start

```bash
bun install
bun run dev
```

Useful commands:

```bash
bun run dev:web
bun run dev:server
bun run typecheck
```
