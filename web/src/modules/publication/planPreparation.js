export const clone = (v) => JSON.parse(JSON.stringify(v));
export const planStates = {
  not_started: "待执行",
  running: "进行中",
  paused: "进行中",
  stopping: "停止中",
  ended: "已结束",
};
export const planProgressLabels = {
  preparing: "待完成批次配置",
  running: "按批次顺序自动发布",
  manual_paused: "计划已暂停",
  waiting_tasks: "当前有待处理任务",
  waiting_verification: "待人工核实",
  waiting_continue: "失败率超过10%，等待确认",
  waiting_skip: "无明确结果，等待确认跳过",
  waiting_resources: "等待下一批原资源恢复",
  awaiting_finish: "等待平台公开结果",
  batch_paused: "当前批次已暂停",
  ready_to_finish: "等待结束计划",
  stopping: "正在安全收尾",
  ended: "计划已结束",
};
export const planLabels = {
  start: "执行",
  pause: "暂停计划",
  resume: "恢复计划",
  "confirm-next": "确认进入下一批",
  finish: "结束计划",
  cancel: "终止计划",
  takeover: "接管当前批次",
};
export function emptyPlan(v) {
  return v
    ? clone({
        name: v.name,
        note: v.note || "",
        date_start: v.date_start || "",
        date_end: v.date_end || "",
        goals: v.goals || [],
        batches: v.batches || [],
      })
    : {
        name: "",
        note: "",
        date_start: "",
        date_end: "",
        goals: [],
        batches: [],
      };
}
export function reorder(p) {
  let group = 0,
    last;
  const counters = {};
  for (const b of p.batches) {
    if (b.platform !== last) {
      group++;
      last = b.platform;
    }
    b.group_order = group;
    b.batch_order = (counters[group] || 0) + 1;
    counters[group] = b.batch_order;
  }
}
export function planProblem(p) {
  if (!p.name.trim() || p.name.length > 100)
    return "请填写计划名称（最多100字）";
  if (!p.date_start || !p.date_end || p.date_end < p.date_start)
    return "请填写有效执行周期";
  if (!p.goals.length || !p.batches.length) return "请填写目标并完成拆批";
  const totals = {};
  for (const g of p.goals) {
    const k = g.platform + ":" + g.category_id;
    if (
      !Number.isSafeInteger(g.target_count) ||
      g.target_count < 1 ||
      totals[k] !== undefined
    )
      return "目标数量或目标范围无效";
    totals[k] = g.target_count;
  }
  const rowKeys = new Set();
  for (const b of p.batches) {
    if (!b.row_key || rowKeys.has(b.row_key)) return "批次标识重复或缺失";
    rowKeys.add(b.row_key);
    if (!b.targets.length) return "每个批次至少分配一项目标";
    const categoryKeys = new Set();
    if (!b.name.trim() || [...b.name].length > 100)
      return "请填写每批名称（最多100字）";
    for (const t of b.targets) {
      const k = b.platform + ":" + t.category_id;
      if (categoryKeys.has(k)) return "批次内类目不能重复";
      categoryKeys.add(k);
      if (
        !Number.isSafeInteger(t.target_count) ||
        t.target_count < 1 ||
        totals[k] === undefined
      )
        return "批次目标无效";
      totals[k] -= t.target_count;
    }
  }
  return Object.values(totals).some((v) => v !== 0)
    ? "批次分目标须与每格计划目标一致"
    : "";
}
export function splitBatch(p, i) {
  const b = p.batches[i];
  if (!b || b.targets.reduce((n, t) => n + t.target_count, 0) < 2)
    throw Error("本批数量不足，无法再拆分");
  const n = {
    ...clone(b),
    row_key: crypto.randomUUID(),
    name: b.name + " · 拆分",
    targets: [],
  };
  delete n.batch_id;
  if (b.targets.every((t) => t.target_count === 1)) {
    n.targets = b.targets.splice(Math.ceil(b.targets.length / 2));
  }
  for (const t of b.targets) {
    const count = Math.floor(t.target_count / 2);
    if (count) {
      n.targets.push({ ...t, target_count: count });
      t.target_count -= count;
    }
  }
  p.batches.splice(i + 1, 0, n);
  reorder(p);
}
export function mergeBatch(p, i) {
  const b = p.batches[i],
    n = p.batches[i + 1];
  if (!n || b.platform !== n.platform) throw Error("只能合并相邻同平台批次");
  for (const t of n.targets) {
    const existing = b.targets.find((v) => v.category_id === t.category_id);
    if (existing) existing.target_count += t.target_count;
    else b.targets.push(clone(t));
  }
  p.batches.splice(i + 1, 1);
  reorder(p);
}
export function planReturn(q) {
  if (!/^[1-9][0-9]*$/.test(q.from_plan || "")) return null;
  const query = { view: "detail", id: q.from_plan };
  for (const k of [
    "search",
    "status",
    "platform",
    "category_id",
    "page",
    "page_size",
    "scroll",
  ])
    if (typeof q["plan_" + k] === "string") query[k] = q["plan_" + k];
  return { name: "PublishPlans", query };
}
export function planJournal(
  origin,
  runtime,
  user,
  id,
  storage = globalThis.localStorage,
  locks = globalThis.navigator?.locks,
) {
  const key = ["wt-media:plan-command:v1", origin, runtime, user, id].join("|");
  const read = () => {
    const raw = storage.getItem(key);
    if (!raw) return null;
    const v = JSON.parse(raw);
    if (!["start", "confirm-next"].includes(v.action) || !v.input?.request_key)
      throw Error("原计划请求无法读取，请保留现场");
    return v;
  };
  return {
    read,
    save: (op) => {
      if (!locks) throw Error("当前客户端无法安全保存原请求");
      return locks.request(key, () => {
        const old = read();
        if (old && JSON.stringify(old) !== JSON.stringify(op))
          throw Error("已有待核对的原计划请求");
        storage.setItem(key, JSON.stringify(op));
      });
    },
    clear: (op) =>
      locks.request(key, () => {
        if (JSON.stringify(read()) === JSON.stringify(op))
          storage.removeItem(key);
      }),
  };
}
const canonical = (v) =>
  JSON.stringify(v, (_, value) =>
    value && typeof value === "object" && !Array.isArray(value)
      ? Object.fromEntries(
          Object.entries(value).sort(([a], [b]) => a.localeCompare(b)),
        )
      : value,
  );
