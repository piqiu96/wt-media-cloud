# Content Mining Domain and Operator Flow Design

- Date: 2026-09-22
- Status: Approved
- Scope: 挖掘策略 / 挖掘任务 / 内容池发现入口 的领域模型与运营链路优化，不改导航、不引新框架、不做复杂编排。

## Domain model

```
Strategy（规则：找什么、怎么找、何时找、何时自动转素材）
   ↓ 执行
Task（一次实际执行：谁触发、为什么触发、过程、结果、失败原因）
   ↓ 沉淀
Content Pool（运营资产：审核、转素材、忽略、后续生产）
```

- 策略代表规则，不代表执行。
- 任务代表一次执行，必须能回答来源、触发方式、发现配置、结果、失败原因。
- 内容池代表最终资产。

## Task source model

`来源策略` 改为 `任务来源`，三种取值：

| 来源 | 判定 | 展示 |
| --- | --- | --- |
| 策略 | `task_type = discovery_task` | 策略名称 |
| 人工 | `task_type = manual_discovery_task` 且 `snapshot.operation != url` | 手动发现 |
| 导入 | `task_type = manual_discovery_task` 且 `snapshot.operation = url` | ID导入 |
| 重试 | `task_type = retry_failed_task` | 重试（沿用原策略名） |

任务名称：`来源名_YYYYMMDD-HHMMSS`。策略任务用 snapshot 策略名；人工任务用 `手动发现`；导入任务用 `ID导入`。

人工/导入任务详情不再缺失来源信息：详情展示「发现配置」（关键词列表或 ID/链接），从 `snapshot` 读出。

## Audit fields

策略、任务、内容池统一补充审计信息：

- 创建时间 `created_at`
- 修改时间 `updated_at`
- 操作人 `created_by`（创建人，联 user 显示昵称）

内容池存在审核流，补充：

- 审核时间 `updated_at`（最近一次状态变更时间）
- 审核人 `updated_by`（最近一次状态变更人，需迁移）

`updated_by` 为后续迁移项；本阶段先暴露已有 `created_by/created_at/updated_at`，并新增内容池状态变更审计人字段（迁移 037）。

## List ordering

所有列表默认 `ID 倒序`：策略、任务、内容池。

## Source strategy drawer

任务详情中，来源为策略时，策略名可点击，打开 Drawer 展示策略信息（名称/游戏/平台/类型/关键词/作者/执行周期/转素材规则），底部「查看策略」跳转策略页并自动定位该策略。

## Clickable accent color (UE 规范)

所有「可点击的业务元素」统一使用强调紫色 `#7c3aed`，与蓝色标题外链 `wt-primary-link` 区分：

- 挖掘策略的「挖掘规则」（关键词 N 个 / 作者 N 个）
- 挖掘任务列表/详情的「任务来源」（策略名，点击打开策略详情 Drawer）

规则：可点击 → 紫色 + hover 下划线；不可点击 → 普通正文色，不做成链接样。

## Non-goals

不做复杂规则引擎、DAG、Trace、AI 推荐、复杂审核流。
