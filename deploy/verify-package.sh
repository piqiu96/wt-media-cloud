#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

for path in \
  bin/server bin/discovery-scheduler bin/discovery-worker bin/migrate bin/ffmpeg bin/ffprobe \
  deploy/init-config.sh deploy/migrate.sh deploy/verify-package.sh deploy/verify-database.sh \
  deploy/verify-runtime.sh deploy/install.sh deploy/activate.sh deploy/rollback.sh \
  release-info.json web/index.cloud.html deploy/nginx-site-locations.conf.example \
  deploy/config-template/app.toml deploy/config-template/database/primary.toml; do
  if [[ "$path" == *.sh ]]; then
    [[ -f "$path" && -x "$path" ]] || { echo "missing or non-executable: $path" >&2; exit 1; }
  else
    [[ -f "$path" ]] || { echo "missing: $path" >&2; exit 1; }
  fi
done
for directory in migrations deploy/config-template deploy/systemd; do
  [[ -d "$directory" ]] || { echo "missing directory: $directory" >&2; exit 1; }
done
[[ -x bin/server ]] || { echo "bin/server is not executable" >&2; exit 1; }
[[ $(find migrations -type f -name '*.sql' | wc -l | tr -d ' ') -gt 0 ]] || { echo "no SQL migrations" >&2; exit 1; }
[[ -f deploy/config-template/credentials/object_storage.toml.example ]] || { echo "missing object-storage example" >&2; exit 1; }
[[ ! -f deploy/config-template/credentials/object_storage.toml ]] || { echo "private object-storage template must not ship" >&2; exit 1; }
echo "cloud package structure ok: $(find migrations -type f -name '*.sql' | wc -l | tr -d ' ') migrations"
