import { it, expect } from "vitest";
import { createBatchStore } from "../../../publishing-preview.mjs";
const data = () => ({
  batches: [
    {
      id: 1,
      name: "批次",
      version: 1,
      phase: "内容预检通过",
      control: "待执行",
      targets: [],
    },
  ],
  tasks: [
    {
      id: 1,
      batchId: 1,
      status: "待执行",
      attempts: 0,
      title: "标题",
      accountId: 1,
      assetId: 1,
      platform: "B站",
      category: "游戏",
    },
  ],
  plans: [],
});
it("服务端并发只有一个成功，重复请求幂等，旧内容不能绕过锁定", async () => {
  const store = createBatchStore({ check: () => [] });
  await store.bootstrap(data());
  const cmd = {
    batchId: 1,
    action: "execute",
    input: { expectedVersion: 1 },
    key: "execute-1",
    revision: 1,
  };
  const results = await Promise.allSettled([
    store.command(cmd),
    store.command({ ...cmd, key: "execute-2" }),
  ]);
  expect(results.filter((r) => r.status === "fulfilled")).toHaveLength(1);
  const replay = await store.command(cmd);
  expect(replay.revision).toBe(2);
  expect(replay.data.tasks[0].attempts).toBe(1);
  const edited = structuredClone(replay.data);
  edited.tasks[0].title = "篡改";
  await expect(store.save(edited, 2)).rejects.toThrow(/锁定/);
  const stopped = await store.command({
    batchId: 1,
    action: "stop",
    revision: 2,
    key: "stop",
  });
  expect(stopped.data.tasks[0].status).toBe("已取消");
  await expect(
    store.command({
      batchId: 1,
      action: "execute",
      revision: 3,
      key: "restart",
    }),
  ).rejects.toThrow(/不可执行/);
});
it("命令失败不产生部分写入", async () => {
  const store = createBatchStore({ check: () => ["不可用"] });
  await store.bootstrap(data());
  await expect(
    store.command({ batchId: 1, action: "execute", revision: 1 }),
  ).rejects.toThrow();
  expect((await store.get()).revision).toBe(1);
  expect((await store.get()).data.batches[0].firstStartedAt).toBeUndefined();
});
it("自动保存后仍保留命令幂等记录，不能换参数复用同一请求", async () => {
  const store = createBatchStore({ check: () => [] });
  await store.bootstrap(data());
  const cmd = { batchId: 1, action: "execute", key: "same", revision: 1 };
  const accepted = await store.command(cmd);
  await store.save(accepted.data, 2);
  const replay = await store.command(cmd);
  expect(replay.revision).toBe(3);
  expect(replay.data.tasks[0].attempts).toBe(1);
  await expect(store.command({ ...cmd, action: "stop" })).rejects.toThrow(
    /请求标识/,
  );
});
it("服务端拒绝执行后取消、删除锁定标记和追加任务", async () => {
  const store = createBatchStore({ check: () => [] });
  await store.bootstrap(data());
  const s = await store.command({ batchId: 1, action: "execute", revision: 1 });
  for (const mutate of [
    (d) => (d.batches[0].control = "已取消"),
    (d) => delete d.tasks[0].batchConfigLocked,
    (d) => d.tasks.push({ ...d.tasks[0], id: 2 }),
  ]) {
    const d = structuredClone(s.data);
    mutate(d);
    await expect(store.save(d, 2)).rejects.toThrow();
  }
});
it("任务核实接口拒绝过期状态与无实际发布时间的已发布结果", async () => {
  const store = createBatchStore({ check: () => [] });
  const d = data();
  d.tasks[0].status = "结果待确认";
  d.tasks[0].attempts = 1;
  await store.bootstrap(d);
  await expect(
    store.command({
      batchId: 1,
      action: "taskOperation",
      revision: 1,
      input: {
        taskId: 1,
        action: "verify",
        expectedStatus: "执行失败",
        expectedAttempts: 1,
      },
    }),
  ).rejects.toThrow(/变化/);
  await expect(
    store.command({
      batchId: 1,
      action: "taskOperation",
      revision: 1,
      input: {
        taskId: 1,
        action: "verify",
        expectedStatus: "结果待确认",
        expectedAttempts: 1,
        status: "已发布",
      },
    }),
  ).rejects.toThrow(/发布时间/);
  expect((await store.get()).revision).toBe(1);
});
it("本地预览重启后保留执行锁定和幂等记录", async () => {
  const { mkdtemp, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const dir = await mkdtemp(join(tmpdir(), "wt-v12-store-test-"));
  try {
    const file = join(dir, "store.json");
    const a = createBatchStore({ file, check: () => [] });
    await a.bootstrap(data());
    const cmd = { batchId: 1, action: "execute", key: "durable", revision: 1 };
    await a.command(cmd);
    const b = createBatchStore({ file, check: () => [] });
    const s = await b.command(cmd);
    expect(s.revision).toBe(2);
    expect(s.data.tasks[0].attempts).toBe(1);
    expect(s.data.tasks[0].batchConfigLocked).toBe(true);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
it("首次读取旧记录的迁移快照在服务重启后保持不变", async () => {
  const { mkdtemp, rm, writeFile } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const dir = await mkdtemp(join(tmpdir(), "wt-v12-migration-test-"));
  try {
    const file = join(dir, "store.json");
    const legacy = data();
    legacy.tasks[0].status = "平台审核中";
    legacy.tasks[0].attempts = 1;
    await writeFile(file, JSON.stringify({ revision: 7, data: legacy }));
    const a = createBatchStore({ file });
    const loaded = await a.get();
    // Move the clock so regenerated migration timestamps cannot accidentally match.
    const { vi } = await import("vitest");
    vi.useFakeTimers();
    vi.setSystemTime(new Date(Date.now() + 60000));
    try {
      const b = createBatchStore({ file });
      const reloaded = await b.get();
      expect(reloaded.data.batches[0].frozenSnapshot).toEqual(
        loaded.data.batches[0].frozenSnapshot,
      );
      await expect(b.save(loaded.data, 7)).resolves.toMatchObject({
        revision: 8,
      });
    } finally {
      vi.useRealTimers();
    }
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
it("历史失败缺少证据时不能重试或放弃，可以核实为仍不明确", async () => {
  const store = createBatchStore({ check: () => [] });
  const d = data();
  d.tasks[0].status = "执行失败";
  d.tasks[0].result = "未知";
  d.tasks[0].attempts = 1;
  const original = await store.bootstrap(d);
  await expect(
    store.command({
      batchId: 1,
      action: "abandon",
      revision: 1,
      input: { taskId: 1 },
    }),
  ).rejects.toThrow(/核实/);
  const input = {
    taskId: 1,
    expectedStatus: "执行失败",
    expectedAttempts: 1,
    attempt: 1,
  };
  await expect(
    store.command({
      batchId: 1,
      action: "taskOperation",
      revision: 1,
      input: { ...input, action: "retry" },
    }),
  ).rejects.toThrow(/证据/);
  const result = await store.command({
    batchId: 1,
    action: "taskOperation",
    revision: 1,
    input: {
      ...input,
      action: "verify",
      status: "结果待确认",
      evidence: "仍未取得平台证据",
    },
  });
  expect(result.data.tasks[0].status).toBe("结果待确认");
  expect(result.data.tasks[0].attempts).toBe(1);
  expect(result.data.tasks[0].attemptHistory[0].ended).toBeUndefined();
  expect(result.data.batches[0].frozenSnapshot).toEqual(
    original.data.batches[0].frozenSnapshot,
  );
});
