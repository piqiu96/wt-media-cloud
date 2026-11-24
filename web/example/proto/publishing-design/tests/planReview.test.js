import { describe, expect, it } from "vitest";
import { seed, defaults, declarationOptions, canRetry } from "../model.js";
import { confirmPlan, dispatchPlan, syncPlan, setResult } from "../relationships.js";
import { planViews, planAction, protectPlanSnapshot } from "../planBridge.js";

import { addPlanReviewScenarios } from "../scenarios.js";
import { createBatchStore } from "../../../publishing-preview.mjs";
import { batchActions } from "../batchLifecycle.js";

const fixture = (db, id) => db.plans.find(plan => plan.id === "PL-109-" + id);
const view = (db, id) => planViews(db).find(plan => plan.id === "PL-109-" + id);

function setup() {
  const db = seed();
  db.plans = [];
  db.batches = [];
  db.tasks = [];
  const platform = "百家号";
  const category = "三角洲行动";
  const plan = confirmPlan(db, {
    name: "计划候选冻结", confirmKey: "review-plan", platformOrder: [platform],
    targets: [{ platform, category, count: 2 }],
    splits: ["首批", "下一批"].map(name => ({ name, platform, targets: [{ category, count: 1 }] })),
  });
  Object.assign(plan, { planReview: true, ownerId: "chen" });
  const account = { id: 5001, name: "计划演示账号", platform, categories: [category],
    enabled: true, loginValid: true, ownerId: "chen", profileId: "preview-profile", platformUid: "preview-uid" };
  db.accounts.push(account);
  for (const [index, batch] of db.batches.entries()) {
    const asset = { ...db.assets.find(item => item.category === category && item.cover),
      id: 6001 + index, category, ready: true, fileAvailable: true };
    db.assets.push(asset);
    Object.assign(batch, { planReview: true, executionReview: true, preparationOnly: true, ownerId: "chen",
      phase: index === 0 ? "内容预检通过" : "待内容配置",
      candidates: [{ ...defaults(platform, asset), platform, category, assetId: asset.id, accountId: account.id,
        key: "row-" + index, declaration: declarationOptions(platform)[0].value }],
      executionSettings: { concurrency: 5, groupInterval: 0, groupPolicy: "自动继续", exceptionLimit: 3 } });
  }
  return { db, plan };
}

describe("CHG109 preview", () => {
  it("shows prepared candidates without formal tasks and requires the later batch before starting", () => {
    const { db, plan } = setup();
    expect(planViews(db)[0].batches.map(batch => batch.configured)).toEqual([true, false]);
    expect(() => dispatchPlan(db, plan)).toThrow(/全部待执行批次|下一批/);
    expect(plan.lockedAt).toBeUndefined();
    expect(db.tasks).toEqual([]);
  });

  it("freezes all candidate configurations and creates only the first batch's tasks at start", () => {
    const { db, plan } = setup();
    db.batches[1].phase = "内容预检通过";
    dispatchPlan(db, plan);
    expect(db.batches.map(batch => batch.planFrozenAt)).toEqual([plan.lockedAt, plan.lockedAt]);
    expect(db.tasks.map(task => task.batchId)).toEqual([db.batches[0].id]);
    expect(db.batches[1].firstStartedAt).toBeUndefined();
  });
});

