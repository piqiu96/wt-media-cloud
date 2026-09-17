# WT Media Cloud 基础运行架构收敛设计

日期：2026-09-18  
状态：已确认的实施基线

## 1. 背景与目标

本次工作是对 WT Media Cloud 的基础运行架构进行收敛，不是业务重写。项目继续采用 Go、Hertz 和模块化单体架构，不引入微服务、消息队列、动态插件或工作流引擎。

目标如下：

- 由 Bootstrap 统一发起所有进程初始化和关闭；
- 使用受控全局基础资源，删除大而全的 `runtime.Runtime`；
- 将 Server、Router、Module、Repository、Client、Scheduler 和 Worker 的职责固定下来；
- 逐个完成现有业务模块的分层迁移；
- 保留数据库结构、主要路由、统一响应信封、错误码和 Cloud–Agent Contract；
- 将人工抖音搜索改为同步调用，并取消人工搜索任务和轮询；
- 使用自动化单元测试验证迁移结果，不依赖真实 MySQL。

## 2. 非目标

本次不执行以下工作：

- 不重新设计业务模型；
- 不修改现有数据库表结构；
- 不把模块拆成微服务；
- 不建设 Cookie 池、代理池或凭证管理服务；
- 不引入 Redis、MQ 或分布式任务框架解决当前不存在的问题；
- 不处理与架构迁移无关的 `web/dist-*` 构建产物；
- 不把已有 SQL 批量改写为 GORM ORM CRUD。

## 3. 最终目录边界

```text
wt-media-cloud/
├── cmd/
│   ├── server/
│   ├── migrate/
│   ├── discovery-scheduler/
│   └── discovery-worker/
├── config/
├── config_online/
├── internal/
│   ├── bootstrap/
│   │   ├── bootstrap.go
│   │   ├── resources.go
│   │   ├── modules.go
│   │   ├── routes.go
│   │   └── jobs.go
│   ├── config/
│   ├── server/
│   ├── infra/
│   │   ├── database/
│   │   ├── cache/
│   │   ├── logger/
│   │   ├── client/
│   │   ├── metrics/
│   │   ├── tracing/
│   │   └── storage/
│   ├── middleware/
│   ├── scheduler/
│   ├── jobs/
│   ├── shared/
│   │   ├── api/
│   │   └── id/
│   └── modules/
├── migrations/
├── scripts/
└── docs/
```

`internal/runtime` 和 `internal/app` 在迁移完成后删除。不得创建 `common`、`utils` 等职责不明确的万能包。

## 4. 进程生命周期

### 4.1 调用方向

Server 的固定调用方向是：

```text
cmd/server
  -> internal/server.Run
  -> bootstrap.InitializeServer
  -> Hertz.Run
  -> bootstrap.Close
```

`internal/server` 只负责启动、信号监听、优雅关闭和调用 Bootstrap。它不读取配置、不创建基础资源、不组装业务模块、不注册路由。

### 4.2 Bootstrap 入口

Bootstrap 提供进程级入口：

- `InitializeServer`：初始化 HTTP Server 所需资源、Middleware、Module 和 Router；
- `InitializeScheduler`：初始化 Scheduler 扫描所需资源；
- `InitializeWorker`：初始化 Worker 执行所需资源和外部 Client；
- `InitializeMigration`：初始化迁移进程所需数据库资源。

所有初始化都必须从 `bootstrap.go` 的上述入口发起。辅助文件只承载私有实现：

- `resources.go`：基础资源初始化与关闭函数；
- `modules.go`：必要的模块组装和单向跨模块连接；
- `routes.go`：连接模块路由，不创建业务资源；
- `jobs.go`：连接 Scheduler、Worker 与 Job，不实现业务任务。

每个入口只初始化当前进程真正需要的资源。初始化任一步骤失败时，按逆序关闭已经成功初始化的资源并返回原始错误。

## 5. 受控全局基础资源

基础资源由各自的 Infra 包私有保存，通过只读 Getter 对外提供。禁止导出可变全局变量。

```go
var primary *gorm.DB

func DB() *gorm.DB
func Named(name string) (*gorm.DB, bool)
```

规则如下：

- Bootstrap 是唯一允许调用资源初始化和关闭函数的生产代码；
- Module、Service 和 Repository 不能初始化 DB、Logger、Client、Metrics 或 Tracing；
- 业务请求状态、用户状态和领域数据不得存放为全局变量；
- 不再通过 `runtime.Runtime` 参数逐层传递资源；
- 不提供 `runtime.Current`、万能 Service Locator 或可变资源注册表；
- Getter 在资源未初始化时必须快速失败，不能静默返回不可用对象。

## 6. 配置体系

### 6.1 配置来源

