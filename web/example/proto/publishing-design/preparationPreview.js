// The CHG107 preview records the configuration separately from the execution demo.
const clone = value => JSON.parse(JSON.stringify(value));
export const previewGroupPolicies = ["自动继续", "异常暂停"];
export const previewExecutionSettings = () => ({
  concurrency: 5,
  groupInterval: 0,
  groupPolicy: "异常暂停",
  exceptionLimit: null,
});

export function preparationRecord(state, { id, actor, existing, executionReview = false, now = new Date().toISOString() }) {
  const settings = state.executionSettings;
  if (!state.name.trim() || state.platforms[0] !== "百家号" || !state.categories.length)
    throw Error("请填写批次名称并添加目标类目。");
  if (!Number.isSafeInteger(settings.concurrency) || settings.concurrency < 1 ||
      !Number.isSafeInteger(settings.groupInterval) || settings.groupInterval < 0 ||
      !previewGroupPolicies.includes(settings.groupPolicy) ||
      !Number.isSafeInteger(settings.exceptionLimit) || settings.exceptionLimit < 1)
    throw Error("请填写有效的执行设置，包括异常任务上限。");
  return {
    ...existing, id, preparationOnly: true,
    ...(executionReview && { executionReview: true, version: (existing?.version || 0) + 1 }),
    ownerId: actor.id, createdBy: actor.name,
    name: state.name.trim(), platform: state.platforms[0], categories: clone(state.categories),
    targets: clone(state.targets), candidates: clone(state.rows), executionSettings: clone(settings),
    draft: { ...clone(state), batchId: id, resourcesLocked: false },
    created: existing?.created || now, updatedAt: now,
    log: [...(existing?.log || []), { actor: actor.name, time: now,
      type: existing ? "保存修改" : "保存批次", text: "候选配置已保存" }],
  };
}
