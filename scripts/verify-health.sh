#!/usr/bin/env sh
set -eu

GO_BIN="${GO_BIN:-go}"
ADDR="${WT_MEDIA_CLOUD_HTTP_ADDR:-127.0.0.1:18080}"
GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}"
export WT_MEDIA_CLOUD_HTTP_ADDR="$ADDR"
export GOCACHE

"$GO_BIN" test ./...

"$GO_BIN" run ./cmd/server &
server_pid=$!

cleanup() {
  kill "$server_pid" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

for _ in 1 2 3 4 5 6 7 8 9 10; do
  if curl --silent --show-error --fail "http://$ADDR/healthz" >/dev/null 2>&1; then
    curl --silent --show-error --fail "http://$ADDR/api/v1/health" >/dev/null
    echo "wt-media-cloud health ok"
    exit 0
  fi
  sleep 1
done

echo "wt-media-cloud health failed" >&2
exit 1
