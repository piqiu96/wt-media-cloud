import { runBatchCommand, batchState } from "../batchLifecycle.js";
import { describe, it, expect } from "vitest";
import { seed, defaults, declarationOptions, nextBatch } from "../model.js";
import {
  migrateRelationships,
  confirmPlan,
  planImpact,
  createPublication,
  planTasks,
  batchTasks,
  batchSummary,
  dispatchPlan,
  dispatchBatch,
  advance,
  syncPlan,
  cancelPlan,
  cancelBatchSafely,
  claimTask,
  setResult,
  queueTask,
  canLink,
  decision,
  clone,
} from "../relationships.js";
const fresh = () => {
  const db = migrateRelationships(seed());
  db.tasks = [];
  db.batches = [];
  db.plans = [];
  return db;
};
const row = (db, platform, id) => {
  const a = db.assets.find((a) => a.id === id);
  return {
    platform,
    assetId: id,
    category: a.category,
    accountId: db.accounts.find(
      (x) =>
        x.enabled &&
        x.platform === platform &&
        x.categories.includes(a.category),
    ).id,
    ...defaults(platform, a),
    declaration: declarationOptions(platform)[0].value,
  };
};
const draft = (counts = [1, 1]) => ({
  name: "百度双批",
  confirmKey: "confirm-once",
  platformOrder: ["百家号"],
  targets: [
    {
      platform: "百家号",
      category: "三角洲行动",
      count: counts.reduce((a, b) => a + b, 0),
    },
  ],
  splits: counts.map((count, i) => ({
    name: "百度 " + String.fromCharCode(65 + i),
    platform: "百家号",
    targets: [{ category: "三角洲行动", count }],
  })),
});
const prepare = (db, counts = [1, 1]) => {
  const d = draft(counts),
    p = confirmPlan(db, d);
  p.items.forEach((i, n) => {
    const b = db.batches.find((b) => b.id === i.id);
    createPublication(db, {
      platform: b.platform,
      name: b.name,
      batchId: b.id,
      planId: p.id,
      rows: [row(db, b.platform, 128 + n * 2)],
    });
  });
  return p;
};
describe("D02 D03 D04 plan confirmation and safe editing", () => {
  it("confirms one atomic skeleton set, retries idempotently, creates no tasks or occupation", () => {
    const db = fresh(),
      d = draft();
    const p = confirmPlan(db, d);
    expect(confirmPlan(db, d).id).toBe(p.id);
    expect(db.batches).toHaveLength(2);
    expect(db.tasks).toHaveLength(0);
    expect(db.batches.every((b) => b.phase === "待资源配置")).toBe(true);
  });
  it("rejects a mismatched matrix without partial write", () => {
    const db = fresh(),
      d = draft();
    d.splits[1].targets[0].count = 10;
    expect(() => confirmPlan(db, d)).toThrow("不一致");
    expect(db.plans).toHaveLength(0);
    expect(db.batches).toHaveLength(0);
  });
  it("saves draft without skeletons, then confirms exact same plan", () => {
    const db = fresh(),
      d = draft();
    const p = confirmPlan(db, d, { draftOnly: true });
    expect(db.batches).toHaveLength(0);
    d.id = p.id;
    d.revision = p.revision;
    confirmPlan(db, d);
    expect(db.plans).toHaveLength(1);
    expect(db.batches).toHaveLength(2);
  });
  it("keeps candidates through safe edits; finalized identities and targets are protected", () => {
    const db = fresh(),
      d = draft(),
      p = confirmPlan(db, d),
      b = db.batches[0];
    b.candidates = [{ title: "人工保留", assetId: 128 }];
    const edit = {
      ...clone(d),
      id: p.id,
      revision: p.revision,
      splits: db.batches.map((b) => ({
        id: b.id,
        name: b.name,
        platform: b.platform,
        targets: clone(b.targets),
      })),
    };
    edit.splits[0].targets[0].count = 2;
    edit.targets[0].count = 3;
    expect(planImpact(db, p, edit)[0].changed).toBe(true);
    confirmPlan(db, edit);
    expect(db.batches[0].candidates[0].title).toBe("人工保留");
    const b2 = db.batches[1];
    createPublication(db, {
      name: b2.name,
      platform: b2.platform,
      batchId: b2.id,
      planId: p.id,
      rows: [row(db, "百家号", 130)],
    });
    edit.splits[1].targets[0].count = 2;
    edit.targets[0].count = 4;
    expect(() => planImpact(db, p, edit)).toThrow("已有正式任务");
  });
  it("quick tasks never belong to a plan or contribute to its projection", () => {
    const db = fresh(),
      p = confirmPlan(db, draft());
    expect(() =>
      createPublication(db, {
        quick: true,
        planId: p.id,
        platform: "百家号",
        rows: [row(db, "百家号", 128)],
      }),
    ).toThrow("计划外");
    createPublication(db, {
      quick: true,
      platform: "百家号",
      rows: [row(db, "百家号", 128)],
    });
    expect(planTasks(db, p)).toEqual([]);
    expect(db.tasks[0].planId).toBeUndefined();
    expect(canLink(db, p, { kind: "task", id: db.tasks[0].id })).toBe(false);
  });
});
describe("D05 D07 D09 D10 D11 dispatch and cancellation", () => {
  it("rejects an unready first batch without obtaining a lock", () => {
    const db = fresh(),
      p = confirmPlan(db, draft());
    expect(() => dispatchPlan(db, p)).toThrow("未就绪");
    expect(p.lockedAt).toBeUndefined();
    expect(p.control).toBe("规划中");
  });
  it("first legal start locks immediately, duplicate start does not duplicate attempts, pause never unlocks", () => {
    const db = fresh(),
      p = prepare(db);
    dispatchPlan(db, p);
    expect(p.lockedAt).toBeTruthy();
    dispatchPlan(db, p);
    expect(db.tasks[0].attempts).toBe(1);
    p.control = "已暂停";
    expect(() => planImpact(db, p, draft())).toThrow("永久锁定");
    db.tasks[0].status = "已发布";
    syncPlan(db, p);
    expect(p.position).toBe(0);
  });
  it("requires explicit takeover and never silently starts a later batch", () => {
    const db = fresh(),
      p = prepare(db);
    expect(() => dispatchBatch(db, db.batches[0])).toThrow("人工接管");
    expect(() => dispatchBatch(db, db.batches[1], { takeover: true })).toThrow(
      "前序",
    );
    dispatchBatch(db, db.batches[0], { takeover: true });
    expect(p.lockedAt).toBeTruthy();
    expect(p.control).toBe("已暂停");
    expect(p.log.some((x) => x.type === "人工接管")).toBe(true);
  });
  it.each(["待执行", "排队中", "执行中", "等待人工", "结果待确认"])(
    "all unresolved states block advance: %s",
    (status) => {
      const db = fresh(),
        p = prepare(db);
      dispatchPlan(db, p);
      setResult(db.tasks[0], status);
      expect(decision(db, p)).toBe("等待核实");
      expect(() => advance(db, p, true)).toThrow("未决");
      expect(p.position).toBe(0);
    },
  );
  it("next ready does not dispatch when paused or cancelled", () => {
    const db = fresh(),
      p = prepare(db);
    dispatchPlan(db, p);
    p.control = "已暂停";
    setResult(db.tasks[0], "已发布");
    syncPlan(db, p);
    expect(db.tasks[1].attempts).toBe(0);
    cancelPlan(p, db);
    expect(() => dispatchPlan(db, p)).toThrow("终结");
    expect(() => advance(db, p, true)).toThrow();
    expect(db.tasks[1].attempts).toBe(0);
  });
  it("rechecks next readiness without moving the cursor or dispatching", () => {
    const db = fresh(),
      p = confirmPlan(db, draft()),
      b = db.batches[0];
    createPublication(db, {
      name: b.name,
      platform: b.platform,
      batchId: b.id,
      planId: p.id,
      rows: [row(db, "百家号", 128)],
    });
    dispatchPlan(db, p);
    setResult(db.tasks[0], "已发布");
    expect(() => syncPlan(db, p)).toThrow("未就绪");
    expect(p.position).toBe(0);
    expect(p.control).toBe("等待配置");
  });
  it("cancellation races with claiming: only unclaimed work cancels, unknown/inflight safely finish", () => {
    const db = fresh();
    const { batch, tasks } = createPublication(db, {
      name: "竞态",
      platform: "百家号",
      rows: [128, 130, 129].map((id) => row(db, "百家号", id)),
    });
    dispatchBatch(db, batch);
    expect(claimTask(db, tasks[0])).toBe(true);
    setResult(tasks[0], "结果待确认");
    expect(() => cancelBatchSafely(db, batch)).toThrow("首次执行后");
    runBatchCommand(db, batch, "stop");
    expect(tasks.map((t) => t.status)).toEqual([
      "结果待确认",
      "已取消",
      "已取消",
    ]);
    expect(claimTask(db, tasks[1])).toBe(false);
    expect(() => dispatchBatch(db, batch)).toThrow("终结");
    expect(tasks.every((t) => t.status !== "已放弃")).toBe(true);
  });
  it("completed execution cannot be cancelled and keeps the published result", () => {
    const db = fresh(), p = prepare(db);
    dispatchPlan(db, p);
    setResult(db.tasks[0], "已发布");
    expect(() => cancelBatchSafely(db, db.batches[0])).toThrow("首次执行后");
    expect(batchState(db, db.batches[0])).toBe("已结束");
    syncPlan(db, p);
    expect(p.position).toBe(1);
  });
  it("unknown/login block only their account while other accounts run; concurrency bounded at 3", () => {
    const db = fresh();
    const { batch, tasks } = createPublication(db, {
      name: "隔离",
      platform: "百家号",
      rows: [128, 130, 129].map((id) => row(db, "百家号", id)),
    });
    dispatchBatch(db, batch);
    expect(claimTask(db, tasks[0])).toBe(true);
    expect(claimTask(db, tasks[1])).toBe(false);
    setResult(tasks[0], "等待人工");
    expect(claimTask(db, tasks[1])).toBe(false);
    expect(claimTask(db, tasks[2])).toBe(true);
  });
  it("plan cancellation cancels unstarted tasks and preserves identities and reservations", () => {
    const db = fresh(),
      p = prepare(db);
    dispatchPlan(db, p);
    const before = clone(db.tasks);
    cancelPlan(p, db);
    expect(db.tasks.every(t => t.status === '已取消')).toBe(true);
    expect(db.tasks.map(t => [t.id,t.assetId,t.platform,t.attempts])).toEqual(before.map(t => [t.id,t.assetId,t.platform,t.attempts]));
    expect(p.control).toBe("已取消");
  });
  it("scheduled future tasks cannot be claimed", () => {
    const db = fresh();
    const { batch, tasks } = createPublication(db, {
      name: "定时",
      platform: "百家号",
      rows: [
        { ...row(db, "百家号", 128), schedule: "2099-01-01T12:00:00+08:00" },
      ],
    });
    dispatchBatch(db, batch);
    expect(claimTask(db, tasks[0])).toBe(false);
    expect(nextBatch(tasks)).toBe("等待核实");
  });
});
describe("D06 D08 D15 creation and result boundaries", () => {
  it("preserves 30 target / 28 actual gap, all 28 public closes 28/28 and permits advance", () => {
    const db = fresh();
    db.assets = Array.from({ length: 30 }, (_, i) => ({
      ...db.assets[0],
      id: 500 + i,
    }));
    const d = draft([30, 1]);
    const p = confirmPlan(db, d),
      b = db.batches[0];
    const rows = db.assets.slice(0, 28).map((a) => row(db, "百家号", a.id));
    expect(() =>
      createPublication(db, {
        name: b.name,
        platform: b.platform,
        batchId: b.id,
        planId: p.id,
        rows,
      }),
    ).toThrow("部分创建");
    createPublication(db, {
      name: b.name,
      platform: b.platform,
      batchId: b.id,
      planId: p.id,
      rows,
      partial: true,
    });
    expect(batchSummary(db, b)).toMatchObject({
      target: 30,
      total: 28,
      creationGap: 2,
    });
    db.tasks.forEach((t) => setResult(t, "已发布"));
    expect(batchSummary(db, b)).toMatchObject({
      closed: 28,
      published: 28,
      target: 30,
    });
    expect(nextBatch(db.tasks)).toBe("自动继续");
    expect(db.tasks).toHaveLength(28);
  });
  it("same platform uniqueness is atomic; cancelled reserves; abandoned releases; cross-platform allowed", () => {
    const db = fresh();
    const create = (platform) =>
      createPublication(db, {
        quick: true,
        platform,
        rows: [row(db, platform, 128)],
      });
    create("百家号");
    expect(() => create("百家号")).toThrow("占用");
    db.tasks[0].status = "已取消";
    expect(() => create("百家号")).toThrow("占用");
    db.tasks[0].status = "已放弃";
    create("百家号");
    create("B站");
    expect(db.tasks).toHaveLength(3);
  });
  it("mixed batch and category identity mismatch fail before mutation", () => {
    const db = fresh();
    expect(() =>
      createPublication(db, {
        name: "错误",
        platform: "百家号",
        rows: [row(db, "百家号", 128), row(db, "B站", 129)],
      }),
    ).toThrow("全部属于");
    expect(() =>
      createPublication(db, {
        name: "错误",
        platform: "百家号",
        rows: [{ ...row(db, "百家号", 128), category: "火影忍者" }],
      }),
    ).toThrow("类目必须");
    expect(db.tasks).toHaveLength(0);
  });
  it("exactly 10% advances, over 10% needs confirmation, no denominator needs skip", () => {
    const published = () => ({ status: "已发布" }),
      failed = () => ({ status: "执行失败", result: "确认未发布" });
    expect(nextBatch([...Array.from({ length: 9 }, published), failed()])).toBe(
      "自动继续",
    );
    expect(
      nextBatch([...Array.from({ length: 8 }, published), failed(), failed()]),
    ).toBe("等待继续确认");
    expect(nextBatch([{ status: "已取消" }])).toBe("等待跳过确认");
  });
  it("late audit changes update facts without rolling back or advancing twice", () => {
    const db = fresh(),
      p = prepare(db);
    dispatchPlan(db, p);
    setResult(db.tasks[0], "平台审核中");
    syncPlan(db, p);
    expect(p.position).toBe(1);
    setResult(db.tasks[0], "执行失败");
    syncPlan(db, p);
    expect(p.position).toBe(1);
    expect(db.tasks[1].attempts).toBe(1);
  });
  it("retry appends immutable attempt snapshots; does not reactivate cancelled batch", () => {
    const db = fresh();
    const { batch, tasks } = createPublication(db, {
      name: "重试",
      platform: "百家号",
      rows: [row(db, "百家号", 128)],
    });
    dispatchBatch(db, batch);
    const t = tasks[0],
      before = clone(t.attemptHistory[0].snapshot);
    setResult(t, "执行失败");
    expect(() => cancelBatchSafely(db, batch)).toThrow("首次执行后");
    t.batchStopped = true;
    batch.stoppedAt = new Date().toISOString();
    expect(() => queueTask(db, t, { retry: true })).toThrow("停止");
    expect(t.attempts).toBe(1);
    expect(t.attemptHistory[0].snapshot).toEqual(before);
    expect(batch.control).toBe("执行中");
  });
  it("migrates old standalone refs out of plans without deleting historical tasks", () => {
    const db = seed();
    db.tasks[0].planId = 301;
    db.plans[0].items.push({ kind: "task", id: db.tasks[0].id });
    migrateRelationships(db);
    expect(db.plans[0].items.every((i) => i.kind === "batch")).toBe(true);
    expect(db.tasks[0].planId).toBeUndefined();
    const saved = JSON.stringify(db);
    migrateRelationships(db);
    expect(JSON.stringify(db)).toBe(saved);
  });
});
it("D05 failed startup preflight must not acquire a plan lock or mutate queue", () => {
  const db = fresh(),
    p = prepare(db);
  db.tasks[0].declaration = "";
  expect(() => dispatchPlan(db, p)).toThrow("声明");
  expect(p.lockedAt).toBeUndefined();
  expect(db.tasks[0].attempts).toBe(0);
  expect(() => dispatchBatch(db, db.batches[0], { takeover: true })).toThrow();
  expect(p.lockedAt).toBeUndefined();
});
it("D07 next-batch preflight failure preserves the old cursor and no attempt", () => {
  const db = fresh(),
    p = prepare(db);
  dispatchPlan(db, p);
  setResult(db.tasks[0], "已发布");
  db.tasks[1].declaration = "";
  expect(() => syncPlan(db, p)).toThrow("声明");
  expect(p.position).toBe(0);
  expect(db.tasks[1].attempts).toBe(0);
  expect(p.control).toBe("等待配置");
});
it("D07 resume after a paused current batch has closed can continue safely", () => {
  const db = fresh(),
    p = prepare(db);
  dispatchPlan(db, p);
  p.control = "已暂停";
  setResult(db.tasks[0], "已发布");
  db.batches[0].control = "执行已结束";
  dispatchPlan(db, p);
  expect(p.position).toBe(1);
  expect(db.tasks[1].attempts).toBe(1);
});
it("D04 confirmed plan cannot be downgraded to an empty draft", () => {
  const db = fresh(),
    p = confirmPlan(db, draft());
  expect(() =>
    confirmPlan(db, { ...draft(), id: p.id }, { draftOnly: true }),
  ).toThrow("退回");
  expect(db.batches).toHaveLength(2);
});
it("D09 a newly invalid login blocks its account without claiming or affecting peers", () => {
  const db = fresh();
  const { batch, tasks } = createPublication(db, {
    name: "登录隔离",
    platform: "百家号",
    rows: [128, 129].map((id) => row(db, "百家号", id)),
  });
  dispatchBatch(db, batch);
  db.accounts.find((a) => a.id === tasks[0].accountId).loginValid = false;
  expect(claimTask(db, tasks[0])).toBe(false);
  expect(tasks[0].status).toBe("等待人工");
  expect(claimTask(db, tasks[1])).toBe(true);
});
it("quick form with an empty batch target array still creates exactly one standalone task", () => {
  const db = fresh();
  const result = createPublication(db, {
    platform: "小红书",
    rows: [row(db, "小红书", 136)],
    quick: true,
    targets: [],
  });
  expect(result.batch).toBeNull();
  expect(result.tasks).toHaveLength(1);
  expect(result.tasks[0].batchId).toBeNull();
  expect(result.tasks[0].status).toBe("待执行");
});
it("D02 missing legacy batch references never capture standalone tasks", () => {
  const db = fresh();
  db.tasks = [{ id: 12, status: "待执行" }];
  expect(planTasks(db, { items: [{ kind: "batch", id: 999 }] })).toEqual([]);
});
it("D09 platform fault blocks only new claims for that platform", () => {
  const db = fresh();
  const a = createPublication(db, {
    name: "百度",
    platform: "百家号",
    rows: [row(db, "百家号", 128)],
  });
  const b = createPublication(db, {
    name: "B站",
    platform: "B站",
    rows: [row(db, "B站", 128)],
  });
  dispatchBatch(db, a.batch);
  dispatchBatch(db, b.batch);
  db.platformBlocks = { 百家号: true };
  expect(claimTask(db, a.tasks[0])).toBe(false);
  expect(claimTask(db, b.tasks[0])).toBe(true);
});

