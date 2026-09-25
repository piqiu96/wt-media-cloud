# wt-media-cloud Agent Index

> 本文件是本仓**全部正式内容**的唯一落点：定位、职责边界、需求路由、**本仓规则**、禁止项与本仓内加载顺序。`CLAUDE.md` 与 `AGENTS.md` 是指针，只声明本文件的位置，不承载任何规则。

## 依赖

关于文档等事实都在`../wt-media-workspace`，需要执行时优先考虑对应的约束边界。

治理上下文（当前 CHG、Spec、Contract）在 `../wt-media-workspace`，按其 `.ai/CURRENT_CONTEXT.md` 指引加载；本仓库不做运行时依赖。

## 定位

本仓库是 WT Media 的**业务事实中心**，同时承载统一 Vue 前端工程。技术栈：Go + Hertz 模块化单体、MySQL 正式业务数据、Vue 3 + Vite。

## 本仓库拥有

- 业务对象、状态流转、权限校验、正式业务数据、事务与幂等。
- Cloud API 与 Agent API。
- Cloud 自有执行链路：当前 M3 内容发现由 Scheduler + Crawler（`internal/scheduler`、`internal/jobs/discovery.go`）直接抓取并写入业务表，依据 ADR-0015，不经过 Python Agent。
- M4-M5 内容生产由 Cloud Scheduler / Worker / FFmpeg 执行；`compose_task`、`file_transfer_task` 与 `composite_output` 事实在 Cloud，依据 ADR-0017。
- **全部 Vue 源码**（`web/`）：Cloud Web 应用（`web/src/apps/cloud`）与 Desktop Vue 应用（`web/src/apps/desktop`）共用 `web/src/modules` 业务模块和 `web/src/shared` 通用能力。

## 本仓库不拥有

- 运营电脑上的 BitBrowser、Cookie、本地文件、浏览器自动化 → `../wt-media-agent`
- 下载到运营电脑、打开本地文件、发布和互动执行 → `../wt-media-agent`
- Tauri 原生能力、Sidecar 进程管理、安装包 → `../wt-media-desktop`

## 需求路由

| 需求是 | 去哪里 |
|---|---|
| 改业务规则、状态流转、API | `internal/modules/<模块>`，先查 `DIRECTORY_MAP.md` 模块表 |
| 改内容发现策略与 M3 抓取执行 | `internal/jobs/discovery.go`、`internal/modules/contentpool/service/` |
| 改调度触发 | `internal/scheduler/` |
| 改 M4-M5 合成、文件准备或对象存储 | Cloud 生产模块、`internal/jobs/`、`internal/infra/media`、`internal/infra/storage`；不存在的目录按已批准 CHG 增量创建 |
| 改 Cloud Web 页面 | `web/src/apps/cloud/` 或 `web/src/modules/` |
| 改 Desktop 使用的 Vue 业务页面 | `web/src/modules/` 或 `web/src/apps/desktop/`（**不在** `wt-media-desktop` 仓库） |
| 改 Desktop 的「本机设置」页或日志查看器 | `web/src/apps/desktop/features/local-settings/`、`local-logs/`（页面逻辑放同名的纯 JS 模块里才有单测——本仓 vitest **不编译 `.vue`**）；命令面在 `../wt-media-desktop`（`invoke` 的参数名必须与 Rust 的 snake_case 对上） |
| 改 Desktop Vue Runtime 适配 | `web/src/apps/desktop/runtime/`，必要时联动 `../wt-media-desktop` |
| 改 Desktop 的 Cloud 地址 / Local Agent 端口来源 | `web/src/apps/desktop/features/local-agent/init.js`（经 `get_public_config`，**不得**落回环字面量），边界由 `web/src/localAgentBoundary.test.js` 常驻守护 |
| 改外部 HTTP 调用 | `internal/infra/client/` |
| 改 Cloud–Agent 契约服务端 | `internal/modules/cloudagent/` 与 `contracts/` |

## 本仓规则

本仓全部规则的唯一落点。一条一行，写清做什么／不做什么。逐项目录事实与禁止扫描区见 `DIRECTORY_MAP.md`。

### 项目定位

