# Chromaprint intro detection

Plum can optionally refine TV/anime intro bounds using **raw Chromaprint** fingerprints from ffmpeg (first ~120s of audio per episode), a disk cache under `PLUM_INTRO_FINGERPRINT_DIR` (default: `<database-directory>/intro_fingerprints` when using a file-backed SQLite path), and **pairwise matching within each season**.

## FFmpeg requirements

The `chromaprint` **muxer** must be available. Many minimal distro packages omit it. Builds that include it (for example **jellyfin-ffmpeg**) typically report the muxer when you run:

```bash
ffmpeg -hide_banner -h muxer=chromaprint
```

If you see `Unknown format 'chromaprint'`, install a build that links Chromaprint or set `PLUM_INTRO_FINGERPRINT_DIR` only after switching ffmpeg.

## API

- `POST /api/libraries/{id}/intro/chromaprint-scan` — optional JSON body `{ "show_key": "tmdb-123" }` to limit to one series; omit for all shows in the library.
- Returns `400` when the muxer is missing or the fingerprint cache directory cannot be resolved.

## Limitations

Detection is heuristic (Hamming distance on subfingerprints); false positives/negatives are possible. Episodes with `intro_locked` on the primary `media_files` row are skipped.
