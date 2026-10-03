#!/usr/bin/env bash
set -euo pipefail

db_host=127.0.0.1
db_port=3306
db_name=wt_media_cloud
db_user=wt_media_cloud
db_password="${WT_MEDIA_DB_PASSWORD:-}"

usage() {
  cat <<'MSG'
Usage: deploy/verify-database.sh [--host H] [--port P] [--database D] [--user U] [--password P]

Checks the migration ledger and the initial enabled admin account with the
mysql CLI. The password can also be supplied through WT_MEDIA_DB_PASSWORD.
MSG
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) db_host=$2; shift 2 ;;
    --port) db_port=$2; shift 2 ;;
    --database) db_name=$2; shift 2 ;;
    --user) db_user=$2; shift 2 ;;
    --password) db_password=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
command -v mysql >/dev/null 2>&1 || { echo "mysql CLI is required" >&2; exit 1; }
if [[ -z "$db_password" ]]; then
  if [[ -t 0 ]]; then
    read -r -s -p "MySQL password for ${db_user}@${db_host}:${db_port}/${db_name}: " db_password
    printf '\n' >&2
  else
    echo "database password is required" >&2
    exit 2
  fi
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
expected=$(find "$root/migrations" -type f -name '*.sql' | wc -l | tr -d ' ')
query() { MYSQL_PWD="$db_password" mysql --batch --skip-column-names -h"$db_host" -P"$db_port" -u"$db_user" "$db_name" -e "$1"; }
actual=$(query 'SELECT COUNT(*) FROM schema_migrations')
admin=$(query "SELECT username, role, status FROM users WHERE username='admin'")
[[ "$actual" == "$expected" ]] || { echo "schema_migrations count is $actual, expected $expected" >&2; exit 1; }
[[ "$admin" == $'admin\tadmin\tenabled' ]] || { echo "initial enabled admin row is missing or unexpected: $admin" >&2; exit 1; }
echo "database verified: migrations=$actual initial_admin=admin/enabled"
