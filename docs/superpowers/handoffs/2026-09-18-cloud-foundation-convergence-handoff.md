# WT Media Cloud 基础运行架构收敛交接记录

更新日期：2026-09-19

状态：Task 1–12 已完成。Task 12 已完成全量 Go、race、Vet、Web 与边界验收；未触碰既有无关工作区改动和 `web/dist-*`。

仓库绝对路径：

```text
/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud
```

权威设计：

```text
docs/superpowers/specs/2026-09-18-cloud-foundation-convergence-design.md
```

权威实施计划：

```text
docs/superpowers/plans/2026-09-18-cloud-foundation-convergence.md
```

当前分支：`main`

当前必须继续使用这个完整工作目录。禁止 reset、checkout、clean 或仅凭提交哈希重建干净 worktree；大量未提交/未跟踪迁移文件仍是后续任务的实际输入。

## 当前提交进度

| Task | 内容 | 最终提交 |
| --- | --- | --- |
| 1 | 架构规则、架构文档与生产代码边界扫描 | `f284956` |
| 2 | 严格 Config Schema、固定 `./config`、独立数据库/Logger/Client/Credential/Scheduler 配置 | `23774d8` |
| 3 | 私有 `*gorm.DB` Registry、`go-sqlmock` 测试、GORM migration runner | `304a7b2` |
| 4 | 六类独立 Logger、Noop Metrics/Tracing、请求关联日志 | `97de3c2` |
| 5 | Agent/Douyin 私有 Client、连接配置与凭证分离、类型化调用 | `5c2b791` |
| 6 | Bootstrap、Cmd Server、Scheduler、Worker、Migration 生命周期收敛 | `0699e8a` |
| 7 | Identity 与 Cloud-Agent 模块迁移 | `bf499d9` |
| 8 | Runtime Binding、Profile Binding、Profile Guard 模块迁移 | `2061780` |
| 9 | Media Account 与 Proxy 模块迁移，Proxy Expiry Job 使用 Service | `a83b71b` |
| 10 | Content Pool、Scheduler、Worker 职责分离 | `100899d` |
| 11 | 同步抖音搜索与选择结果导入 | `4d24d1d` |
| 12 | 过渡层清理与最终全量验收 | `eecc275` |

Task 9 曾产生初步提交 `b4320a3`，随后为删除旧 Agent checker 兼容构造、收紧测试注入边界而 amend；最终以 `a83b71b` 为准。

## Task 1 记录：架构规则与边界扫描

提交：`f284956`

完成内容：

- 同步 `AGENTS.md` 与架构基线。
- 明确 Cloud、Desktop Agent、Cloud-Agent Contract 的职责边界。
- 建立生产代码架构边界扫描。
- 固定模块化单体边界，禁止提前拆分微服务、引入 MQ、动态插件或工作流引擎。

关键结果：

- Cloud 是业务事实中心。
- Cloud 可直接访问抖音公开数据 API。
- Desktop Agent 负责本机浏览器、Profile、登录态、文件和 FFmpeg 等能力。
- 需要本机环境的操作必须经过 Agent。

## Task 2 记录：严格配置 Schema 与 Loader

提交：`23774d8`

完成内容：

- Config Loader 固定读取 `./config`。
- `config_online` 仅作为发布输入，不在运行时切换。
- 建立严格 YAML Schema 与 Validation。
- 配置拆分为：
  - App
  - Databases
  - Cache
  - Loggers
  - Clients
  - Credentials
  - Scheduler
  - Observability
- Client 连接配置与凭证配置分离。
- 默认数据库名称固定为 `primary`。

关键接口：

```go
config.Initialize() error
config.Get() Config
config.LoadFromDir(root string) (Config, error)
```

## Task 3 记录：私有 GORM 数据库 Registry

提交：`304a7b2`

完成内容：

- 数据库资源统一为 `*gorm.DB`。
- 包私有保存 primary 与 named databases。
- Repository 通过 `database.DB()` / `database.Named(name)` 获取连接。
- 保留显式 SQL：
  - 查询 `Raw`
  - 写入 `Exec`
  - 事务 `Transaction`
- 使用 `go-sqlmock` 包装 GORM 做单元测试。
- 新增 GORM migration runner。

关键接口：

