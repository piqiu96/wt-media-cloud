# WT Media Cloud AI Development Rules

## 必须遵守
Follow `AGENT-INDEX.md` for repository boundaries.

## 项目定位

本项目使用 Go、CloudWeGo Hertz 和模块化单体架构。禁止提前拆分微服务，禁止未经决策引入 MQ、动态插件或工作流引擎。

完整设计基线：`docs/superpowers/specs/2026-09-18-cloud-foundation-convergence-design.md`。

## 进程与目录

- `cmd/server/main.go` 只调用同包运行函数；`cmd/server/server.go` 管理 HTTP 启动、信号和优雅关闭。
- `internal/bootstrap` 是生产初始化的唯一发起方。
- `bootstrap.go` 管进程初始化顺序，`resource.go` 管资源初始化和回滚，`routes.go` 只注册路由，`jobs.go` 只连接任务。
- 不创建 `internal/runtime`、`internal/app`、`internal/server` 或 `bootstrap/modules.go`。
- `internal/infra` 保存 database、cache、logger、client、metrics、tracing、storage 等基础能力。
- `internal/modules` 只保存业务模块。
- `internal/shared` 只允许职责明确的公共定义，例如 `shared/api` 和 `shared/id`；禁止万能 `common`、`utils` 或 `resource` 包。

## 受控全局基础资源

- Database、Logger、Client、Metrics、Tracing 和只读 Config 由各自包私有保存，通过公有只读 Getter 使用。
- 禁止导出可变资源变量，禁止 `runtime.Current`、万能 Service Locator 或动态资源注册表。
- Bootstrap 是唯一允许初始化和关闭基础资源的生产代码。
- 业务请求状态、用户状态和领域数据不得保存为全局变量。
- Module、Handler、Service、Repository 不得初始化基础资源。

## 配置规则

- 配置文件只允许位于 `/config` 和 `/config_online`。
- 运行时代码始终读取 `./config`；`config_online` 仅在发布打包时替换产物中的 `config`。
- 禁止在 `internal` 存放 YAML 或 JSON 配置。
- `internal/config` 只负责 Loader、Schema、Validation 和只读 Getter。
- HTTP Server 配置属于 `app.yaml`，不创建独立 `server.yaml`。
- Scheduler/Worker 配置位于 `config/scheduler`，不绑定 App 名称。
- 每个数据库使用独立 YAML；默认数据库名称在代码中固定为 `primary`，配置不使用 `default` 或 `enabled`。
- Client 连接配置与 Cookie、API Key、固定 Header 等认证配置必须分离。

## Database 与 Repository

- 数据库资源统一为 `*gorm.DB`。
- 保留显式 SQL：查询使用 `Raw`，写入使用 `Exec`，事务使用 `Transaction`；不得批量改写为 GORM ORM CRUD。
- Repository 是业务数据访问的唯一入口，通过 `database.DB()` 或 `database.Named(name)` 获取连接。
- Handler 和 Service 禁止执行 SQL。
- Repository 不得通过模块级 `Init` 复制同一个数据库实例。

## Client 与执行边界

- 所有外部 HTTP 协议实现位于 `internal/infra/client`，统一处理 timeout、retry、错误映射、日志、trace 和 metrics。
- 业务代码禁止直接创建 `http.Client`，只能调用 `Search`、`FindAuthor`、`FetchByURL`、`CheckProxy` 等类型化方法。
- Cloud 是业务事实中心，可以直接调用抖音等公开数据 API。
- Desktop Agent 负责本地浏览器、Profile、账号登录态、本地文件和 FFmpeg 等本机能力。
- 需要本机环境的操作必须经过 Agent；纯服务端公开数据抓取不经过 Agent。

## 日志规则

- 禁止使用 `fmt.Println` 记录运行日志。
- 使用 `logger.App()`、`Access()`、`Job()`、`External()`、`Audit()`、`Panic()` 六个独立 Logger。
- 六类 Logger 写入六个独立物理文件。
- 请求相关日志必须能够包含 `trace_id`、`request_id` 和 `module`。

## Scheduler、Job 与 Worker

- HTTP Server 不启动 Scheduler 或 Worker。
- Scheduler 只负责发现到期任务并创建 pending 任务。
- Worker 只负责原子领取任务并调用 Job 执行。
- Job 定义一次任务执行什么，外部 API 调用是同步过程。
- 只有 `internal/scheduler` 可以在生产代码中使用 `time.NewTicker`。
- 业务模块和 `init()` 禁止启动后台任务。

## Module 规则

目标结构：

```text
modules/{module}/
├── router.go
├── handler.go
├── dto/
├── service/
├── repository/
└── model/
```

- Router 只绑定 method、path 和函数 Handler，例如 `h.POST(path, Login)`。
- Handler 位于模块根包，调用包级 Service 函数；禁止 Handler Struct 和 `...any` 依赖查找。
- `dto` 和 `model` 是真实类型源，禁止反向别名到 Service。
- Service 只处理业务规则；Repository 只处理数据访问。
- 模块根包不得重新导出整个 Service、Repository 或 Model API。
- 跨模块只能按固定单向依赖调用目标模块 Service，禁止跨模块访问 Repository 和循环依赖。

## Shared 与 API

- HTTP 响应信封、错误响应和 JSON 解码归 `internal/shared/api`。
- ID、Token 等稳定公共定义归 `internal/shared/id`。
- 不创建单独的 `response` 目录。
- 所有 HTTP 接口保持 `errcode`、`message`、`data`、`logid` 响应信封。
- 除已批准的同步人工搜索外，重构不得改变路由、错误码、数据库结构或 Cloud–Agent Contract。

## 测试与提交

- 新行为和重构必须测试先行，对于测试不需要强行过度测试，只需要关键链接测试严谨即可
- Repository 单元测试使用 `go-sqlmock` 包装为 `*gorm.DB`，不依赖真实 MySQL。
- 每个迁移阶段运行局部测试，最终运行 `go test ./...`、`go vet ./...`、格式和架构边界扫描。
- 保留用户已有工作区改动；禁止使用破坏性 Git 命令。
- 不修改无关的 `web/dist-*` 构建产物。
- 待一个较大重构完成以后，再摸底集成测试

# 禁止扫描
- 对于.gitignore里的包禁止扫描，除此之外还包括dist、node_modules等文件
