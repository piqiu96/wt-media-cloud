# WT Media Cloud Agent Index

本文件只记录 Cloud 的长期工作规则，不记录当前 Milestone、CHG、临时任务状态或本机路径。

## 1. 仓库职责

- Cloud 是业务事实中心，负责业务对象、权限、状态、HTTP API、业务数据，以及 Cloud 自有调度和执行任务状态。
- Cloud 承担已确认的云端执行能力；运营电脑、本地浏览器、登录环境和文件执行归 Agent，Tauri 原生桥与 Local Agent Sidecar 生命周期归 Desktop。
- `web/` 是 Cloud Web 与 Desktop 共用 Vue 业务源码的唯一维护位置。
- Workspace 维护系统级 Product、Architecture、Contract、Decision 和 Delivery；Cloud Runtime 和发布产物不依赖 Workspace。

## 2. Workspace 协作

- 分析、定位和明确的小范围单仓修改可以直接进行，不自动创建 CHG。
- 已关联 CHG 时，以其目标、范围和 References 为任务依据，只实施已确认的 Cloud 范围，执行结束需要把状态写回CHG的status文件。
- References 是优先入口，不限制任务确实需要的进一步查证。
- 涉及跨仓 Contract、系统架构、仓库职责或扩大 CHG 范围时，先回 Workspace 更新共同定义。
- Workspace 发起的任务直接使用当前 Workspace 上下文；独立执行需要 Workspace 时使用已提供的根目录或环境配置，无法定位则要求提供路径。
- 不复制 Workspace 文档，也不在 Cloud 建立第二套 Delivery、任务状态或临时 context。

## 3. 上下文加载

按任务逐步读取：

1. 理解当前任务；有关联 CHG 时读取对应 CHG。
2. 按需要读取相关 Product、Engineering、Contract 或 Decision。
3. 目标位置不明确时用 `DIRECTORY_MAP.md` 定位。
4. 进入目标模块后，再读取直接依赖、调用方和相关测试。

默认不做全仓代码扫描，不读取 `delivery/completed`、历史材料和全量 Skills，也不扫描构建、缓存、临时或生成目录。任务需要时可以扩大范围，并说明目的。

## 4. 本仓架构

- 保持 Go 模块化单体；`internal/bootstrap/` 统一负责基础资源装配和生命周期。
- Handler 负责协议层，Service 负责业务规则，Repository 负责数据访问；跨模块通过 Service 边界协作。
- Repository 延续现有 `*gorm.DB` 和显式 SQL 模式；Handler 与 Service 不直接执行 SQL。
- 外部 HTTP Client 收敛到 `internal/infra/client/`；运行配置从 `config/` 读取，`config_online/` 是版本自包含制品的配置模板源，部署时按环境渲染并校验。
- 对外 HTTP 响应保持 `errcode`、`message`、`data`、`logid`。
- 可独立执行和恢复的长任务走 Scheduler / Worker / Job；Vue 业务源码只在 `web/` 维护。

## 5. 修改与验证

- 从目标代码开始调查；位置不明确时先使用 `DIRECTORY_MAP.md`。
- 实现事实以当前代码、Schema 和 Contract 为准。
- 验证从最小相关范围开始，优先使用仓库已有脚本和测试方式。
- 局部修改验证对应 package / module；影响公共基础设施、启动链路或共享能力时再扩大范围。
- `web/` 变化才执行相关前端验证；Schema、Migration、Contract 或跨仓接口变化需要同时核对受影响的数据路径和消费方。
- 不默认执行全仓构建或全部测试，不修改生成产物，也不覆盖、还原或提交他人已有工作区修改。
- CHG 任务完成后按 Workspace 当前 Delivery 规则回写状态。