运行时代码只读取 `./config`。不支持通过环境变量在 `config` 与 `config_online` 之间切换。

`config_online` 仅作为发布输入，打包脚本在产物中执行：

```text
config_online -> release/config
```

`internal/config` 只保存 Loader、Schema 和 Validation，不保存 YAML 或 JSON 文件。

### 6.2 App 与 Scheduler

`config/app.yaml` 保存应用元数据和 HTTP Server 配置，不再保留独立的 `server.yaml`。

Scheduler 和 Worker 不绑定应用名，其配置继续位于 `config/scheduler`。HTTP Server 不启动 Scheduler 或 Worker。

硬编码的 `1m`、`5s`、`6h` 和批量数全部改为读取并校验配置。配置不增加无意义的全局 `enabled`；具体发现策略是否启用继续由现有业务状态决定。

### 6.3 多数据库

每个数据库在 `config/database` 下使用独立 YAML 文件，并使用 `name` 建立关联：

```yaml
name: primary
host: 127.0.0.1
port: 3306
database: wt_media
username: root
password: secret
charset: utf8mb4
parse_time: true
location: Local
pool:
  max_idle: 10
  max_open: 50
  max_lifetime: 30m
```

规则如下：

- 默认数据库名称在代码中固定为 `primary`；
- 配置不包含 `default` 或 `enabled` 字段；
- 配置文件存在即表示该数据库必须初始化；
- 所有声明的数据库都必须校验和连接成功，否则进程启动失败；
- 缺少 `primary` 时启动失败；
- 密码直接写入相应环境的 YAML，不增加环境变量间接层；
- `database.DB()` 返回 `primary`；
- `database.Named(name)` 返回指定数据库；
- 本次不实现跨数据库事务。

### 6.4 Client 与认证配置分离

每个外部 Client 拥有独立连接配置。Client 配置只描述连接和传输行为，例如名称、协议、主机、端口、超时与重试；不包含具体 API 的 Cookie、API Key 或固定 Header。

```yaml
# config/clients/platforms/douyin.yaml
name: douyin
scheme: https
host: api.example.com
port: 443
timeout: 10s
retry:
  attempts: 2
  interval: 300ms
```

需要认证信息时，使用独立配置域：

```yaml
# config/credentials/douyin.yaml
api_key: secret
cookie: secret
headers:
  user-agent: wt-media-cloud
```

Client 的类型化方法内部读取已经由 Bootstrap 初始化的认证信息并组装请求。业务代码不逐次传递 Cookie、API Key 或 Header。本次只支持一个有效凭证集合，不实现 Cookie/IP 节点池或轮换策略。

## 7. 数据库与 Repository

数据库资源类型统一为 `*gorm.DB`。迁移期间保留显式 SQL 语义：

- 查询使用 `Raw`；
- 写入使用 `Exec`；
- 事务使用 `Transaction`；
- 不批量增加 GORM Model Tag；
- 不把现有 SQL 重写为链式 ORM CRUD。

Repository 是业务数据访问的唯一入口，通过 `database.DB()` 或 `database.Named(name)` 获取连接。Handler 和 Service 不得直接执行 SQL。

每个模块不再调用 Repository `Init` 将同一个数据库复制到模块变量。Repository 的公开能力采用包级函数。

## 8. 日志、Metrics 与 Tracing

### 8.1 六类独立日志

日志分为六个独立配置和六个物理文件：

```text
config/logger/app.yaml       -> logs/app.log
config/logger/access.yaml    -> logs/access.log
config/logger/job.yaml       -> logs/job.log
config/logger/external.yaml  -> logs/external.log
config/logger/audit.yaml     -> logs/audit.log
config/logger/panic.yaml     -> logs/panic.log
```

每个 Logger 私有保存并独立调用：

```go
logger.App()
logger.Access()
logger.Job()
logger.External()
logger.Audit()
logger.Panic()
```

不提供可变的字符串 Logger 注册表。所有日志必须能够携带 `trace_id`、`request_id` 和 `module`。Bootstrap 统一初始化并在关闭阶段刷新全部 Logger。

### 8.2 Metrics 与 Tracing

Metrics 和 Tracing 由各自 Infra 包提供私有实例和只读 Getter，并由 Bootstrap 初始化。当前没有真实后端时保持 Noop 实现，调用方不需要判空。HTTP Middleware、Client 和 Job 使用相同的基础接口。

## 9. Server、Router 与 Module

### 9.1 Router

Router 只声明 Method、Path 和函数 Handler：

```go
h.POST("/api/v1/auth/login", Login)
```

Router 不创建 Service、Repository、Client、Scheduler 或 Worker，不启动后台任务。

### 9.2 Handler

Handler 使用函数，不创建 Handler Struct：

