// Shared policy for the existing example and its loopback preview service.
import { batchCreationDraft, migrateBatchCreationDrafts } from "./creationDrafts.js";
import { validate, canRetry } from "./model.js";
import { createPublication, takeBatchControl, awaitingPlanCheck } from "./relationships.js";
import { executionExceptions, executionPause, profileOf, reconcileExecutionGroup } from "./batchExecutionSettings.js";
const copy = (x) => JSON.parse(JSON.stringify(x));
export const rowsFor = (db, b) => db.tasks.filter((t) => t.batchId === b.id);
export const formalBatch = (db, b) => !!b.preparationOnly || rowsFor(db, b).length > 0;
export const isDisposed = (t) =>
  ["已发布", "已取消", "已放弃"].includes(t.status);
export const started = (b) => Boolean(b?.firstStartedAt);
export const pendingContentEditable = (t, b) => !b?.planFrozenAt && t?.status === "待执行" && !t.claimed && !t.batchStopped && !b?.stoppedAt && b?.control !== "已取消";
export const contentEditable = (db, b) =>
  !!b &&
  !started(b) &&
  !b.planFrozenAt &&
  b.control !== "已取消" &&
  rowsFor(db, b).some((t) => !isDisposed(t)) &&
  b.phase === "内容预检通过";
export const isPendingTask = (t, b) =>
  b.executionReview ? (b.planReview && awaitingPlanCheck(t)) || ["结果待确认", "等待人工", "待补链接"].includes(t.status) || (t.status === "执行失败" && !b.endedAt) :
  ["执行失败", "结果待确认", "等待人工", "平台审核中", "待补链接"].includes(t.status) ||
  (started(b) && t.status === "待执行");
export const trustedPublicTask = (db, t) => t.status === "已发布" && !!(t.publicAt && t.workId &&
  db.works?.some(work => work.id === t.workId && work.platform === t.platform && work.sourceTaskIds?.includes(t.id)));
export function batchMetrics(db, b) {
  const rows = rowsFor(db, b),
    total = rows.length,
    target = (b.targets || []).reduce((n, t) => n + Number(t.count), 0),
    published = rows.filter(t => b.executionReview ? trustedPublicTask(db, t) : t.status === "已发布").length,
    disposed = rows.filter(isDisposed).length;
  const pending = rows.filter((t) => isPendingTask(t, b)).length;
  const failed = rows.filter((t) => t.status === "执行失败").length;
  return {
    total,
    target,
    published,
    failed,
    ...(b.executionReview && {
      exceptions: executionExceptions(db, b).length,
      reviewing: rows.filter(t => t.status === "平台审核中").length,
      shortfall: Math.max(0, target - total),
    }),
    disposed,
    pending,
    progress: target ? Math.round((published / target) * 100) : 0,
    achievement: target ? Math.round((published / target) * 100) : 0,
    result:
      published === total && total
        ? "全部已发布"
        : published
          ? "部分已发布"
          : "未发布",
  };
}
export function batchState(db, b) {
  if (b.executionReview) {
    if (b.control === "已取消") return "已取消";
    if (!started(b)) return "未开始";
    if (b.endedAt) return "已结束";
    if (b.control === "已暂停") return "已暂停";
    const tasks = rowsFor(db, b);
    const running = tasks.some(task => ["排队中", "执行中"].includes(task.status));
    if (running) return "执行中";
    if (executionExceptions(db, b).length || (b.planReview && tasks.some(awaitingPlanCheck))) return "待处理";
    return "已结束";
  }
  if (b.preparationOnly) return "未开始";
  const rows = rowsFor(db, b);
  if (b.control === "已取消") return "已取消";
  if (rows.length && rows.every(isDisposed)) return "已结束";
  if (!started(b))
    return rows.length && b.phase === "内容预检通过" ? "待执行" : "草稿";
  if (
    rows.some(
      (t) =>
        ["排队中", "执行中"].includes(t.status) ||
        (!b.stoppedAt &&
          t.status === "待执行" &&
          executionChecks(t, db).length === 0),
    )
  )
    return "执行中";
  return "待处理";
}
export const stoppable = (db, b) =>
  started(b) &&
  !b.stoppedAt &&
  rowsFor(db, b).some(
    (t) => ["待执行", "排队中"].includes(t.status) && !t.claimed,
  );
