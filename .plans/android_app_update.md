# Plex-Inspired Android TV Refresh

## Summary
Rework the Android TV UI to feel more like Plex’s modern big-screen experience: more cinematic, less blocky, and more driven by artwork, hierarchy, and focus states than by dense panels. The goal is to keep the same behavior and data flow, but make the shell, home screen, details pages, and shared TV components feel lighter and more intentional.

## Key Changes
- Redesign the shared TV theme and primitives in [`apps/android-tv/core-ui/src/main/java/plum/tv/core/ui/PlumTvTheme.kt`](/home/cburns/apps/plum/apps/android-tv/core-ui/src/main/java/plum/tv/core/ui/PlumTvTheme.kt) and [`apps/android-tv/core-ui/src/main/java/plum/tv/core/ui/PlumTvComponents.kt`](/home/cburns/apps/plum/apps/android-tv/core-ui/src/main/java/plum/tv/core/ui/PlumTvComponents.kt):
  - soften the heavy panel look, reduce “boxiness,” and shift toward artwork-backed surfaces and scrims
  - tune typography, spacing, corner radii, and focus glow so cards feel less chunky on a TV
  - add a more Plex-like hierarchy for hero, rails, and metadata chips
- Rework the app shell in [`apps/android-tv/app/src/main/java/plum/tv/app/MainNav.kt`](/home/cburns/apps/plum/apps/android-tv/app/src/main/java/plum/tv/app/MainNav.kt):
  - make navigation feel slimmer and more persistent, with a clearer current-section indicator
  - add a lightweight top/status area when useful, instead of making the side rail carry all the visual weight
  - preserve stable back behavior and focused-route restoration
- Rebuild the Home screen in [`apps/android-tv/feature-home/src/main/java/plum/tv/feature/home/HomeScreen.kt`](/home/cburns/apps/plum/apps/android-tv/feature-home/src/main/java/plum/tv/feature/home/HomeScreen.kt):
  - introduce a cinematic hero/featured area for continue-watching or a highlighted title
  - reduce the “same-size card grid everywhere” feel by mixing rails, featured artwork, and tighter section hierarchy
  - use artwork-led backgrounds or backdrops where available, with readable scrims over them
- Refresh movie and show details in [`apps/android-tv/feature-details/src/main/java/plum/tv/feature/details/MovieDetailScreen.kt`](/home/cburns/apps/plum/apps/android-tv/feature-details/src/main/java/plum/tv/feature/details/MovieDetailScreen.kt) and [`apps/android-tv/feature-details/src/main/java/plum/tv/feature/details/ShowDetailScreen.kt`](/home/cburns/apps/plum/apps/android-tv/feature-details/src/main/java/plum/tv/feature/details/ShowDetailScreen.kt):
  - move toward a poster + backdrop + action strip layout instead of stacked panels
  - make episode browsing feel more like a focused TV details page, not a generic form
  - keep primary actions obvious and reduce the number of competing buttons
- Align the style direction with Plex Android TV patterns from Plex’s official settings docs, especially the ideas of remembered tabs, artwork-driven backgrounds, details backgrounds, and reduce-motion support: [Plex Android TV support](https://support.plex.tv/articles/settings-android-tv/)

## Test Plan
- Run the required repo gates after implementation: `bun lint`, `bun typecheck`, and `go test ./...` in `apps/server`
- Add or update Compose/UI checks for:
  - home layout hierarchy and focus order
  - details page action prominence and poster/backdrop composition
  - side-rail behavior and back navigation
- Validate visually on emulator first, then on a real Android TV device with screenshots or device walkthroughs against a Plex modern-layout reference
- If the device is available later, use live comparison against the TV box to tune spacing, font scale, and focus feedback

## Assumptions
- Targeting Plex’s **modern** big-screen look, not the classic one
- No backend behavior changes are required for this refresh
- The current app logic stays intact; this is a presentation and interaction polish pass
- I could not attach to your Android box from this workspace right now because ADB wasn’t available here, so the plan is based on the repo plus Plex’s official Android TV guidance