```go
database.Initialize(configs []config.DatabaseConfig) error
database.DB() *gorm.DB
database.Named(name string) (*gorm.DB, bool)
database.Close() error
```

验证：

```text
go test ./internal/infra/database/... -count=1
go test -race ./internal/infra/database/... -count=1
go vet ./internal/infra/database/...
git diff --check
```

## Task 4 记录：独立 Logger 与 Observability 资源

提交：`97de3c2`

完成内容：

- 新增六类 Logger：
  - `App`
  - `Access`
  - `Job`
  - `External`
  - `Audit`
  - `Panic`
- 六类 Logger 写入六个独立物理文件。
- `logger.Initialize(config.LoggerConfigs)` 原子发布全部资源。
- 任一文件打开失败时回收已打开文件，保留此前已发布资源。
- `logger.Close()` 同步并关闭全部文件，幂等。
- Metrics 与 Tracing 包私有保存资源，默认和初始化后均为 Noop，调用方无需判空。
- Request Middleware 从 `logger.Access()` 派生日志。
- 请求日志包含 `trace_id`、`request_id`、`module`。

关键测试：

```text
TestInitializeCreatesSixIndependentLogFiles
TestTypedGettersReturnDistinctLoggers
TestRequestLoggerContainsTraceRequestAndModule
TestCloseFlushesAndClosesEveryFile
TestInitializeFailureCleansOpenedFilesAndKeepsPreviousResources
TestCloseIdempotent
Metrics/Tracing Noop 行为测试
```

验证：

```text
go test ./internal/infra/logger ./internal/infra/metrics ./internal/infra/tracing ./internal/middleware -count=1
go test -race ./internal/infra/logger ./internal/infra/metrics ./internal/infra/tracing ./internal/middleware -count=1
go vet ./internal/infra/logger ./internal/infra/metrics ./internal/infra/tracing ./internal/middleware
git diff --check
```

## Task 5 记录：Client Transport 与 Credential 资源分离

提交：`5c2b791`

完成内容：

- Agent Client：
  - `agent.Initialize(config.ClientConfig, config.AgentCredentialConfig) error`
  - `agent.Get() *Client`
  - `agent.Close() error`
  - 类型化 `CheckProxy`、`ExtractProxy`、`MutateProxy`
- Douyin Client：
  - `douyin.Initialize(config.ClientConfig, config.DouyinCredentialConfig) error`
  - `douyin.Get() *Client`
  - `douyin.Close() error`
  - 类型化 `Search`、`FindAuthor`、`FetchByURL`
- Client 连接配置与认证配置分离。
- API Key、Cookie、固定 Header 只在 Client 内部组装。
- 业务请求类型不携带凭证。
- 传输错误按配置重试，HTTP 非 2xx 不重试并映射错误。
- 构造函数保留注入 `*http.Client` 的测试能力。

关键测试：

```text
TestAgentClientAddsCredentialWithoutBusinessPassingHeaders
TestDouyinSearchBuildsTypedRequest
TestDouyinFindAuthorBuildsTypedRequest
TestDouyinFetchByURLBuildsTypedRequest
TestClientRetriesOnlyConfiguredTransportFailures
TestClientMapsNonSuccessStatus
```

验证：

```text
go test ./internal/infra/client/... -count=1
go test ./internal/modules/proxy/service ./internal/modules/contentpool/service -count=1
go test -race ./internal/infra/client/... -count=1
go test -race ./internal/modules/proxy/service ./internal/modules/contentpool/service -count=1
go vet ./internal/infra/client/... ./internal/modules/proxy/service ./internal/modules/contentpool/service
git diff --check
```

保留说明：

- `internal/modules/contentpool/service/discovery.go` 是既有未跟踪迁移文件，为 Task 5 局部编译保留了无参数默认 Crawler 调整。Task 10 开始，该文件在内容池修改范围内可以正式处理。

## Task 6 记录：进程 Bootstrap 与 Cmd Server 生命周期

提交：`0699e8a`

完成内容：

- `internal/bootstrap` 成为生产初始化唯一发起方。
- 新增：
  - `bootstrap.InitializeServer() (*hertzserver.Hertz, func() error, error)`
  - `bootstrap.InitializeScheduler() (func() error, error)`
  - `bootstrap.InitializeWorker() (func() error, error)`
  - `bootstrap.InitializeMigration() (func() error, error)`
