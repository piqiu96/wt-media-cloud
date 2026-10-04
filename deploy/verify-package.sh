#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

required_files=(
  bin/server bin/discovery-scheduler bin/discovery-worker bin/migrate bin/config-check bin/ffmpeg bin/ffprobe
  deploy/init-config.sh deploy/render-config.py deploy/install.sh deploy/activate.sh deploy/rollback.sh
  deploy/migrate.sh deploy/verify-package.sh deploy/verify-database.sh deploy/verify-runtime.sh
  deploy/prepare-database.sql.example deploy/DEPLOYMENT.md
  release-info.json web/index.cloud.html
  config/app.toml config/clients/http/agent.toml config/clients/http/douyin.toml
  config/database/primary.toml.tpl
  config/credentials/object_storage.toml.tpl config/storage/object_storage.toml.tpl
)
for path in "${required_files[@]}"; do
  [[ -f "$path" ]] || { echo "missing: $path" >&2; exit 1; }
  if [[ "$path" == *.sh || "$path" == *.py ]]; then
    [[ -x "$path" ]] || { echo "not executable: $path" >&2; exit 1; }
  fi
done
for path in bin/*; do
  [[ -x "$path" ]] || { echo "not executable: $path" >&2; exit 1; }
done
for directory in migrations config/logger config/clients/http config/scheduler; do
  [[ -d "$directory" ]] || { echo "missing directory: $directory" >&2; exit 1; }
done
for forbidden in config_online config_test deploy/config-template deploy/systemd deploy/nginx-site-locations.conf.example config/.render-info.json; do
  [[ ! -e "$forbidden" ]] || { echo "forbidden package path exists: $forbidden" >&2; exit 1; }
done
[[ $(find migrations -type f -name '*.sql' | wc -l | tr -d ' ') -gt 0 ]] || {
  echo "no SQL migrations" >&2
  exit 1
}
[[ $(find config -type f -name '*.toml.tpl' | wc -l | tr -d ' ') -gt 0 ]] || {
  echo "no TOML templates" >&2
  exit 1
}
echo "cloud package structure ok: $(find migrations -type f -name '*.sql' | wc -l | tr -d ' ') migrations"