```go
func Login(ctx context.Context, c *app.RequestContext) {
    service.Login(...)
}
```

Handler 负责 HTTP 解码、认证上下文、调用 Service 和写入统一响应，不包含业务规则和 SQL。

### 9.3 模块结构

每个模块最终结构为：

```text
modules/{module}/
├── router.go
├── handler.go
├── dto/
├── service/
├── repository/
└── model/
```

Handler 与 Router 位于模块根包，便于直接绑定函数。较大的 Handler 可以拆成多个以 `handler_` 开头的根包文件。

- `dto` 是请求和响应 DTO 的类型源；
- `model` 是领域及持久化模型的类型源；
- `service` 只包含业务规则和包级函数；
- `repository` 只包含数据访问和包级函数；
- `model`、`dto` 和 `repository` 不反向引用 `service`；
- 不允许使用类型别名把 `service` 中的类型伪装成 `model` 或 `dto`；
- 模块根包不重新导出整个 Service、Repository 或 Model API。

模块间调用采用明确的单向依赖图。一个模块只能调用另一个模块公开的 Service 函数，不能跨模块访问 Repository，也不能产生循环依赖。当前不引入内部 DI 容器或契约注册表。

## 10. Shared 包

保留职责明确的公共定义：

- `internal/shared/api`：统一 HTTP 响应信封、错误响应和 JSON 解码；
- `internal/shared/id`：ID 和 Token 等稳定公共定义。

不创建单独的 `response` 目录，也不恢复万能 `common` 包。`contracts` 继续保存跨进程协议和 Schema，不承载 Go 进程内部的公共工具代码。

统一响应保持：

```json
{
  "errcode": 0,
  "message": "success",
  "data": {},
  "logid": "xxx"
}
```

## 11. 外部 Client 与 Cloud–Agent 边界

所有 HTTP 协议实现都位于 `internal/infra/client`，向业务暴露类型化方法，例如：

```go
douyin.Search(ctx, request)
douyin.FindAuthor(ctx, request)
douyin.FetchByURL(ctx, request)
agent.CheckProxy(ctx, request)
```

业务代码不得构造 `http.Client`，不得调用通用的 `Post(path, body)` 绕过类型边界。Client 统一处理连接、超时、重试、错误映射、External Logger、Metrics 和 Tracing。

最终执行边界为：

- Cloud 是业务事实中心；
- Cloud 可以直接调用抖音公开数据接口，并在 Cloud 内维护所需凭证和代理连接；
- Cloud 的 Crawler Worker 是服务端组件，不是 Desktop Agent；
- Desktop Agent 负责本地浏览器、Profile、账号登录态、本地文件和 FFmpeg 等本机能力；
- 需要本机环境的操作必须经过 Agent，纯服务端公开数据抓取不经过 Agent。

该边界替代旧规范中“所有外部平台调用都只能经过 Agent”的表述。

## 12. 人工搜索同步流程

关键词搜索和博主搜索改为同步执行：

```text
Web
  -> Cloud HTTP Handler
  -> contentpool Service
  -> Douyin typed Client
  -> 同步返回搜索结果
```

规则如下：

- `POST /api/v1/content-pool/search` 同步返回结果；
- `POST /api/v1/content-pool/author-search` 同步返回结果；
- 不创建 `manual_discovery_task`；
- 不返回待执行任务；
- Web 不轮询 Crawl Task；
- 搜索结果不因查询动作自动写入内容池。

用户选择结果后调用：

```http
POST /api/v1/content-pool/import-results
```

请求携带平台和选中的标准化结果。Cloud 重新校验团队权限、平台、内容 ID、来源 URL 及必要字段，然后使用现有 Content Pool 去重和写入逻辑同步入池。请求数据一律视为不可信输入。

现有统一响应信封不变。已有 Crawl Task 查询和确认接口暂时保留，用于历史记录兼容；新的人工搜索前端不再调用它们。分享链接导入不在本次同步搜索改动范围内，保持现有业务行为。

## 13. Scheduler、Job 与 Worker

Scheduler、Job 和 Worker 的职责固定为：

```text
Scheduler
  -> 发现到期策略
  -> 创建 pending Crawl Task

Worker
  -> 原子领取 pending Crawl Task
  -> 调用 Job
  -> Job 同步调用 Douyin Client
  -> 持久化结果和任务状态
```

- Scheduler 只决定何时检查和触发；
- Job 定义一次业务任务执行什么；
- Worker 领取任务、调用 Job，并控制批量执行节奏；
- 调用抖音接口本身是同步过程；
- HTTP Server 不启动 Scheduler 或 Worker；
- 保留独立的 `cmd/discovery-scheduler` 和 `cmd/discovery-worker`；
- 只有 `internal/scheduler` 可以在生产代码中使用 `time.NewTicker`；
- 不引入 MQ 或额外服务。