- `bootstrap.go` 只负责入口与顺序。
- `resource.go` 负责资源计划、失败逆序回滚和幂等关闭栈。
- `routes.go` 只注册路由，不启动 Scheduler/Worker。
- `jobs.go` 只连接 Scheduler/Worker Job。
- `cmd/server/main.go` 只调用 `run()`。
- `cmd/server/server.go` 管理 HTTP 启动、信号、先关闭 Hertz、再关闭 Bootstrap 资源。
- Scheduler 不初始化 Douyin Client。
- Worker 初始化 Douyin Client，但不初始化 HTTP-only 资源。
- Migration 通过 Bootstrap 初始化 Config、Logger、Database。
- `internal/app` 已删除。
- `internal/runtime` 原文件是未跟踪迁移文件，已从工作区移除，因此没有 Git 删除记录。

验证：

```text
go test ./internal/bootstrap ./cmd/server -count=1
go test -race ./internal/bootstrap ./cmd/server -count=1
go test ./internal/bootstrap ./cmd/... -count=1
go test ./... -run '^$'
go vet ./internal/bootstrap ./cmd/...
git diff --check
rg -n 'internal/(runtime|app)|NewRuntime|NewDiscoveryRuntime' --glob '*.go'
```

边界扫描结果为空。

## Task 7 记录：Identity 与 Cloud-Agent 模块迁移

提交：`bf499d9`

完成内容：

- Identity 与 Cloud-Agent 均迁移为：
  - 真实 `model`
  - 真实 `dto`
  - `database.DB()` 包级 Repository 入口
  - 私有 `*gorm.DB` Repository 实现
  - 包级 Service 函数
  - 根包 Handler
  - direct-binding Router
- 删除两个模块的 `handler/` 子包。
- 删除根 Router 的整体 aliases/constructors。
- Cloud-Agent Contract 常量、JSON Tag 与协议字段保持不变。

Identity 关键包级操作：

```text
BootstrapAdmin
CreateUser
Login
Authenticate
Team/Game/User 管理
Audit 查询
```

Cloud-Agent 关键包级操作：

```text
RegisterAgent
Heartbeat
GetAgent
CreateTask
GetTask
RetryTask
ClaimTask
ReportTask
CountTasksByStatus
CancelTask
```

验证：

```text
go test ./internal/modules/identity/... ./internal/modules/cloudagent/... -count=1
go test -race ./internal/modules/identity/... ./internal/modules/cloudagent/... -count=1
go vet ./internal/modules/identity/... ./internal/modules/cloudagent/...
git diff --check
rg -n '= .*service\.|NewService|NewMySQL' internal/modules/identity/router.go internal/modules/cloudagent/router.go
```

Router 扫描结果为空。

## Task 8 记录：Runtime/Profile Binding/Guard 模块迁移

提交：`2061780`

完成内容：

- Runtime Binding、Profile Binding、Profile Guard 均迁移为：
  - 真实 `model`
  - 真实 `dto`
  - GORM Repository 包级入口
  - 私有 `*gorm.DB` 实现
  - 包级 Service 函数
  - 根包 Handler
  - direct-binding Router
- 删除三个模块的 `handler/` 子包。
- 固定单向依赖：
  - Runtime Binding -> Identity / Cloud-Agent Service
  - Profile Binding -> Identity / Runtime Binding Service
  - Profile Guard -> Runtime Binding Service
- 禁止跨模块 Repository import。

Runtime Binding 包级函数：

```text
IssueTicket
RegisterLocal
CheckLocalTrust
ReportRuntime
AuthenticateNode
```

Profile Binding 包级函数：

```text
SubmitScan
GetScan
ConfirmScan
ConfirmMainIdentity
ConfirmMainIdentityDirect
ClearMainIdentity
RejectScan
ListProfiles
DeleteProfile
UpdateProfile
AssignProfileOwner
GetActiveProfile
ResolveProfile
ResolveProfileForAccountCheck
ResolveProxyForAccountCheck
BindProxy
UnbindProxy
CountProfilesByProxyID
```

Profile Guard 包级函数：

```text
Preflight
Renew
Finish
CreateAuthorizedTask
```

验证：

