import { describe, it, expect } from "vitest";
import {
  batchMetrics,
  batchState,
  batchActions,
  contentEditable,
  runBatchCommand,
  protectBatchSnapshot,
  batchTaskActions,
} from "../batchLifecycle.js";
const fixture = () => ({
  batches: [
    {
      id: 1,
      name: "测试",
      platform: "B站",
      targets: [{ category: "游戏", count: 4 }],
      phase: "内容预检通过",
      control: "待执行",
      version: 1,
    },
  ],
  tasks: [
    {
      id: 11,
      batchId: 1,
      status: "待执行",
      title: "原内容",
      accountId: 2,
      assetId: 3,
      platform: "B站",
      category: "游戏",
      attempts: 0,
    },
  ],
  accounts: [],
  assets: [],
  plans: [],
});
it("失败缺少安全证据时先核实，不提供重试或放弃", () => {
  const d = fixture(),
    b = d.batches[0],
    t = d.tasks[0];
  t.status = "执行失败";
  t.result = "未知";
  expect(batchTaskActions(d, b, t)).toEqual(["verify"]);
  t.result = "确认未发布";
  expect(batchTaskActions(d, b, t)).toEqual(["verify"]);
});
describe("批次 V1.2 生命周期", () => {
  it("失败、未知和待补链接不算处置", () => {
    const d = fixture();
    d.tasks = [
      "已发布",
      "已取消",
      "已放弃",
      "执行失败",
      "结果待确认",
      "平台审核中",
    ].map((status, id) => ({ id, batchId: 1, status }));
    expect(batchMetrics(d, d.batches[0])).toMatchObject({
      disposed: 3,
      total: 6,
      published: 1,
      pending: 3,
      achievement: 25,
      progress: 50,
    });
  });
  it("只提供六个主状态与终态结果", () => {
    const d = fixture(),
      b = d.batches[0];
    expect(batchState(d, b)).toBe("待执行");
    b.firstStartedAt = "now";
    d.tasks[0].status = "执行失败";
    expect(batchState(d, b)).toBe("待处理");
    d.tasks[0].status = "已取消";
    expect(batchState(d, b)).toBe("已结束");
    expect(batchMetrics(d, b).result).toBe("未发布");
  });
  it("首次接受执行后全部内容永久锁定", () => {
    const d = fixture(),
      b = d.batches[0];
    runBatchCommand(d, b, "execute", { partial: true }, () => []);
    expect(contentEditable(d, b)).toBe(false);
    expect(() =>
      runBatchCommand(d, b, "saveContent", {
        rows: [{ ...d.tasks[0], title: "篡改" }],
      }),
    ).toThrow(/锁定/);
  });
  it("全部阻塞不锁定，部分阻塞需明确确认", () => {
    const d = fixture(),
      b = d.batches[0];
    expect(() =>
      runBatchCommand(d, b, "execute", {}, () => ["资源不可用"]),
    ).toThrow(/没有可执行/);
    expect(b.firstStartedAt).toBeUndefined();
    d.tasks.push({ ...d.tasks[0], id: 12 });
    expect(() =>
      runBatchCommand(d, b, "execute", {}, (t) =>
        t.id === 11 ? ["不可用"] : [],
      ),
    ).toThrow(/确认/);
    expect(b.firstStartedAt).toBeUndefined();
    runBatchCommand(d, b, "execute", { partial: true }, (t) =>
      t.id === 11 ? ["不可用"] : [],
    );
    expect(d.tasks.map((t) => t.status)).toEqual(["待执行", "排队中"]);
  });
  it("停止只取消未启动任务，保留在途和成功，禁止重启及取消批次", () => {
    const d = fixture(),
      b = d.batches[0];
    b.firstStartedAt = "now";
    b.control = "执行中";
    d.tasks = ["待执行", "排队中", "执行中", "已发布", "结果待确认"].map(
      (status, id) => ({
        id,
        batchId: 1,
        status,
        claimed: status === "执行中",
      }),
    );
    runBatchCommand(d, b, "stop");
    expect(d.tasks.map((t) => t.status)).toEqual([
      "已取消",
      "已取消",
      "执行中",
      "已发布",
      "结果待确认",
    ]);
    expect(batchState(d, b)).toBe("执行中");
    expect(() => runBatchCommand(d, b, "execute")).toThrow();
    expect(() => runBatchCommand(d, b, "cancel")).toThrow();
  });
  it("终态复制不带映射和执行记录", () => {
    const d = fixture(),
      b = d.batches[0];
    expect(() => runBatchCommand(d, b, "copy", { name: "副本" })).toThrow();
    runBatchCommand(d, b, "cancel");
    const copy = runBatchCommand(d, b, "copy", { name: "副本" });
    expect(copy.candidates).toEqual([]);
    expect(d.tasks.filter((t) => t.batchId === copy.id)).toHaveLength(0);
    expect(batchState(d, copy)).toBe("草稿");
  });
  it("拒绝映射篡改、首次执行后的旧窗口保存和版本冲突", () => {
    const d = fixture(),
      b = d.batches[0];
    expect(() =>
      runBatchCommand(d, b, "saveContent", {
        rows: [{ ...d.tasks[0], assetId: 99 }],
      }),
    ).toThrow(/资源/);
    expect(() =>
      runBatchCommand(d, b, "cancel", { expectedVersion: 0 }),
    ).toThrow(/变化/);
    runBatchCommand(d, b, "execute", {}, () => []);
    const next = structuredClone(d);
    next.tasks[0].title = "改";
    expect(() => protectBatchSnapshot(d, next)).toThrow(/锁定/);
  });
  it("状态操作不含暂停恢复，最多五个平铺", () => {
    const d = fixture(),
      b = d.batches[0];
    expect(batchActions(d, b)).toEqual(["execute", "edit", "detail", "cancel"]);
    b.firstStartedAt = "now";
    d.tasks[0].status = "已发布";
    expect(batchActions(d, b)).toEqual(["detail", "copy"]);
  });
});

