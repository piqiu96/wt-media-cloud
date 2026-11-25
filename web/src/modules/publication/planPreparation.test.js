import { describe, it, expect } from "vitest";
import {
  emptyPlan,
  planProblem,
  planProgressLabels,
  splitBatch,
  mergeBatch,
  planReturn,
  sendPlanCommand,
} from "./planPreparation.js";
describe("plan preparation", () => {
  it("conserves target when splitting and merging stable batches", () => {
    let p = emptyPlan();
    p.name = "周期";
    p.date_start = "2026-11-25";
    p.date_end = "2026-11-26";
    p.goals = [{ platform: "baidu", category_id: "1", target_count: 5 }];
    p.batches = [
      {
        row_key: "a",
        name: "首批",
        platform: "baidu",
        group_order: 1,
        batch_order: 1,
        targets: [{ category_id: "1", target_count: 5 }],
      },
    ];
    splitBatch(p, 0);
    expect(p.batches.map((b) => b.targets[0].target_count)).toEqual([3, 2]);
    expect(planProblem(p)).toBe("");
    mergeBatch(p, 0);
    expect(p.batches[0].targets[0].target_count).toBe(5);
  });
  it("keeps return paths restricted to plans", () => {
    expect(planReturn({ from_plan: "2", plan_page: "3", evil: "x" })).toEqual({
      name: "PublishPlans",
      query: { view: "detail", id: "2", page: "3" },
    });
    expect(planReturn({ from_plan: "https://evil.test" })).toBe(null);
  });
  it("cannot recover another start from current rows or position", async () => {
    const api = {
      get: async () => ({ plan: { id: "1", current_batch_id: "2" } }),
      start: async () => {
        throw Object.assign(Error("conflict"), { status: 409 });
      },
    };
    await expect(
      sendPlanCommand(api, "1", {
        action: "start",
        input: {
          request_key: "original",
          expected_revision: "1",
          first_batch_id: "2",
          batch_confirmations: [],
        },
      }),
    ).rejects.toMatchObject({ outcomeUnknown: true });
  });
});
import { batchTaskLocation, batchTaskReturn } from "./batchExecution.js";
it("preserves plan context through original task without accepting arbitrary return URL", () => {
  const task = batchTaskLocation(
    "8",
    "9",
    { from_plan: "2", plan_page: "4", evil: "https://evil" },
    {},
  );
  expect(batchTaskReturn(task.query).query).toMatchObject({
    from_plan: "2",
    plan_page: "4",
    id: "8",
  });
  expect(task.query.evil).toBeUndefined();
});
it("requires complete original first batch receipt and immutable confirmation", async () => {
  const c = {
      batch_id: "2",
      expected_revision: "4",
      included_row_keys: ["r"],
      excluded_row_keys: [],
      allow_shortfall: false,
      expected_shortfalls: [{ category_id: "1", shortfall: 0 }],
    },
    op = {
      action: "start",
      input: {
        request_key: "key",
        expected_revision: "3",
        first_batch_id: "2",
        batch_confirmations: [c],
      },
    },
    t = "2026-11-25T04:00:00Z";
  const result = {
    plan_id: "1",
    revision: "4",
    first_started_at: t,
    current_batch_id: "2",
    batch_confirmations: [c],
    first_batch_receipt: {
      batch_id: "2",
      revision: "5",
      execution_status: "running",
      first_started_at: t,
      task_ids: ["7"],
      created_count: 1,
      target_count: 1,
      creation_shortfall: 0,
    },
  };
  await expect(
    sendPlanCommand({ start: async () => result }, "1", op),
  ).resolves.toEqual(result);
  for (const bad of [
    { ...result, first_batch_receipt: { task_ids: ["7"] } },
    { ...result, batch_confirmations: [{ ...c, expected_revision: "5" }] },
    {
      ...result,
      first_batch_receipt: { ...result.first_batch_receipt, batch_id: "9" },
    },
  ])
    await expect(
      sendPlanCommand({ start: async () => bad }, "1", op),
    ).rejects.toMatchObject({ outcomeUnknown: true });
});
import { planJournal } from "./planPreparation.js";
it("keeps original command across tabs and only clears the exact owner command", async () => {
  const data = new Map(),
    storage = {
      getItem: (k) => data.get(k),
      setItem: (k, v) => data.set(k, v),
      removeItem: (k) => data.delete(k),
    },
    locks = { request: async (k, f) => f() },
    a = planJournal("cloud", "web", "9", "2", storage, locks),
    b = planJournal("cloud", "web", "9", "2", storage, locks),
    op = {
      action: "start",
      input: { request_key: "first", expected_revision: "1" },
    };
  await a.save(op);
  await expect(
    b.save({ action: "start", input: { request_key: "second" } }),
  ).rejects.toThrow("已有");
  await b.clear({ action: "start", input: { request_key: "second" } });
  expect(a.read()).toEqual(op);
  await b.clear(op);
  expect(a.read()).toBeNull();
  expect(
    planJournal("cloud", "web", "10", "2", storage, locks).read(),
  ).toBeNull();
});
it("cannot replay the original start when its current plan cannot be read", async () => {
  let writes = 0;
  await expect(
    sendPlanCommand(
      {
        get: async () => {
          throw Error("offline");
        },
        start: async () => {
          writes++;
        },
      },
      "2",
      { action: "start", input: { request_key: "key" } },
      { recover: true },
    ),
  ).rejects.toMatchObject({ outcomeUnknown: true });
  expect(writes).toBe(0);
});
it("preserves unknown original command when recovery authentication expires", async () => {
  await expect(
    sendPlanCommand(
      {
        get: async () => {
          throw Object.assign(Error("expired"), { status: 401 });
        },
      },
      "2",
      { action: "start", input: { request_key: "original" } },
      { recover: true },
    ),
  ).rejects.toMatchObject({ outcomeUnknown: true });
});
it("only accepts a fully bound provider non-application proof", async () => {
  const op = {
      action: "start",
      input: { request_key: "original", expected_revision: "5" },
    },
    proof = {
      command_applied: false,
      command: "start",
      request_key: "original",
      expected_revision: "5",
      current_revision: "6",
    };
  const failure = (details) =>
    Object.assign(Error("conflict"), {
      status: 409,
      code: "publication_conflict",
      details,
    });
  const api = (details) => ({
    get: async () => ({ plan: { id: "2" } }),
    start: async () => {
      throw failure(details);
    },
  });
  await expect(
    sendPlanCommand(api(proof), "2", op, { recover: true }),
  ).rejects.toMatchObject({ planRejection: true, definitiveNotApplied: true });
  for (const bad of [
    { ...proof, command: "confirm-next" },
    { ...proof, request_key: "other" },
    { ...proof, expected_revision: "4" },
    { ...proof, current_revision: "5" },
    { ...proof, current_revision: "05" },
    { ...proof, current_revision: 6 },
    { ...proof, command_applied: null },
    Object.fromEntries(
      Object.entries(proof).filter(([k]) => k !== "request_key"),
    ),
  ])
    await expect(
      sendPlanCommand(api(bad), "2", op, { recover: true }),
    ).rejects.toMatchObject({ outcomeUnknown: true });
});
import { readPlanDraftChoice } from "./planPreparation.js";
it("distinguishes continuation, explicit restart and dismiss without clearing newer input", async () => {
  let clears = 0;
  const local = {
      payload: {
        form: { name: "保留计划" },
        pending: { request_key: "unknown" },
      },
      revision: "3",
    },
    client = {
      get: async () => local,
      clear: async () => {
        clears++;
        throw Error("draft_revision_conflict");
      },
    };
  expect(await readPlanDraftChoice(client, async () => null)).toBe(null);
  expect(clears).toBe(0);
  expect(await readPlanDraftChoice(client, async () => "continue")).toEqual(
    local,
  );
  expect(clears).toBe(0);
  await expect(
    readPlanDraftChoice(client, async () => "restart"),
  ).rejects.toThrow("draft_revision_conflict");
  expect(clears).toBe(1);
  expect(local.payload.pending.request_key).toBe("unknown");
});

