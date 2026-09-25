# wt-media-cloud 目录地图

本文只记录实际存在的目录。定位代码时从本文件出发，禁止全仓库扫描。

## 一、Go 后端

### 入口与生命周期

| 路径 | 职责 | 何时进入 |
|---|---|---|
| `cmd/server/` | 进程入口：`main.go` 只调用运行函数，`server.go` 管理 HTTP 启动、信号和优雅关闭 | 改启动方式、端口、优雅关闭 |
| `internal/bootstrap/` | 生产初始化唯一发起方：`bootstrap.go` 初始化顺序、`resource.go` 资源初始化与回滚、`routes.go` 路由注册、`jobs.go` 任务连接 | 改初始化顺序、新增全局资源、注册路由 |
| `internal/config/` | 配置 Loader、Schema、Validation、只读 Getter | 改配置结构 |

### 基础设施（`internal/infra/`）

| 路径 | 职责 | 何时进入 |
|---|---|---|
| `internal/infra/database/` | `*gorm.DB` 资源与 `DB()` / `Named()` 获取 | 改数据库连接、多库 |
| `internal/infra/logger/` | App / Access / Job / External / Audit / Panic 六类 Logger | 改日志 |
| `internal/infra/client/` | 全部外部 HTTP 协议实现（`agent/`、`observe/`、`platforms/`），统一 timeout、retry、错误映射 | 新增或修改外部 API 调用 |
| `internal/infra/metrics/`、`internal/infra/tracing/` | 指标与链路追踪 | 改可观测性 |

### 调度与任务执行（Cloud 自有执行链路）

| 路径 | 职责 | 何时进入 |
|---|---|---|
| `internal/scheduler/` | Scheduler：发现到期任务并创建 pending 任务，全仓库唯一允许 `time.NewTicker` 的位置 | 改调度周期、触发逻辑 |
| `internal/jobs/` | Job 定义：`discovery.go`（内容发现执行，含抓取与业务表写入，即 M3 Cloud Crawler 链路）、`proxy_expiry.go`（代理过期处理） | 改内容发现执行、抓取逻辑、代理过期任务 |

当前 M3 依据 ADR-0015：内容发现由 Cloud Scheduler + Cloud Crawler（`internal/jobs/discovery.go`、`internal/modules/contentpool/service/discovery_crawler.go`）承担，不经过 Python Agent。修改内容发现策略与抓取执行在本仓库。

### 业务模块（`internal/modules/`）

每个模块结构相同：`router.go`、`handler.go`、`dto/`、`service/`、`repository/`、`model/`。

| 模块 | 业务对象 | 何时进入 |
|---|---|---|
| `identity/` | 用户与权限 | 改登录、权限校验 |
| `mediaaccount/` | 社媒账号业务记录 | 改账号管理 |
| `profilebinding/` | Browser Profile 与账号/环境的绑定关系 | 改 Profile 云端记录 |
| `profileguard/` | Profile 保护规则 | 改 Profile 校验 |
| `proxy/` | 代理资源管理、代理检测结果（`service/check_result.go`） | 改代理管理 |
| `contentpool/` | 内容发现与内容池：`service/discovery.go`、`discovery_crawler.go`（抓取执行）、`manual_search.go`（人工搜索） | 改内容发现、内容池 |
| `cloudagent/` | Agent API 与 Agent 任务管理：任务领取、状态回写（`service/task_store.go`、`registry.go`、`operations.go`） | 改 Cloud–Agent 契约的服务端、任务状态 |
| `runtimebinding/` | 运行时绑定关系 | 改绑定逻辑 |

分层规则：Handler 不执行 SQL；Service 只管业务规则；Repository 是数据访问唯一入口。跨模块只能单向调用目标模块 Service。

### 公共与其他

| 路径 | 职责 |
|---|---|
| `internal/shared/` | `api/`（响应信封、错误响应、JSON 解码）、`id/`、`identity/`、`requestctx/` |
| `internal/middleware/` | HTTP 中间件 |
| `internal/architecture/` | 架构边界守护测试（`boundary_test.go`） |
| `pkg/` | 可独立复用的库代码 |
| `migrations/` | 数据库迁移 |
| `config/`、`config_online/` | 运行时配置与发布打包替换配置（YAML 只允许在这两处） |
| `contracts/` | 跨工程契约：`cloud-api/`、`cloud-agent-api/`、`business-schemas/`、`business-enums/`、`cloud-error-codes/`、`task-schemas/` |
| `scripts/` | 构建与运维脚本 |

