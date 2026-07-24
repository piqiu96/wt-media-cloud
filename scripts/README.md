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

Local scripts source `scripts/local-env.sh` before starting or migrating Cloud. By default local development and acceptance use:

- MySQL database: `wt_media_cloud`
- MySQL DSN: `root:root123@tcp(127.0.0.1:3306)/wt_media_cloud?parseTime=true&multiStatements=true`
- HTTP address: `127.0.0.1:18080`
- Cookie secure flag: `WT_MEDIA_SESSION_COOKIE_SECURE=false`

Do not create a new local database for every CHG/Task. Apply table changes to the stable local database through migrations or explicit acceptance data updates. Override `WT_MEDIA_MYSQL_DSN` only when intentionally running an isolated test.

`WT_MEDIA_CLOUD_HTTP_ADDR` controls the listen address. `WT_MEDIA_CLOUD_PID_FILE` and `WT_MEDIA_CLOUD_LOG_FILE` can override the default files under `.cache/`. Set `WT_MEDIA_CLOUD_GOPATH` when you want scripts to use a project-local Go module cache instead of the current shell `GOPATH`.