```text
go test ./internal/modules/runtimebinding/... ./internal/modules/profilebinding/... ./internal/modules/profileguard/... -count=1
go test -race ./internal/modules/runtimebinding/... ./internal/modules/profilebinding/... ./internal/modules/profileguard/... -count=1
go vet ./internal/modules/runtimebinding/... ./internal/modules/profilebinding/... ./internal/modules/profileguard/...
git diff --check
```

跨模块 Repository import 扫描为空。注意：计划中的宽泛 `rg 'modules/.*/repository'` 会命中模块自身 Service -> Repository 合法依赖，因此判定边界时应使用排除自身 Repository 的跨模块扫描。

## Task 9 记录：Media Account 与 Proxy 模块迁移

最终提交：`a83b71b`

完成内容：

- Media Account 与 Proxy 均迁移为：
  - 真实 `model`
  - 真实 `dto`
  - GORM Repository 包级入口
  - 私有 `*gorm.DB` 实现
  - 包级 Service 函数
  - 根包 Handler
  - direct-binding Router
- 删除两个模块的 `handler/` 子包。
- 删除 Proxy 旧 `NewAgentChecker` / `HTTPAgentChecker` 兼容路径。
- Proxy Router 移除 `deps ...any`。
- Agent 调用固定通过 `agent.Get()` 的类型化 Client：
  - `CheckProxy`
  - `ExtractProxy`
  - `MutateProxy`
- Media Account 生产路径通过单向 Service 调用消费：
  - Identity Service
  - Profile Binding Service
  - Profile Guard Service
- Proxy Expiry Job 改为：
  - `RunProxyExpiry(ctx context.Context) error`
  - 调用 `proxyservice.MarkExpiringProxies(ctx)`
  - 使用 `logger.Job()`
  - 不接收 raw DB 或 Logger

Media Account 包级操作包括：

```text
CreateAccount
GetAccount
GetAccountRecord
GetOwnedAccountRecord
ListAccounts
UpdateAccount
IdentifyAccount
StartAccountCheck
ApplyAccountCheckResult
StartCookieRead
ApplyCookieReadResult
AddTags
RemoveTags
BindProfile
UnbindProfile
CreateAccountGroup
ListAccountGroups
UpdateAccountGroup
DeleteAccountGroup
ListAccountsByGroup
```

Proxy 包级操作包括：

```text
BulkParse
BulkImport
List
ParseAddress
Get
Create
CreateDiscovered
Update
UpdateStatus
Delete
TriggerCheck
CheckQuota
CheckAssignable
SetMaxProfileCount
RecordCheckResult
MarkExpiringProxies
CheckProxy
ExtractProxy
MutateProxy
```

验证：

```text
go test ./internal/modules/mediaaccount/... ./internal/modules/proxy/... ./internal/jobs -count=1
go test -race ./internal/modules/mediaaccount/... ./internal/modules/proxy/... ./internal/jobs -count=1
go vet ./internal/modules/mediaaccount/... ./internal/modules/proxy/... ./internal/jobs
git diff --check
rg -n 'deps \.\.\.any|NewService|NewMySQLStore|\*Service' internal/modules/mediaaccount internal/modules/proxy --glob '*.go'
```

生产边界扫描为空；跨模块 Repository import 为空；业务代码没有直接创建 `http.Client`。

## 当前仍需遵守的工作区注意事项

1. 必须继续使用当前完整工作目录，不得 reset/clean。
2. 交接文档本身处于暂存区，但未提交。后续代码提交必须使用 pathspec，避免混入本文档。
3. `web/dist-*` 是既有无关构建产物，禁止扫描和修改。
4. 禁止扫描 `.gitignore`、`dist`、`node_modules` 或其它编译产物包。
5. 每个任务完成后必须执行局部验证并更新本文档，再停止等待人工确认。
6. 最终版本保留历史 Crawl Task list/get/confirm 和 URL 导入任务链路；它们仍有路由、Web 或测试覆盖。
7. Task 12 已完成全仓最终验收；此前 Task 10–11 的轻量验证限制由本轮完整验收覆盖。
8. 不存在未解决的跨阶段构建阻塞；原 `bootstrap/routes.go` 旧构造器引用已删除。

## Task 10 当前记录

最终提交：`100899d`。未执行 `go test` 或 race：用户要求本轮只进行轻量验证。

- Content Pool 类型已下沉到真实 `model`/`dto`，Repository 改为
  `database.DB()` 的 GORM 包级入口和私有 `*gorm.DB` 实现；显式 SQL 保留。
