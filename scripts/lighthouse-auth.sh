#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

WEB_URL="${PLUM_LIGHTHOUSE_WEB_URL:-http://127.0.0.1:4173}"
AUDIT_PATH="${1:-/}"
CHROME_PATH="${PLUM_LIGHTHOUSE_CHROME_PATH:-/usr/bin/chromium}"
OUTPUT_PATH="${PLUM_LIGHTHOUSE_OUTPUT_PATH:-/tmp/plum-lighthouse-auth.json}"
LIGHTHOUSE_CATEGORY="${PLUM_LIGHTHOUSE_CATEGORY:-accessibility}"
EMAIL="${PLUM_LIGHTHOUSE_EMAIL:-}"
PASSWORD="${PLUM_LIGHTHOUSE_PASSWORD:-}"

if [[ -z "$EMAIL" || -z "$PASSWORD" ]]; then
  cat >&2 <<'EOF'
Set PLUM_LIGHTHOUSE_EMAIL and PLUM_LIGHTHOUSE_PASSWORD before running this script.

Example:
  PLUM_LIGHTHOUSE_EMAIL=admin@example.com \
  PLUM_LIGHTHOUSE_PASSWORD=secret-password \
  ./scripts/lighthouse-auth.sh /discover
EOF
  exit 1
fi

if [[ "$AUDIT_PATH" != /* ]]; then
  AUDIT_PATH="/$AUDIT_PATH"
fi

TMP_DIR="$(mktemp -d)"
COOKIE_JAR="$TMP_DIR/cookies.txt"
trap 'rm -rf "$TMP_DIR"' EXIT

LOGIN_PAYLOAD="$(node -e 'process.stdout.write(JSON.stringify({email: process.argv[1], password: process.argv[2]}))' "$EMAIL" "$PASSWORD")"

curl \
  --silent \
  --show-error \
  --fail \
  --cookie-jar "$COOKIE_JAR" \
  --header 'Content-Type: application/json' \
  --data "$LOGIN_PAYLOAD" \
  "$WEB_URL/api/auth/login" >/dev/null

curl \
  --silent \
  --show-error \
  --fail \
  --cookie "$COOKIE_JAR" \
  "$WEB_URL/api/auth/me" >/dev/null

COOKIE_HEADER="$(
  awk '
    BEGIN { first = 1 }
    NF >= 7 {
      domain = $1
      if (index(domain, "#HttpOnly_") == 1) {
        domain = substr(domain, 11)
      } else if (index(domain, "#") == 1) {
        next
      }
      if (!first) {
        printf "; "
      }
      printf "%s=%s", $6, $7
      first = 0
    }
  ' "$COOKIE_JAR"
)"

if [[ -z "$COOKIE_HEADER" ]]; then
  echo "No auth cookie was captured during login." >&2
  exit 1
fi

EXTRA_HEADERS="$(node -e 'process.stdout.write(JSON.stringify({Cookie: process.argv[1]}))' "$COOKIE_HEADER")"
AUDIT_URL="${WEB_URL}${AUDIT_PATH}"

echo "Running Lighthouse on $AUDIT_URL as $EMAIL"
echo "Saving report to $OUTPUT_PATH"

cd "$ROOT_DIR"

bunx --yes lighthouse "$AUDIT_URL" \
  --only-categories="$LIGHTHOUSE_CATEGORY" \
  --output=json \
  --output-path="$OUTPUT_PATH" \
  --chrome-path="$CHROME_PATH" \
  --extra-headers="$EXTRA_HEADERS" \
  --quiet \
  --chrome-flags='--headless=new --no-sandbox'

echo "Done."
