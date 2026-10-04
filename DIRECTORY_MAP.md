# WT Media Cloud：目录地图

本文件只用于代码定位、验证入口和默认扫描边界。架构与执行规则见 `AGENT-INDEX.md`，实现事实以当前代码为准。

## Go 服务

| 位置 | 主要内容 | 适用任务 |
| --- | --- | --- |
| `cmd/server/`、`internal/bootstrap/` | 进程入口、路由装配、资源初始化与生命周期 | 启动流程、依赖装配、服务初始化 |
| `internal/config/`、`cmd/config-check/`、`config/`、`config_online/`、`cmd/wtmctl/`、`internal/deploy/` | 配置加载、运行配置、发布模板、环境渲染与校验 | 配置读取、默认值、发布配置 |
| `internal/infra/` | 数据库、外部客户端、日志、指标、追踪等基础能力 | 外部连接、基础设施、可观测性 |
| `internal/scheduler/`、`internal/jobs/` | Cloud 调度与后台任务执行 | 定时调度、Worker、Job、内容发现等后台任务 |
| `internal/modules/` | 按业务域组织的 Router、Handler、Service、Repository、Model | 业务规则、API、状态和数据访问 |
| `internal/shared/`、`internal/middleware/` | 跨模块共享能力和 HTTP 中间件 | 公共类型、ID、协议辅助和中间件 |
| `migrations/` | 数据库 Schema 迁移 | 表、字段、索引等结构变化 |
| `contracts/` | Cloud 提供或维护的跨仓契约 | API、字段、状态和跨仓协议变化 |

## Vue 前端

| 位置 | 主要内容 | 适用任务 |
| --- | --- | --- |
| `web/src/apps/cloud/` | Cloud Web 应用入口和专属页面 | Cloud Web 页面、路由和应用级能力 |
| `web/src/apps/desktop/` | Desktop Vue 应用入口、专属页面和运行适配 | Desktop 页面、Tauri 调用和桌面运行适配 |
| `web/src/modules/` | Cloud 与 Desktop 共用的业务模块 | 业务页面、Store、业务组件 |
| `web/src/shared/` | 通用 API、Runtime、UI、布局和样式 | 无业务归属的跨应用能力 |
| `web/src/generated/` | 自动生成内容 | 仅在核对生成结果时读取，不手工修改 |

## 测试与验证入口

| 范围 | 优先入口 |
| --- | --- |
| Go | 目标 package/module 的测试，以及仓库已有 Makefile / scripts |
| Vue | `web/package.json` 中已有 scripts 和对应测试 |
| Schema | `migrations/` 与受影响 Repository / Service |
| Contract | 本仓 `contracts/`、Workspace `docs/contracts/` 和相关消费方 |
| 工程验收 | `scripts/` 中已有开发、验证和验收脚本 |

优先复用仓库已有验证方式，不在本文件复制具体命令。

## 工具

- `scripts/`：开发、验证、验收和维护脚本。
- `bin/`：本地进程控制及运行辅助。

## 默认跳过

日常代码检索默认跳过：

- 依赖目录，如 `web/node_modules/`、`vendor/`（如存在）。
- 构建产物，如 `web/dist-cloud/`、`web/dist-desktop/`、`build/`。
- 自动生成内容，如 `web/src/generated/`。
- 日志、缓存、临时文件和覆盖率结果。
- `.git/` 等版本控制元数据。

任务直接涉及这些内容时再进入。