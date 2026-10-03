#!/usr/bin/env bash
set -euo pipefail
install_root=/www/wt-media-cloud
target=''

usage() {
  cat <<'MSG'
Usage: deploy/rollback.sh --install-root <dir> --to <release>

Switches the current symlink only. It does not roll back MySQL schema or data;
read DEPLOYMENT.md and stop all Cloud processes before using it after a migration.
MSG
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-root) install_root=$2; shift 2 ;;
    --to) target=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n "$target" ]] || { usage >&2; exit 2; }
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
"$script_dir/activate.sh" --install-root "$install_root" --to "$target"
echo "database and object storage were not rolled back"