- Content Pool/Discovery 操作改为包级 Service 函数；根 Handler/Router 直接绑定。
- Scheduler Job 只调用 `RunDue` 创建 pending task；Worker Job 只调用 `RunNext`
  原子领取并同步执行 Douyin crawler。
- `internal/bootstrap/jobs.go` 不再构造 Content Pool Service；Proxy expiry 使用
  无 raw DB/Logger 参数的 Job 接口。
- 轻量验证通过：模块 `go build`、`go vet`、`git diff --check`；Ticker 扫描只命中
  `internal/scheduler`。
- `cmd/discovery-scheduler`/`cmd/discovery-worker` 的 build 被既有
  `internal/bootstrap/routes.go` 对 Task 7/8 旧构造器的引用阻塞。这一文件不在
  Task 10 范围，需在后续 Bootstrap 过渡层清理时统一处理。

## 已归档：Task 10 恢复 Prompt

目标：迁移 Content Pool，并严格分离 Scheduler 与 Worker 职责。

修改范围：

```text
internal/modules/contentpool/**
internal/jobs/discovery.go
internal/jobs/discovery_test.go
internal/scheduler/scheduler.go
internal/scheduler/scheduler_test.go
cmd/discovery-scheduler/main.go
cmd/discovery-worker/main.go
internal/bootstrap/jobs.go
实施计划 Task 10 复选框
```

接口目标：

```go
// Content Pool 包级函数
RunDue(time.Time)
RunNext(context.Context)

// 进程职责
Scheduler discovery-only
Worker execution-only
```

必须先写失败测试：

```go
TestRunDueOnlyCreatesPendingTasks
TestRunNextClaimsOneTaskAndCallsCrawlerSynchronously
TestHTTPBootstrapDoesNotStartSchedulerOrWorker
TestSchedulerCommandNeverCallsDouyin
TestWorkerUsesConfiguredIntervalAndBatchSize
```

Task 10 验证命令：

```text
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go test ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./internal/bootstrap -count=1
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go test -race ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./cmd/discovery-scheduler ./cmd/discovery-worker -count=1
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go vet ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./cmd/discovery-scheduler ./cmd/discovery-worker
git diff --check
rg -n 'time\.NewTicker' internal/modules/contentpool internal/jobs internal/scheduler internal/bootstrap cmd/discovery-scheduler cmd/discovery-worker --glob '*.go'
```

预期只有 `internal/scheduler` 包含 `time.NewTicker`。

Task 10 提交信息：

```text
refactor: separate discovery scheduling and execution
```

### Task 10 恢复 Prompt

把下面全文交给新模型即可继续：

