#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Linux" || "$(uname -m)" != "x86_64" ]]; then
  echo "Cloud release build requires a Linux amd64 runner" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"
mkdir -p bin
for name in server discovery-scheduler discovery-worker migrate; do
  # Use the native Linux toolchain, matching the Cloud CI path. Forcing
  # CGO_ENABLED=0 trips sonic/loader's Go 1.26 linkname check on Linux even
  # though the same source builds successfully with the runner default.
  GOOS=linux GOARCH=amd64 go build -trimpath -o "bin/$name" "./cmd/$name"
  test -x "bin/$name"
done

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
