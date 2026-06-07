#!/bin/sh

set -eu

CONFIG_ROOT="${CONFIG_ROOT:-/root/.config}"
GITHUB_TOKEN_FILE="${GITHUB_TOKEN_FILE:-$CONFIG_ROOT/gh/unyport-token}"
GITLAB_TOKEN_FILE="${GITLAB_TOKEN_FILE:-$CONFIG_ROOT/gl/unyport-token}"
CODEBERG_TOKEN_FILE="${CODEBERG_TOKEN_FILE:-$CONFIG_ROOT/cb/unyport-token}"

GITHUB_REPO="${GITHUB_REPO:-tony-bonnin/unyport}"
GITLAB_PROJECT="${GITLAB_PROJECT:-trinity-labs%2Funyport}"
CODEBERG_REPO="${CODEBERG_REPO:-tony-bonnin/unyport}"
VERSION_FILES="${VERSION_FILES:-unyport/backend/config/version.go ../docker_demo/unyport/backend/config/version.go}"

PUSH_TAG="${PUSH_TAG:-1}"
FORCE_TAG="${FORCE_TAG:-0}"
PUSH_BRANCH="${PUSH_BRANCH:-1}"
BRANCH_NAME="${BRANCH_NAME:-master}"
COMMIT_SIGN="${COMMIT_SIGN:-1}"
TAG_SIGN="${TAG_SIGN:-1}"

usage() {
  cat <<EOF
Usage: $0 <tag> [release-notes-file]

Examples:
  $0 v0.1.0
  $0 v0.1.0 RELEASE-v0.1.0.md

Environment overrides:
  PUSH_BRANCH=1
  PUSH_TAG=1
  FORCE_TAG=0
  BRANCH_NAME=master
  COMMIT_SIGN=1
  TAG_SIGN=1
  CONFIG_ROOT
  GITHUB_TOKEN_FILE
  GITLAB_TOKEN_FILE
  CODEBERG_TOKEN_FILE
  GITHUB_REPO
  GITLAB_PROJECT
  CODEBERG_REPO
  VERSION_FILES
EOF
}

require_file() {
  if [ ! -f "$1" ]; then
    echo "Missing file: $1" >&2
    exit 1
  fi
}

require_tag() {
  if ! git rev-parse "$1" >/dev/null 2>&1; then
    echo "Missing git ref: $1" >&2
    exit 1
  fi
}

tag_exists() {
  git rev-parse "$1" >/dev/null 2>&1
}

