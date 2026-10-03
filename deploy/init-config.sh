#!/usr/bin/env bash
set -euo pipefail

package_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
config_dir="$package_root/config"
environment=''
variables_file=''
variables_url=''
owner=www
validator="$package_root/bin/config-check"

usage() {
  cat <<'MSG'
Usage: deploy/init-config.sh --environment pre|online \
  (--variables-file <json> | --variables-url <https-url>) [options]

Options:
  --config <dir>       Template-state config directory (default: package config/)
  --owner <user>       Owner for rendered config (default: www)
  --validator <path>   Cloud config-check binary (default: package bin/config-check)

For authenticated HTTPS downloads, export WT_CONFIG_BEARER_TOKEN. The JSON
document is parsed as data and is never executed as shell code. Rendering and
Cloud validation finish in a temporary directory before config/ is replaced.
MSG
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) config_dir=$2; shift 2 ;;
    --environment) environment=$2; shift 2 ;;
    --variables-file) variables_file=$2; shift 2 ;;
    --variables-url) variables_url=$2; shift 2 ;;
    --owner) owner=$2; shift 2 ;;
    --validator) validator=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[[ "$environment" == pre || "$environment" == online ]] || {
  echo "--environment must be pre or online" >&2
  exit 2
}
if [[ -n "$variables_file" && -n "$variables_url" ]] || [[ -z "$variables_file" && -z "$variables_url" ]]; then
  echo "exactly one of --variables-file or --variables-url is required" >&2
  exit 2
fi
id "$owner" >/dev/null 2>&1 || { echo "config owner does not exist: $owner" >&2; exit 1; }
[[ -d "$config_dir" ]] || { echo "config template directory does not exist: $config_dir" >&2; exit 1; }
[[ -x "$validator" ]] || { echo "config validator is missing or not executable: $validator" >&2; exit 1; }

command=(
  python3 "$package_root/deploy/render-config.py"
  --config "$config_dir"
  --environment "$environment"
  --validator "$validator"
)
if [[ -n "$variables_file" ]]; then
  command+=(--variables-file "$variables_file")
else
  command+=(--variables-url "$variables_url")
fi
"${command[@]}"

owner_group="$(id -gn "$owner")"
chown -R "$owner:$owner_group" "$config_dir"
find "$config_dir" -type d -exec chmod 700 {} +
find "$config_dir" -type f -exec chmod 600 {} +
echo "configured=$config_dir environment=$environment owner=$owner:$owner_group"