it("splits different one-item goal cells without losing stable original row identity", () => {
  const p = {
    batches: [
      {
        row_key: "stable",
        name: "混合",
        platform: "baidu",
        targets: [
          { category_id: "1", target_count: 1 },
          { category_id: "2", target_count: 1 },
        ],
      },
    ],
  };
  splitBatch(p, 0);
  expect(p.batches[0].row_key).toBe("stable");
  expect(p.batches.map((b) => b.targets)).toEqual([
    [{ category_id: "1", target_count: 1 }],
    [{ category_id: "2", target_count: 1 }],
  ]);
});
it("rejects a batch without a goal and duplicate batch cells", () => {
  const p = {
    name: "计划",
    date_start: "2026-11-25",
    date_end: "2026-11-26",
    goals: [{ platform: "baidu", category_id: "1", target_count: 2 }],
    batches: [
      { row_key: "x", name: "x", platform: "baidu", targets: [] },
      {
        row_key: "y",
        name: "y",
        platform: "baidu",
        targets: [{ category_id: "1", target_count: 2 }],
      },
    ],
  };
  expect(planProblem(p)).not.toBe("");
  p.batches = [
    {
      row_key: "x",
      name: "x",
      platform: "baidu",
      targets: [
        { category_id: "1", target_count: 1 },
        { category_id: "1", target_count: 1 },
      ],
    },
  ];
  expect(planProblem(p)).not.toBe("");
});

it("names platform-review wait and a paused current batch without implying finish permission", () => {
  expect(planProgressLabels.awaiting_finish).toBe("等待平台公开结果");
  expect(planProgressLabels.batch_paused).toBe("当前批次已暂停");
});