is_inside_repo() {
  git rev-parse --path-format=absolute --show-toplevel >/dev/null 2>&1
  repo_root="$(git rev-parse --path-format=absolute --show-toplevel)"
  target_path="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
  case "$target_path" in
    "$repo_root"/*) return 0 ;;
    *) return 1 ;;
  esac
}

json_escape() {
  awk '
    BEGIN {
      first = 1
    }
    {
      gsub(/\r/, "", $0)
      gsub(/\\/, "\\\\", $0)
      gsub(/"/, "\\\"", $0)
      if (!first) {
        printf "\\n"
      }
      printf "%s", $0
      first = 0
    }
  '
}

version_from_tag() {
  printf '%s' "$1" | sed 's/^v//'
}

update_version_file() {
  version_value="$(version_from_tag "$TAG")"
  for version_file in $VERSION_FILES; do
    require_file "$version_file"

    if ! grep -q 'const Version = ' "$version_file"; then
      echo "Version constant not found in $version_file" >&2
      exit 1
    fi

    tmp_file="$(mktemp)"
    sed "s/const Version = \".*\"/const Version = \"$version_value\"/" "$version_file" > "$tmp_file"
    mv "$tmp_file" "$version_file"
    echo "Updated app version in $version_file to $version_value."
  done
}

commit_release_changes() {
  version_value="$(version_from_tag "$TAG")"
  staged_files="$NOTES_FILE"
  external_files=""

  for version_file in $VERSION_FILES; do
    if is_inside_repo "$version_file"; then
      staged_files="$staged_files $version_file"
    else
      external_files="$external_files $version_file"
    fi
  done

  git add $staged_files

  if git diff --cached --quiet; then
    echo "No staged changes to commit for release $version_value."
  else
    echo "Creating release commit for $version_value..."
    if [ "$COMMIT_SIGN" = "1" ]; then
      git commit -S -m "Release $TAG"
    else
      git commit -m "Release $TAG"
    fi
  fi

  if [ -n "$external_files" ]; then
    echo "Updated outside current repo but not committed here:$external_files"
  fi
}

ensure_tag() {
  if tag_exists "$TAG"; then
    echo "Tag $TAG already exists locally."
    return
  fi

  echo "Creating tag $TAG..."
  if [ "$TAG_SIGN" = "1" ]; then
    git tag -s "$TAG" -m "UnyPort $TAG"
  else
    git tag -a "$TAG" -m "UnyPort $TAG"
  fi
}

push_branch() {
  if [ "$PUSH_BRANCH" != "1" ]; then
    return
  fi

  echo "Pushing branch $BRANCH_NAME to origin..."
  git push origin "$BRANCH_NAME"
}

http_json() {
  method="$1"
  url="$2"
  auth_header="$3"
  content_type="$4"
  payload="${5:-}"

  response_file="$(mktemp)"
  if [ -n "$payload" ]; then
    http_code="$(curl --silent --show-error \
      -X "$method" "$url" \
      -H "$auth_header" \
      -H "$content_type" \
      -o "$response_file" \
      -w "%{http_code}" \
      -d "$payload")"
  else
    http_code="$(curl --silent --show-error \
      -X "$method" "$url" \
      -H "$auth_header" \
      -o "$response_file" \
      -w "%{http_code}")"
  fi

  printf '%s\n%s\n' "$http_code" "$response_file"
}

cleanup_response() {
  rm -f "$1"
}

response_has_text() {
  pattern="$1"
  file="$2"
  grep -q "$pattern" "$file"
}

extract_json_number() {
  key="$1"
  file="$2"
  grep -o "\"$key\":[[:space:]]*[0-9][0-9]*" "$file" | head -n 1 | sed "s/\"$key\":[[:space:]]*//"
}

push_tag() {
  if [ "$PUSH_TAG" != "1" ]; then
    return
  fi

  if [ "$FORCE_TAG" = "1" ]; then
    echo "Pushing tag $TAG to origin with --force..."
    git push origin --force "$TAG"
  else
    echo "Pushing tag $TAG to origin..."
    git push origin "$TAG"
  fi
}

github_upsert_release() {
  token="$(cat "$GITHUB_TOKEN_FILE")"
  get_result="$(http_json GET "https://api.github.com/repos/$GITHUB_REPO/releases/tags/$TAG" "Accept: application/vnd.github+json" "Authorization: Bearer $token")"
  github_code="$(printf '%s\n' "$get_result" | sed -n '1p')"
  github_file="$(printf '%s\n' "$get_result" | sed -n '2p')"

  payload="$(cat <<EOF
{
  "tag_name": "$TAG",
  "target_commitish": "master",
  "name": "UnyPort $TAG",
  "body": "$BODY_JSON",
  "draft": false,
  "prerelease": false
}
EOF
)"

  if [ "$github_code" = "200" ]; then
    release_id="$(extract_json_number id "$github_file")"
    cleanup_response "$github_file"
    if [ -z "$release_id" ]; then
      echo "GitHub release lookup succeeded but release id was not found." >&2
      exit 1
    fi
    echo "Updating existing GitHub release for $TAG..."
    patch_result="$(http_json PATCH "https://api.github.com/repos/$GITHUB_REPO/releases/$release_id" "Authorization: Bearer $token" "Content-Type: application/json" "$payload")"
    patch_code="$(printf '%s\n' "$patch_result" | sed -n '1p')"
    patch_file="$(printf '%s\n' "$patch_result" | sed -n '2p')"
    if [ "$patch_code" -lt 200 ] || [ "$patch_code" -ge 300 ]; then
      echo "GitHub release update failed (HTTP $patch_code):" >&2
      cat "$patch_file" >&2
      cleanup_response "$patch_file"
      exit 1
    fi
    cleanup_response "$patch_file"
    return
  fi

  if [ "$github_code" != "404" ]; then
    echo "GitHub release lookup failed (HTTP $github_code):" >&2
    cat "$github_file" >&2
    cleanup_response "$github_file"
    exit 1
  fi

  cleanup_response "$github_file"
  echo "Creating GitHub release for $TAG..."
  post_result="$(http_json POST "https://api.github.com/repos/$GITHUB_REPO/releases" "Authorization: Bearer $token" "Content-Type: application/json" "$payload")"
  post_code="$(printf '%s\n' "$post_result" | sed -n '1p')"
  post_file="$(printf '%s\n' "$post_result" | sed -n '2p')"
  if [ "$post_code" -lt 200 ] || [ "$post_code" -ge 300 ]; then
    echo "GitHub release creation failed (HTTP $post_code):" >&2
    cat "$post_file" >&2
    cleanup_response "$post_file"
    exit 1
  fi
  cleanup_response "$post_file"
}

