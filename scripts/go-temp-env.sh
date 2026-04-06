#!/usr/bin/env bash

# Shared Go tempdir setup for local dev flows.
# Go and Air both need short-lived build trees to avoid /tmp quota failures.

if [[ -z "${ROOT_DIR:-}" ]]; then
  ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

go_tmpdir_writable() {
  local d="$1"
  [[ -n "$d" ]] || return 1
  mkdir -p "$d" 2>/dev/null || return 1
  [[ -w "$d" ]] || return 1
  local probe
  probe="$(mktemp -d "$d/writetest.XXXXXX" 2>/dev/null)" || return 1
  rmdir "$probe" 2>/dev/null || true
  return 0
}

pick_go_tmpdir() {
  local preferred="${PLUM_GO_TMPDIR:-}"
  local candidates=()
  [[ -n "$preferred" ]] && candidates+=("$preferred")
  candidates+=("${HOME:-$ROOT_DIR}/tmp/plum-go")
  candidates+=("$ROOT_DIR/tmp/plum-go")
  [[ -n "${TMPDIR:-}" && "${TMPDIR}" != "/" ]] && candidates+=("${TMPDIR}/plum-go")
  candidates+=("/tmp/plum-go-${USER:-plum}")

  local c
  for c in "${candidates[@]}"; do
    if go_tmpdir_writable "$c"; then
      printf '%s' "$c"
      return 0
    fi
  done

  echo "go-temp-env: no writable Go temp directory (tried: ${candidates[*]})" >&2
  return 1
}

GO_TMPDIR_CHOSEN="$(pick_go_tmpdir)" || exit 1

export TMPDIR="$GO_TMPDIR_CHOSEN"
export GOTMPDIR="$GO_TMPDIR_CHOSEN"