describe("CHG109 scenarios and original commands", () => {
  it("identifies both rows of a duplicated same-platform output before committing any start facts", () => {
    const db = seed();
    addPlanReviewScenarios(db);
    const plan = fixture(db, "DUPLICATE");
    const count = db.tasks.length;
    expect(() => dispatchPlan(db, plan)).toThrow(/批次 01 · 首批第1行与批次 02 · 下一批第1行使用同平台成片/);
    expect(plan.lockedAt).toBeUndefined();
    expect(db.tasks).toHaveLength(count);
  });

  it("keeps the existing threshold, excludes occupied but cancelled rows from the denominator, and preserves confirmed prefix results", () => {
    const db = seed();
    addPlanReviewScenarios(db);
    expect(view(db, "10")).toMatchObject({ control: "执行中", position: 1 });
    const failed = db.tasks.find(task => task.batchId === fixture(db, "10").items[0].id && task.status === "执行失败");
    expect(failed.planHandedOff).toBe(true);
    expect(canRetry(failed)).toBe(false);
    expect(view(db, "20")).toMatchObject({ control: "等待继续确认", position: 0 });
    planAction(db, "PL-109-20", "continue");
    expect(view(db, "20").position).toBe(1);
    expect(view(db, "ZERO")).toMatchObject({ control: "等待跳过确认", position: 0 });
    planAction(db, "PL-109-ZERO", "continue");
    expect(fixture(db, "ZERO").skipped).toEqual([fixture(db, "ZERO").items[0]]);
    expect(view(db, "PENDING")).toMatchObject({ control: "执行中", position: 0 });
    expect(() => planAction(db, "PL-109-PENDING", "continue")).toThrow(/未决任务/);
    expect(view(db, "PAUSED")).toMatchObject({ control: "已暂停", position: 0 });
    planAction(db, "PL-109-PAUSED", "resume");
    expect(view(db, "PAUSED").position).toBe(1);
  });

  it("rejects direct frozen candidate/settings edits and offers only detail for future batches", () => {
    const { db, plan } = setup();
    db.batches[1].phase = "内容预检通过";
    dispatchPlan(db, plan);
    const next = db.batches[1];
    expect(batchActions(db, next)).toEqual(["detail"]);
    for (const modify of [
      batch => { batch.candidates[0].title = "尝试修改"; },
      batch => { batch.executionSettings.concurrency = 6; },
    ]) {
      const projected = structuredClone(db);
      modify(projected.batches[1]);
      expect(() => protectPlanSnapshot(db, projected)).toThrow(/不能修改批次/);
    }
  });

  it("unblocks stopped verification only through the original attempt and still excludes reviewing from public progress", async () => {
    const db = seed();
    addPlanReviewScenarios(db);
    const check = fixture(db, "CHECK");
    const task = db.tasks.find(row => row.batchId === check.items[0].id);
    expect(view(db, "CHECK").batches[0].tasks[0].checkRequired).toBe(true);
    expect(check.position).toBe(0);
    const store = createBatchStore();
    await store.bootstrap(db);
    const result = await store.command({ batchId: task.batchId, revision: 1, key: "check-review",
      action: "taskOperation", input: { action: "verify", taskId: task.id, expectedStatus: "平台审核中",
        expectedAttempts: 1, attempt: 1, status: "平台审核中", evidence: "仍在平台审核" } });
    expect(view(result.data, "CHECK")).toMatchObject({ position: 1, canSettle: false });
    expect(view(result.data, "CHECK").batches[0].success).toBe(0);

    const publicPending = view(db, "PUBLIC");
    expect(publicPending).toMatchObject({ control: "执行已结束", position: 2, awaitingPublic: true, canSettle: false });
    expect(() => planAction(db, publicPending.id, "finish", "提前关闭")).toThrow(/未决结果/);
    const final = view(db, "FINISH");
    expect(final.canSettle).toBe(true);
    expect(() => planAction(db, final.id, "makeup", {})).toThrow(/不能新增/);
    expect(() => planAction(db, final.id, "finish")).toThrow(/未达成原因/);
    planAction(db, final.id, "finish", "已明确未公开，按记录结束");
    expect(view(db, "FINISH")).toMatchObject({ status: "ended", result: "unachieved" });
  });

  it("keeps the frozen next candidate when its original resource becomes unavailable", () => {
    const { db, plan } = setup();
    db.batches[1].phase = "内容预检通过";
    dispatchPlan(db, plan);
    const asset = db.assets.find(row => row.id === db.batches[1].candidates[0].assetId);
    asset.fileAvailable = false;
    setResult(db.tasks[0], "平台审核中", { evidence: "示例已投稿" });
    syncPlan(db, plan);
    expect(plan).toMatchObject({ control: "等待资源", position: 0 });
    expect(db.tasks).toHaveLength(1);
    expect(db.batches[1].candidates[0].assetId).toBe(asset.id);
    asset.fileAvailable = true;
    syncPlan(db, plan);
    expect(plan.position).toBe(1);
    expect(db.tasks[1].assetId).toBe(asset.id);
  });
});