```text
你正在接手 WT Media Cloud 基础运行架构收敛工作。请在现有工作区继续执行 Task 10：Migrate Content Pool and Separate Scheduler/Worker Execution。

仓库绝对路径：
/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud

请先完整阅读：
1. AGENTS.md
2. docs/superpowers/specs/2026-09-18-cloud-foundation-convergence-design.md
3. docs/superpowers/plans/2026-09-18-cloud-foundation-convergence.md
4. docs/superpowers/handoffs/2026-09-18-cloud-foundation-convergence-handoff.md

现有进度：
- Task 1：f284956
- Task 2：23774d8
- Task 3：304a7b2
- Task 4：97de3c2
- Task 5：5c2b791
- Task 6：0699e8a
- Task 7：bf499d9
- Task 8：2061780
- Task 9：a83b71b
- 下一项是 Task 10；Task 11–12 尚未完成。

先检查 git status --short、git log 和 git diff --cached。
必须继续使用当前完整工作目录，禁止 reset/checkout/clean 丢弃既有工作。
注意：交接文档目前处于暂存区但未提交，不能混入 Task 10 代码提交。
禁止扫描 .gitignore、dist、node_modules 或其它编译产物包。
不要扫描或修改 web/dist-*。
internal/modules/contentpool/service/discovery.go 是既有未跟踪迁移文件，且属于 Task 10 修改范围，可在实际文件状态上继续处理。

本次只执行 Task 10，不得进入 Task 11。

修改范围：
- internal/modules/contentpool/**
- internal/jobs/discovery.go
- internal/jobs/discovery_test.go
- internal/scheduler/scheduler.go
- internal/scheduler/scheduler_test.go
- cmd/discovery-scheduler/main.go
- cmd/discovery-worker/main.go
- internal/bootstrap/jobs.go
- 实施计划 Task 10 复选框。

测试驱动流程：
1. 先补齐 Scheduler/Worker 职责测试并执行，确认旧结构缺少目标能力而失败；
2. 记录具体 RED 原因；
3. 实现最小改动；
4. 执行局部测试、race、Vet、边界扫描和 diff 检查；
5. 更新实施计划和本交接文档；
6. 用 pathspec 精确提交 Task 10 文件；
7. 提交信息：refactor: separate discovery scheduling and execution；
8. 停止等待人工确认。

架构要求：
- SourceContent、Material、Strategy、CrawlTask、CrawlStats 移入真实 model。
- filters/inputs 移入真实 dto。
- Repository 使用 database.DB() 包级入口和私有 *gorm.DB 实现。
- Content CRUD/materialization、Strategy CRUD/status/manual scheduled run、RunDue、RunNext、task list/get、historical confirmation 均为包级函数。
- Worker 路径使用 douyin.Get() 同步调用 Douyin。
- Scheduler 只发现到期策略并创建 pending task，不调用 Douyin。
- Worker 只领取 pending task、同步执行、持久化结果。
- Scheduler command 使用 bootstrap.InitializeScheduler。
- Worker command 使用 bootstrap.InitializeWorker。
- interval 和 batch size 来自 config.Get()；不得用命令行 flag 覆盖已验证配置，仅允许诊断性 --once。
- HTTP Bootstrap 不启动 Scheduler 或 Worker。
- Handler 移到 contentpool 根包。
- Router 只 direct bind method/path/function handler。
- 删除 handler/ 子包。
- 保持既有路由、错误码、响应信封和 Cloud–Agent Contract 不变。

至少覆盖：
- TestRunDueOnlyCreatesPendingTasks
- TestRunNextClaimsOneTaskAndCallsCrawlerSynchronously
- TestHTTPBootstrapDoesNotStartSchedulerOrWorker
- TestSchedulerCommandNeverCallsDouyin
- TestWorkerUsesConfiguredIntervalAndBatchSize
- DTO/model 真实类型源测试
- Repository GORM + go-sqlmock 测试
- Root Handler/Router 测试

在仓库根目录运行：
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go test ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./internal/bootstrap -count=1
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go test -race ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./cmd/discovery-scheduler ./cmd/discovery-worker -count=1
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go vet ./internal/modules/contentpool/... ./internal/jobs ./internal/scheduler ./cmd/discovery-scheduler ./cmd/discovery-worker
git diff --check
rg -n 'time\.NewTicker' internal/modules/contentpool internal/jobs internal/scheduler internal/bootstrap cmd/discovery-scheduler cmd/discovery-worker --glob '*.go'

如局部测试被尚未迁移的跨阶段依赖阻塞，请输出准确错误、归属阶段和最小调整建议，等待人工确认，不得扩大范围或声称测试通过。
```

## Task 11 当前记录

最终提交：`4d24d1d`（`feat: make douyin discovery search synchronous`）。未执行 `go test`、race 或 `npm test`：用户要求本轮只进行轻量验证。

- `POST /api/v1/content-pool/search` 与 `/author-search` 改为同步调用
  `douyin.Get()` 类型化 Client 并返回 `{items: [...]}`，不创建 Crawl Task、
  不写 `source_contents`、不调用 Agent。
- 新增 `POST /api/v1/content-pool/import-results`。所有选择结果均按不可信输入
  处理，重新验证当前用户/团队、平台、数量、内容 ID 与抖音来源 URL，再复用
  Content Pool `CreateSource` 和唯一约束去重。
- DTO 增加真实 `SearchResult`、`ImportResultsRequest` 与逐项导入结果类型；
  Service 以包级 `Search`、`FindAuthor`、`ImportResults` 暴露操作；根 Handler
  只做认证、解码、调用和响应映射；Router 只新增 direct bind。
- Web 人工搜索直接读取 `result.items`，删除 `getTask` 轮询与延迟等待；确认时
  仅把用户选中的条目传给 `importResults`。
- 历史 Crawl Task list/get/confirm、定时 Strategy、RunDue、RunNext 和 URL 导入
  行为保持不变；`manual_discovery_task` 仅保留在这些兼容路径。
