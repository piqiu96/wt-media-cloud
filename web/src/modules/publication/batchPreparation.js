export const generationFields = {
  description: "发布文案",
  topics: "话题",
  cover_title: "封面短标题",
};

export const coverProfiles = [
  { key: "horizontal", label: "横版封面 · 16:9", aspect: [16, 9], file: "cover_file_id", render: "cover_render" },
  { key: "portrait", label: "竖版封面 · 3:4", aspect: [3, 4], file: "portrait_cover_file_id", render: "portrait_cover_render" },
];

export const configurationLabels = { ready: "配置就绪", incomplete: "待完善" };
export const executionLabels = { not_started: "未开始", running: "发布中", paused: "已暂停", stopping: "终止收尾", ended: "已结束", cancelled: "已取消" };
export const groupPolicies = [
  { label: "自动继续", value: "auto_continue" },
  { label: "异常暂停", value: "pause_on_exception" },
];

export const snapshot = (value) => JSON.parse(JSON.stringify(value));
const nextRevision = (value) => String(BigInt(value || "0") + 1n);

export function emptyBatch(source) {
  if (source) return snapshot({
    name: source.name,
    platform: source.platform,
    recommendation_quantity: source.recommendation_quantity ?? null,
    targets: source.targets || [],
    config: {
      ...source.config,
      accounts: source.config?.accounts || [],
      output_ids: source.config?.output_ids || [],
      manual_pairs: source.config?.manual_pairs || [],
      candidates: source.config?.candidates || [],
    },
    execution_setting: source.execution_setting,
  });
  return {
    name: "",
    platform: "baidu",
    recommendation_quantity: null,
    targets: [],
    config: { accounts: [], output_ids: [], manual_pairs: [], candidates: [] },
    execution_setting: {
      concurrency_limit: 5,
      group_interval_seconds: 0,
      between_group_policy: "pause_on_exception",
      exception_task_limit: null,
    },
  };
}

export function batchInput(form) {
  return snapshot({
    platform: form.platform,
    targets: form.targets,
    config: {
      ...form.config,
      candidates: form.config.candidates.map((row, order) => ({ ...row, order })),
    },
    execution_setting: form.execution_setting,
  });
}

export function moveEntry(entries, index, delta) {
  const destination = index + delta;
  if (index < 0 || destination < 0 || destination >= entries.length) return;
  const [item] = entries.splice(index, 1);
  entries.splice(destination, 0, item);
}

export function pairOf(row) {
  return { category_id: row.category_id, media_account_id: row.media_account_id, composite_output_id: row.composite_output_id };
}

const pairKey = (pair) => Object.values(pairOf(pair)).join(":");

export function newCandidate(pair) {
  return { ...pairOf(pair), row_key: crypto.randomUUID(), input_revision: "1", field_revisions: {}, content: {}, order: 0 };
}

export function reconcileAllocation(previous, suggested) {
  const available = new Map(suggested.map(row => [row.composite_output_id, row]));
  const rows = [];
  for (const row of previous) {
    const match = available.get(row.composite_output_id);
    if (match) rows.push(replaceCandidatePair(row, match));
    available.delete(row.composite_output_id);
  }
  rows.push(...Array.from(available.values(), row => ({ ...row, row_key: crypto.randomUUID() })));
  return rows;
}

export function replaceCandidatePair(row, pair) {
  if (pairKey(row) === pairKey(pair)) return row;
  const content = { ...row.content, confirmed: false };
  if (row.composite_output_id !== pair.composite_output_id || row.category_id !== pair.category_id) {
    for (const profile of coverProfiles) {
      delete content[profile.file];
      delete content[profile.render];
    }
  }
  return { ...row, ...pairOf(pair), input_revision: nextRevision(row.input_revision), content };
}

export function editCandidate(row, patch) {
  const content = { ...row.content };
  const field_revisions = { ...row.field_revisions };
  let cover_title = row.cover_title;
  let changed = false;
  for (const [field, value] of Object.entries(patch)) {
    const old = field === "cover_title" ? cover_title : content[field];
    if (JSON.stringify(old) === JSON.stringify(value)) continue;
    if (field === "cover_title") cover_title = value;
    else if (value === undefined) delete content[field];
    else content[field] = value;
    field_revisions[field] = nextRevision(field_revisions[field]);
    if (field !== "confirmed") changed = true;
  }
  if (cover_title !== row.cover_title) {
    for (const profile of coverProfiles) {
      if (!content[profile.render] || content[profile.render].title === cover_title?.trim()) continue;
      for (const field of [profile.file, profile.render]) {
        delete content[field];
        field_revisions[field] = nextRevision(field_revisions[field]);
      }
    }
  }
  if (changed) content.confirmed = false;
  if (changed || content.confirmed !== row.content.confirmed) return { ...row, cover_title, field_revisions, content };
  return row;
}

