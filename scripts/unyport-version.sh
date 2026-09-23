#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION_FILE=${UNYPORT_VERSION_FILE:-"$ROOT_DIR/unyport/backend/config/version.go"}

current() {
  sed -n 's/^const Version = "\(.*\)"/\1/p' "$VERSION_FILE" | head -n 1
}

apply_version() {
  version=${1#v}
  tmp=$(mktemp)
  sed "s/^const Version = \".*\"/const Version = \"$version\"/" "$VERSION_FILE" > "$tmp"
  mv "$tmp" "$VERSION_FILE"
}

bump() {
  kind=${1:-fix}
  version=$(current)
  IFS=. read -r major minor patch <<EOF
$version
EOF
  major=${major:-0}
  minor=${minor:-0}
  patch=${patch:-0}
  case "$kind" in
    major) major=$((major + 1)); minor=0; patch=0 ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    fix|patch|*) patch=$((patch + 1)) ;;
  esac
  apply_version "$major.$minor.$patch"
  current
}

case "${1:-current}" in
  current) current ;;
  apply) [ -n "${2:-}" ] || { echo "usage: $0 apply <version>" >&2; exit 1; }; apply_version "$2" ;;
  bump) bump "${2:-fix}" ;;
  *) echo "usage: $0 [current|apply <version>|bump <fix|minor|major>]" >&2; exit 1 ;;
esac