it("创建后清空候选资源不误拦截正常保存，映射仍不可替换", () => {
  const d = fixture();
  const b = d.batches[0];
  b.draft = { resourcesLocked: true };
  b.candidates = [];
  const next = structuredClone(d);
  next.tasks[0].title = "执行前文案";
  expect(() => protectBatchSnapshot(d, next)).not.toThrow();
  next.tasks[0].assetId = 99;
  expect(() => protectBatchSnapshot(d, next)).toThrow(/资源/);
});
it("锁定草稿转正式任务时必须保留原资源组合", () => {
  const d = fixture();
  const original = { ...d.tasks[0] };
  d.tasks = [];
  Object.assign(d.batches[0], {
    draft: { resourcesLocked: true },
    candidates: [original],
  });
  const next = structuredClone(d);
  next.tasks = [structuredClone(original)];
  expect(() => protectBatchSnapshot(d, next)).not.toThrow();
  next.tasks[0].accountId = 99;
  expect(() => protectBatchSnapshot(d, next)).toThrow(/资源/);
});
it("内容保存全量校验后原子更新，不允许一条失败产生部分文案", () => {
  const d = fixture();
  d.tasks.push({ ...d.tasks[0], id: 12 });
  const rows = d.tasks.map((t) => ({ ...t, title: "修改" }));
  expect(() =>
    runBatchCommand(d, d.batches[0], "saveContent", { rows }, (t) =>
      t.id === 12 ? ["缺少封面"] : [],
    ),
  ).toThrow(/封面/);
  expect(d.tasks.map((t) => t.title)).toEqual(["原内容", "原内容"]);
  runBatchCommand(d, d.batches[0], "saveContent", { rows }, () => []);
  expect(d.tasks.map((t) => t.title)).toEqual(["修改", "修改"]);
});
it("未知任务不能放弃，停止后的已确认失败仍可完成处置", () => {
  const d = fixture(),
    b = d.batches[0],
    t = d.tasks[0];
  b.firstStartedAt = "now";
  t.status = "结果待确认";
  t.result = "未知";
  expect(() => runBatchCommand(d, b, "abandon", { taskId: t.id })).toThrow(
    /核实/,
  );
});
it("首次执行前所有任务已处置可结束，不伪造执行时间也不能编辑或执行", () => {
  const d = fixture(),
    b = d.batches[0];
  d.tasks[0].status = "已取消";
  expect(batchState(d, b)).toBe("已结束");
  expect(contentEditable(d, b)).toBe(false);
  expect(b.firstStartedAt).toBeUndefined();
  expect(batchActions(d, b)).toEqual(["detail", "copy"]);
});
it("仅等待人工时属于待处理，不因领取标记误判为正常执行", () => {
  const d = fixture(),
    b = d.batches[0];
  b.firstStartedAt = "now";
  d.tasks[0].status = "等待人工";
  d.tasks[0].claimed = true;
  expect(batchState(d, b)).toBe("待处理");
  expect(batchActions(d, b)).toEqual(["process"]);
});
it.each([
  [["已发布", "已发布"], "全部已发布", 100],
  [["已发布", "已取消"], "部分已发布", 50],
  [["已取消", "已放弃"], "未发布", 0],
])("终态 %j 结果与达成分开统计", (statuses, result, achievement) => {
  const d = fixture(),
    b = d.batches[0];
  b.targets[0].count = 2;
  d.tasks = statuses.map((status, id) => ({ id, batchId: 1, status }));
  expect(batchState(d, b)).toBe("已结束");
  expect(batchMetrics(d, b)).toMatchObject({
    result,
    achievement,
    progress: 100,
  });
});
it("内容阶段草稿禁止变更类目配额与账号顺序", () => {
  const d = fixture(),
    b = d.batches[0];
  b.candidates = structuredClone(d.tasks);
  d.tasks = [];
  b.draft = {
    resourcesLocked: true,
    accountOrder: [2],
    accountLimits: { 2: 1 },
  };
  for (const mutate of [
    (n) => (n.targets[0].count = 99),
    (n) => (n.draft.accountOrder = [9]),
    (n) => (n.draft.accountLimits[2] = 99),
  ]) {
    const next = structuredClone(d);
    mutate(next.batches[0]);
    expect(() => protectBatchSnapshot(d, next)).toThrow(/锁定/);
  }
});
