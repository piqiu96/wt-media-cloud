# WT Media Cloud 架构阶段基线（2026-09-20）

- 基线时间：2026-09-20 22:12:09 CST
- 基线提交：`856482a`（`merge: infrastructure bootstrap convergence`）
- 基线状态：基础设施收敛已完成，当前进入历史债务清理与模块边界矫正阶段。
- 审查范围：Go 生产代码、配置目录、架构测试、脚本与文档；不修改运行时代码。
- 阶段决策：项目尚未上线，不保留额外历史兼容；除 Cloud–Agent 当前契约能力外，纯历史兼容逻辑按理想态清理。
- 成本边界：核对以定向扫描、既有验证结果和最小必要测试为准；不重复全量测试，不为了清理历史债务扩大测试面。
- 结论：整体 Bootstrap、Config、Logger、Hertz Client 基础设施方向已经符合 V2 架构；遗留问题主要集中在安全日志、测试竞态、已删除数据库列的残留写入、跨模块 Model/Service 依赖、历史兼容投影、占位配置和历史文档漂移。

## 1. 当前架构状态

### 1.1 进程与启动

当前存在四个进程入口：

```text
cmd/server
cmd/migrate
cmd/discovery-scheduler
cmd/discovery-worker
```

启动职责已经收敛到 `internal/bootstrap`：

- `InitializeServer`
- `InitializeScheduler`
- `InitializeWorker`
- `InitializeMigration`

Server 初始化顺序为：

1. Config
2. Logger 与 Hertz Logger
3. Metrics
4. Tracing
5. Database
6. HTTP Client
7. Agent / Douyin 语义 Client
8. Hertz Server
9. Routes

其他进程只初始化自身所需资源：

- Migration：Config、Logger、Database。
- Scheduler：Config、Logger、Database、Scheduler 资源，不初始化外部 HTTP Client。
- Worker：Config、Logger、Database、Douyin HTTP/语义 Client。

初始化失败按依赖逆序回滚，关闭操作保持幂等。`cmd/server/main.go` 只调用同包运行函数，`cmd/server/server.go` 负责 HTTP 生命周期、信号与优雅关闭。

### 1.2 配置

运行时配置统一位于 `config/`，发布替换目录为 `config_online/`。当前配置已经迁移为 TOML：

```text
config/app.toml
config/database/primary.toml
config/scheduler/scheduler.toml
config/logger/*.toml
config/clients/http/*.toml
config/credentials/*.toml
```

分层符合设计：

- `pkg/config`：通用文件、目录、递归目录加载，支持 YAML/JSON/TOML，不绑定业务。
- `internal/config`：Cloud 配置 Schema、校验与只读 Getter。
- `config/clients/http/`：连接、超时、连接池、重试等连接配置。
- `config/credentials/`：Cookie、API Key、固定 Header 等认证配置。

当前不符合项：

- `config/cache/redis.toml` 与 `config_online/cache/redis.toml` 是未实现 Redis 的占位配置。
- `config/observability/health.toml` 与 `config_online/observability/health.toml` 中的 `health_path` 已加载但未使用，健康路由实际固定注册为 `/healthz` 与 `/api/v1/health`。
- 本地 `config/credentials/douyin.toml` 包含真实 API Key 与超长 Cookie，并已被 Git 跟踪；`config_online` 对应文件为空。这是当前阶段最重要的安全与发布策略问题。

### 1.3 日志

`pkg/logger` 只提供通用 Logger 创建能力；`internal/infra/logger` 负责 Cloud 语义映射。当前六类物理日志为：

```text
app.log
access.log
job.log
external.log
audit.log
panic.log
```

每类日志同时具备普通日志与 `.wf` 日志；`.wf` 记录 WARN 及以上级别。上下文自动注入：

- `trace_id`
- `request_id`
- `user_id`

Hertz 集成为：

```go
hlog.SetLogger(logger.App())
hlog.SetSystemLogger(logger.Panic())
```

Access 日志在 `internal/middleware/request_context.go` 中接入。

当前不符合项：

