#!/usr/bin/env bash
# Plum: map Node processes when debugging high RSS during local dev.
set -euo pipefail

echo "Expected from a single \`bun dev\`: ~2 Node processes (concurrently + Vite)."
echo "Extra large Node PIDs usually mean a second dev server, Vitest, or IDE tooling."
echo ""
if command -v ps >/dev/null 2>&1; then
  if ps -C node -o pid= >/dev/null 2>&1; then
    ps -ww -o pid,rss,args -C node 2>/dev/null || true
  else
    echo "(no processes named 'node' — try: ps aux | rg '[n]ode')"
  fi
else
  echo "ps not available"
fi
echo ""
echo "Optional: clear Vite prebundle cache if it looks corrupted:"
echo "  bun run dev:clear-vite-cache"
