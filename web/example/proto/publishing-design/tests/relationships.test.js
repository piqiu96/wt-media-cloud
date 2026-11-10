import { describe, it, expect } from "vitest";
import { seed, defaults, declarationOptions, nextBatch } from "../model.js";
import {
  migrateRelationships,
  createPublication,
  validateBatch,
  batchTasks,
  planTasks,
  batchSummary,
  linkItem,
  canLink,
  moveItem,
  dispatchPlan,
  dispatchBatch,
  syncPlan,
  advance,
} from "../relationships.js";
const fresh = () => {
  const db = migrateRelationships(seed());
  db.tasks = [];
  db.batches = [];
  db.plans = [];
  return db;
};
const row = (db, platform, assetId) => {
  const a = db.assets.find((a) => a.id === assetId);
  return {
    platform,
    assetId,
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
const plan = (db) => {
  const p = {
    id: 301,
    name: "计划",
    targets: ["百家号", "小红书", "B站"].flatMap((platform) =>
      ["三角洲行动", "火影忍者"].map((category) => ({
        platform,
        category,
        count: 5,
      })),
    ),
    items: [],
    skipped: [],
    position: 0,
    control: "规划中",
  };
  db.plans.push(p);
  return p;
};
const create = (db, name, platform, ids, planId) =>
  createPublication(db, {
    name,
    platform,
    rows: ids.map((id) => row(db, platform, id)),
    planId,
  });
describe("single-platform batches and cross-platform plans", () => {
  it("rejects mixed platforms and foreign accounts atomically", () => {
    const db = fresh();
    expect(() =>
      createPublication(db, {
        platform: "百家号",
        name: "混合",
        rows: [row(db, "百家号", 128), row(db, "B站", 129)],
      }),
    ).toThrow("全部属于当前平台");
    const r = row(db, "百家号", 128);
    r.accountId = 11;
    expect(validateBatch(db, "百家号", [r])).toContain(
      "请选择可用的同平台账号",
    );
    const wrongCategory = { ...row(db, "百家号", 128), category: "火影忍者" };
    expect(() =>
      createPublication(db, {
        platform: "百家号",
        name: "错误类目",
        rows: [wrongCategory],
      }),
    ).toThrow("任务类目必须与成片类目一致");
    expect(() =>
      createPublication(db, {
        platform: "百家号",
        quick: true,
        rows: [row(db, "百家号", 128), row(db, "百家号", 129)],
      }),
    ).toThrow("快速发布只能创建一条独立任务");
    expect(db.tasks).toHaveLength(0);
    expect(db.batches).toHaveLength(0);
  });
  it("creates a single-platform multi-category batch without dispatch", () => {
    const db = fresh();
    const { batch, tasks } = create(db, "百度 A", "百家号", [128, 129]);
    expect(batch.platform).toBe("百家号");
    expect(batch.platforms).toBeUndefined();
    expect(batch.categories).toHaveLength(2);
    expect(new Set(tasks.map((t) => t.accountId)).size).toBe(2);
    expect(
      tasks.every(
        (t) =>
          t.batchId === batch.id && t.status === "待执行" && t.attempts === 0,
      ),
    ).toBe(true);
  });
  it("links same-platform A/B and other platforms without merging; plan save occupies no resources", () => {
    const db = fresh(),
      p = plan(db);
    expect(db.tasks).toHaveLength(0);
    const a = create(db, "百度 A", "百家号", [128, 129]),
      b = create(db, "百度 B", "百家号", [130]);
    linkItem(db, p, { kind: "batch", id: a.batch.id });
    linkItem(db, p, { kind: "batch", id: b.batch.id });
    create(db, "小红书 A", "小红书", [128], p.id);
    create(db, "B站 A", "B站", [129], p.id);
    expect(p.items).toHaveLength(4);
    expect(planTasks(db, p)).toHaveLength(5);
    expect(batchTasks(db, a.batch)).toHaveLength(2);
    expect(batchTasks(db, b.batch)).toHaveLength(1);
    expect(canLink(db, p, { kind: "batch", id: a.batch.id })).toBe(false);
  });
  it("reserves across same-platform batches while other platforms remain independent", () => {
    const db = fresh();
    create(db, "A", "百家号", [128]);
    expect(() => create(db, "B", "百家号", [128])).toThrow("同平台任务占用");
    expect(() => create(db, "X", "小红书", [128])).not.toThrow();
    db.tasks[0].status = "已取消";
    expect(() => create(db, "B", "百家号", [128])).toThrow();
    db.tasks[0].status = "已放弃";
    expect(() => create(db, "B", "百家号", [128])).not.toThrow();
  });
  it("rejects missing account, empty assets and wrong plan scope without partial creation", () => {
    const db = fresh(),
      p = plan(db);
    const r = row(db, "百家号", 128);
    r.accountId = null;
    expect(() =>
      createPublication(db, { platform: "百家号", rows: [r], name: "空账号" }),
    ).toThrow();
    expect(() =>
      createPublication(db, { platform: "百家号", rows: [], name: "空成片" }),
    ).toThrow();
    p.targets = [];
    expect(() => create(db, "范围错误", "百家号", [128], p.id)).toThrow(
      "不符合计划目标",
    );
    expect(db.tasks).toHaveLength(0);
  });
  it("isolates batch stats and aggregates explicit batch and standalone task refs", () => {
    const db = fresh(),
      p = plan(db);
    const a = create(db, "A", "百家号", [128, 129], p.id),
      b = create(db, "B", "百家号", [130], p.id);
    a.tasks[0].status = "已发布";
    a.tasks[1].status = "执行失败";
    a.tasks[1].result = "确认未发布";
    const quick = createPublication(db, {
      platform: "B站",
      rows: [row(db, "B站", 129)],
      quick: true,
      planId: p.id,
    });
    expect(quick.batch).toBeNull();
    expect(batchSummary(db, a.batch)).toMatchObject({
      total: 2,
      published: 1,
      failed: 1,
    });
    expect(batchSummary(db, b.batch)).toMatchObject({
      total: 1,
      published: 0,
      failed: 0,
    });
    expect(planTasks(db, p)).toHaveLength(4);
    expect(p.items.at(-1).kind).toBe("task");
  });
  it("orders batches by identity and prevents duplicate plan ownership", () => {
    const db = fresh(),
      p = plan(db);
    const a = create(db, "A", "百家号", [128], p.id),
      b = create(db, "B", "百家号", [130], p.id);
    expect(moveItem(p, 1, -1)).toBe(true);
    expect(p.items[0].id).toBe(b.batch.id);
    const p2 = { ...p, id: 302, items: [] };
    db.plans.push(p2);
    expect(() => linkItem(db, p2, { kind: "batch", id: a.batch.id })).toThrow();
    p.control = "执行中";
    expect(moveItem(p, 1, -1)).toBe(false);
  });
  it("dispatches in order; pause blocks advance and resume never resends completed work", () => {
    const db = fresh(),
      p = plan(db);
    const a = create(db, "A", "百家号", [128], p.id),
      b = create(db, "B", "百家号", [130], p.id);
    dispatchPlan(db, p);
    expect(a.tasks[0].attempts).toBe(1);
    expect(b.tasks[0].attempts).toBe(0);
    p.control = "已暂停";
    a.tasks[0].status = "已发布";
    syncPlan(db, p);
    expect(p.position).toBe(0);
    dispatchPlan(db, p);
    expect(p.position).toBe(1);
    expect(a.tasks[0].attempts).toBe(1);
    expect(b.tasks[0].attempts).toBe(1);
    dispatchPlan(db, p);
    expect(b.tasks[0].attempts).toBe(1);
  });
  it("uses current batch ratio; unknown blocks; confirmed continuation preserves failed work", () => {
    const db = fresh(),
      p = plan(db);
    const a = create(db, "A", "百家号", [128, 129], p.id),
      b = create(db, "B", "百家号", [130], p.id);
    dispatchPlan(db, p);
    a.tasks[0].status = "已发布";
    a.tasks[1].status = "结果待确认";
    syncPlan(db, p);
    expect(p.position).toBe(0);
    expect(nextBatch(a.tasks)).toBe("等待核实");
    a.tasks[1].status = "执行失败";
    a.tasks[1].result = "确认未发布";
    syncPlan(db, p);
    expect(p.control).toBe("等待继续确认");
    expect(b.tasks[0].attempts).toBe(0);
    advance(db, p, true);
    expect(p.position).toBe(1);
    expect(a.tasks[1].attempts).toBe(1);
    expect(a.tasks[1].status).toBe("执行失败");
    expect(b.tasks[0].status).toBe("排队中");
  });
  it("batch pause leaves other batches and queued tasks intact", () => {
    const db = fresh();
    const a = create(db, "A", "百家号", [128]),
      b = create(db, "B", "百家号", [130]);
    dispatchBatch(db, a.batch);
    dispatchBatch(db, b.batch);
    a.batch.control = "已暂停";
    expect(b.batch.control).toBe("执行中");
    expect(a.tasks[0].status).toBe("排队中");
    dispatchBatch(db, a.batch);
    expect(a.tasks[0].attempts).toBe(1);
  });
  it("migrates by observed platform without rewriting task identity/results; expands plan refs once", () => {
    const db = fresh(),
      p = plan(db);
    const a = create(db, "旧多平台", "百家号", [128, 129], p.id);
    a.tasks[1].platform = "B站";
    a.tasks[1].status = "已发布";
    a.tasks[1].log = [{ text: "旧记录" }];
    a.batch.platform = "百家号、B站";
    a.batch.platforms = ["百家号", "B站"];
    p.items = [a.batch.id];
    delete db.relationshipVersion;
    const taskId = a.tasks[1].id;
    migrateRelationships(db);
    expect(db.batches).toHaveLength(2);
    expect(p.items).toHaveLength(2);
    expect(db.tasks.find((t) => t.id === taskId)).toMatchObject({
      platform: "B站",
      status: "已发布",
      log: [{ text: "旧记录" }],
    });
    expect(
      db.batches.every((b) =>
        batchTasks(db, b).every((t) => t.platform === b.platform),
      ),
    ).toBe(true);
    const state = JSON.stringify(db);
    migrateRelationships(db);
    expect(JSON.stringify(db)).toBe(state);
  });
  it("missing batch refs never absorb standalone tasks", () => {
    const db = fresh(),
      p = plan(db);
    createPublication(db, {
      platform: "百家号",
      rows: [row(db, "百家号", 128)],
      quick: true,
    });
    p.items = [{ kind: "batch", id: 9999 }];
    expect(planTasks(db, p)).toEqual([]);
  });
  it("resuming a plan cannot dispatch while a different linked batch runs independently", () => {
    const db = fresh(),
      p = plan(db);
    create(db, "A", "百家号", [128], p.id);
    const b = create(db, "B", "百家号", [130], p.id);
    dispatchBatch(db, b.batch);
    expect(() => dispatchPlan(db, p)).toThrow("独立执行");
    expect(db.tasks[0].attempts).toBe(0);
  });
});
