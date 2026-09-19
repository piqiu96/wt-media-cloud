# Infrastructure Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Converge Cloud configuration, Hertz-compatible logging, request context, and Hertz outbound HTTP clients while preserving typed infrastructure boundaries.

**Architecture:** Generic stateless capabilities live under `pkg`; Cloud semantic mapping stays under `internal/config`, `internal/infra`, and `internal/bootstrap`. All production resources are initialized only by bootstrap, exposed through typed getters, and rolled back in reverse order.

**Tech Stack:** Go 1.25, Hertz, `hertz-contrib/logger/zap`, Zap, lumberjack, `pelletier/go-toml/v2`.

**Spec:** `docs/superpowers/specs/2026-09-20-infrastructure-bootstrap-design.md`

## Global Constraints

- Runtime config stays in `config/`; release config stays in `config_online/`.
- Cloud's runtime and release trees use TOML, but generic loading also supports YAML and JSON.
- Do not create Redis or OSS directories, clients, registries, or configuration.
- Do not create a universal cross-type client registry or `any` resource locator.
- Business modules may not construct HTTP clients or import `pkg/clients/http`.
- Only `internal/bootstrap` initializes production resources.
- Preserve the six logger names: app, access, job, external, audit, panic.
- Every logger has `<path>` and `<path>.wf`; `.wf` receives WARN and above.
- Context generic fields are exactly `trace_id`, `request_id`, and `user_id`; no generic `task_id`.
- Preserve current routes, API envelopes, database schema, and Cloud-Agent contracts.
- Do not modify `web/dist-*`.

## Review Focus

- Duplicate config names across extensions must fail rather than silently override.
- Unknown TOML fields must fail under strict Cloud decoding.
- Logger initialization must close already-open normal and `.wf` writers on partial failure.
- `.wf` must receive WARN/ERROR/FATAL while normal files retain configured level behavior.
- HTTP retry attempts include the initial request and must not become an accidental infinite retry.
- Context logging must omit empty IDs rather than emit misleading zero values.
- Agent and Douyin request bodies, headers, credentials, pagination, and error mapping must remain unchanged.
- Worker and scheduler must not initialize Hertz server routes or unnecessary clients.
- `config_online` must remain a drop-in replacement for `config` at release packaging time.

### Task 1: Add generic configuration loading

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `pkg/config/config.go`
- Test: `pkg/config/config_test.go`

**Interfaces:**
- Produces: `Document`, `Format`, `LoadFile`, `LoadDir`, `LoadDirRecursive`, and `Document.Decode`.
- Consumes: existing YAML dependency plus `github.com/pelletier/go-toml/v2`.

**Steps:**
- [ ] Add failing tests for TOML/YAML/JSON file loading, strict decode, non-recursive and recursive scans, deterministic names, duplicate names, and unsupported extensions.
- [ ] Run `go test ./pkg/config` and confirm the expected compile/test failure.
- [ ] Implement the stateless loader and strict decoders.
- [ ] Run `go test ./pkg/config`.
- [ ] Commit.

### Task 2: Migrate Cloud config schema to generic TOML documents

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Create/replace: all `config/**/*.toml`
- Create/replace: all `config_online/**/*.toml`
- Remove: migrated `config/**/*.yaml` and `config_online/**/*.yaml`

**Interfaces:**
- Consumes: `pkg/config`.
- Produces: unchanged `config.Initialize`, `config.Load`, `config.LoadFromDir`, and typed config structs with TOML tags.

**Steps:**
- [ ] Update config fixtures and tests to TOML first and add failures for non-TOML Cloud config, duplicate names, unknown fields, and invalid durations.
- [ ] Run `go test ./internal/config` and confirm failure.
- [ ] Add TOML dependency and make duration parsing support quoted Go duration strings.
- [ ] Replace Cloud and release YAML files with structurally equivalent TOML.
- [ ] Retain credential separation and redact credentials in all command output.
- [ ] Run `go test ./internal/config`.
- [ ] Commit.

### Task 3: Implement Hertz-compatible logger core

**Files:**
- Create: `pkg/logger/logger.go`
- Create: `pkg/logger/logger_test.go`
- Modify: `internal/infra/logger/logger.go`
- Modify: `internal/infra/logger/logger_test.go`

**Interfaces:**
- Produces: `pkg/logger.New(Config)`, `pkg/logger.Close`, and internal typed `hlog.FullLogger` getters.
- Consumes: `hertz-contrib/logger/zap`, Zap, lumberjack, and `internal/config` logger config.

