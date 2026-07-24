# wt-media-cloud

Cloud runtime skeleton for the modular social media operations platform.

## Responsibilities

- Cloud-owned API and contract boundaries.
- Basic HTTP process startup and health checks.
- Infrastructure placeholders for configuration, database, object storage, logging, and scheduler adapters.
- Unified business Web source location.

## Bootstrap

The M0 Cloud backend is a Go modular monolith skeleton using CloudWeGo Hertz for HTTP APIs. It exposes health checks only. Formal user, account, Agent, task, publication, discovery, production, interaction, and analytics modules are intentionally not implemented in M0.

Runtime configuration:

- `WT_MEDIA_CLOUD_HTTP_ADDR`: listen address, default `:8080`
- `WT_MEDIA_MYSQL_DSN`: MySQL DSN for the Cloud database
- `WT_MEDIA_INITIAL_ADMIN_USERNAME`: one-time initial administrator username for an empty identity database
- `WT_MEDIA_INITIAL_ADMIN_PASSWORD`: one-time initial administrator password; no default is provided
- Legacy `WT_MEDIA_INITIAL_TECHNICIAN_*` names remain accepted when the new admin variables are unset.
- `WT_MEDIA_SESSION_COOKIE_SECURE`: secure-cookie flag, default `true`; set `false` only for local HTTP development

Apply migrations with:

```text
scripts/migrate.sh
```

Local scripts default to the stable local database `wt_media_cloud` through `scripts/local-env.sh`:

```text
root:root123@tcp(127.0.0.1:3306)/wt_media_cloud?parseTime=true&multiStatements=true
```

For local M/CHG acceptance, use this stable database and modify its tables through migrations or explicit acceptance data updates. Do not create a new database for every Task. Override `WT_MEDIA_MYSQL_DSN` only when intentionally running isolated CI or destructive experiments.

The migration command creates the DSN database when missing, records applied versions in `schema_migrations`, and skips already applied migrations on repeat runs.

## Key Directories

- `cmd/server`: process entrypoint.
- `internal/app`: app assembly and route wiring.
- `internal/infra`: database, logger, object storage, and scheduler adapters.
- `contracts`: Cloud-owned contracts.
- `web`: unified business Web source.

## M0 Verification

Use Go 1.26.5 for M0 Cloud verification. From this repository:

```text
scripts/bootstrap.sh
scripts/test.sh
scripts/build.sh
WT_MEDIA_CLOUD_HTTP_ADDR=127.0.0.1:18080 scripts/verify-health.sh
```

For foreground process management in a local terminal:

```text
WT_MEDIA_CLOUD_HTTP_ADDR=127.0.0.1:18080 scripts/start.sh
WT_MEDIA_CLOUD_HTTP_ADDR=127.0.0.1:18080 scripts/health.sh
scripts/stop.sh
```
