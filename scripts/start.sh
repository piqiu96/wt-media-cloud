#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

PID_FILE="${WT_MEDIA_CLOUD_PID_FILE:-$ROOT_DIR/.cache/wt-media-cloud.pid}"
LOG_FILE="${WT_MEDIA_CLOUD_LOG_FILE:-$ROOT_DIR/.cache/wt-media-cloud.log}"
mkdir -p "$(dirname "$PID_FILE")" "$(dirname "$LOG_FILE")"

if [[ -f "$PID_FILE" ]] && kill -0 "$(cat "$PID_FILE")" >/dev/null 2>&1; then
  echo "wt-media-cloud already running: $(cat "$PID_FILE")"
  exit 0
fi

export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
export GOPATH="${WT_MEDIA_CLOUD_GOPATH:-${GOPATH:-$ROOT_DIR/.cache/go-path}}"
nohup go run ./cmd/server >"$LOG_FILE" 2>&1 &
echo "$!" >"$PID_FILE"

ADDR="${WT_MEDIA_CLOUD_HTTP_ADDR:-127.0.0.1:18080}"
for _ in {1..50}; do
  if ! kill -0 "$(cat "$PID_FILE")" >/dev/null 2>&1; then
    echo "wt-media-cloud failed to stay running" >&2
    sed -n '1,120p' "$LOG_FILE" >&2 || true
    rm -f "$PID_FILE"
    exit 1
  fi
  if curl --silent --show-error --fail "http://$ADDR/healthz" >/dev/null 2>&1; then
    echo "wt-media-cloud started: $(cat "$PID_FILE")"
    exit 0
  fi
  sleep 0.1
done

echo "wt-media-cloud did not become healthy" >&2
sed -n '1,120p' "$LOG_FILE" >&2 || true
exit 1