- `internal/jobs/proxy_expiry.go` 使用 `Info` 按结构化 key/value 形式传参，但 Hertz `FullLogger.Info` 是 printf 风格，实际输出语义错误。应改为 `Infof` 或 `CtxInfof`，并保持字段格式稳定。
- `internal/infra/client/observe/observe.go` 记录 `request.URI().String()`。抖音请求将 `apiKey` 放在 query string 中，因此 External 日志存在泄漏凭证风险。

### 1.4 HTTP Client

HTTP 基础设施已经复用 Hertz Client：

- `pkg/clients/http`：窄类型 HTTP Client Registry，只负责配置到 Hertz Client Option 的转换。
- `internal/infra/client`：Agent 与 Douyin 语义 Client，负责业务协议、错误映射、日志、trace 与 metrics。
- 业务模块生产代码没有直接创建 `http.Client`，也没有导入 `pkg/clients/http`。
- 重试发生在 transport 层，HTTP 非 2xx/3xx 业务响应不重复提交。
- `trace_id`、`request_id`、`user_id` 通过 `context.Context` 传递。

当前主要问题是 External 日志包含完整 URI，可能带出 Douyin API Key。

### 1.5 Module 边界

目标结构为：

```text
modules/{module}/
├── router.go
├── handler.go
├── dto/
├── service/
├── repository/
└── model/
```

已符合：

- Handler 位于模块根包，未发现 Handler Struct 或 `...any` 依赖查找。
- Repository 通过 `database.DB()` / `database.Named()` 获取连接。
- 生产代码未发现 `fmt.Println` 运行日志。
- 未发现业务模块启动后台任务或 `time.NewTicker`。
- 未发现业务模块导入 `pkg/config`、`pkg/logger`、`pkg/clients/http`。
- 未发现业务代码直接创建 `net/http.Client`。
- `context.WithValue` 只集中在 `internal/shared/requestctx`。

遗留边界债务：

1. 多个模块直接依赖 `identity/model`，包括：
   - `contentpool`
   - `mediaaccount`
   - `profilebinding`
   - `profileguard`
   - `runtimebinding`
2. 多个 Handler 直接导入 `internal/modules/identity` 根包，使用其认证/上下文 helper，而不是统一中间件或明确的 Identity Service API。
3. `profileguard/repository/store_mysql.go` 导入 `runtimebinding/service`，形成 Repository -> 外部 Service 的反向依赖。
4. 多个 Repository 导入本模块 DTO，Repository 层与 API DTO 耦合偏紧：
   - `cloudagent`
   - `contentpool`
   - `mediaaccount`
   - `profileguard`
   - `proxy`
   - `runtimebinding`

这些不是立即会导致编译失败的问题，但会阻碍后续 Cloud/Agent/Worker 复用和模块边界稳定，应分阶段矫正。

### 1.6 Scheduler、Job 与 Worker

当前职责划分符合基线：

```text
Scheduler -> 发现到期策略 -> 创建 pending task
Worker    -> 原子领取 task -> 调用 Job -> 同步执行外部请求 -> 持久化结果
```

- HTTP Server 不启动 Scheduler 或 Worker。
- 只有 `internal/scheduler` 在生产代码中使用 `time.NewTicker`。
- Job 不拥有调度周期。

### 1.7 Observability

Metrics 与 Tracing 当前为 Noop 实现。这是本阶段的刻意状态，不是废弃代码：

- 初始化顺序已经接入 Bootstrap。
- 尚未接入真实后端，也未完成业务埋点。
- 后续引入 OpenTelemetry 或其他后端时，应只替换 `internal/infra/metrics` 与 `internal/infra/tracing` 的实现，不改业务调用边界。

## 2. 清理基线

### P0：必须先处理

