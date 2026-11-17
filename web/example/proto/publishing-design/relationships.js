import { taskInterval, batchTaskCanStart } from "./batchExecutionSettings.js";
import { publishMethod } from "./publishingSettings.js";
import { freezeBatch, isDisposed } from "./batchLifecycle.js";
// Offline prototype business rules. No runtime API or external platform calls.
import { platforms, validate, nextBatch, stats, canRetry } from "./model.js";
export const clone = (x) => JSON.parse(JSON.stringify(x));
export const batchTasks = (db, b) =>
  b ? db.tasks.filter((t) => t.batchId === b.id) : [];
export const planItems = (p) =>
  (p.items || []).filter((i) => i.kind === "batch");
export const itemRecord = (db, i) =>
  i?.kind === "batch" ? db.batches.find((b) => b.id === i.id) : null;
export const itemTasks = (db, i) =>
  i?.kind === "batch" ? batchTasks(db, itemRecord(db, i)) : [];
export const planTasks = (db, p) =>
  planItems(p).flatMap((i) => itemTasks(db, i));
// Old previews can have batch execution records without the plan lock marker.
// Observing a start protects structure; it never grants automatic dispatch authority.
export const planHasStarted = (db, p) =>
  !!(p?.lockedAt || planItems(p || {}).some((i) => itemRecord(db, i)?.firstStartedAt));
export const associatedPlan = (db, i) =>
  i?.kind === "batch"
    ? db.plans.find((p) => planItems(p).some((x) => x.id === i.id))
    : null;
export const targetTotal = (b) =>
  (b.targets || []).reduce((n, t) => n + Number(t.count), 0);
export const unresolved = (t) =>
  ["待执行", "排队中", "执行中", "等待人工", "结果待确认"].includes(t.status);
export const nextId = (db) =>
  Math.max(
    1000,
    ...[
      "tasks",
      "batches",
      "plans",
      "images",
      "templates",
      "assets",
      "accounts",
    ].flatMap((k) => (db[k] || []).map((r) => Number(r.id)).filter(Number.isFinite)),
  ) + 1;
export function batchSummary(db, b) {
  const rows = batchTasks(db, b);
  return {
    ...stats(rows, targetTotal(b)),
    videos: new Set(rows.map((t) => t.assetId)).size,
    categories: b.categories || [],
    waiting: rows.filter((t) => t.status === "待执行").length,
    running: rows.filter((t) => ["排队中", "执行中"].includes(t.status)).length,
    unresolved: rows.filter((t) =>
      ["等待人工", "结果待确认"].includes(t.status),
    ).length,
    target: targetTotal(b),
    creationGap: Math.max(0, targetTotal(b) - rows.length),
    closed: rows.filter(isDisposed).length,
  };
}
export const canLink = () => false;
export function linkItem() {
  throw Error("计划只通过确认拆批生成骨架，不直接绑定独立任务或追加批次");
}
export function moveItem(p, index, delta) {
  const to = index + delta;
  if (p.lockedAt || p.control !== "规划中" || to < 0 || to >= p.items.length)
    return false;
  // Structure changes are saved through the impact-checked plan editor.
  return false;
}
const key = (t) => t.platform + ":" + t.category;
const targetSignature = (targets = []) =>
  JSON.stringify(
    targets
      .filter((t) => Number(t.count) > 0)
      .map((t) => [t.category, Number(t.count)])
      .sort((a, b) => a[0].localeCompare(b[0])),
  );
