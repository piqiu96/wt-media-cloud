# Infrastructure Bootstrap Convergence Design

## Status

Approved for implementation.

## Scope

This design converges configuration loading, structured logging, request context propagation, and outbound HTTP clients without introducing a dynamic resource registry, MQ, plugins, or workflow engine.

The implementation keeps Cloud as the business fact center. Redis and OSS are intentionally out of scope because no current production path uses them. MySQL remains owned by `internal/infra/database`; it is not exposed as a generic business client.

## Approved Decisions

- Runtime configuration stays in `config/`; release replacement stays in `config_online/`.
- Cloud configuration files migrate from YAML to TOML.
- Generic configuration loading supports YAML, JSON, and TOML, but Cloud's own runtime tree accepts TOML only.
- Six business logger names remain: `app`, `access`, `job`, `external`, `audit`, and `panic`.
- Every logger writes its normal file and a `.wf` companion file.
- `.wf` receives `WARN`, `ERROR`, and `FATAL` records.
- A warning/fatal path is `<path>.wf`; for example `logs/app.log` also writes `logs/app.log.wf`.
- Logger formats are `json`, `console`, and compatible `text` (decoded as console).
- Logging uses Hertz's `hlog.FullLogger` contract through `github.com/hertz-contrib/logger/zap`; Zap cores and lumberjack provide rotation.
- HTTP clients use Hertz Client, not per-client `net/http` implementations.
- HTTP registry entries are filename-derived and strongly typed: `httpclient.Get("douyin")`.
- No universal cross-type client registry is allowed.
- `trace_id`, `request_id`, and `user_id` propagate through Go context and are automatically attached to context logs.
- `task_id` remains business-specific and is not added to generic context.
- OSS and Redis clients and configuration directories are not created.

## Package Boundaries

### `pkg/config`

`pkg/config` is a generic, stateless file and directory loader. It has no Cloud schema and no global getter.

```go
type Format string

type Document struct {
    Name   string
    Path   string
    Format Format
    Raw    []byte
    Data   map[string]any
}

func LoadFile(path string) (Document, error)
func LoadDir(path string) ([]Document, error)
func LoadDirRecursive(path string) ([]Document, error)
func (d Document) Decode(target any) error
```

Rules:

- Extension selects the parser: `.yaml`/`.yml`, `.json`, and `.toml`.
- `Name` is the path below the scan root without extension.
- Non-regular files and unknown extensions are rejected.
- Directory scans are sorted by relative path.
- A document name may only occur once.
- `Decode` is strict and rejects unknown fields.
- The package does not read process environment variables and does not choose `config` versus `config_online`.

`internal/config` consumes documents and remains the only owner and validator of Cloud's schema.

### `pkg/logger`

`pkg/logger` creates a generic Hertz-compatible logger from configuration. It knows configuration mechanics, not Cloud business logger names.

```go
type Config struct { /* path, level, format, rotation */ }
type Logger struct { /* Hertz FullLogger implementation */ }

func New(Config) (*Logger, error)
func (l *Logger) Close() error
```

The implementation composes:

- `github.com/hertz-contrib/logger/zap`
- `go.uber.org/zap`
- `github.com/natefinch/lumberjack/v2`

It creates two Zap cores:

1. Normal core at the configured threshold.
2. `.wf` core at `WARN` or above.

The normal file and `.wf` file rotate independently with the same rotation policy.

### `pkg/clients/http`

`pkg/clients/http` owns filename-derived, strongly typed Hertz HTTP client resources.

```go
type Client struct { /* Hertz client and request timeout */ }

func Initialize(documents []config.Document, middlewares map[string][]client.Middleware) (func() error, error)
func Get(name string) *Client
func Close() error
```

It converts explicit connection and retry settings to Hertz options. It does not understand Agent, Douyin, DeepSeek, or any business protocol. Generic observation middleware is supplied by internal infrastructure rather than making `pkg` depend on business loggers.

### `internal/infra/logger`

This package keeps the six immutable business loggers and exposes only typed getters:

```go
App() hlog.FullLogger
Access() hlog.FullLogger
Job() hlog.FullLogger
External() hlog.FullLogger
Audit() hlog.FullLogger
Panic() hlog.FullLogger
```

It initializes all loggers atomically and rolls back opened writers on failure. Bootstrap installs `App` as Hertz's default logger and `Panic` as the Hertz system logger.

### `internal/infra/client`

Existing Agent and Douyin protocol clients remain here. They stop constructing `net/http.Client` and instead consume `httpclient.Get(name)`. Business modules continue calling typed methods such as `CheckProxy`, `Search`, and `FetchByIDs`; they never fetch a generic HTTP client.

## Configuration

### Logger TOML

```toml
path = "logs/app.log"
level = "info"
format = "json"

[rotation]
max_size = 500
max_age = 30
max_backups = 10
compress = true
local_time = true
```

### HTTP client TOML

```toml
timeout = "30s"

[connection]
dial_timeout = "5s"
read_timeout = "15s"
write_timeout = "15s"
max_conns_per_host = 100
max_idle_conn_duration = "90s"
max_conn_duration = "5m"
max_conn_wait_timeout = "3s"
keep_alive = true

[retry]
attempts = 2
delay = "300ms"
max_delay = "2s"
policy = "fixed"
```

HTTP client files live under `config/clients/http/`; the filename is the instance name. Credential files remain separated under `config/credentials/`.

## Context

A small internal package owns request identity values. It writes plain-string context keys only because the selected Hertz Zap adapter reads string context keys in compatibility mode. Application code outside this package may not call `context.WithValue`.

Middleware order:

1. Access/request identity middleware attaches `trace_id` and `request_id` to both Hertz and Go contexts.
2. Identity middleware opportunistically authenticates a valid session and attaches `user_id` to both contexts.
3. Public endpoints remain public; protected handlers retain their existing authorization behavior.
4. Context logger calls automatically carry `trace_id`, `request_id`, and `user_id`.

## Bootstrap Order

Server:

1. Config
2. Logger and Hertz logger integration
3. Metrics
4. Tracing
5. Database
6. HTTP clients
7. Hertz server
8. Routes

Worker:

1. Config
2. Logger and Hertz logger integration
3. Metrics
4. Tracing
5. Database
6. HTTP clients
7. Worker loop

Scheduler:

1. Config
2. Logger and Hertz logger integration
3. Metrics
4. Tracing
5. Database
6. Scheduler loop

Migration:

1. Config
2. Logger
3. Database
4. Migration execution

Resource initialization continues to roll back in reverse order on startup failure.

## Testing Strategy

- Generic config: file, directory, recursive directory, strict decode, duplicate-name, unsupported-format, and sorted-result tests.
- Logger: thresholds, formats, `.wf` routing, rotation writer cleanup, duplicate paths, and close behavior.
- Context: trace/request/user propagation and automatic log fields.
- HTTP: option conversion, registry initialization/rollback, lookup failure, timeout, retry policy, and idle-connection close.
- Agent and Douyin: protocol request/response tests migrate from `httptest.Server` over `net/http` to a Hertz-compatible local server while preserving existing request assertions.
- Bootstrap: process-specific order and rollback tests remain authoritative.
- Final gates: `go test ./...`, `go vet ./...`, and architecture boundary scans.
