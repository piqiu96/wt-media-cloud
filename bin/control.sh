#!/usr/bin/env bash
#
# The one entry point for the Cloud process in local development: start, stop,
# restart and report on it. It merges what `scripts/start.sh`, `scripts/stop.sh`
# and `scripts/health.sh` used to do, unchanged.
#
# No numeric runtime parameter lives in this file. The probe address is
# `WT_MEDIA_CLOUD_HTTP_ADDR`, whose landing point is `scripts/local-env.sh`
# below (and whose reference value for the server itself is `config/app.toml`);
# spelling it out here would be a second source of truth for the same port.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
ROOT_DIR="$(pwd)"

# shellcheck source=../scripts/local-env.sh
source "$ROOT_DIR/scripts/local-env.sh"

PID_FILE="${WT_MEDIA_CLOUD_PID_FILE:-$ROOT_DIR/.cache/wt-media-cloud.pid}"
LOG_FILE="${WT_MEDIA_CLOUD_LOG_FILE:-$ROOT_DIR/.cache/wt-media-cloud.log}"
BINARY_FILE="${WT_MEDIA_CLOUD_BINARY_FILE:-$ROOT_DIR/.cache/wt-media-cloud-server}"
ADDR="$WT_MEDIA_CLOUD_HTTP_ADDR"

usage() {
  cat <<'EOF'
Usage: bin/control.sh <start|stop|restart|status|help>

  start    Build ./cmd/server, run it in the background, and wait for health.
  stop     Stop the process recorded by the PID file.
  restart  Stop it, then start it again.
  status   Report whether that process is alive and whether Cloud answers.
  help     Show this help.
EOF
}

probe_health() {
  curl --silent --show-error --fail "http://$ADDR/healthz" >/dev/null 2>&1 &&
    curl --silent --show-error --fail "http://$ADDR/api/v1/health" >/dev/null 2>&1
}

running_pid() {
  [[ -f "$PID_FILE" ]] || return 1
  local pid
  pid="$(cat "$PID_FILE")"
  [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1 || return 1
  printf '%s' "$pid"
}

do_start() {
  local pid
  if pid="$(running_pid)"; then
    echo "wt-media-cloud already running: $pid"
    return 0
  fi

  mkdir -p "$(dirname "$PID_FILE")" "$(dirname "$LOG_FILE")"
  export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
  export GOPATH="${WT_MEDIA_CLOUD_GOPATH:-${GOPATH:-$ROOT_DIR/.cache/go-path}}"
  go build -o "$BINARY_FILE" ./cmd/server
  nohup "$BINARY_FILE" >"$LOG_FILE" 2>&1 &
  echo "$!" > "$PID_FILE"

  for _ in {1..50}; do
    if ! kill -0 "$(cat "$PID_FILE")" >/dev/null 2>&1; then
      echo "wt-media-cloud failed to stay running" >&2
      sed -n '1,120p' "$LOG_FILE" >&2 || true
      rm -f "$PID_FILE"
      return 1
    fi
    if probe_health; then
      echo "wt-media-cloud started: $(cat "$PID_FILE")"
      return 0
    fi
    sleep 0.1
  done

  echo "wt-media-cloud did not become healthy" >&2
  sed -n '1,120p' "$LOG_FILE" >&2 || true
  return 1
}

do_stop() {
  local pid
  if [[ ! -f "$PID_FILE" ]]; then
    echo "wt-media-cloud not running"
    return 0
  fi
  pid="$(cat "$PID_FILE")"
  if kill -0 "$pid" >/dev/null 2>&1; then
    kill "$pid"
  fi
  rm -f "$PID_FILE"
  echo "wt-media-cloud stopped"
}

do_status() {
  local pid alive=no health=down
  pid="$(running_pid || true)"
  [[ -n "$pid" ]] && alive=yes
  probe_health && health=ok
  # `alive` and `health` answer different questions and are printed apart: the
  # PID file says whether this script started the process, the probe says
  # whether anything is answering. exit=0 needs both; exit=1 most often means
  # "Cloud is up, but not from this harness" -- a stale PID file plus a live
  # server reads alive=no health=ok.
  echo "cloud: pid=${pid:--} alive=$alive health=$health url=http://$ADDR/healthz"
  [[ "$alive" == yes && "$health" == ok ]]
}

case "${1:-help}" in
  start)   do_start ;;
  stop)    do_stop ;;
  restart) do_stop && do_start ;;
  status)  do_status ;;
  help|-h|--help) usage ;;
  *) echo "unknown command: $1" >&2; usage >&2; exit 2 ;;
esac