## 14. 测试设计

### 14.1 数据库测试

Repository 单元测试使用 `go-sqlmock`：

```text
go-sqlmock
  -> *sql.DB
  -> gorm MySQL Dialector
  -> *gorm.DB
  -> Repository test
```

测试不连接真实 MySQL，也不执行真实迁移。Mock 必须验证 SQL、参数、事务提交/回滚和返回值。

受控全局资源的测试安装只允许通过测试辅助函数完成，并在测试结束时恢复；修改全局测试资源的用例不得调用 `t.Parallel()`。普通 Repository 测试优先直接把 Mock `*gorm.DB` 传给包内测试入口，减少全局替换。

### 14.2 分层测试归位

- Handler 测试只验证 HTTP、鉴权、DTO 和响应信封；
- Service 测试验证业务规则；
- Repository 测试验证 SQL；
- Client 测试使用 `httptest.Server` 验证协议、超时、重试和错误映射；
- Scheduler/Worker 测试验证发现与领取职责分离；
- 删除迁移过程中复制到 Handler 目录的 Service 测试。

## 15. 兼容性与明确例外

以下内容保持不变：

- 数据库表结构；
- 除新增同步入池接口外的路由地址；
- `errcode/message/data/logid` 响应信封；
- 既有错误码；
- Cloud–Agent Contract 的字段和版本；
- 现有业务权限和团队隔离规则。

明确允许的行为变化只有：

- 人工关键词和博主搜索由异步任务改为同步返回；
- 新增选择结果同步入池接口；
- 新人工搜索不再生成 Crawl Task；
- Web 取消人工搜索轮询。

## 16. 迁移顺序

实施按以下顺序推进，每一阶段完成后先运行局部测试，再继续下一阶段：

1. 更新 `AGENTS.md` 和架构规范，消除旧 Runtime 与 Agent 边界规则；
2. 调整 Config Schema，并完成 GORM、多数据库和日志基础资源；
3. 建立受控全局 Infra Getter，迁移并删除 `runtime.Runtime`；
4. 建立 `internal/server` 和进程级 Bootstrap，删除 `internal/app`；
5. 完成 Router、Scheduler、Job、Worker 和 Client 边界；
6. 按依赖顺序迁移模块：`identity`、`cloudagent`、`runtimebinding`、`profilebinding`、`profileguard`、`mediaaccount`、`proxy`、`contentpool`；
7. 完成人工同步搜索和 Web 调整；
8. 清理旧兼容层、重复测试和无引用代码；
9. 执行全量自动化验收。

## 17. 验收标准

必须满足：

- `cmd` 入口保持简洁，只解析必要进程参数并调用 Server/Bootstrap；
- `internal/runtime` 和 `internal/app` 已删除；
- 生产代码不存在导出的可变基础资源变量；
- 所有资源初始化都由 Bootstrap 发起；
- Router 不创建资源或启动任务；
- Handler 为直接绑定的函数；
- Module 类型和依赖方向符合本设计；
- 数据库统一使用 `*gorm.DB`，现有 SQL 语义保持；
- 六类 Logger 可独立调用并写入独立物理文件；
- Client 连接配置与认证配置分离；
- 人工搜索同步返回且不创建任务；
- Scheduler 只发现任务，Worker 只领取并执行任务；
- `go test ./...` 通过；
- `go test -race ./...` 通过；
- `go vet ./...` 通过；
- `gofmt -l` 无输出；
- `git diff --check` 通过；
- 边界扫描确认 Module 不直接创建 HTTP Client、不读取进程环境、不创建 Ticker、不执行 SQL；
- `go-sqlmock` 覆盖数据库成功、失败和事务路径；
- 不要求真实 MySQL 验证。

## 18. 规范同步要求

实施的第一步必须更新以下旧规则：

- `AGENTS.md` 中“Runtime 保存全部资源”和“禁止全局单例”；
- `AGENTS.md` 中“基础资源必须通过 Runtime 获取”；
- 架构文档中独立 `server.yaml` 的描述；
- 架构文档中所有外部平台调用只能经过 Agent 的描述；
- 旧实施计划中继续向 Runtime 注入 Client、Metrics 和 Tracing 的任务。

更新后的规则必须明确区分“受控全局基础资源”和“禁止全局业务状态”，避免后续实现再次回到大 Runtime 或裸露的可变全局变量。

## 19. 决策完成状态

本设计不存在待确认的架构项。实施过程中如发现必须修改数据库结构、公共 API、错误码或 Cloud–Agent Contract，必须停止相应改动并重新形成架构决策，不能以重构名义自行扩大范围。
