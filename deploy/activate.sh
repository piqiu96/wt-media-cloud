#!/usr/bin/env bash
set -euo pipefail

install_root=/www/wt-media-cloud
target=''

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-root) install_root=$2; shift 2 ;;
    --to) target=$2; shift 2 ;;
    -h|--help)
      echo 'Usage: deploy/activate.sh --install-root <dir> --to <tag>'
      exit 0 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$target" && ! "$target" =~ [/[:space:]] ]] || { echo "valid --to is required" >&2; exit 2; }
destination="$install_root/releases/$target"
[[ -d "$destination" && -f "$destination/release-info.json" ]] || { echo "unknown release: $destination" >&2; exit 1; }
[[ -f "$destination/config/database/primary.toml" ]] || { echo "private config is missing" >&2; exit 1; }

python3 - "$destination/release-info.json" "$target" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as source:
    actual = json.load(source)["product_tag"]
if actual != sys.argv[2]:
    sys.exit(f"release Tag mismatch: package={actual} requested={sys.argv[2]}")
PY

tmp_link="$install_root/.current.$$"
trap 'rm -f "$tmp_link"' EXIT
ln -s "releases/$target" "$tmp_link"
python3 - "$tmp_link" "$install_root/current" <<'PY'
import os
import sys
os.replace(sys.argv[1], sys.argv[2])
PY
echo "current=$install_root/current -> releases/$target"
