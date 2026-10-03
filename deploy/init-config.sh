#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'MSG'
Usage: deploy/init-config.sh --config <shared-config-dir> [options]

Required unless WT_MEDIA_DB_PASSWORD / WT_MEDIA_ADMIN_PASSWORD is set or a terminal can prompt:
  --db-password <password>       MySQL password for the Cloud database account
  --admin-password <password>    Initial administrator password

Common defaults:
  --db-host 127.0.0.1 --db-port 3306 --db-name wt_media_cloud
  --db-user wt_media_cloud --http-addr 127.0.0.1:8080
  --admin-username admin

The generated directory is private server configuration. Review credentials and
object-storage files before starting Cloud. Run with --owner <user> when a
non-root service account must read the files.
MSG
}

need_print_usage=0
config_dir=''
db_host=127.0.0.1
db_port=3306
db_name=wt_media_cloud
db_user=wt_media_cloud
db_password="${WT_MEDIA_DB_PASSWORD:-}"
http_addr=127.0.0.1:8080
admin_username=admin
admin_password="${WT_MEDIA_ADMIN_PASSWORD:-}"
owner=''

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config) config_dir=$2; shift 2 ;;
    --db-host) db_host=$2; shift 2 ;;
    --db-port) db_port=$2; shift 2 ;;
    --db-name) db_name=$2; shift 2 ;;
    --db-user) db_user=$2; shift 2 ;;
    --db-password) db_password=$2; shift 2 ;;
    --http-addr) http_addr=$2; shift 2 ;;
    --admin-username) admin_username=$2; shift 2 ;;
    --admin-password) admin_password=$2; shift 2 ;;
    --owner) owner=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; need_print_usage=1; shift ;;
  esac
done
if (( need_print_usage )); then
  usage >&2
  exit 2
fi
if [[ -z "$config_dir" ]]; then
  echo "--config is required" >&2
  usage >&2
  exit 2
fi
if [[ -z "$db_password" ]]; then
  if [[ -t 0 ]]; then
    read -r -s -p "MySQL password for ${db_user}@${db_host}:${db_port}/${db_name}: " db_password
    printf '\n' >&2
  else
    echo "database password is required (WT_MEDIA_DB_PASSWORD, --db-password, or terminal prompt)" >&2
    exit 2
  fi
fi
if [[ -z "$admin_password" ]]; then
  if [[ -t 0 ]]; then
    read -r -s -p "Initial password for administrator ${admin_username}: " admin_password
    printf '\n' >&2
  else
    echo "initial admin password is required (WT_MEDIA_ADMIN_PASSWORD, --admin-password, or terminal prompt)" >&2
    exit 2
  fi
fi
[[ "$db_port" =~ ^[0-9]+$ ]] || { echo "db port must be numeric" >&2; exit 2; }
[[ ${#admin_password} -ge 6 ]] || { echo "initial admin password must be at least 6 characters" >&2; exit 2; }

package_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
template="$package_root/deploy/config-template"
[[ -d "$template" ]] || { echo "missing package template: $template" >&2; exit 1; }
if [[ -e "$config_dir" ]]; then
  [[ -d "$config_dir" && -z "$(ls -A "$config_dir")" ]] || {
    echo "config destination is not empty: $config_dir" >&2
    exit 1
  }
fi

umask 077
mkdir -p "$config_dir"
cp -a "$template/." "$config_dir/"

toml_value() {
  python3 -c 'import json, sys; print(json.dumps(sys.argv[1], ensure_ascii=False))' "$1"
}

cat > "$config_dir/database/primary.toml" <<TOML
name = $(toml_value primary)
charset = 'utf8mb4'
database = $(toml_value "$db_name")
host = $(toml_value "$db_host")
location = 'Local'
parse_time = true
password = $(toml_value "$db_password")
port = $db_port
username = $(toml_value "$db_user")

[pool]
max_idle = 10
max_lifetime = '30m'
max_open = 50
TOML

cat > "$config_dir/app.toml" <<TOML
name = 'wt-media-cloud'

[initial_admin]
username = $(toml_value "$admin_username")
password = $(toml_value "$admin_password")

[server]
http_addr = $(toml_value "$http_addr")
session_cookie_secure = true
TOML

if [[ -n "$owner" ]]; then
  if ! chown -R "$owner" "$config_dir"; then
    echo "could not set config owner: $owner" >&2
    exit 1
  fi
fi
find "$config_dir" -type d -exec chmod 700 {} +
find "$config_dir" -type f -exec chmod 600 {} +
echo "configured=$config_dir admin=$admin_username http_addr=$http_addr"
echo "review credentials/object_storage.toml before first startup"
