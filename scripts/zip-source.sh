#!/usr/bin/env bash
set -euo pipefail

OUT="${1:-$HOME/plum.zip}"

cd "$(dirname "$0")/.."

echo "Zipping source to $OUT..."

zip -r "$OUT" . \
  --exclude "*/.git/*" \
  --exclude "*/node_modules/*" \
  --exclude "*/dist/*" \
  --exclude "*/dist-ssr/*" \
  --exclude "*/.vite/*" \
  --exclude "*/build/*" \
  --exclude "*/tmp/*" \
  --exclude "*/.kotlin/*" \
  --exclude "*/.gradle/*" \
  --exclude "*/data/artwork/*" \
  --exclude "*/data/thumbnails/*" \
  --exclude "*.db" \
  --exclude "*.db-wal" \
  --exclude "*.db-shm" \
  --exclude "*.db-journal" \
  --exclude "./apps/server/plum" \
  --exclude "./apps/server/bin/*" \
  --exclude "./.env" \
  --exclude "./.env.local" \
  --exclude "./.env.*.local" \
  --exclude "./*.log" \
  --exclude "*/*.log" \
  > /dev/null

echo "Done: $(du -sh "$OUT" | cut -f1) — $OUT"
