# WT Media Cloud Foundation Convergence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the transitional Runtime-based composition with the approved controlled-resource architecture, complete every module migration, and preserve business behavior except for the approved synchronous Douyin search flow.

**Architecture:** Bootstrap is the only production initializer. Each Infra package privately owns its initialized resource and exposes typed getters; `cmd/server/server.go` owns only HTTP process start/stop. Module routers bind root-package handler functions, services and repositories expose package functions, Scheduler discovers work, and Worker executes it synchronously.

**Tech Stack:** Go 1.24, CloudWeGo Hertz, GORM MySQL driver, `go-sqlmock`, `log/slog`, YAML v3, Vue/Vite tests.

**Spec:** `docs/superpowers/specs/2026-09-18-cloud-foundation-convergence-design.md`

## Global Constraints

- Continue in the existing dirty workspace; never reset, discard, or overwrite unrelated user changes.
- Stage and commit only the exact files completed by each task.
- Do not modify database tables or migrations except for mechanical `*gorm.DB` compatibility.
- Preserve routes, response envelope, error codes, authorization, team scoping, and Cloud–Agent contracts except for the approved synchronous search behavior.
- Do not add microservices, MQ, dynamic plugins, workflow engines, Cookie pools, proxy pools, or DI containers.
- Runtime code reads only `./config`; `config_online` is packaging input.
- Existing SQL remains explicit through GORM `Raw`, `Exec`, and `Transaction`.
- Use test-first changes and run the smallest relevant suite after every task.
- Do not update `web/dist-*` in this migration.

## Target File Map

- `AGENTS.md`: authoritative development rules matching the approved design.
- `docs/arch/wt-media-cloud-arch.md`: human architecture reference.
- `internal/config/config.go`: strict config schema, loading, and validation only.
- `internal/infra/database/mysql.go`: private default/named GORM connections and lifecycle.
- `internal/infra/logger/logger.go`: six private loggers and six typed getters.
- `internal/infra/client/{agent,platforms/douyin}`: private initialized clients plus typed protocol methods.
- `internal/infra/{metrics,tracing}`: private Noop-safe initialized observability resources.
- `internal/bootstrap/bootstrap.go`: exported process initialization entry points and order.
- `internal/bootstrap/resource.go`: private resource startup, rollback, and close helpers.
- `internal/bootstrap/routes.go`: Middleware and module route registration only.
- `internal/bootstrap/jobs.go`: Scheduler/Worker job connection only.
- `cmd/server/main.go`: minimal process entry.
- `cmd/server/server.go`: HTTP start, signal handling, graceful stop, and Bootstrap close.
- `internal/modules/*/{router.go,handler*.go}`: direct function route binding and HTTP adaptation.
- `internal/modules/*/{dto,model,service,repository}`: real type ownership and package-level business/data functions.
- `internal/jobs`: executable business jobs.
- `internal/scheduler`: timing and cancellation only.
- `web/src/modules/contentpool/pages/ContentPoolPage.vue`: synchronous search UX.
- `web/src/shared/api/discovery.js`: synchronous result import API.

---

### Task 1: Synchronize Architecture Rules and Add Boundary Tests

**Files:**
- Modify: `AGENTS.md`
- Modify: `docs/arch/wt-media-cloud-arch.md`
- Create: `internal/architecture/boundary_test.go`
- Modify: `docs/superpowers/plans/2026-09-17-complete-architecture-migration.md`

**Interfaces:**
- Consumes: approved architecture spec.
- Produces: executable source-boundary checks used by the rest of the migration.

- [x] **Step 1: Write a failing boundary test**

Create a table-driven test that walks production `.go` files and rejects forbidden patterns:

