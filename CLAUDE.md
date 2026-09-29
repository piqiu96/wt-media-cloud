# WT Media Cloud（Claude Code 入口）

## 项目定位

本仓是 WT Media 的业务事实中心，包含 Go 后端、Cloud Web，以及供 Desktop 使用的 Vue 业务页面；负责正式业务状态、API、数据、Cloud 自有任务及已确认的云端执行能力。

## 关联工程

- `wt-media-agent`：负责运营电脑上的浏览器、Profile、Cookie、本地文件及其他受控执行。
- `wt-media-desktop`：负责 Tauri 原生壳、系统桥、Local Agent Sidecar 生命周期和安装包。
- `wt-media-workspace`：维护系统级产品、架构、跨仓协议、决策和 Delivery；关联工程物理路径由 Workspace 的 `config/repository-map.yaml` 统一维护。

## 任务执行

先用 [AGENT-INDEX.md](AGENT-INDEX.md) 确认本仓规则；目标位置不明确时使用 [DIRECTORY_MAP.md](DIRECTORY_MAP.md) 定位代码。
任务关联 Workspace CHG 时，读取该 CHG 的 Cloud 范围和 References；单仓分析、定位和明确的小修改可直接处理。
按任务逐步读取相关代码、文档和测试，不默认全仓扫描。

## 相关文档

- 本仓 `contracts/`：Cloud 提供或维护的协议；`migrations/`：数据库 Schema 演进。
- Workspace `docs/product/`、`docs/engineering/`、`docs/contracts/`、`docs/decisions/`：系统级事实来源。
- Workspace `delivery/`：Milestone、CHG 和交付状态。
- Workspace 发起或关联 CHG 的任务，可从 `.ai/CURRENT_CONTEXT.md` 获取当前执行快照，并以对应 CHG 确认范围和进度。

## 相关约束

- Cloud 不操作运营电脑上的浏览器、登录环境或本地文件，这些能力归 Agent。
- Vue 业务源码统一维护在本仓 `web/`，不在 Desktop 仓复制第二套业务前端。
- Cloud 的工程分层、数据访问、外部调用和长任务规则以 `AGENT-INDEX.md` 为准。
- Workspace 提供研发和交付上下文，不成为 Cloud Runtime 或发布产物的运行时依赖。