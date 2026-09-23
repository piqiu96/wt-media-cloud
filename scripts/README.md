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

Runtime connection values come from `config/`. In particular, migration reads `config/database/primary.toml`, and the server listen address comes from `config/app.toml`.

`WT_MEDIA_CLOUD_HTTP_ADDR` controls only the health-probe address used by scripts. Keep it aligned with `config/app.toml`. `WT_MEDIA_CLOUD_PID_FILE`, `WT_MEDIA_CLOUD_LOG_FILE`, `WT_MEDIA_CLOUD_BINARY_FILE`, and `WT_MEDIA_CLOUD_GOPATH` remain script-level operational overrides.

`cmd/server` starts only the API Server (`serverResourcePlan()` has no scheduler or worker step). The Discovery Scheduler and Discovery Worker are separate CMD entrypoints, `cmd/discovery-scheduler` and `cmd/discovery-worker`, and start nothing else: the Scheduler only creates pending `crawl_tasks`; the Worker exclusively claims and executes them. Both take no flags and loop on the intervals in `config/scheduler/scheduler.toml` (`discovery_interval`, `worker_interval`). To run one due-strategy scan without waiting for the interval, call the admin-controlled `POST /api/v1/discovery-scheduler/run-due`.

Starting `cmd/server` alone performs no discovery execution: pending `crawl_tasks` stay pending until a Worker process claims them.
