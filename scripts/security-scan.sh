#!/bin/sh

set -eu

export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.27.0}"

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BACKEND_DIR="$ROOT_DIR/unyport/backend"

"$ROOT_DIR/scripts/csp-audit.sh"

cd "$BACKEND_DIR"
go test ./...
go vet ./...

if command -v govulncheck >/dev/null 2>&1; then
  govulncheck ./...
elif [ -x "$(go env GOPATH)/bin/govulncheck" ]; then
  "$(go env GOPATH)/bin/govulncheck" ./...
else
  echo "govulncheck not found; install with:" >&2
  echo "go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
  exit 1
fi

