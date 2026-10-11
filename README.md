# wt-media-cloud

WT Media Cloud is a Go + CloudWeGo Hertz modular monolith. It owns Cloud APIs, business facts, public-platform data integration, discovery scheduling, and the unified business Web source.

## Responsibilities

- Cloud API and Cloud–Agent contract boundaries.
- Config, logger, database, HTTP client, and process bootstrap.
- Identity, media account, content pool, profile, proxy, and discovery modules.
- Server, migration, scheduler, and worker process entrypoints.

## Configuration

Runtime code always reads `./config`. `config_online/` is the release template source: fixed `.toml` files stay unchanged and environment values use `{{...}}` placeholders in `.toml.tpl`. Packaging copies it to the artifact as template-state `config/`; the deployment renderer calls `bin/config-check` before atomically replacing that release's private `config/`.

Important files:

```text
config/app.toml
config/database/primary.toml
config/scheduler/scheduler.toml
config/logger/*.toml
config/clients/http/*.toml
config/credentials/*.toml
```

The HTTP address, cookie secure flag, initial administrator, database connection, scheduler intervals, loggers, clients, and credentials are configured in TOML. Go runtime code does not read `WT_MEDIA_MYSQL_DSN`, initial-admin environment variables, or cookie-secure environment variables.

Apply migrations with:

```text
scripts/migrate.sh
```

Local development uses the stable `wt_media_cloud` database configured by `config/database/primary.toml`. Apply schema changes through migrations instead of creating a new database for each task.

## Key Directories

```text
cmd/                 process entrypoints
internal/bootstrap/  production initialization orchestration
internal/config/     Cloud config schema, validation, and getters
internal/infra/      database, logger, clients, metrics, and tracing
internal/modules/    business modules
pkg/                 reusable generic config, logger, and HTTP-client code
contracts/           Cloud-owned contracts
web/                 unified business Web source
```


## BaoTa Deployment

Deployment is controlled by `bin/wtmctl`. It does not start, stop, or restart
Cloud processes; BaoTa manages Server, Worker, and Scheduler.

The three processes resolve one absolute release root from
`$WT_MEDIA_CLOUD_HOME`, then from the running binary's
`<home>/bin/<binary>` location, then from the working directory for local
development. Startup initializes these paths once; runtime code reads them
through `config.GetRuntimePaths()`. Reading before initialization or failing
to resolve the root stops startup with a panic. `WT_MEDIA_CLOUD_HOME` must be
absolute when set. The config,
log, and Web paths default to `<home>/config`, `<home>/logs`, and `<home>/web`.
Their `WT_MEDIA_CLOUD_CONFIG_PATH`, `WT_MEDIA_CLOUD_LOG_PATH`, and
`WT_MEDIA_CLOUD_WEB_PATH` overrides may be absolute or relative to the release
root. The migration command defaults to `<home>/migrations`.

Startup writes the resolved paths and initialization failures to the process
manager's stdout/stderr log. Once the app logger opens, authentication results
and failures go to `<home>/logs/app.log` (or the configured log directory).

### Prepare

```bash
cd /home/www/wt-media-cloud/output/wt-media-cloud_v0.1.0-rc.12_linux-amd64
./bin/wtmctl artifact verify --profile /home/www/wt-media-cloud/output/online-deploy.toml
./bin/wtmctl doctor --profile /home/www/wt-media-cloud/output/online-deploy.toml
./bin/wtmctl deploy plan --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

`deploy plan` pulls the remote TOML variables into a temporary file and checks
the package, variable schema, required values, and rendered configuration
without changing `current`.

### One-command deploy

Stop Server, Worker, and Scheduler in BaoTa first. Then run:

```bash
sudo ./bin/wtmctl deploy apply   --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

This verifies the package, pulls `online.toml`, validates all required values,
renders the private configuration, runs migrations, installs the release, and
atomically switches `current`. Service restart remains a separate BaoTa action.

### Verify

```bash
sudo /home/www/wt-media-cloud/current/bin/wtmctl deploy verify   --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

### Roll back

Stop all three BaoTa processes before switching `current`:

```bash
sudo /home/www/wt-media-cloud/current/bin/wtmctl release rollback   --profile /home/www/wt-media-cloud/output/online-deploy.toml   --to <old-tag>   --services-stopped
```

### Process ownership

| Process | Manager |
| --- | --- |
| Server | BaoTa Go project |
| Worker | BaoTa Process Manager |
| Scheduler | BaoTa Process Manager |

## Verification

The module requires Go 1.24.0 or newer; the current local verification toolchain is Go 1.26.5.

```text
scripts/bootstrap.sh
scripts/test.sh
scripts/build.sh
scripts/verify-health.sh
```

For local process management:

```text
bin/control.sh start
bin/control.sh status
bin/control.sh stop
```

`bin/control.sh help` lists the remaining verbs.

`WT_MEDIA_CLOUD_HTTP_ADDR` only selects the health-probe address used by scripts. The server listen address comes from `config/app.toml`; keep those values aligned in local development.
