import { platforms, resourceStatus } from "./model.js";
export const businessDate = (date) =>
  new Intl.DateTimeFormat("sv-SE", {
    timeZone: "Asia/Shanghai",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(date));
// Capacities belong to accounts. A shared account can send its remaining quota
// to either category, so a bipartite flow estimates the joint allocation once.
function jointEstimate(accounts, categories) {
  const edges = new Map();
  function edge(a, b, capacity) {
    if (!edges.has(a)) edges.set(a, new Map());
    if (!edges.has(b)) edges.set(b, new Map());
    edges.get(a).set(b, capacity);
    edges.get(b).set(a, 0);
  }
  accounts.forEach((a) => {
    edge("source", "a" + a.id, a.remaining);
    categories
      .filter((c) => a.categories?.includes(c.category))
      .forEach((c) => edge("a" + a.id, "c" + c.category, c.free));
  });
  categories.forEach((c) => edge("c" + c.category, "sink", c.free));
  let total = 0;
  while (true) {
    const path = new Map([["source", null]]),
      queue = ["source"];
    for (let i = 0; i < queue.length && !path.has("sink"); i++)
      for (const [next, capacity] of edges.get(queue[i]) || [])
        if (capacity > 0 && !path.has(next)) {
          path.set(next, queue[i]);
          queue.push(next);
        }
    if (!path.has("sink")) return total;
    let amount = Infinity;
    for (let at = "sink"; path.get(at) !== null; at = path.get(at))
      amount = Math.min(amount, edges.get(path.get(at)).get(at));
    for (let at = "sink"; path.get(at) !== null; at = path.get(at)) {
      const previous = path.get(at);
      edges.get(previous).set(at, edges.get(previous).get(at) - amount);
      edges.get(at).set(previous, edges.get(at).get(previous) + amount);
    }
    total += amount;
  }
}
export function platformInventory(db, now = new Date().toISOString()) {
  return platforms.map((platform) => {
    const accounts = db.accounts.filter((a) => a.platform === platform),
      rows = db.tasks.filter((t) => t.platform === platform);
    const today = rows.filter(
      (t) =>
        t.status === "已发布" &&
        t.publicAt &&
        businessDate(t.publicAt) === businessDate(now),
    );
    const scheduled = rows.filter(
      (t) =>
        ["待执行", "排队中", "执行中", "等待人工", "结果待确认"].includes(
          t.status,
        ) &&
        (t.schedule || t.created) &&
        businessDate(t.schedule || t.created) === businessDate(now),
    );
    const used = rows.filter(
      (t) =>
        today.includes(t) ||
        scheduled.includes(t) ||
        (t.status === "平台审核中" &&
          (t.submittedAt || t.schedule || t.created) &&
          businessDate(t.submittedAt || t.schedule || t.created) ===
            businessDate(now)),
    );
    const usable = accounts.filter((a) => a.enabled && a.loginValid !== false);
    const available = usable.map((a) => ({
      ...a,
      remaining: Number.isFinite(a.dailyCapacity)
        ? Math.max(
            0,
            a.dailyCapacity - used.filter((t) => t.accountId === a.id).length,
          )
        : null,
    }));
    const byCategory = [...new Set(db.assets.map((a) => a.category))].map(
      (category) => {
        const free = db.assets.filter(
          (a) => a.category === category && !resourceStatus(db, a, platform),
        ).length;
        const eligible = available.filter((a) =>
          a.categories?.includes(category),
        );
        return {
          category,
          free,
          occupied: db.assets.filter(
            (a) =>
              a.category === category &&
              rows.some((t) => t.assetId === a.id && t.status !== "已放弃"),
          ).length,
          eligibleAccounts: eligible.length,
          estimate: eligible.some((a) => a.remaining === null)
            ? null
            : Math.min(
                free,
                eligible.reduce((n, a) => n + a.remaining, 0),
              ),
        };
      },
    );
    const known = available.every((a) => a.remaining !== null);
    return {
      platform,
      total: accounts.length,
      available: usable.length,
      quality: ["优质", "普通", "新号"]
        .map((q) => usable.filter((a) => a.quality === q).length)
        .join(" / "),
      invalid: accounts.filter((a) => !a.enabled || a.loginValid === false)
        .length,
      published: today.length,
      publishAccounts: new Set(today.map((t) => t.accountId)).size,
      scheduled: scheduled.length,
      capacity: known ? available.reduce((n, a) => n + a.remaining, 0) : null,
      estimate: known ? jointEstimate(available, byCategory) : null,
      byCategory,
      free: db.assets.filter((a) => !resourceStatus(db, a, platform)).length,
      refreshedAt: now,
    };
  });
}
