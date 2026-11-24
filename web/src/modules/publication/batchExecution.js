export const batchPhases = {
  not_started: "未开始", running: "分组执行中", waiting_resources: "等待可用资源",
  waiting_interval: "组间等待", awaiting_processing: "待处理",
  paused: "已暂停", stopping: "终止收尾", ended: "已结束",
};

export const batchControlLabels = {
  start: "开始发布", pause: "暂停发布", resume: "继续发布", stop: "终止发布", finish: "结束批次",
};

const taskRef = ({ id, revision }) => ({ task_id: id, revision });
const pauseRef = ({ task_id, revision }) => ({ task_id, revision });
function conflictAfterReplay(error, id, operation, latest, readAfterFailure) {
  const { status, code, details } = error || {};
  if (status !== 409 || !details || (details.batch_id != null && details.batch_id !== id)) return false;
  const { action, input } = operation;
  if (action === "start") {
    // A prior read can predate another start. Only fresh unstarted evidence
    // supports clearing a generic replay conflict.
    const phase = details.execution_status;
    const unstartedConflict = details.batch_id === id && phase === "not_started";
    const unstartedRead = readAfterFailure && latest?.execution_status === "not_started";
    if (latest?.execution?.first_started_at || (phase !== undefined && phase !== "not_started") ||
        !(unstartedConflict || unstartedRead)) return false;
  }
  if (["publication_conflict", "publication_batch_parent_blocked", "publication_batch_shortfall_changed",
    "publication_batch_pause_changed", "publication_output_occupied"].includes(code) &&
    typeof details.current_revision === "string" && /^[1-9][0-9]*$/.test(details.current_revision)) return true;

  // A02 looks up the same-key receipt before reporting row or shortfall
  // conflicts. A replay can therefore prove rejection without a revision.
  if (action !== "start" || latest?.id !== id || latest.execution_status !== "not_started") return false;
  if (["publication_conflict", "publication_output_occupied"].includes(code) && Array.isArray(details.issues))
    return details.issues.some(issue => input.included_row_keys.includes(issue?.row_key) &&
      typeof issue.message === "string" && issue.message.trim());
  return code === "publication_batch_shortfall_changed" && Array.isArray(details.shortfalls) &&
    details.shortfalls.some(gap => input.expected_shortfalls.some(expected => expected.category_id === gap?.category_id) &&
      [gap.target_count, gap.created_count, gap.shortfall].every(value => Number.isSafeInteger(value) && value >= 0));
}

const knownRejection = error => [400, 403, 404, 409, 422].includes(error?.status) || [
  "publication_conflict", "publication_invalid", "publication_not_found", "publication_platform_closed",
].includes(error?.code);

export function buildBatchStart(batch, preview, requestKey, allowShortfall = false) {
  if (preview?.batch_id !== batch.id || preview.revision !== batch.revision)
    throw new Error("批次配置已变化，请重新检查");
  const all = batch.config.candidates.map(row => row.row_key);
  const selectedRows = preview.rows || [];
  const included = new Set(selectedRows.map(row => row.row_key));
  if (!preview.can_start || !included.size || included.size !== selectedRows.length ||
      selectedRows.some(row => row.ready !== true || !all.includes(row.row_key)))
    throw new Error("当前范围还有待处理候选，请重新检查");
  const perCategory = new Map((preview.shortfalls || []).map(row => [row.category_id, row.shortfall]));
  if (preview.shortfalls?.length !== batch.targets.length || perCategory.size !== batch.targets.length || batch.targets.some(target =>
    !Number.isSafeInteger(perCategory.get(target.category_id)) || perCategory.get(target.category_id) < 0))
    throw new Error("启动检查未返回完整目标缺口");
  const expected_shortfalls = batch.targets.map(target => ({
    category_id: target.category_id, shortfall: perCategory.get(target.category_id),
  }));
  if (!allowShortfall && expected_shortfalls.some(row => row.shortfall > 0))
    throw new Error("请确认本次发布范围和目标缺口");
  return {
    request_key: requestKey,
    expected_revision: batch.revision,
    included_row_keys: all.filter(key => included.has(key)),
    excluded_row_keys: all.filter(key => !included.has(key)),
    allow_shortfall: allowShortfall,
    expected_shortfalls,
  };
}

export function buildBatchControl(batch, action, { reason = "", failures = [], requestKey } = {}) {
  if (action === "resume") {
    const pause = batch.execution?.pause;
    if (!pause?.id || !Array.isArray(pause.exceptions)) throw new Error("暂停依据已变化，请刷新后核对");
    return {
      request_key: requestKey, expected_revision: batch.revision, expected_pause_id: pause.id,
      acknowledged_exceptions: pause.exceptions.map(pauseRef),
    };
  }
  const input = { expected_revision: batch.revision, reason: reason.trim() };
  if (action === "finish") input.expected_failed_tasks = failures.map(taskRef);
  return input;
}

export function batchProgress(execution) {
  if (!execution) return null;
  const { target_count: target, published_count: published, created_count: created,
    creation_shortfall: creationShortfall, target_shortfall: targetShortfall, task_counts: counts } = execution;
  return {
    target, published, created, creationShortfall, targetShortfall,
    rate: target > 0 ? Math.round(published * 10000 / target) / 100 : 0,
    reviewing: counts?.reviewing || 0,
    exceptions: (counts?.failed || 0) + (counts?.awaiting_human || 0) + (counts?.uncertain || 0),
  };
}

