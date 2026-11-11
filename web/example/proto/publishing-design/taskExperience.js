import { canRetry, validate } from "./model.js";
export const displayStatus = (t) =>
  t.status === "执行失败" ? "确认失败" : t.status;
export const canEditTask = (t) =>
  !t.claimed && (["待执行", "排队中"].includes(t.status) || canRetry(t));
export const canCancelTask = (t) =>
  !t.claimed && ["待执行", "排队中"].includes(t.status);
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
  const checks = taskChecks(t, db);
  const map = {
    待执行: [
      "准备启动任务",
      checks.length
        ? `还有 ${checks.length} 项发布前检查需要处理。`
        : "资源与内容检查通过，启动后进入执行队列。",
      "启动任务",
      "start",
    ],
    排队中: [
      "正在等待执行",
      t.schedule
        ? `指定时间：${t.schedule}。到时仍需满足账号和执行条件。`
        : "按账号顺序等待执行，当前不会重复上传。",
      "查看排队状态",
      "inspect",
    ],
    执行中: [
      "任务正在执行",
      "关注当前步骤与最近操作，当前尝试使用的内容版本已锁定。",
      "查看进度",
      "inspect",
    ],
    等待人工: [
      "需要你处理",
      t.evidence || "账号登录或平台验证需要人工完成，请回到原账号窗口继续。",
      "去处理",
      "human",
    ],
    结果待确认: [
      "先核实提交结果",
      "提交结果尚不明确。请检查同账号稿件，核实前保留成片占用。",
      "核实结果",
      "verify",
    ],
    平台审核中: [
      "等待平台审核",
      "平台已接收，尚未确认公开发布；暂不计入已发布目标。",
      "查看审核状态",
      "verify",
    ],
    执行失败: [
      "确认失败，先检查原因",
      canRetry(t)
        ? "已确认未发布且上次执行已结束。修复内容并通过检查后，可在原任务安全重试。"
        : "先核实原稿件与执行是否安全结束，当前不能重新上传。",
      canRetry(t) ? "安全重试" : "查看失败原因",
      canRetry(t) ? "start" : "inspect",
    ],
    已发布: [
      "已公开发布",
      "查看最终作品、发布时间与本次提交使用的内容。",
      "查看作品",
      "work",
    ],
    已取消: [
      "任务已取消",
      "成片占用仍保留，原任务不能恢复。确认可安全释放后，可放弃并释放。",
      "查看记录",
      "inspect",
    ],
    已放弃: [
      "任务已放弃",
      "旧任务已永久结束，成片占用已释放。复制内容后需重新选择和校验资源。",
      "复制配置创建新任务",
      "copy",
    ],
  };
  const [title, description, label, action] = map[t.status] || [
    "查看任务",
    "请检查当前执行记录。",
    "查看记录",
    "inspect",
  ];
  return {
    title,
    description,
    label,
    action,
    checks,
    disabled:
      action === "start" &&
      (checks.length > 0 ||
        t.claimed ||
        (t.status !== "待执行" && !canRetry(t))),
  };
}
