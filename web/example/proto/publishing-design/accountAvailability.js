import { businessDate } from "./inventory.js";
export function accountAvailability(
  db,
  a,
  platform,
  category,
  ignoreTaskId,
  now = new Date().toISOString(),
) {
  const used = db.tasks.filter(
    (t) =>
      t.id !== ignoreTaskId &&
      t.accountId === a.id &&
      [
        "待执行",
        "排队中",
        "执行中",
        "等待人工",
        "结果待确认",
        "平台审核中",
        "已发布",
      ].includes(t.status) &&
      (t.publicAt || t.submittedAt || t.schedule || t.createdAt || t.created) &&
      businessDate(
        t.publicAt || t.submittedAt || t.schedule || t.createdAt || t.created,
      ) === businessDate(now),
  ).length;
  const remaining = Number.isFinite(a.dailyCapacity)
    ? Math.max(0, a.dailyCapacity - used)
    : null;
  const reason = !a.enabled
    ? "账号已停用"
    : a.platform !== platform
      ? "平台不匹配"
      : !a.categories?.includes(category)
        ? "类目不匹配"
        : a.loginValid === false
          ? "登录已失效"
          : remaining === 0
            ? "今日额度已用完"
            : "";
  return {
    remaining,
    limit: a.dailyCapacity ?? null,
    reason,
    level: a.level ?? null,
  };
}
