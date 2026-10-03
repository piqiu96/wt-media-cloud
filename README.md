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
