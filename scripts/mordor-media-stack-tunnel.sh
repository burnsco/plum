#!/usr/bin/env bash
set -euo pipefail

exec ssh \
  -N \
  -p 2222 \
  -L 7878:127.0.0.1:7878 \
  -L 8989:127.0.0.1:8989 \
  192.168.2.124
