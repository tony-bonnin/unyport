#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE=${UNYPORT_GIT_SYNC_ENV:-"$ROOT_DIR/unyport-git-sync.env"}
GIT_SYNC_SCRIPT=${GIT_SYNC_SCRIPT:-"$ROOT_DIR/scripts/sync-unyport-git.sh"}
BUILD_SCRIPT=${BUILD_SCRIPT:-"$ROOT_DIR/scripts/build-unyport.sh"}
POLL_INTERVAL=${POLL_INTERVAL:-30}
WATCH_DEBOUNCE=${WATCH_DEBOUNCE:-6}
WATCH_QUIET_CHECKS=${WATCH_QUIET_CHECKS:-3}
PID_FILE=${PID_FILE:-/tmp/unyport-watch.pid}
LOG_FILE=${LOG_FILE:-/tmp/unyport-watch.log}
BUILD_LOCK_DIR=${BUILD_LOCK_DIR:-/tmp/unyport-build.lock}
WATCH_STATE_DIR=${WATCH_STATE_DIR:-/tmp/unyport-watch-state}
MODE=${1:-watch}

load_env() {
  if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
  fi
}

watch_fingerprint() {
  find "$ROOT_DIR" \
    -path "$ROOT_DIR/.git" -prune -o \
    -path "$ROOT_DIR/dist" -prune -o \
    -path "$ROOT_DIR/unyport/backend/tmp" -prune -o \
    -path "$ROOT_DIR/unyport/backend/logs" -prune -o \
    -path "$ROOT_DIR/unyport/backend/server/assets" -prune -o \
    -type f -exec stat -c '%Y:%s:%n' {} + \
    | sort \
    | sha256sum \
    | awk '{print $1}'
}

ensure_watch_state() {
  mkdir -p "$WATCH_STATE_DIR"
  [ -f "$WATCH_STATE_DIR/tree.fp" ] || watch_fingerprint > "$WATCH_STATE_DIR/tree.fp"
}

tree_changed() {
  ensure_watch_state
  old=$(cat "$WATCH_STATE_DIR/tree.fp")
  new=$(watch_fingerprint)
  [ "$old" != "$new" ]
}

mark_clean() {
  ensure_watch_state
  watch_fingerprint > "$WATCH_STATE_DIR/tree.fp"
}

wait_for_quiet_tree() {
  previous=$(watch_fingerprint)
  quiet=0
  while [ "$quiet" -lt "$WATCH_QUIET_CHECKS" ]; do
    sleep "$WATCH_DEBOUNCE"
    current=$(watch_fingerprint)
    if [ "$previous" = "$current" ]; then
      quiet=$((quiet + 1))
    else
      quiet=0
      previous=$current
    fi
  done
}

pid_is_running() {
  [ -f "$PID_FILE" ] || return 1
  pid=$(cat "$PID_FILE" 2>/dev/null || true)
  [ -n "${pid:-}" ] && kill -0 "$pid" 2>/dev/null
}

acquire_build_lock() {
  if mkdir "$BUILD_LOCK_DIR" 2>/dev/null; then
    echo "$$" > "$BUILD_LOCK_DIR/pid"
    return 0
  fi
  lock_pid=$(cat "$BUILD_LOCK_DIR/pid" 2>/dev/null || true)
  [ -n "${lock_pid:-}" ] && kill -0 "$lock_pid" 2>/dev/null && return 1
  rm -rf "$BUILD_LOCK_DIR"
  mkdir "$BUILD_LOCK_DIR"
  echo "$$" > "$BUILD_LOCK_DIR/pid"
}

release_build_lock() {
  [ -d "$BUILD_LOCK_DIR" ] && rm -rf "$BUILD_LOCK_DIR"
}

sync_git() {
  [ -x "$GIT_SYNC_SCRIPT" ] || { echo "[unyport] git sync script absent: $GIT_SYNC_SCRIPT"; return 0; }
  UNYPORT_GIT_BUILD_VERIFIED=1 "$GIT_SYNC_SCRIPT" sync
}

build_and_sync() {
  if ! acquire_build_lock; then
    echo "[unyport] build deja en cours"
    return 0
  fi
  rc=0
  load_env
  if "$BUILD_SCRIPT" && sync_git; then
    mark_clean
  else
    rc=$?
  fi
  release_build_lock
  return "$rc"
}

start_daemon() {
  load_env
  if pid_is_running; then
    echo "[unyport] watcher deja actif pid $(cat "$PID_FILE")"
    return 0
  fi
  setsid "$ROOT_DIR/watch-unyport.sh" watch >>"$LOG_FILE" 2>&1 </dev/null &
  echo "$!" > "$PID_FILE"
}

stop_daemon() {
  if pid_is_running; then
    pid=$(cat "$PID_FILE")
    kill -TERM "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
  fi
  rm -f "$PID_FILE"
}

watch_loop() {
  load_env
  trap 'release_build_lock; exit 143' INT TERM HUP
  ensure_watch_state
  echo "[unyport] watching $ROOT_DIR every ${POLL_INTERVAL}s"
  while true; do
    if tree_changed; then
      echo "[unyport] changement detecte"
      if wait_for_quiet_tree; then
        build_and_sync || echo "[unyport] build/sync echoue"
      fi
    fi
    sleep "$POLL_INTERVAL"
  done
}

case "$MODE" in
  start) start_daemon ;;
  stop) stop_daemon ;;
  status) pid_is_running && echo "[unyport] watcher actif pid $(cat "$PID_FILE")" || echo "[unyport] watcher inactif" ;;
  once) build_and_sync ;;
  watch) watch_loop ;;
  *) echo "usage: $0 [start|stop|status|once|watch]" >&2; exit 1 ;;
esac