it("D03 incomplete plan drafts can be repeatedly saved without skeletons", () => {
  const db = fresh(),
    d = {
      name: "草稿",
      confirmKey: "incomplete",
      targets: [],
      splits: [],
      platformOrder: [],
    };
  const p = confirmPlan(db, d, { draftOnly: true });
  d.id = p.id;
  d.name = "草稿二次保存";
  expect(confirmPlan(db, d, { draftOnly: true }).name).toBe(d.name);
  expect(db.plans).toHaveLength(1);
  expect(db.batches).toHaveLength(0);
  expect(db.tasks).toHaveLength(0);
});
it("D04 zero-target categories and target ordering do not falsely modify a finalized sibling batch", () => {
  const db = fresh(),
    p = prepare(db),
    d = draft();
  d.id = p.id;
  d.revision = p.revision;
  d.splits.forEach((s, i) => {
    s.id = p.items[i].id;
    s.targets.unshift({ category: "火影忍者", count: 0 });
  });
  expect(planImpact(db, p, d).every((c) => !c.changed)).toBe(true);
  expect(() => confirmPlan(db, d)).not.toThrow();
  expect(db.tasks).toHaveLength(2);
});

it('copied creation preserves provenance without attaching to or starting the source plan',()=>{
 const db=fresh(),input={platform:'B站',rows:[row(db,'B站',128)],name:'副本',targets:[{category:'三角洲行动',count:1}],sourceBatchId:203,sourcePlanId:999,creationKey:'copy-create-once'};
 const result=createPublication(db,input);
 expect(result.batch).toMatchObject({sourceBatchId:203,sourcePlanId:999,planId:null,control:'待执行'});
 expect(result.batch.firstStartedAt).toBeUndefined();
 expect(createPublication(db,input).batch.id).toBe(result.batch.id);
 expect(db.batches).toHaveLength(1);expect(db.tasks).toHaveLength(1);
});
