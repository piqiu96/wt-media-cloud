#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Linux" || "$(uname -m)" != "x86_64" ]]; then
  echo "Cloud release build requires a Linux amd64 runner" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"
mkdir -p bin
for name in discovery-scheduler discovery-worker migrate config-check wtmctl; do
  # This runs only on a native Linux amd64 runner. Keep the Go invocation the
  # same as the tested Cloud build path: linker flags can change how sonic's
  # runtime linkname is treated by the current Go toolchain.
  go build -o "bin/$name" "./cmd/$name"
  test -x "bin/$name"
done
# The HTTP entrypoint lives in cmd/server but the released binary is named
# wt-media-cloud so it is not confused with a generic "server" process.
go build -o "bin/wt-media-cloud" "./cmd/server"
test -x "bin/wt-media-cloud"

npm ci --prefix web --no-audit --no-fund
npm run build:cloud --prefix web
npm run build:desktop --prefix web
test -s web/dist-cloud/index.cloud.html
test -s web/dist-desktop/index.desktop.html
if rg -q '127\.0\.0\.1:18080' web/dist-desktop; then
  echo "packaged Desktop Web still contains a hard-coded local Cloud address" >&2
  exit 1
fi
cp web/dist-desktop/index.desktop.html web/dist-desktop/index.html
python3 scripts/dev/stamp_desktop_web.py --source-commit "$(git rev-parse --short HEAD)"
