#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BACKEND_DIR="$ROOT_DIR/unyport/backend"
DIST_DIR=${UNYPORT_DIST_DIR:-"$ROOT_DIR/dist"}

mkdir -p "$DIST_DIR"

cd "$BACKEND_DIR"
go test ./...

rm -rf ./server/assets
cp -r ../frontend/public ./server/assets
trap 'rm -rf "$BACKEND_DIR/server/assets"' EXIT INT TERM

CGO_ENABLED="${CGO_ENABLED:-0}" go build -tags prod -trimpath -ldflags "-s -w" -o "$DIST_DIR/unyport" ./cmd/unyport

printf '[unyport-build] built %s\n' "$DIST_DIR/unyport"