export function validatePlan(draft) {
  const errors = [];
  if (!draft.name?.trim()) errors.push("请输入计划名称");
  if (!draft.targets?.some((t) => Number(t.count) > 0))
    errors.push("至少填写一个正目标");
  const sums = new Map();
  for (const b of draft.splits || []) {
    if (!platforms.includes(b.platform) || !b.name?.trim())
      errors.push("每批必须有平台和名称");
    for (const t of b.targets || []) {
      if (!Number.isInteger(Number(t.count)) || Number(t.count) < 0)
        errors.push("目标须为非负整数");
      sums.set(
        b.platform + ":" + t.category,
        (sums.get(b.platform + ":" + t.category) || 0) + Number(t.count),
      );
    }
  }
  if (!(draft.splits || []).length) errors.push("请按平台拆分批次");
  if ((draft.splits || []).some((b) => targetTotal(b) <= 0))
    errors.push("每批至少需要一个正目标");
  if (
    new Set(draft.platformOrder).size !== draft.platformOrder?.length ||
    (draft.splits || []).some((b) => !draft.platformOrder?.includes(b.platform))
  )
    errors.push("平台分组顺序必须完整且不重复");
  if (new Set(draft.targets?.map(key)).size !== draft.targets?.length)
    errors.push("目标维度不可重复");
  for (const t of draft.targets || []) {
    if (!Number.isInteger(Number(t.count)) || Number(t.count) < 0)
      errors.push("目标须为非负整数");
    if ((sums.get(key(t)) || 0) !== Number(t.count))
      errors.push(key(t) + "拆批合计与目标不一致");
    sums.delete(key(t));
  }
  if ([...sums.values()].some(Boolean))
    errors.push("存在目标矩阵以外的批次目标");
  return [...new Set(errors)];
}
export function planImpact(db, p, draft) {
  if (planHasStarted(db, p) || ["已取消", "执行已结束"].includes(p?.control))
    throw Error("首次启动后结构永久锁定");
  const errors = validatePlan(draft);
  if (errors.length) throw Error(errors.join("；"));
  const changes = [];
  for (const b of db.batches.filter((b) => b.planId === p?.id)) {
    const s = draft.splits.find((s) => s.id === b.id);
    const changed =
      !s ||
      s.platform !== b.platform ||
      targetSignature(s.targets) !== targetSignature(b.targets);
    if (changed && batchTasks(db, b).length)
      throw Error(b.name + "已有正式任务，禁止修改目标、平台或类目");
    changes.push({
      id: b.id,
      name: b.name,
      candidates: b.candidates?.length || 0,
      changed,
      removed: !s,
    });
  }
  return changes;
}
export function confirmPlan(db, draft, { draftOnly = false } = {}) {
  const existing = db.plans.find(
    (p) => p.id === draft.id || p.confirmKey === draft.confirmKey,
  );
  if (existing?.confirmed && draftOnly)
    throw Error("已确认计划不能退回无骨架草稿，请确认安全修改");
  if (existing?.confirmed && draft.revision === undefined) return existing; // retry after lost response
  const impact = existing?.confirmed ? planImpact(db, existing, draft) : [];
  const errors = validatePlan(draft);
  if (errors.length && !draftOnly) throw Error(errors.join("；"));
  let seq = nextId(db);
  const p = existing || {
    id: draft.id || seq++,
    position: 0,
    items: [],
    skipped: [],
    control: "规划中",
    log: [],
  };
  const splits = clone(draft.splits || []);
  // Stage everything before the single synchronous commit; failures leave no partial objects.
  const staged = draftOnly
    ? []
    : splits.map((s, index) => {
        const old = db.batches.find((b) => b.id === s.id && b.planId === p.id);
        const changed = impact.find((i) => i.id === old?.id)?.changed;
        return {
          ...old,
          id: old?.id || s.id || seq++,
          name: s.name,
          platform: s.platform,
          planId: p.id,
          targets: clone(s.targets),
          categories: s.targets
            .filter((t) => t.count > 0)
            .map((t) => t.category),
          groupOrder: draft.platformOrder.indexOf(s.platform),
          batchOrder: index,
          candidates: old?.candidates || [],
          phase: old?.phase || "待资源配置",
          control: old?.control || "待配置",
          needsPreflight: !!changed,
        };
      });
  Object.assign(p, {
    name: draft.name,
    targets: clone(draft.targets),
    splits,
    platformOrder: clone(draft.platformOrder),
    confirmKey: draft.confirmKey,
    confirmed: !draftOnly,
    revision: (existing?.revision || 0) + 1,
  });
  if (!draftOnly) {
    const ids = new Set(staged.map((b) => b.id));
    db.batches = db.batches.filter((b) => b.planId !== p.id || ids.has(b.id));
    for (const b of staged) {
      const i = db.batches.findIndex((x) => x.id === b.id);
      if (i < 0) db.batches.push(b);
      else db.batches[i] = b;
    }
    p.items = staged.map((b) => ({ kind: "batch", id: b.id }));
  }
  if (!existing) db.plans.push(p);
  return p;
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
  {
    platform,
    rows,
    name,
    planId = null,
    batchId = null,
    sourceBatchId = null,
    sourcePlanId = null,
    quick = false,
    targets = null,
    partial = false,
    creationKey = null,
    taskIntervalSeconds = 0,
  },
) {
  const old = batchId ? db.batches.find((b) => b.id === batchId) : null;
  if (old && batchTasks(db, old).length)
    return { batch: old, tasks: batchTasks(db, old) };
  if (creationKey) {
    const done = db.tasks.filter((t) => t.creationKey === creationKey);
    if (done.length)
      return {
        batch: db.batches.find((b) => b.id === done[0].batchId) || null,
        tasks: done,
      };
  }
  const interval = taskInterval(taskIntervalSeconds);
  const errors = validateBatch(db, platform, rows);
  if (quick && (rows.length !== 1 || planId || batchId))
    errors.push("快速发布只能创建一条计划外独立任务");
  if (!quick && !name?.trim()) errors.push("请填写批次名称");
  if (planId && (!old || old.planId !== planId))
    errors.push("计划只能配置已确认的批次骨架");
  const derived = [...new Set(rows.map((r) => r.category))].map((category) => ({
    category,
    count: rows.filter((r) => r.category === category).length,
  }));
  const target = quick ? derived : old?.targets || targets || derived;
  if (
    target.some(
      (t) => !Number.isInteger(Number(t.count)) || Number(t.count) < 0,
    )
  )
    errors.push("目标须为非负整数");
  if (old?.platform !== undefined && old.platform !== platform)
    errors.push("继承平台不可修改");
  for (const r of rows)
    if (!target.some((t) => t.category === r.category && t.count > 0))
      errors.push("候选超出批次目标类目");
  for (const t of target) {
    const n = rows.filter((r) => r.category === t.category).length;
    if (n > Number(t.count)) errors.push("候选超过类目目标");
    if (n < Number(t.count) && !partial)
      errors.push("创建不足目标，须明确确认部分创建");
  }
  if (errors.length) throw Error([...new Set(errors)].join("；"));
  let seq = nextId(db);
  const b = quick ? null : old || { id: seq++, planId: null, sourceBatchId, sourcePlanId };
  const tasks = rows.map((r) => {
    const { planId: ignored, ...content } = clone(r);
    content.publishMethod ||= publishMethod;
    return {
      ...content,
      id: seq++,
      batchId: b?.id || null,
      creationKey,
      status: "待执行",
      result: "未提交",
      attempts: 0,
      attemptHistory: [],
      log: [],
      version: r.version || 1,
      created: new Date().toISOString(),
      snapshot: clone(content),
    };
  });
  if (b) {
    Object.assign(b, {
      name: name.trim(),
      platform,
      targets: clone(target),
      categories: target.filter((t) => t.count > 0).map((t) => t.category),
      phase: "内容预检通过",
      control: "待执行",
      partialConfirmed: partial,
      rules: { ...(b.rules || {}), taskIntervalSeconds: interval },
      candidates: [],
    });
    b.created ||= new Date().toISOString();
    b.createdBy ||= "本地演示用户";
    b.updatedAt = new Date().toISOString();
    b.version = (b.version || 0) + 1;
    b.log ||= [];
    b.log.push({
      time: b.updatedAt,
      actor: "本地演示用户",
      type: "确认配置",
      text: `已创建 ${tasks.length} 条待执行任务，未启动发布`,
      result: "成功",
    });
    if (!old) db.batches.push(b);
  }
  db.tasks.push(...tasks);
  return { batch: b, tasks };
}
export function queueTask(db, t, { retry = false } = {}) {
  const batch = db.batches.find((b) => b.id === t.batchId);
  if (batch?.stoppedAt || t.batchStopped || batch?.control === "已取消")
    throw Error("批次已停止或取消，不允许新执行");
  if (t.claimed || !(t.status === "待执行" || (retry && canRetry(t))))
    return false;
  const errors = validate(t, db, t.id);
  if (errors.length) throw Error(errors.join("；"));
  if (batch) freezeBatch(db, batch);
  t.status = "排队中";
  t.result = "未提交";
  t.claimed = false;
  t.verifiedAt = null;
  t.attempts++;
  t.attemptHistory ||= [];
  t.attemptHistory.push({
    number: t.attempts,
    started: new Date().toISOString(),
    status: t.status,
    snapshot: clone(t.snapshot || t),
    version: t.version || 1,
  });
  t.log ||= [];
  t.log.push({
    time: new Date().toISOString(),
    text: "本地演示入队 · 内容版本 " + (t.version || 1),
  });
  return true;
}
function ready(db, b) {
  if (!b || b.phase !== "内容预检通过" || !batchTasks(db, b).length)
    throw Error("批次未就绪，请先配置资源和内容");
  if (b.stoppedAt || ["已取消", "执行已结束"].includes(b.control))
    throw Error("批次已终结，不可恢复队列");
}
function preflightDispatch(db, b) {
  ready(db, b);
  for (const t of batchTasks(db, b).filter((t) => t.status === "待执行")) {
    const errors = validate(t, db, t.id);
    if (errors.length) throw Error(errors.join("；"));
  }
}
function lock(p, owner) {
  p.lockedAt ||= new Date().toISOString();
  p.owner = owner;
  p.log ||= [];
  if (!p.log.some((l) => l.type === "首次锁定"))
    p.log.push({ type: "首次锁定", time: p.lockedAt });
}
export function takeBatchControl(db, b, confirmed = false) {
  const p = associatedPlan(db, { kind: "batch", id: b.id });
  if (!p) return;
  if (!confirmed) throw Error("请确认人工接管及首次启动锁定");
  if (["已取消", "执行已结束"].includes(p.control))
    throw Error("计划已终结，不能接管整批");
  if (p.items[p.position]?.id !== b.id)
    throw Error("尚未轮到此批，禁止绕过前序批次");
  preflightDispatch(db, b);
  lock(p, "manual");
  p.control = "已暂停";
  p.log.push({
    type: "人工接管",
    batchId: b.id,
    time: new Date().toISOString(),
  });
}
export function dispatchBatch(
  db,
  b,
  { fromPlan = false, takeover = false } = {},
) {
  preflightDispatch(db, b);
  const p = associatedPlan(db, { kind: "batch", id: b.id });
  if (p) {
    if (fromPlan) {
      if (
        p.control !== "执行中" ||
        p.owner !== "plan" ||
        p.items[p.position]?.id !== b.id
      )
        throw Error("无有效计划调度权");
    } else takeBatchControl(db, b, takeover);
  }
  const pending = batchTasks(db, b).filter((t) => t.status === "待执行");
  for (const t of pending) {
    const errors = validate(t, db, t.id);
    if (errors.length) throw Error(errors.join("；"));
  }
  pending.forEach((t) => queueTask(db, t));
  b.control = "执行中";
}
export function dispatchPlan(db, p) {
  if (["已取消", "执行已结束"].includes(p.control))
    throw Error("计划已终结，补量请另行规划");
  if (!p.confirmed || !p.items.length)
    throw Error("请先确认计划并生成批次骨架");
  let position = p.position;
  if (!planHasStarted(db, p)) {
    while (position < p.items.length) {
      const cancelled = itemRecord(db, p.items[position]);
      if (cancelled?.control !== "已取消" || !batchTasks(db, cancelled).every(isDisposed)) break;
      position++;
    }
  }
  const b = itemRecord(db, p.items[position]);
  if (
    p.lockedAt &&
    b &&
    batchTasks(db, b).length &&
    !batchTasks(db, b).some(unresolved)
  ) {
    lock(p, "plan");
    p.control = "执行中";
    syncPlan(db, p);
    return;
  }
  preflightDispatch(db, b);
  p.position = position;
  lock(p, "plan");
  p.control = "执行中";
  dispatchBatch(db, b, { fromPlan: true });
  syncPlan(db, p);
}
export function decision(db, p) {
  const b = itemRecord(db, p.items[p.position]);
  return nextBatch(batchTasks(db, b));
}
export function advance(db, p, confirmed = false) {
  if (
    !["执行中", "等待继续确认", "等待跳过确认", "等待配置"].includes(
      p.control,
    ) ||
    p.owner !== "plan"
  )
    throw Error("请先启动或恢复计划");
  const d = decision(db, p);
  if (d === "等待核实") throw Error("当前批次仍有未决任务，不能衔接");
  if (d !== "自动继续" && !confirmed) throw Error("需要人工确认后继续");
  if (confirmed) p.boundaryApproved = p.position;
  const next = itemRecord(db, p.items[p.position + 1]);
  if (next && next.control !== "已取消") {
    try {
      preflightDispatch(db, next);
      if (batchTasks(db, next).some((t) => t.attempts > 0))
        throw Error("下一批已有执行记录，请核查调度权，禁止重复派发");
    } catch (e) {
      p.control = "等待配置";
      throw e;
    }
  }
  const current = itemRecord(db, p.items[p.position]);
  if (d === "等待跳过确认") p.skipped.push({ ...p.items[p.position] });
  if (current.control !== "已取消") current.control = "执行已结束";
  p.log.push({
    type: "批次已处理",
    batchId: current.id,
    time: new Date().toISOString(),
  });
  p.position++;
  delete p.boundaryApproved;
  if (p.position >= p.items.length) {
    p.control = "执行已结束";
    return;
  }
  p.control = "执行中";
  if (next.control !== "已取消") dispatchBatch(db, next, { fromPlan: true });
  syncPlan(db, p);
}
export function syncPlan(db, p) {
  if (!["执行中", "等待配置"].includes(p.control) || p.owner !== "plan") return;
  const d = decision(db, p);
  if (d === "自动继续" || p.boundaryApproved === p.position) {
    advance(db, p, p.boundaryApproved === p.position);
  }
  else if (d !== "等待核实") p.control = d;
}
export function cancellationImpact(db, b) {
  const rows = batchTasks(db, b);
  return {
    pending: rows.filter(
      (t) => ["待执行", "排队中"].includes(t.status) && !t.claimed,
    ).length,
    inFlight: rows.filter(
      (t) =>
        (t.claimed && unresolved(t)) ||
        ["执行中", "等待人工"].includes(t.status),
    ).length,
    unknown: rows.filter((t) => t.status === "结果待确认").length,
  };
}
export function cancelBatchSafely(db, b) {
  if (b.firstStartedAt) throw Error("首次执行后不可取消批次，请使用停止发布");
  b.control = "已取消";
  for (const t of batchTasks(db, b))
    if (["待执行", "排队中"].includes(t.status) && !t.claimed)
      setResult(t, "已取消");
  return cancellationImpact(db, b);
}
export function pausePlan(p) {
  if (!["执行中", "等待配置", "等待继续确认", "等待跳过确认"].includes(p.control)) return;
  p.control = "已暂停";
  p.log.push({ type: "暂停后续调度", time: new Date().toISOString() });
}
export function pauseBatch(b) {
  if (b.control !== "执行中") return;
  b.control = "已暂停";
  b.log ||= [];
  b.log.push({ type: "暂停批次派发", time: new Date().toISOString() });
}
export function cancelPlan(p, db) {
  if (p.control === "已取消") return;
  if (p.endedAt) throw Error("计划已结束");
  if (!db) throw Error("终止计划需要完整批次记录");
  const time = new Date().toISOString();
  for (const item of planItems(p)) {
    const b = itemRecord(db, item);
    if (!b) continue;
    const tasks = batchTasks(db, b);
    if (b.control === "已取消" || (tasks.length && tasks.every(isDisposed))) continue;
    b.stoppedAt ||= time;
    if (!b.firstStartedAt) b.control = "已取消";
    for (const t of tasks) {
      t.batchStopped = true;
      if (["待执行", "排队中"].includes(t.status) && !t.claimed)
        setResult(t, "已取消", { evidence: "终止所属计划：未开始任务已取消" });
    }
  }
  p.control = "已取消";
  p.owner = null;
  p.log.push({ type: "终止计划及所属批次", time });
}
export function claimTask(db, t, now = Date.now()) {
  if (db.platformBlocks?.[t.platform]) return false;
  const b = db.batches.find((b) => b.id === t.batchId);
  if (!batchTaskCanStart(db, b, t, now)) return false;
  if (
    b?.stoppedAt ||
    t.status !== "排队中" ||
    (b && b.control !== "执行中" && !t.manualDispatch) ||
    (t.schedule && Date.parse(t.schedule) > now)
  )
    return false;
  if (
    db.tasks.some(
      (x) =>
        x.id !== t.id &&
        x.accountId === t.accountId &&
        ["执行中", "等待人工", "结果待确认"].includes(x.status),
    )
  )
    return false;
  const account = db.accounts.find((a) => a.id === t.accountId);
  if (!account?.enabled || account.loginValid === false) {
    setResult(t, "等待人工", { evidence: "演示账号登录失效；原会话待处理" });
    return false;
  }
  if (db.tasks.filter((x) => x.status === "执行中").length >= 3) return false;
  t.claimed = true;
  setResult(t, "执行中");
  return true;
}
export function setResult(t, status, { evidence = "", publicAt = null } = {}) {
  if (status === "执行中" && t.status !== "执行中") delete t.executionEndedAt;
  t.status = status;
  t.result =
    status === "已发布"
      ? "已公开"
      : status === "平台审核中"
        ? "平台已接收"
        : status === "结果待确认"
          ? "未知"
          : status === "执行失败"
            ? "确认未发布"
            : "未提交";
  if (["平台审核中", "已发布"].includes(status))
    t.submittedAt ||= new Date().toISOString();
  if (status === "已发布") t.publicAt = publicAt || new Date().toISOString();
  if (evidence) t.evidence = evidence;
  if (!unresolved(t)) {
    t.claimed = false;
    t.executionEndedAt ||= new Date().toISOString();
  }
  const a = t.attemptHistory?.at(-1);
  if (a) {
    a.status = status;
    a.steps ||= [];
    a.steps.push({ time: new Date().toISOString(), status, evidence });
    if (evidence) a.receipt = evidence;
    if (status === "执行失败") a.errorCode = "DEMO_CONFIRMED_REJECTED";
    if (!unresolved(t)) a.ended = new Date().toISOString();
  }
  t.log ||= [];
  t.log.push({ time: new Date().toISOString(), text: "演示结果：" + status });
}
// Migration preserves task identity/results. Mixed old groups split only by observed platform,
// never by invented historical creation boundaries. Plan refs expand in the same position.
function migrateLegacy(db) {
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

export function migrateRelationships(db) {
  migrateLegacy(db);
  if (db.workflowVersion === 3) return db;
  for (const t of db.tasks) {
    if (t.planId) {
      t.legacyPlanId = t.planId;
      delete t.planId;
    }
    t.version ||= 1;
    t.snapshot ||= clone(t);
  }
  for (const p of db.plans) {
    p.items = planItems(p);
    p.confirmed = true;
    p.platformOrder = [
      ...new Set(
        p.items.map((i) => itemRecord(db, i)?.platform).filter(Boolean),
      ),
    ];
    p.log ||= [];
    p.items.sort(
      (a, b) =>
        p.platformOrder.indexOf(itemRecord(db, a)?.platform) -
        p.platformOrder.indexOf(itemRecord(db, b)?.platform),
    );
    if (p.control !== "规划中") {
      p.lockedAt ||= new Date().toISOString();
      p.owner = "plan";
    }
  }
  for (const b of db.batches) {
    const rows = batchTasks(db, b);
    b.targets ||= [...new Set(rows.map((t) => t.category))].map((category) => ({
      category,
      count: rows.filter((t) => t.category === category).length,
    }));
    b.categories = b.targets.map((t) => t.category);
    b.planId = associatedPlan(db, { kind: "batch", id: b.id })?.id || null;
    b.phase = rows.length ? "内容预检通过" : "待资源配置";
    b.candidates ||= [];
  }
  for (const p of db.plans) {
    p.targets = p.items
      .flatMap((i) => {
        const b = itemRecord(db, i);
        return (b?.targets || []).map((t) => ({ ...t, platform: b.platform }));
      })
      .reduce((out, t) => {
        const prev = out.find((x) => key(x) === key(t));
        if (prev) prev.count += t.count;
        else out.push(t);
        return out;
      }, []);
  }
  db.workflowVersion = 3;
  return db;
}
