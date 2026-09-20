# Cloud Scripts

| Script | Purpose |
|---|---|
| `bootstrap.sh` | Download Go modules and install Web dependencies with `npm ci`. |
| `test.sh` | Run Cloud Go tests and Cloud Web Vitest tests. |
| `build.sh` | Build the Cloud server binary and Cloud Web Vite assets. |
| `migrate.sh` | Apply migrations from `migrations/` using `config/database/primary.toml`. |
| `start.sh` | Start the Cloud server in the background, wait for `/healthz`, and write a PID file. |
| `health.sh` | Probe `/healthz` and `/api/v1/health`. |
| `stop.sh` | Stop the process recorded by the PID file. |
| `verify-health.sh` | Run tests, start the server temporarily, probe health, and stop it. |
| `run-discovery-scheduler.sh` | Run one due-strategy scan by default; pass `--interval 1m` without `--once` for a standalone scheduler loop. |
| `run-discovery-worker.sh` | Drain one bounded pending-task batch by default; pass `--interval 5s` without `--once` for a standalone worker loop. |

Runtime connection values come from `config/`. In particular, migration reads `config/database/primary.toml`, and the server listen address comes from `config/app.toml`.

`WT_MEDIA_CLOUD_HTTP_ADDR` controls only the health-probe address used by scripts. Keep it aligned with `config/app.toml`. `WT_MEDIA_CLOUD_PID_FILE`, `WT_MEDIA_CLOUD_LOG_FILE`, `WT_MEDIA_CLOUD_BINARY_FILE`, and `WT_MEDIA_CLOUD_GOPATH` remain script-level operational overrides.

The normal Cloud server starts the API Server, Discovery Scheduler, and Discovery Worker together. Scheduler and Worker scripts are separate operational entrypoints: the Scheduler only creates pending `crawl_tasks`; the Worker exclusively claims and executes them.