| 编号 | 问题 | 位置 | 处理建议 |
| --- | --- | --- | --- |
| A-01 | External HTTP 日志可能泄漏 Douyin `apiKey` | `internal/infra/client/observe/observe.go` | 不记录完整 URI/query；仅记录 method、host/path、status、duration、client name；增加日志脱敏测试 |
| A-02 | 测试存在数据竞态 | `internal/modules/contentpool/service/discovery_crawler_test.go` | Mock server handler 中 `batches` 与 `shortCalls` 需要加锁或改用 channel；修复后仅收尾执行一次 `go test -race ./...` |
| A-03 | 真实 Douyin 凭证被 Git 跟踪，且 `config_online` 凭证为空 | `config/credentials/douyin.toml`、`config_online/credentials/douyin.toml` | 阶段策略已确认允许凭证写入配置文件；本地配置继续服务开发，线上凭证由部署注入。由于真实值已进入 Git 历史，上线前必须轮换，不建议把凭证迁移混入本轮架构清理 |
| A-04 | Identity 写入已删除的 `users.legacy_id` 列 | `internal/modules/identity/repository/store_mysql.go`；`migrations/20260806_023_drop_legacy_string_ids.sql` | 迁移 023 已删除 `users.legacy_id`，但 `createUserTx` 仍插入该列，创建用户会在当前结构上失败；按理想态删除 `legacy_id` 写入并补核心链路测试 |

### P1：本阶段架构矫正

| 编号 | 问题 | 位置 | 处理建议 |
| --- | --- | --- | --- |
| B-01 | 跨模块直接依赖 `identity/model` | `contentpool`、`mediaaccount`、`profilebinding`、`profileguard`、`runtimebinding` | 将确实稳定的 `UserID`、`TeamID`、公开用户契约迁移到明确的 shared contract，或统一通过 Identity Service API 暴露 |
| B-02 | Repository 依赖外部 Service | `internal/modules/profileguard/repository/store_mysql.go` | Repository 只接收本地模型或 primitive 字段；跨模块 Service 调用上移到 profileguard Service |
| B-03 | Handler 直接导入 identity 根包 | 多个模块 Handler | 认证 helper 迁移到 middleware 或明确的 Identity Service API，模块根包保持只暴露自身路由 |
| B-04 | Repository 导入 DTO | 多个模块 Repository | 分阶段改为 persistence model/local input；Handler/Service 负责转换 |
| B-05 | Redis 占位配置仍存在 | `config/cache/redis.toml`、`config_online/cache/redis.toml` | 按已确认决策删除文件、`CacheConfig`/`RedisConfig`、加载逻辑与测试 |
| B-06 | Health 配置加载但未使用 | `config/observability/health.toml`、`config_online/observability/health.toml` | 推荐删除配置与 Schema；健康路径固定，避免不必要配置面 |
| B-07 | 本地存在禁止或多余空目录 | `internal/app`、`internal/runtime`、`internal/infra/config` | 删除空目录并防止再次创建；配置职责只保留在 `internal/config` 与 `pkg/config` |
| B-08 | 依赖分类不准确 | `go.mod` | 执行 `go mod tidy`，将直接依赖从 indirect 修正 |
| B-09 | Hertz 日志调用签名错误 | `internal/jobs/proxy_expiry.go` | 改用 `Infof`/`CtxInfof` |
| B-10 | 架构测试对 `database/sql` 规则过宽 | `internal/architecture` | 当前 Repository 使用 `sql.ErrNoRows`、`*sql.Rows` 是合理的；应改为禁止 `sql.Open` 和直接持有 `*sql.DB`，避免大 allowlist |
| B-11 | MediaAccount 保留单游戏 `game_id` 兼容投影 | `internal/modules/mediaaccount/model/model.go`、`dto/dto.go`、`handler.go`、`service/service.go`、`repository/store_mysql.go` | 项目未上线，直接收敛到 `game_ids` 多游戏模型；删除 `GameID` 字段、请求兼容参数、`CompatibilityGameID` 与单游戏投影。定向扫描显示 Cloud 前端未直接消费 MediaAccount 的 `game_id`，不需要为了该清理扩大前端改动 |
| B-12 | Discovery Worker 保留无 `operation` 快照的历史任务回退 | `internal/modules/contentpool/service/discovery.go` | 按理想态要求任务快照必须显式携带 `operation`；删除历史回退与对应 legacy 测试，避免继续扩大旧数据兼容 |

### P2：文档、脚本与低风险清理

