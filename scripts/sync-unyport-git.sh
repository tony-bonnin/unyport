#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE=${UNYPORT_GIT_SYNC_ENV:-"$ROOT_DIR/unyport-git-sync.env"}
VERSION_SCRIPT=${VERSION_SCRIPT:-"$ROOT_DIR/scripts/unyport-version.sh"}

UNYPORT_GIT_SYNC_ENABLED=${UNYPORT_GIT_SYNC_ENABLED:-0}
UNYPORT_GIT_SYNC_PUSH=${UNYPORT_GIT_SYNC_PUSH:-1}
UNYPORT_GIT_BRANCH=${UNYPORT_GIT_BRANCH:-master}
UNYPORT_GIT_RELEASE_KIND=${UNYPORT_GIT_RELEASE_KIND:-auto}
UNYPORT_GIT_RELEASE_VERSION=${UNYPORT_GIT_RELEASE_VERSION:-}
UNYPORT_GIT_AUTO_BUMP=${UNYPORT_GIT_AUTO_BUMP:-1}
UNYPORT_GIT_RELEASE_PUSH_TAGS=${UNYPORT_GIT_RELEASE_PUSH_TAGS:-1}
UNYPORT_GIT_REQUIRE_BUILD=${UNYPORT_GIT_REQUIRE_BUILD:-1}
UNYPORT_GIT_BUILD_VERIFIED=${UNYPORT_GIT_BUILD_VERIFIED:-0}
GH_TOKEN_FILE=${GH_TOKEN_FILE:-${GITHUB_TOKEN_FILE:-}}
GIT_TERMINAL_PROMPT=0
GIT_ASKPASS=/bin/false
export GIT_TERMINAL_PROMPT GIT_ASKPASS

log() { printf '[unyport-git] %s\n' "$*"; }

safe_remote_url() {
  git -C "$ROOT_DIR" remote get-url origin 2>/dev/null \
    | sed -E 's#(https?://)[^/@]+@#\1***@#'
}

load_env() {
  if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
  fi
}

auth_header() {
  [ -f "$GH_TOKEN_FILE" ] || return 0
  token=$(tr -d '\r\n' < "$GH_TOKEN_FILE")
  [ -n "$token" ] || return 0
  printf '%s' "x-access-token:$token" | base64 | tr -d '\n'
  printf '\n'
}

git_auth() {
  header=$(auth_header || true)
  if [ -n "$header" ]; then
    git -C "$ROOT_DIR" -c credential.helper= -c core.askPass=/bin/false -c "http.extraHeader=Authorization: Basic $header" "$@"
  else
    git -C "$ROOT_DIR" -c credential.helper= -c core.askPass=/bin/false "$@"
  fi
}

release_kind_rank() {
  case "$1" in
    fix) printf '1\n' ;;
    minor) printf '2\n' ;;
    major) printf '3\n' ;;
    *) printf '0\n' ;;
  esac
}

max_kind() {
  if [ "$(release_kind_rank "$2")" -gt "$(release_kind_rank "$1")" ]; then
    printf '%s\n' "$2"
  else
    printf '%s\n' "$1"
  fi
}

detect_release_kind() {
  case "$UNYPORT_GIT_RELEASE_KIND" in
    fix|minor|major) printf '%s\n' "$UNYPORT_GIT_RELEASE_KIND"; return 0 ;;
    auto|'') ;;
    *) log "release kind invalide: $UNYPORT_GIT_RELEASE_KIND"; return 1 ;;
  esac

  kind=fix
  changes=$(git -C "$ROOT_DIR" diff --cached --name-status)
  [ -n "$changes" ] || { printf 'fix\n'; return 0; }

  OLDIFS=$IFS
  IFS='
