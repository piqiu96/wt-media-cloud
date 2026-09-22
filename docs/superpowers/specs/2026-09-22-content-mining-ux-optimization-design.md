# Content Mining UX Optimization Design

- Date: 2026-09-22
- Status: Approved
- Scope: `挖掘策略` and `挖掘任务` pages only; no navigation, framework, or task-system changes.

## Goal

Make the content-mining pages readable by operators. Strategy page communicates rules and effectiveness; task page communicates execution results and next actions. Keep the current WT Media visual style and component system.

## Product decisions and data boundaries

1. **Strategy list fields**: 策略名称 / 类型 / 挖掘规则 / 执行周期 / 转素材规则 / 最近效果 / 状态 / 操作.
   - `游戏` is **not** in the current strategy data model (`discovery_strategies` has no game column). It is deferred; adding it requires a strategy-game association migration and is out of scope for this UI pass. The column is omitted rather than fabricated.
2. **挖掘规则** renders as `关键词 5个` or `作者 3个` and opens a detail modal. Keyword lists come from `config.keywords`; author entries from `config.author`/`author_id` are placeholders because author strategies remain maintenance-disabled (`粉丝量` shown as `-`).
3. **转素材规则** renders business language: `赞≥1万 且 藏≥500`, `赞≥1万 或 藏≥500`, or `关闭`.
4. **最近效果** is derived from the latest task snapshot stats (found/added/auto/pending).
5. **Operations** (max five): `执行` `记录` `编辑` `复制` `删除`.
   - `记录` navigates to `/crawl-tasks?strategy_id=`.
   - `复制` is frontend-only: opens the editor prefilled with a `副本` name.
   - `删除` uses a new `DELETE /api/v1/discovery-strategies/:id` endpoint.
   - Disabled/enabled and running states follow the spec.
6. **Task name** is derived frontend from `snapshot.strategy_name` + `created_at` as `策略名_YYYYMMDD-HHMMSS` (snapshot name, not live name).
7. **来源策略** is clickable and navigates to `/discovery-strategies?strategy_id=` which auto-opens that strategy.
8. **触发方式** is derived: empty `schedule_key` → 手动, otherwise 定时.
9. **发现结果** renders `发现X 新增Y 自动素材Z 待审核W 失败V`.
10. **Task detail** is a drawer with tabs: `结果概览` `发现内容` `执行过程` `异常记录`.
    - `执行过程` is a synthesized business timeline from lifecycle timestamps and stats (no dev logs).
    - `异常记录` derives from failed results (`failure_reason`) plus the task `error`.
    - Retry is relabeled `重新执行` (internally still `retry-failed`; successes are skipped).
11. **Title truncation**: content titles truncate to 120 characters with hover tooltip.

## Non-goals

No new task orchestration, DAG, trace, scheduling center, AI recommendations, or complex rule editor. No game-association migration. No new UI framework.

## Acceptance

- Operator can search/filter strategies, see effectiveness, inspect keyword/author config, edit thresholds, copy, run, and delete.
- Operator can see which strategy ran, how many items were found, inspect items, view execution process and failure reasons, and re-run failed tasks.