async function readBatchTaskSet(api, id, status) {
  const all = new Map();
  let total;
  for (let page = 1; ; page++) {
    const result = await api.tasks(id, { ...(status && { status }), page, page_size: 100 });
    if (!Number.isSafeInteger(result?.total) || !Array.isArray(result.items) ||
      (total != null && result.total !== total))
      throw new Error("正式任务集合已变化，请重新核对");
    total = result.total;
    for (const row of result.items) {
      if (all.has(row.id) || typeof row.id !== "string" || !row.id || typeof row.revision !== "string" || !row.revision || (status && row.status !== status))
        throw new Error("正式任务集合已变化，请重新核对");
      all.set(row.id, row);
    }
    if (all.size === total) return [...all.values()];
    if (!result.items.length || all.size > total) throw new Error("正式任务集合已变化，请重新核对");
  }
}

export const readBatchFailures = (api, id) => readBatchTaskSet(api, id, "failed");

export async function sendBatchCommand(api, id, operation, { retry = false } = {}) {
  const { action, input } = operation;
  let latest;
  function unknown(cause) {
    const unconfirmedStart = retry && action === "start" && latest?.execution?.first_started_at &&
      cause?.status === 409 && cause.code === "publication_conflict";
    const error = new Error(unconfirmedStart
      ? "批次已开始，Cloud 未确认原启动请求。请核对现有正式任务。"
      : "操作结果待确认，请核对原请求后继续", { cause });
    error.outcomeUnknown = true;
    error.latest = latest;
    return error;
  }
  async function readBack() {
    const current = await api.get(id);
    if (current?.id !== id) throw new Error("批次读回与原请求不一致");
    latest = current;
  }
  if (retry) {
    try {
      await readBack();
    } catch (error) {
      throw unknown(error);
    }
  }
  try {
    // Only the same-key, same-digest A02 receipt proves which start occurred.
    // The detail and formal task list can show a later revision with the same rows.
    const result = await api[action](id, input);
    const complete = action === "start"
      ? result?.execution_status === "running" && !!result.first_started_at &&
        result.revision === String(BigInt(input.expected_revision) + 1n) &&
        Array.isArray(result.task_ids) && result.task_ids.length === input.included_row_keys.length &&
        new Set(result.task_ids).size === result.task_ids.length &&
        result.task_ids.every(taskID => typeof taskID === "string" && /^[1-9][0-9]*$/.test(taskID))
      : !!result?.execution?.phase;
    if (result?.batch_id !== id || !result.revision || !complete) throw new Error("操作响应不完整");
    return action === "start" && retry ? { ...result, recovered: true } : result;
  } catch (error) {
    if (action === "start" && error?.status === 409 && error.code === "publication_conflict" &&
        error.details?.batch_id === id && error.details.start_request_applied === false) {
      const rejection = new Error("批次已由另一请求开始，请核对正式任务", { cause: error });
      rejection.batchRejection = true;
      rejection.supersededStart = true;
      rejection.latest = latest?.execution?.first_started_at ? latest : undefined;
      throw rejection;
    }
    if (!retry && knownRejection(error)) {
      error.batchRejection = true;
      throw error;
    }
    let readAfterFailure = false;
    try {
      await readBack();
      readAfterFailure = true;
    } catch {
      // Keep the last known detail for display even if it cannot settle the intent.
    }
    if (retry && conflictAfterReplay(error, id, operation, latest, readAfterFailure)) {
      error.batchRejection = true;
      throw error;
    }
    throw unknown(error);
  }
}

export function batchCommandJournal(origin, runtime, userID, batchID,
  storage = globalThis.localStorage, locks = globalThis.navigator?.locks) {
  const key = ["wt-media:publication-command:v1", origin, runtime, userID, batchID].join("|");
  function read() {
    const stored = storage.getItem(key);
    if (!stored) return null;
    const item = JSON.parse(stored);
    if (!["start", "resume"].includes(item?.action) || !item.input?.request_key || !item.input.expected_revision)
      throw new Error("无法读取待核对的批次请求");
    return item;
  }
  return {
    read,
    save(operation) {
      if (!locks) throw new Error("当前客户端无法安全保留待核对的批次请求");
      return locks.request(key, () => {
        const current = read();
        if (current && JSON.stringify(current) !== JSON.stringify(operation))
          throw new Error("已有待核对的批次请求");
        storage.setItem(key, JSON.stringify(operation));
      });
    },
    clear(operation) {
      if (!locks) throw new Error("当前客户端无法安全清除待核对的批次请求");
      return locks.request(key, () => {
        const current = read();
        if (current?.action === operation.action && current.input.request_key === operation.input.request_key)
          storage.removeItem(key);
      });
    },
  };
}

const listFilterKeys = ["search", "platform", "configuration_status", "execution_status", "page", "page_size"];
const taskFilterKeys = ["search", "status", "media_account_id", "page", "page_size"];

export function batchTaskLocation(batchID, taskID, listFilters, taskScope) {
  const query = { publication_id: taskID, from_batch: batchID };
  for (const [prefix, keys, values] of [
    ["batch_", listFilterKeys, listFilters], ["task_", taskFilterKeys, taskScope],
  ]) {
    for (const key of keys) if (values[key]) query[prefix + key] = String(values[key]);
  }
  return { name: "Publish", query };
}

export function batchTaskReturn(query) {
  if (typeof query.from_batch !== "string" || !/^[1-9][0-9]*$/.test(query.from_batch)) return null;
  const back = { view: "detail", id: query.from_batch, tab: "tasks" };
  for (const key of listFilterKeys) {
    const value = query["batch_" + key];
    if (typeof value === "string") back[key] = value;
  }
  for (const key of taskFilterKeys) {
    const value = query["task_" + key];
    if (typeof value === "string") back["task_" + key] = value;
  }
  return { name: "PublishBatches", query: back };
}
