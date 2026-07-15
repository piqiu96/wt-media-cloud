#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
export GOPATH="${WT_MEDIA_CLOUD_GOPATH:-${GOPATH:-$ROOT_DIR/.cache/go-path}}"
go build -o "${WT_MEDIA_CLOUD_BUILD_OUT:-/tmp/wt-media-cloud-server}" ./cmd/server
npm run build --prefix web
