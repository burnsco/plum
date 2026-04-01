# Fix Arr Profile Drift and Lock In Search-on-Add

## Summary

We verified on `mordor` that:

- `Radarr` profile `id=1` is `Any`
- `Sonarr TV` profile `id=1` is `Any`
- the intended live profiles are `Radarr: id=8 = UHD Bluray + WEB` and `Sonarr TV: id=10 = WEB-2160p`
- qBittorrent is configured and auto-search is working in practice

Your latest note changes the diagnosis: this is not primarily an “auto download is broken” issue. The add flow is succeeding, and releases can still come down in the desired quality even when the item is assigned the `Any` profile. The real issue is profile assignment/persistence drifting to `Any`, which makes Arr’s UI misleading and weakens future filtering.

## Key Changes

### 1. Immediate operator fix
- Update the active Plum media-stack config so it no longer uses profile id `1`.
- Set:
  - `PLUM_RADARR_QUALITY_PROFILE_ID=8`
  - `PLUM_SONARR_TV_QUALITY_PROFILE_ID=10`
- If saved settings in Plum override env values, run the existing `Validate & load defaults` flow and save the corrected values so the DB matches the live Arr profile IDs.

### 2. Backend hardening for profile stability
- Extend `MediaStackServiceSettings` to store both:
  - `qualityProfileId`
  - `qualityProfileName`
- Add env support for:
  - `PLUM_RADARR_QUALITY_PROFILE_NAME`
  - `PLUM_SONARR_TV_QUALITY_PROFILE_NAME`
- Resolution order when adding media:
  1. exact name match against live Arr profiles
  2. fallback to stored numeric id
  3. fail validation if neither resolves
- Keep the numeric id as cached state, but treat the profile name as the durable source of truth.

### 3. Validation/default-loading behavior
- When `Validate & load defaults` runs, populate both the selected profile id and name from the live Arr response.
- Keep the existing preferred defaults:
  - Radarr: `UHD Bluray + WEB`
  - Sonarr TV: `WEB-2160p`
- If the saved id is invalid but the saved name still exists, auto-repair the id from the live profile list instead of falling back to `Any`.

### 4. Search-on-add correctness cleanup
- Fix `normalizeMediaStackServiceSettings()` so it does not forcibly overwrite `SearchOnAdd` to `true`.
- Make `AddMovie()` and `AddSeries()` actually honor `settings.SearchOnAdd`.
- Keep the default value `true` so current behavior does not regress.

## Public Interface Changes

- Add optional `qualityProfileName` to the media-stack settings JSON shape returned by and accepted by `/api/settings/media-stack`.
- Add optional env vars:
  - `PLUM_RADARR_QUALITY_PROFILE_NAME`
  - `PLUM_SONARR_TV_QUALITY_PROFILE_NAME`
- No routing or add API changes are needed.

## Test Plan

- Backend unit test: env/profile resolution prefers `qualityProfileName` over stale `qualityProfileId`.
- Backend unit test: validate flow repairs stale ids when the saved name still matches a live profile.
- Backend unit test: `AddMovie()` sends the resolved Radarr profile id and respects `SearchOnAdd`.
- Backend unit test: `AddSeries()` sends the resolved Sonarr profile id and respects `SearchOnAdd`.
- HTTP/settings test: `GET` and `PUT` round-trip `qualityProfileName`.
- UI/settings test: `Validate & load defaults` stores both the preferred id and matching name.
- Manual verification on `mordor`:
  - Add one movie through Plum and confirm Radarr item shows profile `UHD Bluray + WEB`
  - Add one TV show through Plum and confirm Sonarr item shows profile `WEB-2160p`
  - Confirm each add either enters queue immediately or creates the expected Arr search command when no release is available

## Assumptions

- We will treat the profile-assignment mismatch as the main bug; auto-search is already functioning.
- `UHD Bluray + WEB` and `WEB-2160p` remain the desired defaults for this server.
- We are not expanding scope in this pass to route anime to `sonarr-anime` or redesign the Downloads UX.
