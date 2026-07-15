# Cloud Scripts

Expose common commands with stable names.

| Script | Purpose |
|---|---|
| `bootstrap.sh` | Download Go modules and install Web dependencies with `npm ci`. |
| `test.sh` | Run Cloud Go tests and Cloud Web Vitest tests. |
| `build.sh` | Build the Cloud server binary and Cloud Web Vite assets. |
| `migrate.sh` | Apply MySQL migrations from `migrations/` using `WT_MEDIA_MYSQL_DSN`. |
| `start.sh` | Start the Cloud server in the background, wait for `/healthz`, and write a PID file. |
| `health.sh` | Probe `/healthz` and `/api/v1/health`. |
| `stop.sh` | Stop the process recorded by the PID file. |
| `verify-health.sh` | Run tests, start the server temporarily, probe health, and stop it. |

`WT_MEDIA_CLOUD_HTTP_ADDR` controls the listen address. `WT_MEDIA_CLOUD_PID_FILE` and `WT_MEDIA_CLOUD_LOG_FILE` can override the default files under `.cache/`. Set `WT_MEDIA_CLOUD_GOPATH` when you want scripts to use a project-local Go module cache instead of the current shell `GOPATH`.
