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
- `WT_MEDIA_INITIAL_TECHNICIAN_USERNAME`: one-time initial technician username for an empty identity database
- `WT_MEDIA_INITIAL_TECHNICIAN_PASSWORD`: one-time initial technician password; no default is provided
- `WT_MEDIA_SESSION_COOKIE_SECURE`: secure-cookie flag, default `true`; set `false` only for local HTTP development

Apply `migrations/20260714_001_identity.sql` before enabling the M2-C1 identity API with `WT_MEDIA_MYSQL_DSN`.

## Key Directories

- `cmd/server`: process entrypoint.
- `internal/app`: app assembly and route wiring.
- `internal/infra`: database, logger, object storage, and scheduler adapters.
- `contracts`: Cloud-owned contracts.
- `web`: unified business Web source.

## M0 Verification

Use Go 1.26.5 for M0 Cloud verification. From this repository:

```text
GO_BIN=../../devenv/go26/go/bin/go scripts/verify-health.sh
```