- 轻量验证通过：Content Pool `go build`、`go vet`、`git diff --check`、生产边界
  扫描，以及输出到 `/private/tmp/wt-media-cloud-task11-cloud` 的 Cloud Web 构建。
  tracked `web/dist-*` 保持任务前既有状态。
- 既有跨阶段阻塞仍是 `internal/bootstrap/routes.go` 对 Task 7/8 旧构造器的引用；
  Task 11 没有扩大范围修改 Bootstrap，Task 12 应统一清理并做全仓验收。

## Task 12 当前记录

最终提交：`eecc275`（`refactor: complete cloud foundation convergence`）。

- Bootstrap `routes.go` 删除旧的 `*sql.DB`、Service/Store 构造与参数化 Router
  组装，改为仅执行初始管理员引导和模块根 Router 直接注册。
- Identity、Runtime Binding、Profile Binding 与 Profile Guard 的生产包级操作改为
  调用私有构造；`NewService` 仅保留在 `_test.go` 测试 seam。
- 删除无调用的共享 API 兼容响应包装器；补齐 Content Pool 的空事务期望，并让其
  test-only Discovery Service 不再隐式读取未初始化的 Douyin Client。
- 移除确认无引用的旧 `internal/common`、`infra/config`、`infra/scheduler`、
  `infra/worker`、`modules/migration` 过渡路径，并保留其已迁移的
  `internal/shared`、`internal/config`、`internal/scheduler` 和
  `internal/infra/database/migration` 实现。
- 全量验证通过：`go test ./...`、`go test -race ./...`、`go vet ./...`、
  `npm --prefix web test`（78 tests）、Cloud/Desktop 临时目录构建、架构测试、
  格式、边界扫描和 `git diff --check`。
- 最终边界：无 Runtime/App 引用、无模块 Handler 子目录、仅
  `internal/scheduler` 使用 `time.NewTicker`；历史 Crawl Task、URL 导入和
  同步搜索后的选择入池接口保持。

## 已归档：Task 12 恢复 Prompt

目标：只清理经过扫描确认的过渡层，修复 Bootstrap 等剩余旧引用，并完成全仓
Go、Web、格式、架构边界和 diff 验收；不得改变数据库结构、路由、错误码或
Cloud–Agent Contract。

### Task 12 恢复 Prompt

把下面全文交给新模型即可继续：

