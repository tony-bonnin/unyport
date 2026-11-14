#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BINARY=${UNYPORT_DEPLOY_BINARY:-"$ROOT_DIR/dist/unyport"}
CONTAINER=${UNYPORT_DEPLOY_CONTAINER:-unyport}
ENABLED=${UNYPORT_DEPLOY_ENABLED:-0}
REMOTE_BINARY=${UNYPORT_DEPLOY_REMOTE_BINARY:-/usr/local/bin/unyport}

[ "$ENABLED" = 1 ] || { echo '[unyport-deploy] deployment disabled'; exit 0; }
[ "$CONTAINER" = unyport ] || { echo "[unyport-deploy] refused container: $CONTAINER" >&2; exit 1; }
[ -f "$BINARY" ] || { echo "[unyport-deploy] missing binary: $BINARY" >&2; exit 1; }
docker inspect "$CONTAINER" >/dev/null

local_sha=$(sha256sum "$BINARY" | awk '{print $1}')
docker cp "$BINARY" "$CONTAINER:$REMOTE_BINARY.new"
docker exec "$CONTAINER" sh -eu -c "cp '$REMOTE_BINARY' '$REMOTE_BINARY.previous'; chmod 755 '$REMOTE_BINARY.new'; mv '$REMOTE_BINARY.new' '$REMOTE_BINARY'"
remote_sha=$(docker exec "$CONTAINER" sha256sum "$REMOTE_BINARY" | awk '{print $1}')
[ "$local_sha" = "$remote_sha" ] || { echo "[unyport-deploy] checksum mismatch" >&2; exit 1; }
docker restart "$CONTAINER" >/dev/null
echo "[unyport-deploy] deployed $local_sha to $CONTAINER"
