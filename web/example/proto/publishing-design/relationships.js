// Prototype-only relationships and dispatch. No platform API or production data access.
import { platforms, validate, nextBatch, stats } from "./model.js";
export const batchTasks = (db, batch) =>
  db.tasks.filter((t) => t.batchId === batch.id);
export const planItems = (plan) => plan.items || [];
export const itemTasks = (db, item) =>
  item?.kind === "batch"
    ? db.batches.some((b) => b.id === item.id)
      ? batchTasks(
          db,
          db.batches.find((b) => b.id === item.id),
        )
      : []
    : item?.kind === "task"
      ? db.tasks.filter((t) => t.id === item.id && !t.batchId)
      : [];
export const planTasks = (db, plan) => [
  ...new Map(
    planItems(plan)
      .flatMap((item) => itemTasks(db, item))
      .map((t) => [t.id, t]),
  ).values(),
];
export const itemRecord = (db, item) =>
  (item.kind === "batch" ? db.batches : db.tasks).find((r) => r.id === item.id);
export const sameItem = (a, b) => a.kind === b.kind && a.id === b.id;
export const associatedPlan = (db, item) =>
  db.plans.find((p) => planItems(p).some((i) => sameItem(i, item)));
export function batchSummary(db, batch) {
  const rows = batchTasks(db, batch);
  return {
    ...stats(rows),
    videos: new Set(rows.map((t) => t.assetId)).size,
    categories: [...new Set(rows.map((t) => t.category))],
    waiting: rows.filter((t) => t.status === "待执行").length,
    running: rows.filter((t) => ["排队中", "执行中"].includes(t.status)).length,
    unresolved: rows.filter((t) =>
      ["等待人工", "结果待确认"].includes(t.status),
    ).length,
  };
}
export function canLink(db, plan, item) {
  const record = itemRecord(db, item),
    rows = itemTasks(db, item);
  if (!record || !rows.length || associatedPlan(db, item)) return false;
  if (item.kind === "task" && record.batchId) return false;
  return rows.every((r) =>
    plan.targets.some(
      (t) => t.platform === r.platform && t.category === r.category,
    ),
  );
}
export function linkItem(db, plan, item) {
  if (!canLink(db, plan, item))
    throw Error("此项目已关联计划、没有任务或不符合计划目标");
  plan.items.push({ ...item });
  if (plan.position >= plan.items.length - 1 && plan.control === "执行已结束")
    plan.control = "已暂停";
}
export function moveItem(plan, index, delta) {
  const to = index + delta;
  if (to < 0 || to >= plan.items.length) return false;
  // Completed/current dispatch order is immutable once dispatch starts.
  if (
    plan.control !== "规划中" &&
    (index <= plan.position || to <= plan.position)
  )
    return false;
  [plan.items[index], plan.items[to]] = [plan.items[to], plan.items[index]];
  return true;
}
export function validateBatch(db, platform, rows) {
  const errors = [];
  if (!platforms.includes(platform))
    errors.push("一个批次必须选择一个有效平台");
  if (!rows.length) errors.push("批次至少需要一条任务");
  if (rows.some((r) => r.platform !== platform))
    errors.push("批次任务必须全部属于当前平台");
  if (new Set(rows.map((r) => r.assetId)).size !== rows.length)
    errors.push("同平台成片不能重复安排");
  for (const r of rows) {
    if (db.assets.find((a) => a.id === r.assetId)?.category !== r.category)
      errors.push("任务类目必须与成片类目一致");
    errors.push(...validate(r, db));
  }
  return [...new Set(errors)];
}
export function createPublication(
  db,
  { platform, rows, name, planId = null, quick = false },
) {
  const errors = validateBatch(db, platform, rows);
  if (quick && rows.length !== 1) errors.push("快速发布只能创建一条独立任务");
  if (!quick && !name?.trim()) errors.push("请填写批次名称");
  const plan = db.plans.find((p) => p.id === planId);
  if (planId && !plan) errors.push("关联计划不存在");
  if (
    plan &&
    rows.some(
      (r) =>
        !plan.targets.some(
          (t) => t.platform === r.platform && t.category === r.category,
        ),
    )
  )
    errors.push("内容不符合计划目标");
  if (errors.length) throw Error(errors.join("；"));
  const ids = ["tasks", "batches", "plans", "images", "templates"].flatMap(
    (key) => db[key].map((r) => r.id),
  );
  let nextId = Math.max(1000, ...ids) + 1;
  const batch = quick
    ? null
    : {
        id: nextId++,
        name: name.trim(),
        platform,
        categories: [...new Set(rows.map((r) => r.category))],
        control: "待执行",
      };
  const tasks = rows.map((r) => ({
    ...JSON.parse(JSON.stringify(r)),
    id: nextId++,
    batchId: batch?.id || null,
    status: "待执行",
    result: "未提交",
    attempts: 0,
    log: [],
    created: new Date().toLocaleString("zh-CN"),
  }));
  if (batch) db.batches.push(batch);
  db.tasks.push(...tasks);
  if (plan)
    linkItem(db, plan, {
      kind: batch ? "batch" : "task",
      id: batch?.id || tasks[0].id,
    });
  return { batch, tasks };
}
export function queueTask(db, task) {
  if (task.status !== "待执行") return false;
  if (validate(task, db, task.id).length) return false;
  task.status = "排队中";
  task.result = "未提交";
  task.attempts++;
  task.attemptHistory ||= [];
  task.attemptHistory.push({
    number: task.attempts,
    started: new Date().toLocaleString("zh-CN"),
    status: "排队中",
  });
  task.log ||= [];
  task.log.push({
    time: new Date().toLocaleTimeString("zh-CN"),
    text: "本地演示入队",
  });
  return true;
}
export function dispatchBatch(db, batch) {
  const rows = batchTasks(db, batch);
  if (
    !platforms.includes(batch.platform) ||
    rows.some((t) => t.platform !== batch.platform)
  )
    throw Error("批次平台与任务不一致，不能启动");
  const pending = rows.filter((t) => t.status === "待执行");
  if (pending.some((t) => validate(t, db, t.id).length))
    throw Error("当前批次有任务未通过执行前检查，请编辑后重试");
  pending.forEach((t) => queueTask(db, t));
  batch.control = rows.some((t) =>
    ["排队中", "执行中", "等待人工", "结果待确认"].includes(t.status),
  )
    ? "执行中"
    : "执行已结束";
}
export function dispatchPlan(db, plan) {
  if (!plan.items.length) throw Error("请先创建或关联批次、独立任务");
  if (plan.position >= plan.items.length)
    throw Error("所有计划项均已结束，请关联补充批次");
  const current = plan.items[plan.position];
  if (
    plan.items.some(
      (i, index) =>
        index !== plan.position &&
        i.kind === "batch" &&
        itemRecord(db, i)?.control === "执行中",
    )
  )
    throw Error("其他计划项正在独立执行，请先暂停该批次再恢复计划");
  if (current.kind === "batch") dispatchBatch(db, itemRecord(db, current));
  else {
    const t = itemRecord(db, current);
    if (t?.status === "待执行" && !queueTask(db, t))
      throw Error("独立任务未通过执行前检查");
  }
  plan.control = "执行中";
  syncPlan(db, plan);
}
export function advance(db, plan, confirmed = false) {
  if (plan.control === "已暂停" || plan.control === "规划中")
    throw Error("请先启动或恢复计划");
  const decision = nextBatch(itemTasks(db, plan.items[plan.position]));
  if (decision === "等待核实") throw Error("当前项目仍有未决任务，不能衔接");
  if (decision !== "自动继续" && !confirmed) throw Error("需要人工确认后继续");
  if (decision === "等待跳过确认")
    plan.skipped.push({ ...plan.items[plan.position] });
  const current = itemRecord(db, plan.items[plan.position]);
  if (plan.items[plan.position]?.kind === "batch" && current)
    current.control = "执行已结束";
  plan.position++;
  if (plan.position >= plan.items.length) {
    plan.control = "执行已结束";
    return;
  }
  dispatchPlan(db, plan);
}
export function syncPlan(db, plan) {
  if (plan.control !== "执行中") return;
  const item = plan.items[plan.position];
  if (!item) {
    plan.control = "执行已结束";
    return;
  }
  const decision = nextBatch(itemTasks(db, item));
  if (decision === "自动继续") advance(db, plan);
  else if (decision === "等待继续确认" || decision === "等待跳过确认")
    plan.control = decision;
}
// Migration preserves task identity/results. Mixed old groups split only by observed platform,
// never by invented historical creation boundaries. Plan refs expand in the same position.
export function migrateRelationships(db) {
  if (db.relationshipVersion === 2) return db;
  let nextId =
    Math.max(
      1000,
      ...["tasks", "batches", "plans", "images", "templates"].flatMap((k) =>
        (db[k] || []).map((r) => Number(r.id) || 0),
      ),
    ) + 1;
  const expansions = new Map();
  for (const batch of [...db.batches]) {
    const rows = batchTasks(db, batch),
      groups = [...new Set(rows.map((t) => t.platform))];
    const original = JSON.parse(JSON.stringify(batch));
    if (groups.length > 1) {
      batch.legacySource = original;
      const first = groups.includes(batch.platform)
        ? batch.platform
        : groups[0];
      const ordered = [first, ...groups.filter((p) => p !== first)];
      expansions.set(
        batch.id,
        ordered.map((platform, index) => {
          const b = index
            ? {
                ...JSON.parse(JSON.stringify(original)),
                id: nextId++,
                legacySource: original,
                legacyBatchId: original.id,
              }
            : batch;
          b.platform = platform;
          b.name = `${original.name} · ${platform}`;
          delete b.platforms;
          b.categories = [
            ...new Set(
              rows
                .filter((t) => t.platform === platform)
                .map((t) => t.category),
            ),
          ];
          if (index) {
            db.batches.push(b);
            rows
              .filter((t) => t.platform === platform)
              .forEach((t) => {
                t.legacyBatchId ||= original.id;
                t.batchId = b.id;
              });
          }
          return b.id;
        }),
      );
    } else {
      if (batch.platforms || (groups[0] && batch.platform !== groups[0]))
        batch.legacySource = original;
      if (groups.length) batch.platform = groups[0];
      delete batch.platforms;
      batch.categories = [...new Set(rows.map((t) => t.category))];
    }
  }
  for (const plan of db.plans) {
    const oldItems = plan.items || [],
      oldPosition = plan.position || 0;
    let position = 0;
    plan.items = oldItems.flatMap((i, index) => {
      const item = typeof i === "object" ? i : { kind: "batch", id: i };
      const refs =
        item.kind === "batch"
          ? (expansions.get(item.id) || [item.id]).map((id) => ({
              kind: "batch",
              id,
            }))
          : [item];
      if (index < oldPosition) position += refs.length;
      return refs;
    });
    plan.position = position;
    plan.skipped = (plan.skipped || []).flatMap((i) =>
      typeof i === "object"
        ? [i]
        : (expansions.get(i) || [i]).map((id) => ({ kind: "batch", id })),
    );
  }
  if (db.publishDraft?.platforms?.length > 1) {
    db.legacyPublishDraft = db.publishDraft;
    db.publishDraft = null;
  }
  db.relationshipVersion = 2;
  return db;
}
