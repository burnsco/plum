#!/usr/bin/env bash

# Shared Go tempdir setup for local dev flows.
# Go and Air both need short-lived build trees to avoid /tmp quota failures.

if [[ -z "${ROOT_DIR:-}" ]]; then
  ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

GO_TMPDIR_DEFAULT="${PLUM_GO_TMPDIR:-${HOME:-$ROOT_DIR}/tmp/plum-go}"

export TMPDIR="$GO_TMPDIR_DEFAULT"
export GOTMPDIR="$GO_TMPDIR_DEFAULT"

mkdir -p "$GO_TMPDIR_DEFAULT"
