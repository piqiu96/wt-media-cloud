# wt-media-cloud Agent Index

## 依赖

关于文档等事实都在`../wt-media-workspace`，需要执行时优先考虑对应的约束边界

## 定位

本仓库是 WT Media 的**业务事实中心**，同时承载统一 Vue 前端工程。技术栈：Go + Hertz 模块化单体、MySQL 正式业务数据、Vue 3 + Vite。

## 本仓库拥有

- 业务对象、状态流转、权限校验、正式业务数据、事务与幂等。
- Cloud API 与 Agent API。
- Cloud 自有执行链路：当前 M3 内容发现由 Scheduler + Crawler（`internal/scheduler`、`internal/jobs/discovery.go`）直接抓取并写入业务表，依据 ADR-0015，不经过 Python Agent。
- **全部 Vue 源码**（`web/`）：Cloud Web 应用（`web/src/apps/cloud`）与 Desktop Vue 应用（`web/src/apps/desktop`）共用 `web/src/modules` 业务模块和 `web/src/shared` 通用能力。

## 本仓库不拥有

- 运营电脑上的 BitBrowser、Cookie、本地文件、浏览器自动化 → `../wt-media-agent`
- 云端 Python 执行（FFmpeg 合成等）→ `../wt-media-agent`（Cloud Agent 模式）
- Tauri 原生能力、Sidecar 进程管理、安装包 → `../wt-media-desktop`

## 需求路由

| 需求是 | 去哪里 |
|---|---|
| 改业务规则、状态流转、API | `internal/modules/<模块>`，先查 `DIRECTORY_MAP.md` 模块表 |
| 改内容发现策略与 M3 抓取执行 | `internal/jobs/discovery.go`、`internal/modules/contentpool/service/` |
| 改调度触发 | `internal/scheduler/` |
| 改 Cloud Web 页面 | `web/src/apps/cloud/` 或 `web/src/modules/` |
| 改 Desktop 使用的 Vue 业务页面 | `web/src/modules/` 或 `web/src/apps/desktop/`（**不在** `wt-media-desktop` 仓库） |
| 改 Desktop Vue Runtime 适配 | `web/src/apps/desktop/runtime/`，必要时联动 `../wt-media-desktop` |
| 改 Desktop 的 Cloud 地址 / Local Agent 端口来源 | `web/src/apps/desktop/features/local-agent/init.js`（经 `get_public_config`，**不得**落回环字面量），边界由 `web/src/localAgentBoundary.test.js` 常驻守护 |
| 改外部 HTTP 调用 | `internal/infra/client/` |
| 改 Cloud–Agent 契约服务端 | `internal/modules/cloudagent/` 与 `contracts/` |

## 禁止

- 绕过 Local Agent 直接操作运营电脑的 BitBrowser、Cookie、本地文件。
- Go HTTP 接口直接承担需要独立执行和恢复的长时间任务。
- 跨模块直接改其他模块的数据（只能单向调用目标模块 Service）。
- Cloud Web 页面直接执行 Tauri 原生操作。
- 在 Desktop 仓库复制一套 Vue 业务页面。
- 重新引入已被取消的 Agent 内容发现任务链路。

## 上下文加载顺序

1. 本文件（职责与路由）
2. `AGENTS.md` 与 `CLAUDE.md`（架构与编码规则）
3. `DIRECTORY_MAP.md`（目录导航，只记录实际存在的目录）
4. 只读目标模块的代码与直接依赖、测试

治理上下文（当前 CHG、Spec、Contract）在 `../wt-media-workspace`，按其 `.ai/CURRENT_CONTEXT.md` 指引加载；本仓库不做运行时依赖。禁止默认扫描 `web/dist-*`、`node_modules`、`logs` 等产物目录。
