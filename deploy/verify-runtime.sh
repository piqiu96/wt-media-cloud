#!/usr/bin/env bash
set -euo pipefail
base_url=http://127.0.0.1:8080
check_login=false
admin_username=admin
admin_password="${WT_MEDIA_ADMIN_PASSWORD:-}"

usage() {
  cat <<'MSG'
Usage: deploy/verify-runtime.sh [--base-url URL] [--login-admin] [--admin-user U] [--admin-password P]

Checks both health endpoints and optionally checks the initial admin login.
MSG
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url) base_url=${2%/}; shift 2 ;;
    --login-admin) check_login=true; shift ;;
    --admin-user) admin_username=$2; shift 2 ;;
    --admin-password) admin_password=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }

curl --silent --show-error --fail "$base_url/healthz" >/dev/null
curl --silent --show-error --fail "$base_url/api/v1/health" >/dev/null
if $check_login; then
  if [[ -z "$admin_password" ]]; then
    if [[ -t 0 ]]; then
      read -r -s -p "Password for ${admin_username}: " admin_password
      printf '\n' >&2
    else
      echo "admin password is required for login verification" >&2
      exit 2
    fi
  fi
  body_file=$(mktemp)
  trap 'rm -f "$body_file"' EXIT
  printf '%s\0%s\0' "$admin_username" "$admin_password" \
    | python3 -c 'import json,sys; user,password=sys.stdin.buffer.read().split(b"\0")[:2]; print(json.dumps({"username":user.decode(),"password":password.decode(),"replace_existing":True}))' \
    | curl --silent --show-error --fail -H 'Content-Type: application/json' \
      --data-binary @- "$base_url/api/v1/auth/login" >"$body_file"
  python3 - "$body_file" "$admin_username" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as source:
    result = json.load(source)
if result.get("errcode") != 0 or result.get("data", {}).get("username") != sys.argv[2]:
    sys.exit("admin login verification failed")
PY
  echo "runtime verified: health=ok login=$admin_username"
else
  echo "runtime verified: health=ok"
fi