const validID = (v) => typeof v === "string" && /^[1-9][0-9]*$/.test(v);
function receipt(r, id, started, c) {
  if (
    !r ||
    r.batch_id !== id ||
    r.execution_status !== "running" ||
    r.first_started_at !== started ||
    !validID(r.revision) ||
    !Array.isArray(r.task_ids) ||
    !r.task_ids.length ||
    new Set(r.task_ids).size !== r.task_ids.length ||
    !r.task_ids.every(validID) ||
    r.created_count !== r.task_ids.length ||
    !Number.isSafeInteger(r.target_count) ||
    !Number.isSafeInteger(r.creation_shortfall) ||
    r.creation_shortfall < 0 ||
    r.target_count !== r.created_count + r.creation_shortfall
  )
    return false;
  if (
    c &&
    (r.created_count !== c.included_row_keys.length ||
      r.creation_shortfall !==
        c.expected_shortfalls.reduce((n, v) => n + v.shortfall, 0) ||
      r.revision !== String(BigInt(c.expected_revision) + 1n))
  )
    return false;
  return true;
}
export function planCommandNotApplied(error, operation) {
  const d = error?.details,
    input = operation?.input;
  return (
    error?.status === 409 &&
    error.code === "publication_conflict" &&
    ["start", "confirm-next"].includes(operation.action) &&
    d?.command_applied === false &&
    d.command === operation.action &&
    typeof d.request_key === "string" &&
    d.request_key.length > 0 &&
    d.request_key === input.request_key &&
    d.expected_revision === input.expected_revision &&
    validID(d.current_revision) &&
    validID(input.expected_revision) &&
    BigInt(d.current_revision) > BigInt(input.expected_revision)
  );
}
export async function sendPlanCommand(api, id, op, { recover = false } = {}) {
  try {
    if (recover) {
      const current = await api.get(id);
      if (current?.plan?.id !== id) throw Error("原计划读取失败");
    }
    const r = await api[op.action](id, op.input);
    if (r?.plan_id !== id || !validID(r.revision))
      throw Error("计划响应不完整");
    if (op.action === "start") {
      const first = op.input.batch_confirmations.find(
        (c) => c.batch_id === op.input.first_batch_id,
      );
      if (
        !first ||
        !r.first_started_at ||
        !Number.isFinite(Date.parse(r.first_started_at)) ||
        r.current_batch_id !== op.input.first_batch_id ||
        r.revision !== String(BigInt(op.input.expected_revision) + 1n) ||
        canonical(r.batch_confirmations) !==
          canonical(op.input.batch_confirmations) ||
        !receipt(
          r.first_batch_receipt,
          op.input.first_batch_id,
          r.first_started_at,
          first,
        )
      )
        throw Error("原启动回执无法核对");
    }
    if (op.action === "confirm-next") {
      if (
        r.previous_batch_id !== op.input.current_batch_id ||
        r.current_batch_id !== op.input.next_batch_id ||
        r.decision !== op.input.decision ||
        !r.advanced_at ||
        !Number.isFinite(Date.parse(r.advanced_at)) ||
        r.revision !== String(BigInt(op.input.expected_revision) + 1n)
      )
        throw Error("原推进回执无法核对");
      if (
        r.batch_receipt === null
          ? r.current_batch_status !== "cancelled"
          : !receipt(r.batch_receipt, r.current_batch_id, r.advanced_at)
      )
        throw Error("原推进建单回执无法核对");
    }
    return r;
  } catch (e) {
    if (planCommandNotApplied(e, op)) {
      e.planRejection = true;
      e.definitiveNotApplied = true;
      e.message = "原计划请求未生效，请按最新配置重新检查";
      throw e;
    }
    if (
      e.status === 409 &&
      e.details?.plan_id === id &&
      Array.isArray(e.details.issues) &&
      e.details.issues.some(
        (i) =>
          i.code === "duplicate_plan_output" &&
          i.batch_id &&
          i.row_key &&
          i.related_batch_id &&
          i.related_row_key,
      )
    ) {
      e.planRejection = true;
      throw e;
    }
    if (!recover && [400, 401, 403, 404, 422].includes(e.status)) throw e;
    throw Object.assign(
      new Error("操作结果待确认，请核对原请求；不要更换请求键", { cause: e }),
      { outcomeUnknown: true },
    );
  }
}