const finishable = (db, batch) => started(batch) && !batch.endedAt &&
  rowsFor(db, batch).some(task => task.status === "执行失败") &&
  !rowsFor(db, batch).some(task => ["排队中", "执行中", "等待人工", "结果待确认", "待执行"].includes(task.status));

export function batchActions(db, b) {
  if (b.executionReview) {
    const state = batchState(db, b);
    if (state === "未开始") {
      if (b.planReview) {
        const plan = db.plans.find(item => item.id === b.planId);
        if (!plan?.lockedAt) return ["configure", "detail", "cancel"];
        return plan.items[plan.position]?.id === b.id && !["已取消", "执行已结束"].includes(plan.control)
          ? ["execute", "detail"] : ["detail"];
      }
      return ["execute", "configure", "detail", "cancel"];
    }
    if (["已结束", "已取消"].includes(state)) return ["detail", "copy"];
    return [
      batchMetrics(db, b).pending ? "process" : "detail",
      ...(state === "已暂停" && !b.stoppedAt ? ["resume"] : []),
      ...(state === "执行中" && !b.stoppedAt ? ["pause"] : []),
      ...(finishable(db, b) ? ["finish"] : []),
      ...(!b.stoppedAt && rowsFor(db, b).some(task => ["排队中", "执行中"].includes(task.status)) ? ["stop"] : []),
    ];
  }
  if (b.preparationOnly) return ["configure", "detail"];
  const s = batchState(db, b);
  if (s === "草稿") return ["configure", "detail", "cancel"];
  if (s === "待执行") return ["execute", "edit", "detail", "cancel"];
  if (["已结束", "已取消"].includes(s)) return ["detail", "copy"];
  return [
    batchMetrics(db, b).pending ? "process" : "detail",
    ...(stoppable(db, b) ? ["stop"] : []),
  ];
}
const actionLabels = {
  configure: "继续配置", execute: "执行", pause: "暂停发布", resume: "继续发布",
  finish: "结束批次", edit: "编辑", detail: "详情", process: "去处理", stop: "停止发布",
  cancel: "终止", copy: "复制",
};
export function batchActionLabel(action, batch) {
  if (batch?.executionReview && action === "execute") return "开始发布";
  if (batch?.executionReview && action === "stop") return "终止发布";
  if (batch?.planReview && action === "configure" && batch.phase !== "内容预检通过") return "去配置";
  return batch?.preparationOnly && action === "configure" ? "编辑配置" : actionLabels[action];
}
export const contentKeys = [
  "title",
  "shortTitle",
  "subTitle",
  "description",
  "topics",
  "tags",
  "partition",
  "declaration",
  "visibility",
  "schedule",
  "watermark",
  "mount",
  "cover",
  "coverMode",
  "coverTreatment",
  "templateId",
  "templateDocument",
  "coverResults",
  "coverSource",
  "coverVariants",
  "coverApproved",
  "contentApproved",
  "coverSignature",
  "coverRender",
  "coverError",
  "templateVersion",
  "coverConfirmed",
  "coverVersion",
  "contentVersion",
  "adoptedVersion",
  "overrides",
  "ai",
  "approved",
  "approvedSignature",
  "version",
];
const identityKeys = [
  "accountId",
  "assetId",
  "platform",
  "category",
  "batchId",
];
export function executionChecks(t, db) {
  const e = validate(t, db, t.id);
  if (db.accounts.find((a) => a.id === t.accountId)?.loginValid === false)
    e.push("原账号登录失效，请恢复原账号");
  if (db.platformBlocks?.[t.platform]) e.push("平台限制尚未解除");
  return [...new Set(e)];
}
export function batchTaskActions(db, b, t) {
  if (t.status === "结果待确认") return ["verify"];
  if (t.status === "等待人工") return ["human"];
  if (["平台审核中", "待补链接"].includes(t.status)) return [b.planReview && t.status === "平台审核中" ? "verify" : "supplement"];
  if (t.status === "执行失败") {
    if (!canRetry({ ...t, batchStopped: false })) return ["verify"];
    if (b.executionReview && b.endedAt) return [];
    return [
      ...(!b.stoppedAt && !b.endedAt && (!b.executionReview || b.control !== "已暂停") ? ["retry"] : []),
      ...(executionChecks(t, db).length ? ["repair"] : []),
      "abandon",
    ];
  }
  if (t.status === "待执行") {
    const invalid = executionChecks(t, db).length > 0;
    return [
      ...(b.firstStartedAt && !b.stoppedAt && !invalid ? ["start"] : []),
      ...(invalid ? ["repair", "abandon"] : []),
      "cancel",
    ];
  }
  return [];
}
export function executionPreview(db, b, check = executionChecks) {
  if (b.executionReview && !started(b)) return b.candidates.map(task => {
    const errors = [...check(task, db)];
    const account = db.accounts.find(item => item.id === task.accountId);
    if (account?.ownerId !== b.ownerId) errors.push("只能使用批次所属运营本人的发布账号");
    if (!profileOf(account)) errors.push("账号没有可用的浏览器环境");
    return { task, errors };
  });
  return rowsFor(db, b)
    .filter((t) => t.status === "待执行" && !t.claimed)
    .map((t) => ({ task: t, errors: check(t, db) }));
}
export function startReviewScope(db, batch, included, check = executionChecks) {
  const preview = executionPreview(db, batch, check);
  const ready = preview.filter(entry => !entry.errors.length && (!included || included.includes(entry.task.key)));
  const shortfalls = batch.targets.map(target => ({
    category: target.category, target: target.count,
    shortfall: Math.max(0, target.count - ready.filter(entry => entry.task.category === target.category).length),
  }));
  return { preview, ready, shortfalls, signature: JSON.stringify({
    rows: preview.map(entry => [entry.task.key, entry.errors]),
    included: ready.map(entry => entry.task.key), shortfalls,
  }) };
}
export function freezeBatch(db, b) {
  if (started(b)) return;
  b.firstStartedAt = new Date().toISOString();
  b.frozenSnapshot = copy({
    name: b.name,
    platform: b.platform,
    targets: b.targets,
    categories: b.categories,
    rules: {
      ...(b.rules || {}),
      ...(b.draft
        ? {
            accountOrder: b.draft.accountOrder || [],
            accountLimits: b.draft.accountLimits || {},
          }
        : {}),
    },
    ...(b.executionReview && { executionSettings: b.executionSettings }),
    tasks: rowsFor(db, b).map((t) =>
      Object.fromEntries(
        ["id", ...identityKeys, ...contentKeys]
          .filter((k) => Object.hasOwn(t, k))
          .map((k) => [k, t[k]]),
      ),
    ),
    at: b.firstStartedAt,
  });
  rowsFor(db, b).forEach((t) => {
    t.batchConfigLocked = true;
  });
}
function record(b, type, text) {
  b.updatedAt = new Date().toISOString();
  b.version = (b.version || 0) + 1;
  b.log ||= [];
  b.log.push({
    time: b.updatedAt,
    actor: "本地演示用户",
    type,
    text,
    result: "成功",
  });
}
export function runBatchCommand(
  db,
  b,
  action,
  input = {},
  check = executionChecks,
) {
  if (!b) throw Error("批次不存在");
  if (
    input.expectedVersion !== undefined &&
    input.expectedVersion !== (b.version || 0)
  )
    throw Error("批次已变化，请刷新后重试");
  const state = batchState(db, b);
  if (b.executionReview && action === "execute") {
    if (state !== "未开始") throw Error("当前批次已启动或终止");
    const scope = startReviewScope(db, b, input.included, check);
    if (scope.signature !== input.scope) throw Error("候选或缺口已变化，批次仍保留为已保存未开始，请重新检查");
    if (!scope.ready.length) throw Error("没有可开始发布的候选，批次已保存未开始");
    if ((scope.shortfalls.some(item => item.shortfall) || scope.ready.length !== scope.preview.length) && !input.partial)
      throw Error("请确认本次发布范围与目标缺口");
    if (b.planReview && !input.planAuthorized) {
      if (scope.ready.length !== b.candidates.length) throw Error("须使用计划冻结的全部候选");
      takeBatchControl(db, b, input.takeover);
    }
    const candidates = b.candidates;
    const { tasks } = createPublication(db, {
      platform: b.platform, rows: scope.ready.map(entry => entry.task), name: b.name, batchId: b.id,
      partial: input.partial,
    });
    b.candidates = candidates;
    b.log.pop();
    freezeBatch(db, b);
    tasks.forEach(task => { task.status = "排队中"; });
    b.control = "执行中";
    reconcileExecutionGroup(db, b);
    record(b, "开始发布", `已建立 ${tasks.length} 条正式任务并授权，原目标保留`);
  } else if (b.executionReview && action === "pause") {
    if (!batchActions(db, b).includes(action)) throw Error("当前不能暂停批次");
    executionPause(b, input.reason || "人工暂停后续发布");
    record(b, "暂停发布", b.pause.reason);
  } else if (b.executionReview && action === "resume") {
    const keys = executionExceptions(db, b).map(entry => entry.key);
    if (!batchActions(db, b).includes(action) || b.pause?.id !== input.pauseId ||
        JSON.stringify(keys) !== JSON.stringify(input.exceptionKeys))
      throw Error("暂停依据或异常范围已变化，请重新核对");
    b.acknowledgedExceptions = [...new Set([...(b.acknowledgedExceptions || []), ...keys])];
    b.control = "执行中";
    b.pause = null;
    reconcileExecutionGroup(db, b);
    record(b, "继续发布", "已确认当前异常范围，保留待办与历史");
  } else if (b.executionReview && action === "finish") {
    if (!finishable(db, b)) throw Error("仍有未决执行，当前不能结束此批");
    b.endedAt = new Date().toISOString();
    b.control = "已结束";
    record(b, "结束批次", "保留明确失败及成片占用，关闭本批重试");
  } else if (action === "execute") {
    if (state !== "待执行" || b.stoppedAt)
      throw Error("当前批次不可执行，已停止或终结的批次不能重启");
    const preview = executionPreview(db, b, check),
      ready = preview.filter((x) => !x.errors.length);
    if (!ready.length)
      throw Error("没有可执行任务，请恢复原资源或处置异常任务");
    if (ready.length !== preview.length && !input.partial)
      throw Error("部分任务异常，请明确确认仅执行可用任务");
    const p = db.plans?.find((p) =>
      p.items?.some((i) => i.kind === "batch" && i.id === b.id),
    );
    if (p) {
      if (
        ["已取消", "执行已结束"].includes(p.control) ||
        p.items[p.position]?.id !== b.id
      )
        throw Error("计划尚未轮到此批或已终结");
      if (!input.takeover) throw Error("请确认人工接管当前计划批次");
      p.lockedAt ||= new Date().toISOString();
      p.owner = "manual";
      p.control = "已暂停";
      p.log ||= [];
      p.log.push({ type: "人工接管", batchId: b.id, time: p.lockedAt });
    }
    freezeBatch(db, b);
    ready.forEach(({ task: t }) => {
      t.status = "排队中";
      t.result = "未提交";
      t.attempts = (t.attempts || 0) + 1;
      t.attemptHistory ||= [];
      t.attemptHistory.push({
        number: t.attempts,
        started: new Date().toISOString(),
        status: "排队中",
        snapshot: copy(t.snapshot || t),
        version: t.version || 1,
      });
    });
    b.control = "执行中";
    record(
      b,
      "执行批次",
      `已接受 ${ready.length} 条任务；异常 ${preview.length - ready.length} 条`,
    );
  } else if (action === "stop") {
    if (b.executionReview ? !batchActions(db, b).includes("stop") : !stoppable(db, b))
      throw Error("当前没有可停止的未启动任务");
    b.stoppedAt = new Date().toISOString();
    if (b.executionReview) {
      b.control = "已停止";
      b.pause = null;
    }
    rowsFor(db, b).forEach((t) => {
      t.batchStopped = true;
      if (["待执行", "排队中"].includes(t.status) && !t.claimed) {
        t.status = "已取消";
        t.result = "未提交";
        t.cancelReason = "停止发布：未启动任务已取消";
        const a = t.attemptHistory?.at(-1);
        if (a && !a.ended) a.ended = b.stoppedAt;
      }
    });
    record(
      b,
      "停止发布",
      b.executionReview && input.reason ? input.reason + "。已停止后续派发，在途等待原尝试结果。" :
        "只停止未启动任务，已发布与在途任务保持原状态；禁止重启及新重试",
    );
  } else if (action === "cancel") {
    if (b.planFrozenAt) throw Error("计划已冻结，不能取消尚未轮到的批次");
    if (started(b) || !["草稿", "待执行", "未开始"].includes(state))
      throw Error("首次执行后不能取消批次，请使用停止发布");
    rowsFor(db, b).forEach((t) => {
      if (t.status === "待执行") {
        t.status = "已取消";
        t.result = "未提交";
      }
    });
    b.control = "已取消";
    record(b, "取消批次", input.reason || "首次执行前取消");
  } else if (action === "copy") {
    if (!["已结束", "已取消"].includes(state))
      throw Error("只能复制已结束或已取消批次");
    if (!input.name?.trim()) throw Error("请输入新批次名称");
    if (db.publishDraft?.active) throw Error("存在未完成的批次配置，请先继续编辑或重新开始");
    const draft = batchCreationDraft(b);
    draft.name = input.name.trim();
    db.publishDraft = draft;
    return draft;
  } else if (action === "saveContent") {
    if (!contentEditable(db, b))
      throw Error("首次执行后发布配置永久锁定，当前不可编辑");
    const rows = rowsFor(db, b);
    if (input.rows?.length !== rows.length) throw Error("任务数量不可修改");
    const patches = rows.map((t) => {
      const row = input.rows.find((r) => r.id === t.id);
      if (!row || identityKeys.some((k) => row[k] !== t[k]))
        throw Error("资源已锁定，不能替换账号、成片或映射");
      return { t, row };
    });
    for (const { row } of patches) {
      const errors = check(row, db);
      if (errors.length) throw Error(`T-${row.id}：${errors.join("；")}`);
    }
    for (const { t, row } of patches) {
      for (const k of contentKeys)
        if (Object.hasOwn(row, k)) t[k] = copy(row[k]);
      t.version = (t.version || 0) + 1;
      t.snapshot = Object.fromEntries(
        contentKeys
          .filter((k) => Object.hasOwn(t, k))
          .map((k) => [k, copy(t[k])]),
      );
    }
    record(b, "编辑内容", "仅更新发布文案、封面和平台内容配置");
  } else if (action === "abandon") {
    const t = rowsFor(db, b).find((t) => t.id === input.taskId);
    if (
      !t ||
      !["待执行", "执行失败"].includes(t.status) ||
      t.claimed ||
      t.result === "未知" ||
      (t.status === "执行失败" && !canRetry({ ...t, batchStopped: false }))
    )
      throw Error("须确认原任务未发布且执行已结束；结果不明确请先核实");
    t.status = "已放弃";
    t.evidence = input.reason || "原资源永久不可用，放弃任务；需新批次补足目标";
    record(b, "放弃任务", `T-${t.id}：${t.evidence}`);
  } else if (action === "repair") {
    const t = rowsFor(db, b).find((t) => t.id === input.taskId);
    if (!t || isDisposed(t)) throw Error("任务已变化");
    const a = db.accounts.find((a) => a.id === t.accountId),
      v = db.assets.find((v) => v.id === t.assetId);
    if (!a || !v || a.permanentlyUnavailable || v.permanentlyUnavailable)
      throw Error("原资源永久不可用，请取消或放弃任务，由新批次补足目标");
    a.enabled = true;
    a.loginValid = true;
    v.ready = true;
    v.fileAvailable = true;
    record(
      b,
      "恢复原资源",
      `T-${t.id}：仅恢复原账号 ${a.id} 和原成片 ${v.id} 的演示可用性`,
    );
  } else throw Error("不支持的批次操作");
  return b;
}
export function migrateBatchLifecycle(db) {
  migrateBatchCreationDrafts(db);
  for (const b of db.batches) {
    const rows = rowsFor(db, b);
    b.version ||= 1;
    b.created ||= rows[0]?.created || new Date().toISOString();
    if (
      rows.some(
        (t) =>
          (t.attempts || 0) > 0 ||
          [
            "排队中",
            "执行中",
            "等待人工",
            "结果待确认",
            "平台审核中",
            "已发布",
            "执行失败",
          ].includes(t.status),
      )
    ) {
      if (!started(b)) {
        freezeBatch(db, b);
        b.legacySnapshot = true;
      }
    }
    if (started(b))
      rows.forEach((t) => {
        t.batchConfigLocked = true;
        t.batchStopped = !!b.stoppedAt;
      });
    b.status = batchState(db, b);
  }
}
export function protectBatchSnapshot(before, after) {
  for (const b of before.batches || []) {
    const n = after.batches?.find((x) => x.id === b.id);
    if (!n) {
      const p=before.plans?.find(p=>p.id===b.planId),next=after.plans?.find(x=>x.id===p?.id);
      const removable=p&&next&&!p.lockedAt&&!next.lockedAt&&!started(b)&&!rowsFor(before,b).length&&!rowsFor(after,b).length&&!b.draft?.resourcesLocked&&p.items?.some(i=>i.id===b.id)&&!next.items?.some(i=>i.id===b.id);
      if(removable)continue;
      throw Error("不能删除已有批次");
    }
    const oldRows = rowsFor(before, b),
      newRows = rowsFor(after, b);
    if (
      oldRows.length &&
      (oldRows.length !== newRows.length ||
        oldRows.some((t) => !newRows.some((n) => n.id === t.id)))
    )
      throw Error("已创建批次不能增删任务");
    if (started(b) && n.control === "已取消")
      throw Error("首次执行后不可取消批次");
    if (!oldRows.length && b.draft?.resourcesLocked) {
      if (
        ["platform", "targets", "categories", "planId"].some(
          (k) => JSON.stringify(b[k]) !== JSON.stringify(n[k]),
        ) ||
        ["accountLimits", "accountOrder"].some(
          (k) => JSON.stringify(b.draft[k]) !== JSON.stringify(n.draft?.[k]),
        )
      )
        throw Error("资源已锁定，不能修改平台、类目目标、账号顺序或配额");
      const signature = (rows) =>
        JSON.stringify(
          (rows || [])
            .map((t) => [t.accountId, t.assetId, t.platform, t.category])
            .sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b))),
        );
      const mapped = newRows.length ? newRows : n.candidates;
      if (signature(b.candidates) !== signature(mapped))
        throw Error("进入内容配置后原资源映射已锁定");
      if (!newRows.length && !n.draft?.resourcesLocked)
        throw Error("资源锁定不可撤销");
    }
    if (
      rowsFor(before, b).length &&
      [
        "name",
        "platform",
        "targets",
        "categories",
        "planId",
        "sourcePlanId",
        "sourceBatchId",
      ].some((k) => JSON.stringify(b[k]) !== JSON.stringify(n[k]))
    )
      throw Error("已创建批次只能编辑内容，目标与名称不可修改");
    if (!b.stoppedAt && n.stoppedAt)
      throw Error("停止发布必须通过批次操作接口");
    if (
      started(b) &&
      (!n.firstStartedAt ||
        n.firstStartedAt !== b.firstStartedAt ||
        JSON.stringify(n.frozenSnapshot) !== JSON.stringify(b.frozenSnapshot))
    )
      throw Error(
        `B-${b.id} 首次执行锁定不可撤销（${n.firstStartedAt !== b.firstStartedAt ? "执行时间" : "冻结快照"}已变化）`,
      );
    if (started(b) && (JSON.stringify(b.rules) !== JSON.stringify(n.rules) ||
        JSON.stringify(b.executionSettings) !== JSON.stringify(n.executionSettings)))
      throw Error("首次执行后发布设置不可修改");
    if (b.stoppedAt && n.stoppedAt !== b.stoppedAt)
      throw Error("停止发布不能恢复");
    for (const t of rowsFor(before, b)) {
      const nt = after.tasks?.find((x) => x.id === t.id);
      if (!nt || identityKeys.some((k) => nt[k] !== t[k]))
        throw Error("资源锁定，不能变更映射");
      if (isDisposed(t) && nt.status !== t.status)
        throw Error("已处置任务不能恢复执行");
      if (started(b) && !nt.batchConfigLocked)
        throw Error("发布配置锁定标记不可删除");
      if (t.status === "执行失败" && nt.status === "排队中" && !canRetry(t))
        throw Error("未发布证据不足，禁止重试");
      if (
        t.status === "结果待确认" &&
        ["待执行", "排队中", "已放弃"].includes(nt.status)
      )
        throw Error("结果不明确，必须先核实");
      if (b.stoppedAt && nt.attempts !== t.attempts)
        throw Error("停止发布后禁止新重试");
      if (
        (started(b) || n.firstStartedAt) &&
        !(pendingContentEditable(t, b) && pendingContentEditable(nt, n)) &&
        contentKeys.some((k) => JSON.stringify(t[k]) !== JSON.stringify(nt[k]))
      )
        throw Error("发布配置已永久锁定，请刷新页面");
      if (
        (b.stoppedAt || ["已结束", "已取消"].includes(batchState(before, b))) &&
        ["排队中", "待执行"].includes(nt.status) &&
        nt.status !== t.status
      )
        throw Error("终结批次不能重启");
    }
  }
  return after;
}
