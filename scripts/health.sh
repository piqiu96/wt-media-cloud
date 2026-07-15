#!/usr/bin/env bash
set -euo pipefail

ADDR="${WT_MEDIA_CLOUD_HTTP_ADDR:-127.0.0.1:18080}"
curl --silent --show-error --fail "http://$ADDR/healthz" >/dev/null
curl --silent --show-error --fail "http://$ADDR/api/v1/health" >/dev/null
echo "wt-media-cloud health ok"