'
  for line in $changes; do
    IFS='	'
    set -- $line
    IFS=$OLDIFS
    path=${2:-}
    case "$1" in R*|C*) path=${3:-$path} ;; esac
    case "$path" in
      unyport/backend/go.mod|unyport/backend/go.sum|unyport/backend/cmd/*|unyport/backend/server/*|unyport/backend/auth/*|unyport/backend/middleware/*|unyport/backend/sse/*|unyport/backend/proxy/*|docker-compose.yml|scripts/*)
        kind=$(max_kind "$kind" minor)
        ;;
    esac
  done
  IFS=$OLDIFS
  printf '%s\n' "$kind"
}

release_version() {
  if [ -n "$UNYPORT_GIT_RELEASE_VERSION" ]; then
    printf '%s\n' "${UNYPORT_GIT_RELEASE_VERSION#v}"
  else
    "$VERSION_SCRIPT" current
  fi
}

format_change_list() {
  awk '
    BEGIN { labels["A"]="Added"; labels["M"]="Changed"; labels["D"]="Removed"; labels["R"]="Renamed"; labels["C"]="Copied" }
    NF {
      code=substr($1, 1, 1)
      label=(code in labels) ? labels[code] : "Changed"
      if (code == "R" || code == "C") printf "- %s `%s` -> `%s`\n", label, $2, $3
      else printf "- %s `%s`\n", label, $2
    }'
}

commit_body() {
  version=$(release_version)
  printf 'Automated UnyPort git sync.\n\n'
  printf 'Changes in UnyPort v%s:\n\n' "$version"
  git -C "$ROOT_DIR" diff --cached --name-status | format_change_list
  printf '\nStat:\n'
  git -C "$ROOT_DIR" diff --cached --stat || true
}

commit_and_push() {
  git -C "$ROOT_DIR" add -A
  if git -C "$ROOT_DIR" diff --cached --quiet; then
    log "aucune modification a commit"
  else
    kind=$(detect_release_kind)
    if [ -z "$UNYPORT_GIT_RELEASE_VERSION" ] && [ "$UNYPORT_GIT_AUTO_BUMP" = "1" ]; then
      "$VERSION_SCRIPT" bump "$kind" >/dev/null
      git -C "$ROOT_DIR" add -A
    fi
    version=$(release_version)
    git -C "$ROOT_DIR" commit -m "unyport: ${kind} v${version}" -m "$(commit_body)"
  fi

  if [ "$UNYPORT_GIT_SYNC_PUSH" = "1" ]; then
    git_auth push origin "$UNYPORT_GIT_BRANCH"
  fi
}

create_release_tag() {
  version=$(release_version)
  tag="v$version"
  head=$(git -C "$ROOT_DIR" rev-parse HEAD)
  if git -C "$ROOT_DIR" rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1; then
    target=$(git -C "$ROOT_DIR" rev-list -n 1 "$tag")
    [ "$target" = "$head" ] || { log "tag $tag existe deja sur un autre commit"; return 1; }
    log "tag $tag deja present"
  else
    git -C "$ROOT_DIR" tag -a "$tag" -m "unyport: release $tag"
    log "tag $tag cree"
  fi

  if [ "$UNYPORT_GIT_SYNC_PUSH" = "1" ] && [ "$UNYPORT_GIT_RELEASE_PUSH_TAGS" = "1" ]; then
    git_auth push origin "refs/tags/$tag"
  fi
}

run_sync() {
  load_env
  [ "$UNYPORT_GIT_SYNC_ENABLED" = "1" ] || { log "sync git desactive"; return 0; }
  if [ "$UNYPORT_GIT_REQUIRE_BUILD" = "1" ] && [ "$UNYPORT_GIT_BUILD_VERIFIED" != "1" ]; then
    log "build non verifiee: commit/push refuse"
    return 1
  fi
  git_auth fetch origin --prune >/dev/null 2>&1 || true
  commit_and_push
  create_release_tag
}

case "${1:-sync}" in
  sync|all) run_sync ;;
  release)
    load_env
    version=${2:-}
    [ -n "$version" ] || { echo "usage: $0 release <version>" >&2; exit 1; }
    UNYPORT_GIT_RELEASE_VERSION=${version#v}
    export UNYPORT_GIT_RELEASE_VERSION
    "$VERSION_SCRIPT" apply "$UNYPORT_GIT_RELEASE_VERSION"
    UNYPORT_GIT_SYNC_ENABLED=1
    export UNYPORT_GIT_SYNC_ENABLED
    run_sync
    ;;
  status)
    load_env
    printf 'enabled: %s\nbranch: %s\nversion: %s\nremote: %s\n' "$UNYPORT_GIT_SYNC_ENABLED" "$UNYPORT_GIT_BRANCH" "$("$VERSION_SCRIPT" current)" "$(safe_remote_url || true)"
    ;;
  *) echo "usage: $0 [sync|release <version>|status]" >&2; exit 1 ;;
esac
