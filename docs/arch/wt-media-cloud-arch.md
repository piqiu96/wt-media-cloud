# WT Media Cloud 工程架构规范 V2

本规范是日常开发摘要，完整设计见：

- `docs/superpowers/specs/2026-09-18-cloud-foundation-convergence-design.md`
- `docs/superpowers/plans/2026-09-18-cloud-foundation-convergence.md`

## 1. 项目定位

WT Media Cloud 是 Go + CloudWeGo Hertz 的模块化单体服务，负责 HTTP API、业务事实、后台任务和公开平台数据集成。当前阶段不拆分微服务，不引入 MQ、动态插件或工作流引擎。

## 2. 目录边界

```text
cmd/
├── server/
├── migrate/
├── discovery-scheduler/
└── discovery-worker/

internal/
├── bootstrap/
│   ├── bootstrap.go
│   ├── resource.go
│   ├── routes.go
│   └── jobs.go
├── config/
├── infra/
├── middleware/
├── scheduler/
├── jobs/
├── shared/
│   ├── api/
│   └── id/
└── modules/
```

不保留 `internal/runtime`、`internal/app`、`internal/server` 或 `bootstrap/modules.go`。

## 3. 生命周期

HTTP 进程调用方向：

```text
cmd/server/main.go
  -> cmd/server/server.go
  -> bootstrap.InitializeServer
  -> Hertz Run/Shutdown
  -> Bootstrap Close
```

`cmd/server/server.go` 只负责启动、信号和优雅关闭。所有 Config、Logger、Database、Client、Metrics、Tracing、Middleware 和 Router 初始化都从 Bootstrap 发起。

Bootstrap 提供四个进程入口：

- `InitializeServer`
- `InitializeScheduler`
- `InitializeWorker`
- `InitializeMigration`

每个进程只初始化自身所需资源。Server 初始化 Config、Logger/Hertz Logger、Metrics、Tracing、Database、HTTP Client、Agent/Douyin 语义 Client 后启动 Hertz Server；Worker 只初始化 Douyin HTTP Client；Scheduler 不初始化外部 HTTP Client。初始化失败时按逆序回滚，关闭操作必须幂等。

## 4. 受控基础资源

各 Infra 包私有保存资源并提供只读 Getter：

```go
var db *gorm.DB

func DB() *gorm.DB
func Named(name string) (*gorm.DB, bool)
```

禁止导出可变资源变量、全局业务状态、`runtime.Current`、万能 Service Locator 和动态资源注册表。Bootstrap 是唯一生产初始化入口。

## 5. 配置

- 运行时始终读取 `./config`。
- `config_online` 只在发布打包时替换产物中的 `config`。
- `internal/config` 只包含加载、Schema、校验和只读 Getter。
- HTTP 参数位于 `app.toml`。
- Scheduler/Worker 配置独立位于 `config/scheduler`。
- 每个数据库使用独立配置文件；代码固定 `primary` 为默认库，不使用 `default` 或 `enabled`。
- HTTP Client 文件位于 `config/clients/http/`，文件名即实例名，只描述 `base_url`、超时、连接池和重试。
- Cookie、API Key 和固定 Header 按需放在独立 credentials 配置中。

## 6. Database 与 Repository

数据库统一使用 `*gorm.DB`，但保留显式 SQL：

- `Raw` 查询；
- `Exec` 写入；
- `Transaction` 事务。

Repository 是唯一业务数据访问层，通过 `database.DB()` 或 `database.Named()` 获取连接。Handler 和 Service 不执行 SQL，不为每个模块复制数据库实例。

## 7. 日志和可观测性

六类 Logger 独立初始化、调用和写入物理文件：

```text
app.log
access.log
job.log
external.log
audit.log
panic.log
```

调用入口为 `logger.App()`、`Access()`、`Job()`、`External()`、`Audit()`、`Panic()`。每个 Logger 同时写入普通文件和 `.wf` 文件；`.wf` 记录 WARN、ERROR 和 FATAL。上下文日志自动携带非空的 `trace_id`、`request_id` 和 `user_id`。Metrics 和 Tracing 未接入后端时使用 Noop 实现。

## 8. Client 与执行边界

所有外部 HTTP 协议位于 `internal/infra/client`，底层复用 Hertz Client 和 `pkg/clients/http` 的窄类型 Registry，向业务暴露类型化方法，统一处理连接、超时、重试、错误映射、日志、Metrics 和 Tracing。业务代码不直接创建 `http.Client`，也不导入 `pkg/config`、`pkg/logger` 或 `pkg/clients/http`。

Cloud 可以直接抓取抖音等公开平台数据，并维护服务端所需凭证和代理连接。Desktop Agent 只负责本地浏览器、Profile、账号登录态、文件和 FFmpeg 等本机能力。

## 9. Module

```text
modules/{module}/
├── router.go
├── handler.go
├── dto/
├── service/
├── repository/
└── model/
```

- Router 直接绑定函数 Handler，不创建依赖。
- Handler 位于模块根包并调用包级 Service 函数。
- DTO/Model 是真实类型源，不反向引用 Service。
- Service 处理业务规则，Repository 处理数据访问。
- 跨模块只能按单向依赖调用 Service，禁止访问其他模块 Repository。
- 模块根包不重新导出完整的 Service/Repository/Model API。

## 10. Shared 与响应

`shared/api` 保存响应信封、错误响应和 JSON 解码，`shared/id` 保存稳定 ID/Token 定义。不创建万能 `common`、`utils`、`resource` 或独立 `response` 包。

统一响应：

```json
{
  "errcode": 0,
  "message": "success",
  "data": {},
  "logid": "xxx"
}
```

## 11. Scheduler、Job 与 Worker

```text
Scheduler -> 发现到期策略 -> 创建 pending task
Worker    -> 原子领取 task -> 调用 Job -> 同步执行外部请求 -> 持久化结果
```

HTTP Server 不启动 Scheduler 或 Worker。只有 `internal/scheduler` 可以创建 Ticker，Job 只定义一次业务执行内容。

## 12. 人工搜索例外

关键词和博主搜索同步调用 Douyin Client 并直接返回结果，不创建人工 Crawl Task，也不轮询。用户选择结果后调用同步入池接口。历史 Crawl Task 查询和确认接口继续保留。

除该批准例外外，架构迁移不得修改数据库结构、路由、响应信封、错误码、权限规则或 Cloud–Agent Contract。

## 13. 验证

最终必须通过：

```text
go test ./...
go test -race ./...
go vet ./...
gofmt
架构边界扫描
git diff --check
```

Repository 使用 `go-sqlmock` 构造 Mock `*gorm.DB`，不要求真实 MySQL。