export function candidateShortfalls(form) {
  const byCategory = new Map();
  for (const row of form.config.candidates) byCategory.set(row.category_id, (byCategory.get(row.category_id) || 0) + 1);
  return form.targets.map(target => {
    const count = byCategory.get(target.category_id) || 0;
    return { ...target, candidate_count: count, shortfall: Math.max(0, target.target_count - count) };
  });
}

export function generationInput(row, platform, fields, instruction) {
  const keys = Object.keys(generationFields);
  return {
    row_key: row.row_key,
    input_revision: row.input_revision,
    platform,
    category_id: row.category_id,
    composite_output_id: row.composite_output_id,
    fields,
    field_revisions: Object.fromEntries(fields.map(field => [field, row.field_revisions[field] || "0"])),
    content: Object.fromEntries(keys.map(field => [field, (field === "cover_title" ? row.cover_title : row.content[field]) || ""])),
    instruction,
  };
}

export function matchesCandidateField(row, basis, field) {
  return !!row && row.row_key === basis.row_key && row.input_revision === basis.input_revision &&
    (row.field_revisions[field] || "0") === basis.field_revisions[field];
}

export function displayAvailability(value) {
  if (value?.error) return value.error;
  if (value?.available_output_count > 0) return `${value.available_output_count} 个可用成片`;
  if (value?.unknown_output_count > 0) return "可用量未知";
  if (value?.availability === "empty") return "暂无可用成片";
  return "可用量未知";
}

const definiteSaveFailure = (error) => [
  "publication_invalid", "publication_not_found", "publication_conflict",
  "publication_output_occupied", "publication_platform_closed",
].includes(error?.code);

export async function savedBatchForRequest(api, key) {
  const result = await api.list({ request_key: key, page: 1, page_size: 1 });
  const id = result.items?.[0]?.id;
  return id ? api.get(id) : null;
}

export async function sendNewBatch(api, input, { retry = false } = {}) {
  if (retry) {
    const found = await savedBatchForRequest(api, input.request_key);
    if (found) return found;
  }
  try {
    const result = await api.save(input);
    if (!result?.id || !result?.revision) throw new Error("保存响应不完整");
    return result;
  } catch (error) {
    if (definiteSaveFailure(error)) {
      error.batchSaveRejected = true;
      throw error;
    }
    const found = await savedBatchForRequest(api, input.request_key);
    if (found) return found;
    throw new Error("保存结果待确认，请用原请求查询或重试", { cause: error });
  }
}

export function publicContentPatch(row, values, overwrite = false) {
  return Object.fromEntries(Object.entries(values).filter(([field, value]) =>
    value?.trim() && (overwrite || !(field === "cover_title" ? row.cover_title : row.content[field])?.trim())));
}

export function batchFormProblem(form, { requireName = true } = {}) {
  if (requireName && !form.name.trim()) return "请填写批次名称";
  if (!form.targets.length) return "请至少添加一个目标类目";
  const positive = (n) => Number.isSafeInteger(n) && n > 0;
  if (form.targets.some(target => !Number.isSafeInteger(target.target_count) || target.target_count < 0)) return "发布目标须为非负整数";
  const settings = form.execution_setting;
  if (!positive(settings.concurrency_limit)) return "请填写并发数（正整数）";
  if (!Number.isSafeInteger(settings.group_interval_seconds) || settings.group_interval_seconds < 0) return "请填写组间发布间隔";
  if (!positive(settings.exception_task_limit)) return "请填写异常任务上限（正整数）";
  return "";
}

export function reviewSelectedConfirmations(rows, issues) {
  if (!Array.isArray(issues)) throw new Error("批次检查未返回完整结果");
  const firstIssue = new Map();
  for (const issue of issues) {
    if (issue.code === "confirmation_required" && issue.field === "confirmed") continue;
    if (issue.row_key && !firstIssue.has(issue.row_key))
      firstIssue.set(issue.row_key, issue.message || "候选配置仍有待处理项");
  }
  const confirmable = [];
  const blocked = [];
  for (const row of rows) {
    if (row.content.confirmed) continue;
    if (firstIssue.has(row.row_key)) blocked.push({ rowKey: row.row_key, message: firstIssue.get(row.row_key) });
    else confirmable.push(row.row_key);
  }
  return { confirmable, blocked };
}