gitlab_upsert_release() {
  token="$(cat "$GITLAB_TOKEN_FILE")"
  get_result="$(http_json GET "https://gitlab.alpinelinux.org/api/v4/projects/$GITLAB_PROJECT/releases/$TAG" "PRIVATE-TOKEN: $token" "Content-Type: application/json")"
  gitlab_code="$(printf '%s\n' "$get_result" | sed -n '1p')"
  gitlab_file="$(printf '%s\n' "$get_result" | sed -n '2p')"

  payload="$(cat <<EOF
{
  "name": "UnyPort $TAG",
  "tag_name": "$TAG",
  "description": "$BODY_JSON"
}
EOF
)"

  if [ "$gitlab_code" = "200" ]; then
    cleanup_response "$gitlab_file"
    echo "Updating existing GitLab release for $TAG..."
    put_result="$(http_json PUT "https://gitlab.alpinelinux.org/api/v4/projects/$GITLAB_PROJECT/releases/$TAG" "PRIVATE-TOKEN: $token" "Content-Type: application/json" "$payload")"
    put_code="$(printf '%s\n' "$put_result" | sed -n '1p')"
    put_file="$(printf '%s\n' "$put_result" | sed -n '2p')"
    if [ "$put_code" -lt 200 ] || [ "$put_code" -ge 300 ]; then
      echo "GitLab release update failed (HTTP $put_code):" >&2
      cat "$put_file" >&2
      cleanup_response "$put_file"
      exit 1
    fi
    cleanup_response "$put_file"
    return
  fi

  if [ "$gitlab_code" != "404" ]; then
    echo "GitLab release lookup failed (HTTP $gitlab_code):" >&2
    cat "$gitlab_file" >&2
    cleanup_response "$gitlab_file"
    exit 1
  fi

  cleanup_response "$gitlab_file"
  echo "Creating GitLab release for $TAG..."
  post_result="$(http_json POST "https://gitlab.alpinelinux.org/api/v4/projects/$GITLAB_PROJECT/releases" "PRIVATE-TOKEN: $token" "Content-Type: application/json" "$payload")"
  post_code="$(printf '%s\n' "$post_result" | sed -n '1p')"
  post_file="$(printf '%s\n' "$post_result" | sed -n '2p')"
  if [ "$post_code" -lt 200 ] || [ "$post_code" -ge 300 ]; then
    echo "GitLab release creation failed (HTTP $post_code):" >&2
    cat "$post_file" >&2
    cleanup_response "$post_file"
    exit 1
  fi
  cleanup_response "$post_file"
}