export const planRecordLabels = {
  "publication_plan.create": "创建计划",
  "publication_plan.structure": "修改计划安排",
  "publication_plan.start": "首次启动计划",
  "publication_plan.advance": "推进下一批",
  "publication_plan.handoff": "推进下一批",
  "publication_plan.cancel_settled": "终止后收尾完成",
  "publication_plan.pause": "暂停计划",
  "publication_plan.resume": "恢复计划",
  "publication_plan.takeover": "接管当前批次",
  "publication_plan.cancel": "终止计划",
  "publication_plan.finish": "结束计划",
};

export async function readPlanDraftChoice(client, choose) {
  const local = await client.get({ reload: true });
  if (!local.payload) return local;
  const choice = await choose(local.saved_at);
  if (choice === "continue") return local;
  if (choice === "restart") {
    await client.clear();
    return client.get();
  }
  return null;
}

export function planRecordDescription(record) {
  const details = record?.details;
  if (!details || typeof details !== 'object') return '';
  const result = details.result;
  if (record.event === 'publication_plan.handoff' && result) {
    const from = result.previous_batch_id, to = result.current_batch_id;
    if (!from || !to) return '';
    const decision = {automatic:'自动推进',continue:'人工确认继续',skip:'人工确认跳过'}[result.decision] || '按条件推进';
    return `批次 #${from} → 批次 #${to} · ${decision}${details.reason ? ' · '+details.reason : ''}`;
  }
  return typeof details.reason === 'string' ? details.reason : '';
}