| 编号 | 问题 | 位置 | 处理建议 |
| --- | --- | --- | --- |
| C-01 | 文档仍描述已废弃环境变量 | `README.md`、`scripts/README.md` | 删除或更新 `WT_MEDIA_MYSQL_DSN`、初始化管理员、Cookie secure 等旧说明 |
| C-02 | 脚本健康检查地址与 TOML 配置可能漂移 | `scripts/*.sh` | 脚本读取固定本地配置或显式说明变量只作用于脚本，不再宣称控制服务监听地址 |
| C-03 | README 架构描述过期 | `README.md` | 删除 `internal/app`、对象存储占位等内容，更新为 V2 目录与 Bootstrap 说明 |
| C-04 | Go 版本表述不精确 | `README.md`、`go.mod` | 区分模块最低版本 `go 1.24.0` 与本地验证工具链 `go1.26.5` |
| C-05 | 未使用 helper | `internal/bootstrap/routes.go` 的 `configServerAddr` | 删除 |
| C-06 | 死代码标记 | `internal/modules/mediaaccount/dto/dto.go` 的 `var _ = time.Time{}` | 删除并清理 import |

## 3. 保留判断与兼容清理原则

阶段原则：项目尚未上线，除当前真实契约能力外，不为历史数据、旧 API 字段或旧任务形态额外保留兼容层。

1. **Metrics / Tracing Noop 实现**
   - 位置：`internal/infra/metrics`、`internal/infra/tracing`
   - 判断：保留。
   - 原因：初始化边界已完成，后端尚未接入，属于阶段策略，不是历史兼容。

2. **CloudAgent Contract 版本协商**
   - 位置：`internal/modules/cloudagent/service/compatibility.go`
   - 判断：保留。
   - 原因：这是当前 Cloud–Agent Contract 的注册与握手能力，不是旧版本兼容残留；不能按历史兼容代码删除。

3. **MediaAccount 的 `CompatibilityGameID` / `game_id` 投影**
   - 位置：`internal/modules/mediaaccount` 的 model、DTO、Handler、Service、Repository。
   - 判断：清理。
   - 原因：数据库已经迁移到 `media_account_games` 多游戏关系并删除 `media_accounts.game_id`；当前 `game_id` 仅是 API/内存投影兼容。项目未上线，应直接收敛到 `game_ids`。

4. **Discovery Worker 的历史任务回退**
   - 位置：`internal/modules/contentpool/service/discovery.go`
   - 判断：清理。
   - 原因：当前新任务都会写入 `snapshot.operation`；无该字段的回退只服务历史数据，不符合未上线阶段的理想态。

5. **Service `store_adapter.go`**
   - 判断：暂保留。
   - 原因：用于 Store 接口默认实现和测试替换，不是历史兼容层。后续可在边界重构中简化，但不能按“废弃文件”批量删除。

6. **Repository 中的 `database/sql` 标准类型**
   - 判断：保留。
   - 原因：`sql.ErrNoRows`、`*sql.Rows` 用于 GORM Raw 扫描是合理的；真正应禁止的是绕过 `internal/infra/database` 直接创建或持有数据库连接。

7. **`*_test.go` 中的 `compat_test.go`**
   - 判断：暂保留或后续重命名。
   - 原因：这些文件是测试注入 seam，不参与生产兼容逻辑；文件名容易误导，可后续统一改为 `test_seams_test.go`，但不属于本阶段必须清理项。

## 4. 当前验证状态

本轮在基线提交 `856482a` 上重新验证：

| 命令 | 结果 | 结论 |
| --- | --- | --- |
| `go test ./...` | 通过 | 全部包测试通过 |
| `go vet ./...` | 通过 | 静态检查通过 |
| `go test -race ./...` | 失败 | `TestDouyinCrawlerBatchURLChunksNumericIDsAndKeepsShortURLFallback` 存在测试 Mock 数据竞态 |
| `go mod tidy -diff` | 有差异 | 直接依赖被错误标记为 indirect |

竞态详情：

- 测试主 goroutine 在断言处读取 `batches` 与 `shortCalls`。
- `httptest.Server` handler goroutine 并发写入同一批变量。
- 失败位置：`internal/modules/contentpool/service/discovery_crawler_test.go:139` 及附近写入点。
- 目前判断为测试代码竞态，不是生产链路竞态。

