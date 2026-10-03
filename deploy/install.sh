#!/usr/bin/env bash
set -euo pipefail

install_root=/www/wt-media-cloud
release=''
service_user=www

usage() {
  cat <<'MSG'
Usage: deploy/install.sh --install-root <dir> --release <tag> [--service-user <user>]

Run this from the extracted release directory. It copies that immutable release
to <install-root>/releases/<tag> and creates that version's private logs and
temporary-data directories. The packaged config/ remains in template state until
init-config.sh renders it. This script does not activate or overwrite a release.
MSG
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-root) install_root=$2; shift 2 ;;
    --release) release=$2; shift 2 ;;
    --service-user) service_user=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

source_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
[[ -n "$release" ]] || { echo "--release is required" >&2; exit 2; }
[[ ! "$release" =~ [/[:space:]] ]] || { echo "invalid release name: $release" >&2; exit 1; }
[[ "$release" != "current" && "$release" != "shared" ]] || { echo "reserved release name: $release" >&2; exit 1; }
[[ -f "$source_root/release-info.json" ]] || { echo "release-info.json is required" >&2; exit 1; }
python3 - "$source_root/release-info.json" "$release" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as source:
    actual = json.load(source)["product_tag"]
if actual != sys.argv[2]:
    sys.exit(f"release Tag mismatch: package={actual} requested={sys.argv[2]}")
PY
id "$service_user" >/dev/null 2>&1 || { echo "service user does not exist: $service_user" >&2; exit 1; }
[[ -d "$source_root/config" && -n "$(find "$source_root/config" -type f -name '*.toml.tpl' -print -quit)" ]] || {
  echo "release config templates are missing" >&2
  exit 1
}
[[ ! -e "$source_root/logs" && ! -e "$source_root/data" ]] || {
  echo "release contains unexpected logs or data directory" >&2
  exit 1
}

destination="$install_root/releases/$release"
if [[ -e "$destination" ]]; then
  echo "release already exists: $destination" >&2
  exit 1
fi
mkdir -p "$install_root/releases"
cp -a "$source_root/." "$destination/"
mkdir -p "$destination/logs" "$destination/data/tmp"
service_group="$(id -gn "$service_user")"
chown -R "$service_user:$service_group" "$destination/config" "$destination/logs" "$destination/data"
find "$destination/config" -type d -exec chmod 700 {} +
find "$destination/config" -type f -exec chmod 600 {} +
chmod 750 "$destination/logs" "$destination/data" "$destination/data/tmp"
find "$destination/bin" -type f -exec chmod 755 {} +
find "$destination/deploy" -type f -name '*.sh' -exec chmod 755 {} +
chmod 755 "$destination/deploy/render-config.py"
echo "installed=$destination"
echo "current was not changed; render config, migrate, and verify before deploy/activate.sh"
