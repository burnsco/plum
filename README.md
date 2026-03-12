# Plum

**Plum** is a modern, lightweight, and high-performance media server and player suite. Built for those who want full control over their media collection without the overhead or costs of commercial solutions.

## Why Plum?

The media server landscape is dominated by a few big names, but each comes with its own set of trade-offs. Plum was born out of a desire for a better middle ground:

- **Free and Open:** Plex is powerful, but locking core features (like mobile playback or hardware transcoding) behind a paid "Plex Pass" subscription can be frustrating. Plum is committed to keeping full power in the user's hands without a paywall.
- **Modern UI/UX:** Jellyfin is a fantastic open-source project, but its user interface can often feel dated or inconsistent. Plum focuses on a clean, responsive, and intuitive interface that rivals commercial alternatives.
- **Built for Performance:** Leveraging a Go backend and a modern React frontend, Plum is designed to be lightweight enough for low-power devices while staying fast and responsive.

## Key Features

- **Multi-Library Support:** Organize your Movies, TV Shows, and Music with ease.
- **Smart Metadata Matching:** Automatic identification using TMDB, TVDB, and OMDb.
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
- `packages/contracts`: Shared TypeScript types and API definitions.
- `packages/shared`: Common utilities used across the monorepo.

## Quick Start

### Prerequisites

- [Bun](https://bun.sh/) installed.
- [Go](https://go.dev/) installed (for backend development).

### Setup

1. Clone the repository.
2. Install dependencies:

   ```bash
   bun install
   ```

3. Start the development environment:

   ```bash
   bun run dev
   ```

### Useful Commands

- `bun run dev:web` — Start only the web client.
- `bun run dev:server` — Start only the Go backend.
- `bun run typecheck` — Run TypeScript type checking across the project.
- `bun run validate` — Run linting, type checking, and tests.

## Roadmap

Plum is actively developed. Check out our [Milestones](milestones.md) for a detailed look at the current roadmap and upcoming features.
