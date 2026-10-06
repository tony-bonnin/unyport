#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
failed=0
tmp="${TMPDIR:-/tmp}/unyport-csp-audit.$$"

check_absent() {
  pattern="$1"
  shift
  if grep -R \
    --exclude-dir=tmp \
    --exclude-dir=logs \
    --exclude-dir=assets \
    --exclude='*.min.js' \
    --exclude='*.min.css' \
    "$pattern" "$@" >"$tmp" 2>/dev/null; then
    cat "$tmp" >&2
    failed=1
  fi
}

check_absent "unsafe-" \
  "$ROOT_DIR/unyport/backend"

check_absent "sha256-" \
  "$ROOT_DIR/unyport/backend"

check_absent "cdn\\.jsdelivr" \
  "$ROOT_DIR/unyport/backend"

check_absent "@import url(['\"]https\\?://" \
  "$ROOT_DIR/unyport/frontend/public/css"

check_absent "createElement(['\"]style" \
  "$ROOT_DIR/unyport/frontend/public/app"

check_absent "<script>[[:space:]]*$" \
  "$ROOT_DIR/unyport/frontend/public/index.html"

check_absent "<style>[[:space:]]*$" \
  "$ROOT_DIR/unyport/frontend/public/index.html"

rm -f "$tmp"

if [ "$failed" -ne 0 ]; then
  echo "UnyPort CSP audit failed" >&2
  exit 1
fi

echo "UnyPort CSP audit passed"
