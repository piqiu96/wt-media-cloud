import { it, expect } from "vitest";
import { seed, defaults } from "../model.js";
import * as experience from "../taskExperience.js";
import { freezeBatch, protectBatchSnapshot } from "../batchLifecycle.js";

it("new content uses file-provided defaults, ignores retired browser settings and preserves saved content", () => {
  const db = seed(), asset = db.assets[0];
  db.publishingConfig = { "B站": { version: 99, tags: "旧页面配置", partition: "旧分区", visibility: "仅自己可见" } };
  const next = defaults("B站", asset, db);
  expect(next.tags).toBe(asset.category);
  expect(next.partition).toBe("游戏");
  expect(next.visibility).toBe("公开");
  expect(next.publishMethod).toBe("local_assisted");
  expect(next.publishingConfigVersion).toBe(1);
  expect(next.declaration).toBe("");
  const saved = { ...next, title: "已保存标题", visibility: "仅自己可见" };
  asset.title = "新成片标题";
  db.publishingConfig["B站"].tags = "另一份旧配置";
  expect(defaults("B站", asset, db).title).toBe("新成片标题");
  expect(saved.title).toBe("已保存标题");
  expect(saved.visibility).toBe("仅自己可见");
  expect(next.tags).toBe(asset.category);
});
it("platform defaults select the supported options without applying another platform's fields", () => {
  const db=seed(), asset=db.assets[1];
  expect(defaults("百家号",asset,db)).toMatchObject({description:asset.title,partition:"",visibility:"公开"});
  expect(defaults("小红书",asset,db)).toMatchObject({description:"",partition:"",topics:"",visibility:"公开"});
});

function lockedFixture() {
  const db = seed();
  db.tasks = [{ ...db.tasks[0], batchId: 1, claimed: false, batchStopped: false, status: "待执行" }];
  db.batches = [{ id: 1, name: "已启动批次", platform: db.tasks[0].platform, targets: [{category: db.tasks[0].category, count: 1}], phase: "内容预检通过", control: "执行中" }];
  freezeBatch(db, db.batches[0]);
  return db;
}
it("an unclaimed pending task remains content editable after its batch starts", () => {
  const db = lockedFixture();
  expect(experience.canEditTask(db.tasks[0], db)).toBe(true);
  const after = structuredClone(db);
  after.tasks[0].title = "修订后的内容";
  after.tasks[0].version++;
  expect(() => protectBatchSnapshot(db, after)).not.toThrow();
  expect(after.batches[0].frozenSnapshot).toEqual(db.batches[0].frozenSnapshot);
});
it("edits cannot change identity, an active attempt, or stopped batch content", () => {
  const db = lockedFixture();
  for (const key of ["accountId", "assetId", "platform", "category"]) {
    const after = structuredClone(db); after.tasks[0][key] = "changed";
    expect(() => protectBatchSnapshot(db, after)).toThrow(/映射|锁定/);
  }
  for (const status of ["排队中", "执行中", "已发布"]) {
    const before = structuredClone(db); before.tasks[0].status = status;
    const after = structuredClone(before); after.tasks[0].title = "changed";
    expect(experience.canEditTask(before.tasks[0], before)).toBe(false);
    expect(() => protectBatchSnapshot(before, after)).toThrow(/锁定/);
  }
  db.batches[0].stoppedAt = "now";
  expect(experience.canEditTask(db.tasks[0], db)).toBe(false);
});
it("a pending edit cannot simultaneously enqueue or claim the task", () => {
  const db = lockedFixture(), after = structuredClone(db);
  after.tasks[0].title = "changed";
  after.tasks[0].status = "排队中";
  expect(() => protectBatchSnapshot(db, after)).toThrow(/锁定/);
});
it("list and detail use executed content snapshots and the platform publication title", () => {
  expect(typeof experience.taskContent).toBe("function");
  const task = { platform: "百家号", status: "已发布", title: "来源标题", description: "当前内容", cover: "current", attemptHistory: [{snapshot: { description: "实际发布描述", cover: "published" }}] };
  expect(experience.taskTitle(task)).toBe("实际发布描述");
  expect(experience.taskContent(task).cover).toBe("published");
  task.status = "待执行";
  expect(experience.taskTitle(task)).toBe("当前内容");
});

import { createBatchStore } from "../../../publishing-preview.mjs";
it("preview persistence accepts a pending content revision and rejects stale or simultaneous claim writes", async () => {
  const store = createBatchStore();
  const initial = await store.bootstrap(lockedFixture());
  const frozen = structuredClone(initial.data.batches[0].frozenSnapshot);
  const draft = structuredClone(initial.data);
  draft.tasks[0].title = "服务端保存的修订";
  draft.tasks[0].version++;
  const saved = await store.save(draft, initial.revision);
  expect(saved.data.tasks[0].title).toBe("服务端保存的修订");
  expect(saved.data.batches[0].frozenSnapshot).toEqual(frozen);
  await expect(store.save(draft, initial.revision)).rejects.toThrow(/409/);
  const claimed = structuredClone(saved.data);
  claimed.tasks[0].claimed = true;
  claimed.tasks[0].title = "领取时不能同时改内容";
  await expect(store.save(claimed, saved.revision)).rejects.toThrow(/锁定/);
});
