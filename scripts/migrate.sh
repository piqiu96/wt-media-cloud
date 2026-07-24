#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

source "$ROOT_DIR/scripts/local-env.sh"

export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
export GOPATH="${WT_MEDIA_CLOUD_GOPATH:-${GOPATH:-$ROOT_DIR/.cache/go-path}}"
go run ./cmd/migrate -dir migrations "$@"
