import { canRetry, validate } from "./model.js";
export const displayStatus = (t) =>
  t.status === "执行失败" ? "发布失败" : t.status;
export const canEditTask = (t) => t.status === "待执行" && !t.claimed;
export const canCancelTask = (t) => {
  if (
    t.submittedAt ||
    t.platformId ||
    ["未知", "平台已接收", "已公开"].includes(t.result)
  )
    return false;
  if (t.status === "待执行") return !t.claimed;
  if (t.status === "排队中") return !t.claimed;
  return (
    ["执行中", "等待人工"].includes(t.status) &&
    t.executorEvidence?.safeToCancel === true &&
    t.executorEvidence?.attempt === t.attemptHistory?.at(-1)?.number &&
    !t.submittedAt
  );
};
export function allowedActions(t) {
  const actions = ["detail"];
  if (canEditTask(t)) actions.push("edit", "start");
  if (canCancelTask(t)) actions.push("cancel");
  if (t.status === "等待人工") actions.push("human");
  if (t.status === "结果待确认") actions.push("verify");
  if (canRetry(t)) actions.push("retry");
  if (["执行失败", "已发布", "已取消", "已放弃"].includes(t.status))
    actions.push("copy");
  return actions;
}
export const actionLabels = {
  detail: "详情",
  edit: "编辑",
  start: "执行",
  cancel: "取消",
  human: "去处理",
  verify: "核实结果",
  retry: "重试",
  copy: "复制",
};
export const canReleaseTask = (t) =>
  !t.claimed &&
  !["未知", "平台已接收", "已公开"].includes(t.result) &&
  ((t.status === "已取消" && !t.submittedAt && !t.platformId) || canRetry(t));
export function taskChecks(t, db) {
  const issues = validate(t, db, t.id);
  const a = db.accounts.find((a) => a.id === t.accountId);
  if (a?.loginValid === false) issues.push("账号登录已失效，请先处理账号登录");
  if (db.platformBlocks?.[t.platform])
    issues.push("当前平台暂停执行，请先处理平台限制");
  return [...new Set(issues)];
}
export function taskPresentation(t, db) {
  const checks = taskChecks(t, db),
    actions = allowedActions(t);
  const action =
    ["start", "human", "verify", "retry", "cancel"].find((a) =>
      actions.includes(a),
    ) || "detail";
  const descriptions = {
    待执行: "内容准备完成后执行，校验通过进入队列。",
    排队中: "等待调度领取，不重复执行。",
    执行中: "执行器正在处理，请关注实际阶段反馈。",
    等待人工: t.evidence || "需要登录或平台验证。",
    结果待确认: "提交结果不明，请核实原尝试。",
    平台审核中: "平台已接收，尚未公开，不计入已发布目标。",
    执行失败: t.evidence || "检查失败原因与执行结束证据。",
    已发布: "已确认公开发布。",
    已取消: "任务已安全终止，资源占用仍保留。",
    已放弃: "原任务已终止，资源已按既有规则释放。",
  };
  return {
    title: displayStatus(t),
    description: descriptions[t.status],
    action,
    label: actionLabels[action],
    checks,
    disabled: action === "start" && checks.length > 0,
  };
}
