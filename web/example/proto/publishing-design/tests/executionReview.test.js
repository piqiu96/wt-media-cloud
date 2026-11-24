import { expect, it } from "vitest";
import { batchActions, batchMetrics, batchState, executionPreview, runBatchCommand, startReviewScope } from "../batchLifecycle.js";
import { executionExceptions } from "../batchExecutionSettings.js";
import { defaults, declarationOptions, seed } from "../model.js";
import { claimTask, setResult } from "../relationships.js";

function fixture({ policy = "自动继续", limit = 3, shared = true } = {}) {
  const db = seed();
  db.tasks = [];
  db.plans = [];
  const platform = "百家号";
  const category = "三角洲行动";
  const account = db.accounts.find(row => row.platform === platform && row.enabled && row.categories.includes(category));
  db.accounts.push({ ...account, id: 991, platformUid: "second", window: shared ? account.window : "independent" });
  const candidates = db.assets.filter(row => row.category === category && row.ready !== false).slice(0, 3)
    .map((asset, index) => ({ ...defaults(platform, asset), key: `row-${index}`, accountId: index === 1 ? 991 : account.id,
      assetId: asset.id, category, platform, declaration: declarationOptions(platform)[0].value }));
  const batch = { id: 765, name: "走查批次", preparationOnly: true, executionReview: true, ownerId: account.ownerId, platform,
    targets: [{ category, count: 4 }], categories: [category], candidates,
    executionSettings: { concurrency: 5, groupInterval: 10, groupPolicy: policy, exceptionLimit: limit } };
  db.batches = [batch];
  return { db, batch };
}
function start({ db, batch }) {
  runBatchCommand(db, batch, "execute", { scope: startReviewScope(db, batch).signature, partial: true });
}
it("starts only the preflight scope and retains the original target and missing count", () => {
  const context = fixture();
  const { db, batch } = context;
  expect(db.tasks).toHaveLength(0);
  expect(batchActions(db, batch)).toContain("execute");
  expect(executionPreview(db, batch)).toHaveLength(3);
  const scope = startReviewScope(db, batch).signature;
  expect(() => runBatchCommand(db, batch, "execute", { scope })).toThrow(/缺口/);
  expect(db.tasks).toHaveLength(0);
  start(context);
  expect(db.tasks.map(task => task.attempts)).toEqual([0, 0, 0]);
  expect(batchMetrics(db, batch)).toMatchObject({ target: 4, total: 3, published: 0 });
});
it("admits a single available profile at concurrency five, with no refill and no early next group", () => {
  const context = fixture();
  const { db, batch } = context;
  start(context);
  const [first, later] = db.tasks;
  expect(batch.executionGroup.taskIds).toEqual([first.id]);
  const now = Date.now();
  expect(claimTask(db, later, now)).toBe(false);
  expect(claimTask(db, first, now)).toBe(true);
  setResult(first, "平台审核中");
  expect(claimTask(db, later, now + 9_000)).toBe(false);
  expect(claimTask(db, later, now + 11_000)).toBe(true);
  expect(later.executionGroup).toBe(2);
});
it("lets two independent profiles in the fixed group run without an in-group interval", () => {
  const context = fixture({ shared: false });
  const { db, batch } = context;
  start(context);
  const [first, second, outside] = db.tasks;
  const now = Date.now();
  expect(batch.executionGroup.taskIds).toEqual([first.id, second.id]);
  expect(claimTask(db, first, now)).toBe(true);
  expect(claimTask(db, second, now)).toBe(true);
  setResult(first, "平台审核中");
  expect(claimTask(db, outside, now + 60_000)).toBe(false);
});
it("separates unknown scenes and audit results from failures and trusted public totals", () => {
  const context = fixture({ policy: "异常暂停" });
  const { db, batch } = context;
  start(context);
  const [first] = db.tasks;
  expect(claimTask(db, first)).toBe(true);
  setResult(first, "结果待确认");
  expect(batchActions(db, batch)).not.toContain("finish");
  expect(batchMetrics(db, batch)).toMatchObject({ exceptions: 1, published: 0 });
  runBatchCommand(db, batch, "stop", { reason: "演示终止" });
  expect(batchState(db, batch)).toBe("待处理");
  expect(db.tasks.map(task => task.status)).toEqual(["结果待确认", "已取消", "已取消"]);
  expect(batchActions(db, batch)).not.toContain("resume");
});

it("keeps known failures and occupancy after explicitly ending the finished batch", () => {
  const context = fixture();
  context.batch.candidates = context.batch.candidates.slice(0, 1);
  const { db, batch } = context;
  start(context);
  const [task] = db.tasks;
  expect(claimTask(db, task)).toBe(true);
  setResult(task, "执行失败");
  expect(batchActions(db, batch)).toContain("finish");
  runBatchCommand(db, batch, "finish");
  expect(batchState(db, batch)).toBe("已结束");
  expect(task.status).toBe("执行失败");
  expect(batchMetrics(db, batch)).toMatchObject({ failed: 1, pending: 0 });
});

it("pauses automatic continuation at the exception limit and checks the current basis on resume", () => {
  const context = fixture({ limit: 1 });
  const { db, batch } = context;
  start(context);
  const [task, later] = db.tasks;
  expect(claimTask(db, task)).toBe(true);
  setResult(task, "等待人工", { evidence: "原现场已挂起" });
  task.suspendedAt = new Date().toISOString();
  expect(claimTask(db, later)).toBe(false);
  expect(batchState(db, batch)).toBe("已暂停");
  const { id: pauseId } = batch.pause;
  const current = executionExceptions(db, batch).map(item => item.key);
  expect(() => runBatchCommand(db, batch, "resume", { pauseId, exceptionKeys: [] })).toThrow(/异常范围/);
  runBatchCommand(db, batch, "resume", { pauseId, exceptionKeys: current });
  expect(batchState(db, batch)).toBe("执行中");
  expect(task.status).toBe("等待人工");
  expect(claimTask(db, later, Date.now() + 11_000)).toBe(false);
  expect(batchState(db, batch)).toBe("执行中");
});

it("keeps a shared Profile occupied by another platform without preventing independent work", () => {
  const context = fixture({ shared: false });
  const { db, batch } = context;
  const account = db.accounts.find(row => row.id === batch.candidates[0].accountId);
  db.accounts.push({ ...account, id: 999, platform: "B站" });
  db.tasks.push({ id: 998, accountId: 999, platform: "B站", status: "结果待确认", batchId: 777 });
  start(context);
  const [waiting, independent] = db.tasks.filter(row => row.batchId === batch.id);
  expect(batch.executionGroup.taskIds).toEqual([independent.id]);
  expect(claimTask(db, waiting)).toBe(false);
  expect(waiting.attempts).toBe(0);
  expect(claimTask(db, independent)).toBe(true);
  expect(db.tasks[0].status).toBe("结果待确认");
});