```text
你正在接手 WT Media Cloud 基础运行架构收敛工作。请在现有完整工作区继续执行 Task 12：Remove Transitional Layers and Complete Verification。

仓库绝对路径：
/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud

当前日期：
2026-09-19

请先完整阅读：
1. AGENTS.md
2. docs/superpowers/specs/2026-09-18-cloud-foundation-convergence-design.md
3. docs/superpowers/plans/2026-09-18-cloud-foundation-convergence.md
4. docs/superpowers/handoffs/2026-09-18-cloud-foundation-convergence-handoff.md

现有提交进度：
- Task 1：f284956
- Task 2：23774d8
- Task 3：304a7b2
- Task 4：97de3c2
- Task 5：5c2b791
- Task 6：0699e8a
- Task 7：bf499d9
- Task 8：2061780
- Task 9：a83b71b
- Task 10：100899d
- Task 11：以交接文档提交表中的最终哈希为准
- 下一项是 Task 12；不得开启计划外 Task。

先执行只读检查：
git status --short
git log --oneline -15
git diff --cached --name-only
git diff --cached --stat

工作区约束：
1. 必须继续使用当前完整工作目录。
2. 禁止 reset、checkout、clean，禁止丢弃任何既有未提交或未跟踪文件。
3. 不得仅凭提交记录判断文件状态，必须检查实际工作目录。
4. 交接文档仍单独处于暂存区，Task 12 提交必须使用精确 pathspec，不能混入该文档。
5. 禁止 git add .。
6. 禁止扫描 .gitignore 中的包、dist、node_modules 或其它构建产物。
7. 特别禁止修改或重新生成 tracked web/dist-*；Web 构建必须输出到临时目录。
8. 保留全部无关工作区改动。

Task 12 目标：
- 扫描并只删除确认无生产调用、无必要兼容测试覆盖的旧 Handler 目录、Router facade、Runtime/App 兼容文件、重复测试和无用配置。
- 修复 internal/bootstrap/routes.go 等剩余旧构造器引用，使完整模块化单体可构建。
- 保留历史 Crawl Task list/get/confirm、URL 导入等仍被产品使用的兼容 API。
- 不改变数据库结构、公共路由、错误码、响应信封或 Cloud–Agent Contract。
- 不引入 MQ、微服务、动态插件或工作流引擎。

先运行结构扫描并逐项判断：
rg -n 'internal/(runtime|app)|runtime\.Runtime|NewRuntime|NewService|NewMySQLStore|deps \.\.\.any' --glob '*.go'
find internal/modules -type d -name handler -print
rg -n 'http\.Client\s*\{|os\.(Getenv|LookupEnv)\(|time\.NewTicker\(' internal/modules --glob '*.go'

预期扫描会列出剩余过渡引用。只迁移或删除确认的生产引用；不得删除历史 migration，
也不得仅因名称命中就删除仍被路由、Web 或测试覆盖的兼容 API。重点核对：
- Bootstrap 只负责初始化/关闭与路由/任务连接；routes.go 只注册路由。
- Module Router 只 direct bind method/path/function Handler。
- Handler 位于模块根包并调用包级 Service。
- dto/model 是真实类型源。
- Repository 通过 database.DB()/Named 获取 GORM，保留显式 SQL。
- 外部 HTTP 只通过 infra/client 类型化 Client。
- 只有 internal/scheduler 可使用 time.NewTicker。

清理后执行：
gofmt -w cmd internal
go mod tidy

完整 Go 验证：
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go test -count=1 ./...
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go test -race -count=1 ./...
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go vet ./...

Web 验证不得写 tracked dist：
npm --prefix web test
npm --prefix web run build:cloud -- --outDir /private/tmp/wt-media-cloud-build-cloud
npm --prefix web run build:desktop -- --outDir /private/tmp/wt-media-cloud-build-desktop
git status --short web/dist-cloud web/dist-desktop

最终边界与卫生检查：
env GOCACHE=/Users/aqiuye/Develop/workspace/wt-media/wt-media-cloud/.cache/go-build go test ./internal/architecture -count=1
gofmt -l cmd internal
git diff --check
rg -n 'internal/(runtime|app)|runtime\.Runtime|NewRuntime' --glob '*.go'
rg -n 'time\.NewTicker' internal cmd --glob '*.go'

预期：
- 所有 Go test/race/vet 与 Web test/build 通过。
- gofmt -l 无输出，git diff --check 通过。
- 无 Runtime/App 生产引用。
- 只有 internal/scheduler 创建 ticker。
- tracked web/dist-* 相对任务前基线完全不变。

若完整验证失败：
1. 记录准确命令和完整错误。
2. 判断是 Task 12 范围内过渡引用、真实回归还是既有无关工作区问题。
3. 只修复本计划范围内问题，不扩大为产品功能改造。
4. 无法在范围内解决时，给出最小调整建议并等待人工确认，不得声称通过。

完成后：
1. 更新实施计划 Task 12 全部复选框。
2. 更新交接文档最终状态、提交表、完整验证结果和保留兼容项。
3. 用精确文件清单暂存 Task 12 修改，禁止 git add .。
4. 提交时使用精确 pathspec，避免混入已暂存交接文档、web/dist-* 和无关改动。
5. 提交信息：refactor: complete cloud foundation convergence
6. 单独暂存更新后的交接文档，不要混入 Task 12 代码提交。
7. 报告最终提交哈希、精确修改文件、最终目录/进程边界、同步搜索这一唯一业务行为变化、全部验证结果、Mock 测试说明、保留兼容代码和未触碰的既有脏文件。
8. Task 12 完成后停止并等待人工确认。
```

## 完成任务后的固定收尾协议

每个 Task 完成后必须：

1. 运行该任务规定的局部测试、race、Vet 和 diff check；
2. 更新实施计划复选框；
3. 更新本文档的：
   - 状态；
   - 提交表；
   - 本 Task 完整记录；
   - 验证结果；
   - 当前工作区注意事项；
   - 下一 Task 恢复 Prompt；
4. 用精确 pathspec 提交代码，不混入交接文档；
5. 报告提交哈希并停止等待人工确认；
6. 保留全部无关工作区改动和 `web/dist-*`。