codeberg_upsert_release() {
  token="$(cat "$CODEBERG_TOKEN_FILE")"
  repo_result="$(http_json GET "https://codeberg.org/api/v1/repos/$CODEBERG_REPO" "Authorization: token $token" "Content-Type: application/json")"
  repo_code="$(printf '%s\n' "$repo_result" | sed -n '1p')"
  repo_file="$(printf '%s\n' "$repo_result" | sed -n '2p')"

  if [ "$repo_code" -lt 200 ] || [ "$repo_code" -ge 300 ]; then
    echo "Codeberg repo lookup failed (HTTP $repo_code):" >&2
    cat "$repo_file" >&2
    cleanup_response "$repo_file"
    exit 1
  fi

  if response_has_text '"has_releases":false' "$repo_file"; then
    echo "Skipping Codeberg release: releases are disabled for $CODEBERG_REPO."
    cleanup_response "$repo_file"
    return
  fi

  cleanup_response "$repo_file"

  get_result="$(http_json GET "https://codeberg.org/api/v1/repos/$CODEBERG_REPO/releases/tags/$TAG" "Authorization: token $token" "Content-Type: application/json")"
  codeberg_code="$(printf '%s\n' "$get_result" | sed -n '1p')"
  codeberg_file="$(printf '%s\n' "$get_result" | sed -n '2p')"

  payload="$(cat <<EOF
{
  "tag_name": "$TAG",
  "target_commitish": "master",
  "name": "UnyPort $TAG",
  "body": "$BODY_JSON",
  "draft": false,
  "prerelease": false
}
EOF
)"

  if [ "$codeberg_code" = "200" ]; then
    release_id="$(extract_json_number id "$codeberg_file")"
    cleanup_response "$codeberg_file"
    if [ -z "$release_id" ]; then
      echo "Codeberg release lookup succeeded but release id was not found." >&2
      exit 1
    fi
    echo "Updating existing Codeberg release for $TAG..."
    patch_result="$(http_json PATCH "https://codeberg.org/api/v1/repos/$CODEBERG_REPO/releases/$release_id" "Authorization: token $token" "Content-Type: application/json" "$payload")"
    patch_code="$(printf '%s\n' "$patch_result" | sed -n '1p')"
    patch_file="$(printf '%s\n' "$patch_result" | sed -n '2p')"
    if [ "$patch_code" -lt 200 ] || [ "$patch_code" -ge 300 ]; then
      echo "Codeberg release update failed (HTTP $patch_code):" >&2
      cat "$patch_file" >&2
      cleanup_response "$patch_file"
      exit 1
    fi
    cleanup_response "$patch_file"
    return
  fi

  if [ "$codeberg_code" != "404" ]; then
    echo "Codeberg release lookup failed (HTTP $codeberg_code):" >&2
    cat "$codeberg_file" >&2
    cleanup_response "$codeberg_file"
    exit 1
  fi

  cleanup_response "$codeberg_file"
  echo "Creating Codeberg release for $TAG..."
  post_result="$(http_json POST "https://codeberg.org/api/v1/repos/$CODEBERG_REPO/releases" "Authorization: token $token" "Content-Type: application/json" "$payload")"
  post_code="$(printf '%s\n' "$post_result" | sed -n '1p')"
  post_file="$(printf '%s\n' "$post_result" | sed -n '2p')"
  if [ "$post_code" -lt 200 ] || [ "$post_code" -ge 300 ]; then
    echo "Codeberg release creation failed (HTTP $post_code):" >&2
    cat "$post_file" >&2
    cleanup_response "$post_file"
    exit 1
  fi
  cleanup_response "$post_file"
}

if [ "${1:-}" = "" ] || [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

TAG="$1"
NOTES_FILE="${2:-RELEASE-$TAG.md}"

require_file "$NOTES_FILE"
require_file "$GITHUB_TOKEN_FILE"
require_file "$GITLAB_TOKEN_FILE"
require_file "$CODEBERG_TOKEN_FILE"

update_version_file
commit_release_changes
ensure_tag
require_tag "$TAG"
BODY_JSON="$(json_escape < "$NOTES_FILE")"

push_branch
push_tag
github_upsert_release
gitlab_upsert_release
codeberg_upsert_release
echo "Release flow completed for $TAG."