## 二、统一 Vue 前端（`web/`）

**Cloud Web 与 Desktop Vue 两套应用共用本工程的模块，不另建第二套 Vue 源码。**

### 应用入口（`web/src/apps/`）

| 路径 | 职责 | 何时进入 |
|---|---|---|
| `web/src/apps/cloud/` | Cloud Web 应用入口：`main.ts`、`router.ts`、`pages/` | 改 Cloud Web 专属页面 |
| `web/src/apps/desktop/` | Desktop Vue 应用入口：`main.ts`、`router.ts`、`App.vue`、`features/`、`runtime/`（Runtime 适配）、`desktopRoleGuard.test.js` | 改 Desktop 专属页面、Desktop Runtime 适配、Webview 错误处理 |
| `web/src/apps/desktop/features/local-agent/init.js` | Desktop 侧唯一知道「Cloud 地址与 Local Agent 端口从哪来」的地方：经 `invoke('get_public_config')` 取得，不落任何回环字面量 | 改 Desktop 的地址来源 |
| `web/src/apps/desktop/features/local-settings/`、`local-logs/` | 「本机设置」页与日志查看器（CHG-058）。**页面逻辑在各自的纯 JS 模块里**（`local-settings-view.js` / `local-logs-view.js` / `service.js`），`.vue` 只负责渲染——本仓 vitest **不编译 `.vue`**，所以放页面里的逻辑没有单测、放页面里的错 import 只有构建能发现；`service.js` 的 `createMockInvoke()` 是 `invoke` 的替身（按 Rust 拼写应答），**不是第二套服务** | 改本机设置页、日志查看器或它们的命令调用 |
| `web/src/localSettingsService.test.js`、`localSettingsView.test.js`、`localLogsView.test.js`、`localSettingsWiring.test.js` | 上面那两页的测试：命令名与**精确**参数对象、纯派生规则的边界、路由名 ↔ 免鉴权名单 ↔ 导航路径三处一致、以及「页面里 import 的名字真的被导出」 | 改页面、服务层或路由装配 |
| `web/src/localAgentBoundary.test.js` | 把「Desktop 页面不直连 Agent 回环端口」变成**常驻断言**（`init.js`、`LocalLogsPage.vue` 加一份**写死的**模块清单；新模块要显式进清单） | 改 Desktop–Agent 边界规则 |

### 共享业务模块（`web/src/modules/`）——按业务域

`accounts`（社媒账号）、`analytics`（数据统计）、`auth`（认证）、`compose`（素材生产）、`contentpool`（内容池）、`dashboard`（仪表盘）、`discovery`（内容发现）、`interaction`（互动管理）、`materials`（素材管理）、`outputs`（成片管理）、`profiles`（Browser Profile）、`proxy`（代理）、`publication`（发布管理）、`tasks`（任务）。

两个应用共用的业务页面、Store、业务组件在此。改 Desktop 业务页面优先在 `modules/` 或 `apps/desktop/` 定位，不去 `wt-media-desktop` 仓库。

### 通用能力（`web/src/shared/`）——与业务无关

`api/`（API 调用层）、`runtime/`（Runtime 接口适配）、`ui/`（通用组件）、`layouts/`、`composables/`、`constants/`、`styles/`、`utils/`。

区分标准：含业务语义 → `modules/`；跨应用通用且无业务语义 → `shared/`。

### 其他

| 路径 | 说明 |
|---|---|
| `web/src/generated/` | 生成代码，禁止手改 |
| `web/src/layout/`、`web/src/styles/` | 全局布局与样式 |
| `web/src/utils.js` | 根级工具 |

## 三、禁止扫描区

以下目录为构建产物或依赖，除非诊断具体失败，禁止扫描或修改：

- `.gitignore` 列出的全部路径，其中还包括 `dist`、`node_modules` 等
- `web/dist-cloud/`、`web/dist-desktop/`（构建产物快照）
- `web/node_modules/`、任何 `node_modules/`
- `logs/`
- `.git/`