**Steps:**
- [ ] Add failing tests for normal/writer routing, WARN+ `.wf` behavior, JSON/console/text formats, level parsing, duplicate paths, and close/rollback.
- [ ] Run logger package tests and confirm failure.
- [ ] Add approved dependencies and implement the thin two-core logger builder.
- [ ] Replace `slog` resources while keeping the six typed getters and atomic lifecycle behavior.
- [ ] Run `go test ./pkg/logger ./internal/infra/logger`.
- [ ] Commit.

### Task 4: Propagate request identity through Go context

**Files:**
- Create: `internal/shared/requestctx/requestctx.go`
- Test: `internal/shared/requestctx/requestctx_test.go`
- Modify: `internal/middleware/request_context.go`
- Modify: `internal/middleware/request_context_test.go`
- Modify: identity authentication/context integration as needed

**Interfaces:**
- Produces: `WithTraceID`, `WithRequestID`, `WithUserID`, and corresponding getters.
- Consumes: existing identity authentication helpers.

**Steps:**
- [ ] Add failing tests proving handlers receive Go context IDs and logs include trace/request/user IDs.
- [ ] Run focused middleware/requestctx tests and confirm failure.
- [ ] Implement the narrow context package and middleware propagation.
- [ ] Add opportunistic identity context enrichment without changing public endpoint behavior.
- [ ] Run focused tests.
- [ ] Commit.

### Task 5: Build the Hertz HTTP client resource

**Files:**
- Create: `pkg/clients/http/client.go`
- Create: `pkg/clients/http/registry.go`
- Test: `pkg/clients/http/client_test.go`
- Test: `pkg/clients/http/registry_test.go`

**Interfaces:**
- Produces: strongly typed `Client`, `Initialize`, `Get`, and `Close`.
- Consumes: Hertz client/retry options and `pkg/config.Document`.

**Steps:**
- [ ] Add failing tests for TOML decoding, Hertz option conversion, filename-derived names, retry policy, timeout wrapper, duplicate names, lookup failure, and rollback.
- [ ] Run `go test ./pkg/clients/http` and confirm failure.
- [ ] Implement the narrow HTTP resource and registry.
- [ ] Run `go test ./pkg/clients/http`.
- [ ] Commit.

### Task 6: Migrate Agent and Douyin to Hertz

**Files:**
- Modify: `internal/infra/client/agent/client.go`
- Modify: `internal/infra/client/agent/client_test.go`
- Modify: `internal/infra/client/platforms/douyin/client.go`
- Modify: `internal/infra/client/platforms/douyin/client_test.go`
- Create: `config/clients/http/agent.toml`
- Create: `config/clients/http/douyin.toml`
- Create: `config_online/clients/http/agent.toml`
- Create: `config_online/clients/http/douyin.toml`

**Interfaces:**
- Consumes: `httpclient.Get("agent")` and `httpclient.Get("douyin")`.
- Produces: unchanged typed protocol methods and package lifecycle functions.

**Steps:**
- [ ] Preserve current protocol assertions in tests while changing transport expectations to Hertz.
- [ ] Run both client test packages and confirm migration failure.
- [ ] Replace `net/http` transport fields with Hertz wrappers.
- [ ] Move client TOML under `clients/http` while leaving credentials unchanged.
- [ ] Add external observation middleware without leaking credentials or response bodies.
- [ ] Run `go test ./internal/infra/client/...`.
- [ ] Commit.

### Task 7: Integrate bootstrap ordering and Hertz logger

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`
- Modify: `internal/bootstrap/resource.go`
- Modify: `internal/bootstrap/bootstrap_test.go`

**Interfaces:**
- Consumes: all new resource packages.
- Produces: unchanged public bootstrap entrypoints.

**Steps:**
- [ ] Update order tests for config, logger/Hertz integration, metrics, tracing, database, HTTP clients, and process-specific runtime.
- [ ] Run bootstrap tests and confirm failure.
- [ ] Add the HTTP resource step after database.
- [ ] Install Hertz default/system loggers after logger initialization.
- [ ] Run `go test ./internal/bootstrap`.
- [ ] Commit.

### Task 8: Final verification and cleanup

**Files:**
- Modify: architecture boundary tests as needed
- Modify: `docs/arch/wt-media-cloud-arch.md`
- No Redis/OSS files

**Steps:**
- [ ] Add architecture checks forbidding business imports of `pkg/config`, `pkg/logger`, and `pkg/clients/http`.
- [ ] Scan for stale YAML references and obsolete `slog` production use.
- [ ] Run `gofmt` over changed Go files.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run migration against local `wt_media_cloud`.
- [ ] Start server and worker smoke checks.
- [ ] Commit final cleanup.