```go
func TestProductionArchitectureBoundaries(t *testing.T) {
    checks := []check{
        {root: "../modules", pattern: `http\.Client\s*\{`, message: "modules must use infra clients"},
        {root: "../modules", pattern: `time\.NewTicker\(`, message: "modules must not own schedulers"},
        {root: "../modules", pattern: `os\.(Getenv|LookupEnv)\(`, message: "modules must not read process config"},
        {root: "../modules", pattern: `database/sql`, message: "modules must use the repository GORM boundary"},
    }
    runChecks(t, checks)
}
```

- [x] **Step 2: Run the boundary test and record current failures**

Run: `go test ./internal/architecture -run TestProductionArchitectureBoundaries -count=1`

Expected RED result: the strict test lists the ten module repositories that still import `database/sql`. Record those exact files as an explicit legacy baseline so Task 1 ends green, no new violation can be introduced, and later module tasks must shrink the baseline as they migrate each repository.

- [x] **Step 3: Rewrite the architecture rules**

Replace the old Runtime and Agent-only requirements with these exact rules:

```text
基础资源由 Infra 包私有保存，通过公有只读 Getter 使用；禁止导出可变资源变量和全局业务状态。
所有生产初始化由 Bootstrap 发起。
Cloud 可直接调用公开平台数据 API；本地浏览器、Profile、文件和 FFmpeg 操作必须经过 Agent。
```

Document `cmd/server/server.go`, `bootstrap/resource.go`, the absence of `modules.go`, GORM, six physical logs, synchronous search, and `shared/api`/`shared/id`.

- [x] **Step 4: Mark the superseded implementation plan as historical**

Add a notice linking to this plan and remove instructions that add resources to `runtime.Runtime`.

- [x] **Step 5: Verify documentation consistency**

Run:

```bash
rg -n 'must use Runtime|必须通过 Runtime|internal/server|server.yaml|所有外部平台.*Agent' AGENTS.md docs/arch docs/superpowers/plans
git diff --check
```

Expected: no active rule contradicts the approved spec; historical text is explicitly labeled.

- [x] **Step 6: Commit**

```bash
git add AGENTS.md docs/arch/wt-media-cloud-arch.md docs/superpowers/plans/2026-09-17-complete-architecture-migration.md internal/architecture/boundary_test.go
git commit -m "docs: align cloud architecture boundaries"
```

### Task 2: Replace the Transitional Config Schema

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `config/app.yaml`
- Delete: `config/server.yaml`
- Replace: `config/database/mysql.yaml`
- Create: `config/logger/access.yaml`
- Create: `config/logger/job.yaml`
- Create: `config/logger/external.yaml`
- Create: `config/logger/audit.yaml`
- Create: `config/logger/panic.yaml`
- Modify: `config/clients/agent.yaml`
- Modify: `config/clients/platforms/douyin.yaml`
- Create: `config/credentials/agent.yaml`
- Create: `config/credentials/douyin.yaml`
- Modify: `config/scheduler/scheduler.yaml`
- Mirror the same file layout under: `config_online/`

**Interfaces:**
- Produces: `config.Initialize() error`, `config.Get() Config`, `config.Load() (Config, error)`, `config.LoadFromDir(string) (Config, error)`, `Config.Validate() error`.
- Produces: `Config.App.Server`, `Config.Databases`, `Config.Loggers`, `Config.Clients`, `Config.Credentials`, `Config.Scheduler`.

- [x] **Step 1: Replace tests with the approved config contract**

Cover these exact behaviors:

```go
func TestLoadAlwaysUsesConfigDirectory(t *testing.T)
func TestLoadFromDirReadsServerFromAppYAML(t *testing.T)
func TestLoadFromDirRequiresPrimaryDatabase(t *testing.T)
func TestLoadFromDirRejectsDuplicateDatabaseNames(t *testing.T)
func TestLoadFromDirLoadsSixLoggerConfigs(t *testing.T)
func TestClientAndCredentialConfigsAreIndependent(t *testing.T)
func TestSchedulerDurationsAndBatchSizeAreValidated(t *testing.T)
func TestInitializePublishesValidatedReadOnlyConfig(t *testing.T)
```

- [x] **Step 2: Run tests and verify the old schema fails**

Run: `go test ./internal/config -count=1`

Expected: FAIL because Server is still separate, MySQL uses DSN, and credentials are embedded in Client config.

- [x] **Step 3: Implement the strict schema**

Use these core types:

```go
type Config struct {
    App         AppConfig
    Databases   []DatabaseConfig
    Cache       CacheConfig
    Loggers     LoggerConfigs
    Clients     ClientsConfig
    Credentials CredentialsConfig
    Scheduler   SchedulerConfig
    Observability ObservabilityConfig
}

type AppConfig struct {
    Name         string       `yaml:"name"`
    Server       ServerConfig `yaml:"server"`
    InitialAdmin InitialAdminConfig `yaml:"initial_admin"`
}

type DatabaseConfig struct {
    Name, Host, Database, Username, Password, Charset, Location string
    Port int
    ParseTime bool `yaml:"parse_time"`
    Pool DatabasePoolConfig
}

type ClientConfig struct {
    Name, Scheme, Host string
    Port int
    Timeout Duration
    Retry RetryConfig
}

type Duration struct { time.Duration }
```

Implement `Duration.UnmarshalYAML` with `time.ParseDuration`, and use it for Client, Scheduler, Worker, pool-lifetime, and retry intervals. Decode YAML with `yaml.Decoder.KnownFields(true)`. Scan only database connection YAML files, exclude migration configuration explicitly, require the code-fixed name `primary`, and remove all environment overrides and `WT_MEDIA_CONFIG_DIR` handling. `Initialize` atomically publishes a validated private Config; `Get` returns the immutable value and fails fast before initialization.

- [x] **Step 4: Create the approved YAML layout**

Move HTTP settings into `app.yaml`; keep Scheduler/Worker values under `scheduler/scheduler.yaml`; keep transport-only values in Client files and secrets/headers in Credential files. Each logger config contains `path`, `level`, and `format`, with paths fixed to the corresponding `logs/*.log` file. Do not add `default` or `enabled` to database files.

- [x] **Step 5: Run config tests**

Run: `go test ./internal/config -count=1`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/config config config_online
git commit -m "refactor: establish strict cloud configuration"
```

### Task 3: Establish the Private GORM Database Registry

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Rewrite: `internal/infra/database/mysql.go`
- Rewrite: `internal/infra/database/mysql_test.go`
- Modify: `internal/infra/database/migration/runner.go`
- Modify: `internal/infra/database/migration/*_test.go`

**Interfaces:**
- Consumes: `[]config.DatabaseConfig`.
- Produces: `database.Initialize([]config.DatabaseConfig) error`, `database.DB() *gorm.DB`, `database.Named(string) (*gorm.DB, bool)`, `database.Close() error`.

- [x] **Step 1: Add failing GORM registry tests using `go-sqlmock`**

Test the private registry through a dependency seam:

```go
func TestInitializeRequiresPrimary(t *testing.T)
func TestInitializeRegistersDefaultAndNamedDatabases(t *testing.T)
func TestInitializeClosesOpenedDatabasesWhenLaterOpenFails(t *testing.T)
func TestCloseClosesEveryUniqueSQLConnectionOnce(t *testing.T)
func TestMigrationRunnerUsesGORMTransaction(t *testing.T)
```

Build each Mock GORM handle with:

```go
sqlDB, mock, err := sqlmock.New()
gormDB, err := gorm.Open(mysql.New(mysql.Config{
    Conn: sqlDB,
    SkipInitializeWithVersion: true,
}), &gorm.Config{})
```

- [x] **Step 2: Run database tests and verify failure**

Run: `go test ./internal/infra/database/... -count=1`

Expected: FAIL because the package still exposes `OpenMySQL(string) (*sql.DB, error)`.

- [x] **Step 3: Add GORM dependencies and implement the registry**

Use:

```go
const defaultDatabaseName = "primary"

var (
    mu       sync.RWMutex
    db       *gorm.DB
    namedDBs map[string]*gorm.DB
)
```

Production `Initialize` constructs a MySQL DSN from split fields, opens every configured database, applies pool settings to the underlying `*sql.DB`, pings it, and publishes the complete immutable map only after all opens succeed. `DB()` must panic with a clear initialization error if called before Bootstrap initializes it.

- [x] **Step 4: Convert migration execution without changing SQL**

Change the migration runner to accept `*gorm.DB`, use `Transaction`, and execute existing migration SQL with `tx.Exec`. Do not edit migration SQL or table definitions.

- [x] **Step 5: Run database and migration tests**

Run:

```bash
go test ./internal/infra/database/... -count=1
go test -race ./internal/infra/database/... -count=1
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add go.mod go.sum internal/infra/database
git commit -m "refactor: manage mysql through private gorm registry"
```

### Task 4: Implement Independent Logger and Observability Resources

**Files:**
- Rewrite: `internal/infra/logger/logger.go`
- Rewrite: `internal/infra/logger/logger_test.go`
- Modify: `internal/infra/metrics/metrics.go`
- Create: `internal/infra/metrics/metrics_test.go`
- Modify: `internal/infra/tracing/tracing.go`
- Create: `internal/infra/tracing/tracing_test.go`
- Modify: `internal/middleware/request_context.go`
- Modify: `internal/middleware/request_context_test.go`

**Interfaces:**
- Produces: `logger.Initialize(config.LoggerConfigs) error`, six typed logger getters, and `logger.Close() error`.
- Produces: Noop-safe `metrics.Get() Recorder` and `tracing.Get() Tracer`.

- [x] **Step 1: Write failing six-file logger tests**

Use a temporary log directory and verify:

```go
func TestInitializeCreatesSixIndependentLogFiles(t *testing.T)
func TestTypedGettersReturnDistinctLoggers(t *testing.T)
func TestRequestLoggerContainsTraceRequestAndModule(t *testing.T)
func TestCloseFlushesAndClosesEveryFile(t *testing.T)
func TestMetricsAndTracingDefaultToNoop(t *testing.T)
```

- [x] **Step 2: Run tests and verify failure**

Run: `go test ./internal/infra/logger ./internal/infra/metrics ./internal/infra/tracing ./internal/middleware -count=1`

Expected: FAIL because the current logger writes only to stdout and has no typed global getters.

- [x] **Step 3: Implement the resource APIs**

Expose only:

```go
func App() *slog.Logger
func Access() *slog.Logger
func Job() *slog.Logger
func External() *slog.Logger
func Audit() *slog.Logger
func Panic() *slog.Logger
```

Keep file handles and logger instances private. Make initialization atomic: do not publish any logger until all six files open successfully; close already-opened files on failure. Update request middleware to derive correlation fields from `logger.Access()`.

- [x] **Step 4: Run tests**

Run: `go test -race ./internal/infra/logger ./internal/infra/metrics ./internal/infra/tracing ./internal/middleware -count=1`

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/infra/logger internal/infra/metrics internal/infra/tracing internal/middleware
git commit -m "refactor: add typed cloud observability resources"
```

### Task 5: Separate Client Transport and Credential Resources

**Files:**
- Modify: `internal/infra/client/agent/client.go`
- Modify: `internal/infra/client/agent/client_test.go`
- Modify: `internal/infra/client/platforms/douyin/client.go`
- Create: `internal/infra/client/platforms/douyin/client_test.go`
- Modify: `internal/modules/proxy/service/agent_client.go`
- Modify: `internal/modules/contentpool/service/discovery_crawler.go`
- Modify: `internal/modules/contentpool/service/discovery_crawler_test.go`

**Interfaces:**
- Produces: `agent.Initialize(config.ClientConfig, config.AgentCredentialConfig) error`, `agent.Get() *Client`.
- Produces: `douyin.Initialize(config.ClientConfig, config.DouyinCredentialConfig) error`, `douyin.Get() *Client`.
- Produces typed methods `Search`, `FindAuthor`, `FetchByURL`; removes production use of generic `Post` outside the client package.

- [x] **Step 1: Write failing transport/credential tests**

Test with `httptest.Server`:

```go
func TestAgentClientAddsCredentialWithoutBusinessPassingHeaders(t *testing.T)
func TestDouyinSearchBuildsTypedRequest(t *testing.T)
func TestDouyinFindAuthorBuildsTypedRequest(t *testing.T)
func TestDouyinFetchByURLBuildsTypedRequest(t *testing.T)
func TestClientRetriesOnlyConfiguredTransportFailures(t *testing.T)
func TestClientMapsNonSuccessStatus(t *testing.T)
```

- [x] **Step 2: Run Client tests and verify failure**

Run: `go test ./internal/infra/client/... -count=1`

Expected: FAIL because credentials are embedded in Client config and Douyin exposes generic `Post`.

- [x] **Step 3: Implement private initialized clients and typed calls**

Keep constructors accepting an injected `*http.Client` for tests. Production `Initialize` builds the HTTP transport from host, port, timeout, and retry configuration, then combines separately loaded credentials internally. Business request types must not contain Cookie, API Key, or arbitrary Header maps.

- [x] **Step 4: Update current consumers**

Replace `NewAgentChecker(rt.Clients.Agent)` and Douyin crawler injection with `agent.Get()` and `douyin.Get()`. Keep Cloud direct Douyin access and Agent-only local execution boundaries.

- [x] **Step 5: Run Client and consumer tests**

Run:

```bash
go test ./internal/infra/client/... -count=1
go test ./internal/modules/proxy/service ./internal/modules/contentpool/service -count=1
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/infra/client internal/modules/proxy/service/agent_client.go internal/modules/contentpool/service/discovery_crawler.go internal/modules/contentpool/service/discovery_crawler_test.go
git commit -m "refactor: separate client transport and credentials"
```

### Task 6: Replace Runtime/App with Process Bootstrap and Cmd Server Lifecycle

**Files:**
- Rewrite: `internal/bootstrap/bootstrap.go`
- Create: `internal/bootstrap/resource.go`
- Rename/Rewrite: `internal/bootstrap/router.go` -> `internal/bootstrap/routes.go`
- Create: `internal/bootstrap/jobs.go`
- Rewrite: `internal/bootstrap/bootstrap_test.go`
- Rewrite: `internal/bootstrap/router_test.go` -> `internal/bootstrap/routes_test.go`
- Modify: `cmd/server/main.go`
- Create: `cmd/server/server.go`
- Create: `cmd/server/server_test.go`
- Modify: `cmd/migrate/main.go`
- Modify: `cmd/discovery-scheduler/main.go`
- Modify: `cmd/discovery-worker/main.go`
- Delete: `internal/runtime/runtime.go`
- Delete: `internal/runtime/runtime_test.go`
- Delete: `internal/app/server.go`
- Delete: `internal/app/server_test.go`
- Delete: `internal/app/discovery_runtime.go`

**Interfaces:**
- Produces: `bootstrap.InitializeServer() (*hertzserver.Hertz, func() error, error)`.
- Produces: `bootstrap.InitializeScheduler() (func() error, error)`.
- Produces: `bootstrap.InitializeWorker() (func() error, error)`.
- Produces: `bootstrap.InitializeMigration() (func() error, error)`.

- [x] **Step 1: Write failing lifecycle tests**

Cover ordered initialization, rollback, idempotent close, and cmd-level stop:

```go
func TestInitializeServerInitializesResourcesInOrder(t *testing.T)
func TestInitializeServerRollsBackInReverseOrder(t *testing.T)
func TestInitializeWorkerDoesNotInitializeHTTPOnlyResources(t *testing.T)
func TestInitializeSchedulerDoesNotInitializeDouyinClient(t *testing.T)
func TestServerRunClosesEngineBeforeBootstrapResources(t *testing.T)
```

- [x] **Step 2: Run lifecycle tests and verify failure**

Run: `go test ./internal/bootstrap ./cmd/server -count=1`

Expected: FAIL because Bootstrap still returns Runtime/App wrappers and starts Scheduler jobs from route registration.

- [x] **Step 3: Implement Bootstrap entry points**

`bootstrap.go` controls the sequence; `resource.go` contains private helpers and a reverse-order close stack. `routes.go` installs Middleware and calls module route functions only. `jobs.go` exposes private Scheduler/Worker job connection helpers and never starts them for HTTP Server.

- [x] **Step 4: Implement `cmd/server/server.go`**

Keep `main.go` minimal:

```go
func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}
```

`run` initializes through Bootstrap, starts Hertz, handles `SIGINT`/`SIGTERM`, shuts down HTTP first, and then calls the idempotent resource closer.

- [x] **Step 5: Update non-HTTP commands and remove Runtime/App**

Each command calls only its matching Bootstrap initializer. Scheduler and Worker read their already validated intervals and batch size from `config.Get()` after Bootstrap initializes Config; they do not reload files. Delete `internal/runtime` and `internal/app` after all imports are gone.

- [x] **Step 6: Run lifecycle tests and a compile sweep**

Run:

```bash
go test ./internal/bootstrap ./cmd/... -count=1
go test ./... -run '^$'
rg -n 'internal/(runtime|app)|NewRuntime|NewDiscoveryRuntime' --glob '*.go'
```

Expected: tests and compile pass; `rg` prints nothing.

- [x] **Step 7: Commit**

```bash
git add internal/bootstrap cmd internal/runtime internal/app
git commit -m "refactor: centralize process bootstrap lifecycle"
```

### Task 7: Migrate Identity and Cloud-Agent Foundation Modules

**Files:**
- Modify/Create/Delete within: `internal/modules/identity/**`
- Modify/Create/Delete within: `internal/modules/cloudagent/**`

**Interfaces:**
- Produces root functions `identity.RegisterRoutes(*server.Hertz)` and authentication helpers without Service parameters.
- Produces package-level Identity operations matching the current Service method names.
- Produces package-level Cloud-Agent registry/task operations and keeps contract constants unchanged.

- [x] **Step 1: Move type-source tests to `model` and `dto`**

Add compile-time tests proving `model.User`, `model.PublicUser`, `model.Task`, and DTO request types are declarations, not aliases back to Service. Keep contract values and JSON tags unchanged.

- [x] **Step 2: Run module tests and verify the type tests fail**

Run: `go test ./internal/modules/identity/... ./internal/modules/cloudagent/... -count=1`

Expected: FAIL while root routers still re-export Service types.

- [x] **Step 3: Convert repositories to package functions**

Public repository functions obtain `database.DB()` and call private DB-accepting implementations, for example:

```go
func FindUserByUsername(username string) (model.User, bool, error) {
    return findUserByUsername(database.DB(), username)
}

func findUserByUsername(db *gorm.DB, username string) (model.User, bool, error)
```

Use `go-sqlmock` for repository tests and remove constructors that only copy the same database pointer.

- [x] **Step 4: Convert services to package functions**

Preserve current operation names and behavior: Identity includes `BootstrapAdmin`, `CreateUser`, `Login`, `Authenticate`, team/game/user administration and audit operations. Cloud-Agent includes compatibility checks, registration, heartbeat, task create/claim/report/cancel, and task type/status validation. Private helpers may accept clocks/generators for deterministic tests.

- [x] **Step 5: Move handlers to module root and simplify routers**

Create root `handler*.go`; make `router.go` contain only direct bindings such as:

```go
func RegisterRoutes(h *server.Hertz) {
    h.POST("/api/v1/auth/login", Login)
}
```

Delete `handler/` after tests move to the root package. Remove root aliases and constructor exports.

- [x] **Step 6: Run tests and boundary scan**

Run:

```bash
go test ./internal/modules/identity/... ./internal/modules/cloudagent/... -count=1
go test -race ./internal/modules/identity/... ./internal/modules/cloudagent/... -count=1
rg -n '= .*service\.|NewService|NewMySQL' internal/modules/identity/router.go internal/modules/cloudagent/router.go
```

Expected: tests pass; router scan prints nothing.

- [x] **Step 7: Commit**

```bash
git add internal/modules/identity internal/modules/cloudagent
git commit -m "refactor: migrate identity and cloudagent modules"
```

### Task 8: Migrate Runtime-Binding, Profile-Binding, and Profile-Guard Modules

**Files:**
- Modify/Create/Delete within: `internal/modules/runtimebinding/**`
- Modify/Create/Delete within: `internal/modules/profilebinding/**`
- Modify/Create/Delete within: `internal/modules/profileguard/**`

**Interfaces:**
- Consumes: package-level Identity and Cloud-Agent APIs from Task 7.
- Produces: package-level runtime trust, profile binding, and sensitive-operation guard functions.

- [x] **Step 1: Add failing dependency-direction and behavior tests**

Test ticket issue/register/trust, scan submit/confirm/profile ownership, and guard preflight/renew/finish. Add an import scan that rejects cross-module Repository imports.

- [x] **Step 2: Run current tests and capture failures**

Run: `go test ./internal/modules/runtimebinding/... ./internal/modules/profilebinding/... ./internal/modules/profileguard/... -count=1`

- [x] **Step 3: Move actual DTO/model declarations**

Move Binding, Profile, Scan, SensitiveTask, Permit, request, and response types out of Service. Update Service and Repository imports so `model` and `dto` never import Service.

- [x] **Step 4: Convert repositories and services**

Repositories use `database.DB()` wrappers with private `*gorm.DB` implementations. Convert these public receiver methods to same-name package functions: Runtime Binding `IssueTicket`, `RegisterLocal`, `CheckLocalTrust`, `ReportRuntime`, `AuthenticateNode`; Profile Binding `SubmitScan`, `GetScan`, confirmations, profile CRUD/ownership; Profile Guard `Preflight`, `Renew`, `Finish`.

- [x] **Step 5: Replace injected cross-module objects with one-way Service calls**

Runtime Binding may call Identity Service; Profile Binding may call Identity and Cloud-Agent Service; Profile Guard may call Runtime-Binding Service. No module may import another module's Repository.

- [x] **Step 6: Move handlers to roots and delete facades**

Register direct handler functions in each root `router.go`, delete handler subpackages, and remove aliases/constructors from root routers.

- [x] **Step 7: Verify**

Run:

```bash
go test -race ./internal/modules/runtimebinding/... ./internal/modules/profilebinding/... ./internal/modules/profileguard/... -count=1
rg -n 'modules/.*/repository' internal/modules/runtimebinding internal/modules/profilebinding internal/modules/profileguard --glob '*.go'
```

Expected: tests pass; cross-module Repository scan prints nothing.

- [x] **Step 8: Commit**

```bash
git add internal/modules/runtimebinding internal/modules/profilebinding internal/modules/profileguard
git commit -m "refactor: migrate profile runtime modules"
```

### Task 9: Migrate Media-Account and Proxy Modules

**Files:**
- Modify/Create/Delete within: `internal/modules/mediaaccount/**`
- Modify/Create/Delete within: `internal/modules/proxy/**`
- Modify: `internal/jobs/proxy_expiry.go`

**Interfaces:**
- Consumes: Identity, Cloud-Agent, Runtime-Binding, Profile-Binding, Profile-Guard Service functions and `agent.Get()`.
- Produces: package-level account/proxy operations and Job-compatible expiry operation.

- [x] **Step 1: Add failing account/proxy boundary tests**

Cover account CRUD/identification/check/cookie/profile/tag/group flows; proxy parse/import/CRUD/status/check/quota flows; verify Agent calls remain behind the typed Agent Client.

- [x] **Step 2: Run module tests**

Run: `go test ./internal/modules/mediaaccount/... ./internal/modules/proxy/... ./internal/jobs -count=1`

- [x] **Step 3: Move DTO/model ownership and convert repositories**

Move actual Account, AccountGroup, ProxyConfig and request/filter types to `model`/`dto`. Convert SQL stores to `database.DB()` package functions and private GORM helpers. Preserve SQL and error mapping.

- [x] **Step 4: Convert Service receivers to package functions**

Keep every current public Media Account operation (`CreateAccount` through account-group queries) and Proxy operation (`BulkParse` through `SetMaxProfileCount`) with the same inputs/results, adjusted only to import `model`/`dto` types. Replace resolver/options injection with the approved one-way Service calls.

- [x] **Step 5: Move root handlers and simplify routers**

Eliminate `deps ...any` from Proxy routing. Each Router binds named handler functions only; handlers call package-level Service functions.

- [x] **Step 6: Update proxy expiry Job**

Make the Job call Proxy Service functions and `logger.Job()`; it must not accept raw DB or Logger parameters.

- [x] **Step 7: Verify**

Run:

```bash
go test -race ./internal/modules/mediaaccount/... ./internal/modules/proxy/... ./internal/jobs -count=1
rg -n 'deps \.\.\.any|NewService|NewMySQLStore|\*Service' internal/modules/mediaaccount internal/modules/proxy --glob '*.go'
```

Expected: tests pass; scan prints nothing in production code.

- [x] **Step 8: Commit**

```bash
git add internal/modules/mediaaccount internal/modules/proxy internal/jobs/proxy_expiry.go
git commit -m "refactor: migrate account and proxy modules"
```

### Task 10: Migrate Content Pool and Separate Scheduler/Worker Execution

**Files:**
- Modify/Create/Delete within: `internal/modules/contentpool/**`
- Modify: `internal/jobs/discovery.go`
- Modify: `internal/jobs/discovery_test.go`
- Modify: `internal/scheduler/scheduler.go`
- Modify: `internal/scheduler/scheduler_test.go`
- Modify: `cmd/discovery-scheduler/main.go`
- Modify: `cmd/discovery-worker/main.go`
- Modify: `internal/bootstrap/jobs.go`

**Interfaces:**
- Produces: package-level Content Pool CRUD, discovery strategy, `RunDue(time.Time)`, and `RunNext(context.Context)` functions.
- Produces: Scheduler discovery-only command and Worker execution-only command.

- [ ] **Step 1: Add failing Scheduler/Worker responsibility tests**

Verify:

```go
func TestRunDueOnlyCreatesPendingTasks(t *testing.T)
func TestRunNextClaimsOneTaskAndCallsCrawlerSynchronously(t *testing.T)
func TestHTTPBootstrapDoesNotStartSchedulerOrWorker(t *testing.T)
func TestSchedulerCommandNeverCallsDouyin(t *testing.T)
func TestWorkerUsesConfiguredIntervalAndBatchSize(t *testing.T)
```

- [ ] **Step 2: Run tests and verify current composition failure**

Run: `go test ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./internal/bootstrap -count=1`

Expected: FAIL while HTTP route bootstrap starts scheduled jobs and service structs remain.

- [x] **Step 3: Move Content Pool DTO/model types and repository functions**

Move SourceContent, Material, Strategy, CrawlTask and CrawlStats declarations to Model; move filters/inputs to DTO. Convert both stores to package-level wrappers over private GORM functions.

- [x] **Step 4: Convert Content Pool and Discovery services**

Keep package functions for content CRUD/materialization, strategy CRUD/status/manual scheduled run, `RunDue`, `RunNext`, task list/get, and historical confirmation. Scheduled execution uses `douyin.Get()` synchronously inside the Worker path.

- [x] **Step 5: Enforce process separation**

Scheduler loads config through `InitializeScheduler`, runs `RunDue`, and creates tasks only. Worker loads through `InitializeWorker`, atomically claims pending tasks, runs the crawler synchronously, persists outcomes, and honors configured batch size. Remove command flags that override validated config unless they are diagnostic `--once` controls.

- [x] **Step 6: Move handlers to module root**

Root router files bind named handler functions. No route registers or starts Scheduler jobs.

- [x] **Step 7: Verify (reduced at user request)**

Run:

```bash
go test -race ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./cmd/discovery-scheduler ./cmd/discovery-worker -count=1
rg -n 'time\.NewTicker' internal cmd --glob '*.go'
```

Expected: tests pass; only `internal/scheduler` contains `time.NewTicker`.

User-requested reduced verification recorded on 2026-09-19: no `go test` or
race run. Module `go build`, `go vet`, `git diff --check`, and the ticker and
production-boundary scans were run instead. Command-package build remains
blocked by pre-existing transitional references in `internal/bootstrap/routes.go`.

- [x] **Step 8: Commit**

```bash
git add internal/modules/contentpool internal/jobs internal/scheduler internal/bootstrap/jobs.go cmd/discovery-scheduler cmd/discovery-worker
git commit -m "refactor: separate discovery scheduling and execution"
```

### Task 11: Implement Synchronous Douyin Search and Result Import

**Files:**
- Modify: `internal/modules/contentpool/router.go`
- Modify/Create: `internal/modules/contentpool/handler*.go`
- Modify: `internal/modules/contentpool/service/*.go`
- Modify: `internal/modules/contentpool/dto/dto.go`
- Modify: `internal/modules/contentpool/service/*_test.go`
- Modify: `web/src/shared/api/discovery.js`
- Modify: `web/src/modules/contentpool/pages/ContentPoolPage.vue`
- Modify: `web/src/modules/contentpool/pages/ContentPoolPage.test.js`

**Interfaces:**
- Produces: synchronous `POST /api/v1/content-pool/search` and `/author-search` response `{items: [...]}`.
- Produces: `POST /api/v1/content-pool/import-results` for validated selected-result ingestion.
- Preserves: historical Crawl Task list/get/confirm APIs.

- [ ] **Step 1: Write failing Cloud API tests**

Verify search returns results in the same request, no Crawl Task is inserted, and import validates scope and deduplicates:

```go
func TestKeywordSearchReturnsItemsWithoutCreatingTask(t *testing.T)
func TestAuthorSearchReturnsItemsWithoutCreatingTask(t *testing.T)
func TestImportResultsRejectsInvalidPlatformOrSource(t *testing.T)
func TestImportResultsUsesExistingContentPoolDeduplication(t *testing.T)
func TestHistoricalManualTaskConfirmationStillWorks(t *testing.T)
```

- [ ] **Step 2: Run Content Pool tests and verify failure**

Run: `go test ./internal/modules/contentpool/... -count=1`

Expected: FAIL because current search creates `manual_discovery_task`.

- [ ] **Step 3: Implement synchronous Service functions and handlers**

Use these DTO shapes:

```go
type SearchResult struct {
    PlatformContentID string `json:"platform_content_id"`
    Title string `json:"title"`
    Description string `json:"description"`
    CoverURL string `json:"cover_url"`
    SourceURL string `json:"source_url"`
    AuthorID string `json:"author_id"`
    AuthorName string `json:"author_name"`
    PublishedAt *time.Time `json:"published_at,omitempty"`
}

type ImportResultsRequest struct {
    Platform string `json:"platform"`
    Items []SearchResult `json:"items"`
}
```

Search calls the typed Douyin Client directly and returns normalized items. Import treats the payload as untrusted, rechecks actor/team scope and required fields, then calls existing create/deduplication logic. Do not create a search task.

- [ ] **Step 4: Write failing Web tests**

Assert `ContentPoolPage.vue` does not call `getTask` or poll, displays returned items immediately, and sends selected items to `importResults`.

- [ ] **Step 5: Update Web source only**

Change `discovery.search` and `authorSearch` consumers to read `result.items`; add `importResults(data)`; remove `waitForSearchTask`; keep Crawl Task pages for scheduled/history records. Do not build or modify `web/dist-*`.

- [ ] **Step 6: Verify Cloud and Web**

Run:

```bash
go test ./internal/modules/contentpool/... -count=1
npm --prefix web test -- --run src/modules/contentpool/pages/ContentPoolPage.test.js
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/modules/contentpool web/src/shared/api/discovery.js web/src/modules/contentpool/pages/ContentPoolPage.vue web/src/modules/contentpool/pages/ContentPoolPage.test.js
git commit -m "feat: make douyin discovery search synchronous"
```

### Task 12: Remove Transitional Layers and Complete Verification

**Files:**
- Delete any remaining obsolete handler directories, router facades, Runtime/App compatibility files, duplicated tests, and unused config files.
- Modify: `README.md` and applicable docs only when commands or paths changed.

**Interfaces:**
- Consumes: all previous task outputs.
- Produces: one clean, buildable modular-monolith architecture matching the spec.

- [ ] **Step 1: Run structural scans and remove only confirmed dead code**

Run:

```bash
rg -n 'internal/(runtime|app)|runtime\.Runtime|NewRuntime|NewService|NewMySQLStore|deps \.\.\.any' --glob '*.go'
find internal/modules -type d -name handler -print
rg -n 'http\.Client\s*\{|os\.(Getenv|LookupEnv)\(|time\.NewTicker\(' internal/modules --glob '*.go'
```

Expected before cleanup: remaining transitional references are listed. Delete or migrate each listed production reference; do not delete historical migrations or compatibility APIs still covered by tests.

- [ ] **Step 2: Format and tidy**

Run:

```bash
gofmt -w cmd internal
go mod tidy
```

- [ ] **Step 3: Run complete Go verification**

Run:

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

Expected: all commands exit 0.

- [ ] **Step 4: Run Web verification without rebuilding tracked distributions**

Run:

```bash
npm --prefix web test
npm --prefix web run build:cloud -- --outDir /private/tmp/wt-media-cloud-build-cloud
npm --prefix web run build:desktop -- --outDir /private/tmp/wt-media-cloud-build-desktop
git status --short web/dist-cloud web/dist-desktop
```

Expected: tests and both source builds pass; tracked `web/dist-*` status is unchanged from the pre-task baseline.

- [ ] **Step 5: Run final boundary and hygiene checks**

Run:

```bash
go test ./internal/architecture -count=1
gofmt -l cmd internal
git diff --check
rg -n 'internal/(runtime|app)|runtime\.Runtime|NewRuntime' --glob '*.go'
rg -n 'time\.NewTicker' internal cmd --glob '*.go'
```

Expected: all tests/checks pass; `gofmt -l` prints nothing; no Runtime/App references; only `internal/scheduler` owns ticker creation.

- [ ] **Step 6: Review the final diff by responsibility**

Confirm the diff contains no database schema changes, unrelated build artifacts, secret material outside approved config files, or accidental route/error-code changes. Record any intentionally preserved compatibility code in the final report.

- [ ] **Step 7: Commit**

```bash
git add AGENTS.md README.md cmd config config_online docs internal go.mod go.sum web/src
git commit -m "refactor: complete cloud foundation convergence"
```

## Completion Report

After Task 12, report:

- exact commits and modified areas per task;
- final directory and process lifecycle;
- business/API behavior changes limited to synchronous search;
- `go test`, Race, Vet, Web test/build, formatting, boundary-scan, and diff-check results;
- any test deliberately using Mock instead of real MySQL;
- any pre-existing unrelated dirty files left untouched.