本项目使用 Go、CloudWeGo Hertz 和模块化单体架构。禁止提前拆分微服务，禁止未经决策引入 MQ、动态插件或工作流引擎。

`docs/superpowers/` 下的设计、计划与交接是**分析材料，不作为新开发依据**；分析材料必须经 Delivery 或 Decision 确认后才可驱动修改（依据 `../wt-media-workspace/AGENT-INDEX.md` §3）。

### 进程与目录

- `cmd/server/main.go` 只调用同包运行函数；`cmd/server/server.go` 管理 HTTP 启动、信号和优雅关闭。
- `internal/bootstrap` 是生产初始化的唯一发起方。
- `bootstrap.go` 管进程初始化顺序，`resource.go` 管资源初始化和回滚，`routes.go` 只注册路由，`jobs.go` 只连接任务。
- 不创建 `internal/runtime`、`internal/app`、`internal/server` 或 `bootstrap/modules.go`。
- `internal/infra` 保存 `client`、`database`、`logger`、`metrics`、`tracing` 等基础能力。
- `internal/infra/media` 与 `internal/infra/storage` **尚未创建**：M4-M5 落地时按已批准 CHG 增量创建（见「需求路由」）。目录事实一律以 `DIRECTORY_MAP.md` 为准。
- `internal/modules` 只保存业务模块。
- `internal/shared` 只允许职责明确的公共定义，例如 `shared/api` 和 `shared/id`；禁止万能 `common`、`utils` 或 `resource` 包。

### 受控全局基础资源

- Database、Logger、Client、Metrics、Tracing 和只读 Config 由各自包私有保存，通过公有只读 Getter 使用。
- 禁止导出可变资源变量，禁止 `runtime.Current`、万能 Service Locator 或动态资源注册表。
- Bootstrap 是唯一允许初始化和关闭基础资源的生产代码。
- 业务请求状态、用户状态和领域数据不得保存为全局变量。
- Module、Handler、Service、Repository 不得初始化基础资源。

### 配置规则

- 配置文件只允许位于 `/config` 和 `/config_online`。
- 运行时代码始终读取 `./config`；`config_online` 仅在发布打包时替换产物中的 `config`。
- 禁止在 `internal` 存放 YAML 或 JSON 配置。
- `internal/config` 只负责 Loader、Schema、Validation 和只读 Getter。
- HTTP Server 配置属于 `app.yaml`，不创建独立 `server.yaml`。
- Scheduler/Worker 配置位于 `config/scheduler`，不绑定 App 名称。
- 每个数据库使用独立 YAML；默认数据库名称在代码中固定为 `primary`，配置不使用 `default` 或 `enabled`。
- Client 连接配置与 Cookie、API Key、固定 Header 等认证配置必须分离。

### Database 与 Repository

- 数据库资源统一为 `*gorm.DB`。
- 保留显式 SQL：查询使用 `Raw`，写入使用 `Exec`，事务使用 `Transaction`；不得批量改写为 GORM ORM CRUD。
- Repository 是业务数据访问的唯一入口，通过 `database.DB()` 或 `database.Named(name)` 获取连接。
- Handler 和 Service 禁止执行 SQL。
- Repository 不得通过模块级 `Init` 复制同一个数据库实例。

### Client 与执行边界

- 所有外部 HTTP 协议实现位于 `internal/infra/client`，统一处理 timeout、retry、错误映射、日志、trace 和 metrics。
- 业务代码禁止直接创建 `http.Client`，只能调用 `Search`、`FindAuthor`、`FetchByURL`、`CheckProxy` 等类型化方法。
- Cloud 是业务事实中心，可以直接调用抖音等公开数据 API。
- Desktop / Local Agent 负责本地浏览器、Profile、账号登录态、本地文件落地、发布和互动等本机能力。
- M4-M5 视频合成由 Cloud Compose Worker 调用受控 FFmpeg；Desktop、Local Agent 和 Cloud Agent 均不得执行该视频合成链路。
- 需要本机环境的操作必须经过 Agent；纯服务端公开数据抓取不经过 Agent。

### 外部服务接入

新增第三方服务必须：