前序基础设施合并阶段还完成过迁移、Server、Worker 与健康检查 smoke test。本基线不将旧输出替代为当前实时验证结果。

## 5. 分批执行方案

### 第一批：正确性与确定性清理

目标是一次小变更解决明确错误和历史残留，不做大范围模块重构。

1. External HTTP 日志去掉完整 URI/query，仅保留 method、host/path、status、duration、client name；补充一条凭证不落日志的核心断言。
2. 修复 `discovery_crawler_test.go` Mock handler 的并发读写。
3. 删除 `users.legacy_id` 残留写入，保留用户创建核心链路测试。
4. 删除 Redis / Health 占位配置及对应 Schema、加载逻辑和测试期望。
5. 删除空目录、未使用 helper、死代码标记。
6. 删除 MediaAccount 单游戏投影，统一 `game_ids`。
7. 删除 Discovery Worker 无 `operation` 快照的历史回退与 legacy 测试。
8. 修正 proxy expiry 日志调用为 Hertz printf 风格。
9. 执行 `go mod tidy`。
10. 更新 README 与脚本中的旧环境变量、旧目录和旧架构描述。

第一批开发期验证：

```text
gofmt
go test ./internal/infra/client/observe
go test ./internal/modules/contentpool/service
go test ./internal/modules/identity
go test ./internal/modules/mediaaccount/...
go test ./internal/config
```

只测试直接受影响包；不重复执行全量 race。

### 第二批：模块边界矫正

第一批完成后再处理边界，避免正确性修复和架构迁移混在同一变更中。

1. 将稳定的 `UserID`、`TeamID` 与公开用户契约放入职责明确的 shared contract，例如 `internal/shared/identity`；业务用户视图继续由 Identity Service 提供。
2. 消除 `profileguard/repository -> runtimebinding/service` 反向依赖，Repository 只接收本地模型或 primitive 参数。
3. 将认证与请求身份提取收敛到 middleware 或 Identity Service API，消除 Handler 直接导入 identity 根包。
4. 分阶段解除 Repository 对 DTO 的依赖，Repository 使用 persistence model/local input。
5. 收窄架构测试：允许 `sql.ErrNoRows`、`*sql.Rows` 等标准类型，禁止模块直接 `sql.Open` 或持有 `*sql.DB`；增加跨模块 Repository/Model 边界检查。

### 收尾验证

每个批次结束时只执行一次：

```text
gofmt
go test ./...
go vet ./...
go test ./internal/architecture
git diff --check
```

第一批包含竞态修复，因此在第一批收尾时额外执行一次：

```text
go test -race ./...
```

后续批次没有新增并发逻辑时不重复全量 race，只在最终发布前再执行一次。

## 6. 已定决策与执行边界

1. **历史兼容**
   - 项目尚未上线，除 Cloud–Agent 当前契约能力外，不为旧数据、旧字段和旧任务形态保留额外兼容。
   - MediaAccount 直接收敛 `game_ids`；Discovery 任务快照必须显式携带 `operation`。

2. **凭证**
   - 已确认允许 Cookie/API Key 写入配置文件。
   - 本地 `config/credentials` 服务开发；`config_online` 保持部署注入模式。
   - 真实凭证已进入 Git 历史，上线前必须轮换；凭证迁移不混入本轮架构清理。

3. **Health 配置**
   - 删除 `observability/health.toml`。
   - 保留固定 `/healthz` 与 `/api/v1/health`，不扩大配置面。

4. **Identity 共享类型**
   - 稳定标识符与公开契约放 `internal/shared/identity`。
   - `internal/shared/id` 继续只负责进程内 opaque ID/token 生成，不混入用户语义。
   - 业务用户视图仍通过 Identity Service 暴露。

5. **测试成本**
   - 开发期只跑受影响包。
   - 全量测试、vet、架构测试和 diff 检查每批收尾执行一次。
   - race 测试只在与并发修复相关的批次和最终发布前执行，不反复跑细致测试。

6. **变更顺序**
   - 先做第一批正确性与确定性清理。
   - 再做第二批模块边界矫正。
   - 不把两批混合成一个大而全重构。
