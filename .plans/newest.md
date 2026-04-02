# Plum Android TV Web-Parity Pass

## Summary

- Rework Android TV to mirror the web app’s core media experience as closely as TV constraints allow.
- Keep the same content hierarchy, visual language, and media flows; preserve native TV focus and remote behavior instead of forcing desktop chrome.

## Key Changes

- Align the top-level navigation and screen order with the web app’s core areas: Home, Libraries, Search, Settings, detail pages, and playback.
- Shift the shell away from a launcher-like feel and toward the web app’s structure: stronger chrome, clearer selected states, and consistent back-stack behavior.
- Tune the Compose TV design system to match the web token set as closely as possible: background layers, panel/card surfaces, accent color, borders, typography, poster sizing, hero/backdrop treatment, chips, and loading/error states.
- Rework Home so it emphasizes the same priorities as the web app, especially continue-watching and recently-added rails with a more web-like hero composition.
- Rework library browsing and search to use the same poster-card language, metadata density, and result presentation as the web app, while keeping D-pad-friendly focus restoration and pagination.
- Reshape movie and show detail pages to mirror the web composition: backdrop layering, poster plus metadata block, cast section, season switching, episode rows, and resume/play actions.
- Keep playback behavior aligned with the web experience: resume handling, audio and subtitle switching, progress sync, and playback-session attachment.
- Prefer shared helpers for common logic and contracts where they already exist, but keep rendering native; do not introduce cross-platform UI sharing.

## Test Plan

- Run the repo gates: `bun lint`, `bun typecheck`, and `go test ./...` in `apps/server`.
- Add or extend Android tests for auth bootstrap and server switching, home/library/detail/search state rendering, D-pad focus entry and restore, back-stack flow, and show season switching plus episode play/resume paths.
- Verify parity manually on emulator and one real Android TV device against the same server and media set, checking destination order, home rails, detail-page structure, search behavior, and playback controls.

## Assumptions

- Scope stays on core media parity only; web-only Discover, Downloads, and admin-style areas stay out of this pass.
- TV-specific controls and focus affordances stay native, but the screen structure and styling should feel nearly identical to web.
- If Android is missing a web-visible behavior, prefer a small backend or contract change over inventing an Android-only shape.
