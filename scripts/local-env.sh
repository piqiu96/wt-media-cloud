#!/usr/bin/env bash
# Local development defaults shared by Cloud scripts.
#
# Local M/CHG acceptance uses one stable database. Do not create a new
# database for every task; apply schema changes to this database through
# migrations or explicit acceptance data updates.

# Script health-probe address only; the server listen address comes from config/app.toml.
export WT_MEDIA_CLOUD_HTTP_ADDR="${WT_MEDIA_CLOUD_HTTP_ADDR:-127.0.0.1:8188}"

# Load repository-local dotenv values without evaluating them as shell code.
# Cookies commonly contain spaces, semicolons, and other shell metacharacters.
LOCAL_ENV_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -f "$LOCAL_ENV_ROOT/.env.local" ]]; then
  while IFS='=' read -r key value || [[ -n "$key" ]]; do
    [[ -z "$key" || "$key" == \#* ]] && continue
    export "$key=$value"
  done < "$LOCAL_ENV_ROOT/.env.local"
fi
unset LOCAL_ENV_ROOT key value