1. 增加 `config/clients` 配置；
2. 创建 `internal/infra/client` 实现；
3. 基础资源由 Bootstrap 统一初始化，业务侧经各包公有只读 Getter 使用（不引入 `internal/runtime`）；
4. 增加日志、trace、metrics。

禁止业务代码直接 HTTP 调用。

### 日志规则

- 禁止使用 `fmt.Println` 记录运行日志。
- 使用 `logger.App()`、`Access()`、`Job()`、`External()`、`Audit()`、`Panic()` 六个独立 Logger。
- 六类 Logger 写入六个独立物理文件。
- 请求相关日志必须能够包含 `trace_id`、`request_id` 和 `module`。

### Scheduler、Job 与 Worker

- HTTP Server 不启动 Scheduler 或 Worker。
- Scheduler 只负责发现到期任务并创建 pending 任务。
- Worker 只负责原子领取任务并调用 Job 执行。
- Job 定义一次任务执行什么，外部 API 调用是同步过程。
- 只有 `internal/scheduler` 可以在生产代码中使用 `time.NewTicker`。
- 业务模块和 `init()` 禁止启动后台任务。

### Module 规则

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

### Shared 与 API

- HTTP 响应信封、错误响应和 JSON 解码归 `internal/shared/api`。
- ID、Token 等稳定公共定义归 `internal/shared/id`。
- 不创建单独的 `response` 目录。
- 所有 HTTP 接口保持 `errcode`、`message`、`data`、`logid` 响应信封。
- 除已批准的同步人工搜索外，重构不得改变路由、错误码、数据库结构或 Cloud–Agent Contract。

### 测试与提交

- 新行为和重构必须测试先行，对于测试不需要强行过度测试，只需要关键链接测试严谨即可
- Repository 单元测试使用 `go-sqlmock` 包装为 `*gorm.DB`，不依赖真实 MySQL。
- 每个迁移阶段运行局部测试，最终运行 `go test ./...`、`go vet ./...`、格式和架构边界扫描。
- 保留用户已有工作区改动；禁止使用破坏性 Git 命令。
- 不修改无关的 `web/dist-*` 构建产物。
- 待一个较大重构完成以后，再摸底集成测试

### 修改原则

修改代码前：

1. 阅读已有架构；
2. 优先复用已有基础能力；
3. 不新增重复框架；
4. 不创建万能目录。

### 提交要求

代码必须：

- 可运行；
- 符合分层；
- 无重复初始化；
- 不破坏架构边界。

## 禁止

- 绕过 Local Agent 直接操作运营电脑的 BitBrowser、Cookie、本地文件。
- Go HTTP 接口直接承担需要独立执行和恢复的长时间任务。
- 跨模块直接改其他模块的数据（只能单向调用目标模块 Service）。
- Cloud Web 页面直接执行 Tauri 原生操作。
- 在 Desktop 仓库复制一套 Vue 业务页面。
- 重新引入已被取消的 Agent 内容发现任务链路。
- 把 M4-M5 视频合成或 FFmpeg 执行下放到 Desktop、Local Agent 或 Cloud Agent。
- Desktop 页面里出现 `127.0.0.1` / `localhost` / `fetch(`：窗口的 CSP 挡着，直连只会得到一句「Agent 不可达」；本机能力一律经 `invoke` 走 Rust（`web/src/localAgentBoundary.test.js` 常驻守护，**新增模块要显式进那份模块清单**）。
- 在 `.vue` 页面里放可测逻辑：本仓 vitest 不编译 `.vue`，写在那里等于写在没有单测的地方（页面逻辑放同名的纯 JS 模块，页面只负责渲染）。

## 本仓内加载顺序

本节只写**本仓内**的入口顺序；跨仓读取顺序与全部红线的唯一落点是 `../wt-media-workspace/AGENT-INDEX.md` §4 与 §2，本节不复述。

1. 本文件（职责、路由与本仓规则）
2. `DIRECTORY_MAP.md`（目录导航，只记录实际存在的目录）
3. 只读目标模块的代码与直接依赖、测试

禁止默认扫描的目录见 `DIRECTORY_MAP.md` 的「禁止扫描区」。
