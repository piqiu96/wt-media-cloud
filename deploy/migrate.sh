#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dry_run=false

usage() {
  cat <<'MSG'
Usage: deploy/migrate.sh [--dry-run]

Runs the release's migration executable against the explicitly configured
database. The target database and account must already exist; this entry
always passes ---create-database=false.
MSG
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

cd "$root"
[[ -f "config/database/primary.toml" ]] || { echo "missing $root/config/database/primary.toml" >&2; exit 1; }
[[ -x "bin/migrate" ]] || { echo "missing executable $root/bin/migrate" >&2; exit 1; }
[[ -d migrations ]] || { echo "missing $root/migrations" >&2; exit 1; }

if $dry_run; then
  echo "cd $root && bin/migrate -dir migrations --create-database=false"
  exit 0
fi
exec bin/migrate -dir migrations --create-database=false
