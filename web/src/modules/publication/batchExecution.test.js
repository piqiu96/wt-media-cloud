import { expect, it, vi } from "vitest";
import * as execution from "./batchExecution.js";
import { createPublicationBatchClient } from "../../shared/api/publication.js";

const batch = {
  id: "9007199254740993", revision: "9007199254740995",
  config: { candidates: [{ row_key: "b", order: 1 }, { row_key: "a", order: 0 }] },
  targets: [{ category_id: "9", target_count: 3 }], allowed_actions: ["start"],
};
const preview = {
  batch_id: batch.id, revision: batch.revision, can_start: true,
  rows: [{ row_key: "a", ready: true, issues: [] }],
  shortfalls: [{ category_id: "9", target_count: 3, created_count: 1, shortfall: 2 }],
};

it("uses the provider shortfall and disjoint saved row keys after explicit confirmation", () => {
  expect(() => execution.buildBatchStart(batch, preview, "original")).toThrow("缺口");
  expect(execution.buildBatchStart(batch, preview, "original", true)).toEqual({
    request_key: "original", expected_revision: batch.revision,
    included_row_keys: ["a"], excluded_row_keys: ["b"],
    allow_shortfall: true, expected_shortfalls: [{ category_id: "9", shortfall: 2 }],
  });
});

it("rejects a preflight result for another configuration revision", () => {
  expect(() => execution.buildBatchStart(batch, { ...preview, revision: "8" }, "original", true)).toThrow("批次配置已变化");
});

it("shows only the server's verified public count against the original target", () => {
  expect(execution.batchProgress({ target_count: 30, published_count: 24, created_count: 28,
    creation_shortfall: 2, target_shortfall: 6, task_counts: { reviewing: 4, failed: 2 } }))
    .toEqual({ target: 30, published: 24, created: 28, creationShortfall: 2, targetShortfall: 6, rate: 80, reviewing: 4, exceptions: 2 });
});

it("reads current Cloud state before retry and keeps the original start input", async () => {
  const events = [];
  const api = {
    get: vi.fn(async () => { events.push("get"); return { id: batch.id, execution_status: "not_started" }; }),
    start: vi.fn(async (id, input) => { events.push(input); return { batch_id: id, revision: "9007199254740996", execution_status: "running", first_started_at: "2026-11-25T01:00:00Z", task_ids: ["94"] }; }),
  };
  const original = execution.buildBatchStart(batch, preview, "original", true);
  await expect(execution.sendBatchCommand(api, batch.id, { action: "start", input: original }, { retry: true }))
    .resolves.toMatchObject({ batch_id: batch.id, task_ids: ["94"] });
  expect(events).toEqual(["get", original]);
});

it("does not replay automatically when the first response is lost", async () => {
  const api = { get: vi.fn().mockResolvedValue({ id: batch.id }), start: vi.fn().mockRejectedValue(new TypeError("断开")) };
  const command = { action: "start", input: execution.buildBatchStart(batch, preview, "original", true) };
  await expect(execution.sendBatchCommand(api, batch.id, command)).rejects.toMatchObject({ outcomeUnknown: true });
  expect(api.get).toHaveBeenCalledWith(batch.id);
  expect(api.start).toHaveBeenCalledOnce();
});

it("collects exact failed task revisions across all pages", async () => {
  const api = { tasks: vi.fn()
    .mockResolvedValueOnce({ total: 101, items: Array.from({ length: 100 }, (_, index) => ({ id: `${index + 1}`, revision: "4", status: "failed" })) })
    .mockResolvedValueOnce({ total: 101, items: [{ id: "9007199254740997", revision: "9", status: "failed" }] }) };
  const results = await execution.readBatchFailures(api, batch.id);
  expect(api.tasks).toHaveBeenLastCalledWith(batch.id, { status: "failed", page: 2, page_size: 100 });
  expect(results).toHaveLength(101);
  expect(results.at(-1)).toMatchObject({ id: "9007199254740997", revision: "9" });
});

it("uses the batch control endpoints and preserves string values in requests", async () => {
  const fetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ errcode: 0, data: {} }) });
  const api = createPublicationBatchClient({ fetch });
  for (const [method, endpoint] of [["executionPreview", "execution-preview"], ["start", "start"], ["pause", "pause"], ["resume", "resume"], ["stop", "stop"], ["finish", "finish"]]) {
    const input = { expected_revision: batch.revision };
    await api[method](batch.id, input);
    expect(fetch).toHaveBeenLastCalledWith(`/api/v1/publication-batch/${batch.id}/${endpoint}`,
      expect.objectContaining({ method: "POST", body: JSON.stringify(input) }));
  }
  await api.tasks(batch.id, { page: 2, media_account_id: "47" });
  expect(fetch).toHaveBeenLastCalledWith(`/api/v1/publication-batch/${batch.id}/task?page=2&media_account_id=47`, expect.objectContaining({ method: "GET" }));
});

it("recovers a lost start ACK only from the original key receipt after later revisions", async () => {
  const firstStartedAt = "2026-11-25T01:00:00Z";
  const latest = { id: batch.id, revision: "9007199254741010", execution_status: "paused",
    execution: { first_started_at: firstStartedAt } };
  const receipt = { batch_id: batch.id, revision: "9007199254740996", execution_status: "running",
    first_started_at: firstStartedAt, task_ids: ["101"] };
  const api = {
    get: vi.fn().mockResolvedValue(latest),
    tasks: vi.fn().mockResolvedValue({ total: 1, items: [{ id: "101", revision: "20", row_key: "a" }] }),
    start: vi.fn().mockRejectedValueOnce(new TypeError("原回执丢失")).mockResolvedValueOnce(receipt),
  };
  const input = execution.buildBatchStart(batch, preview, "original", true);
  const command = { action: "start", input };
  await expect(execution.sendBatchCommand(api, batch.id, command)).rejects.toMatchObject({ outcomeUnknown: true, latest });
  expect(api.start).toHaveBeenCalledTimes(1);
  await expect(execution.sendBatchCommand(api, batch.id, command, { retry: true }))
    .resolves.toEqual({ ...receipt, recovered: true });
  expect(api.start.mock.calls).toEqual([[batch.id, input], [batch.id, input]]);
  expect(api.tasks).not.toHaveBeenCalled();
});

it("keeps the original intent when another revision started the same rows with another key", async () => {
  const firstStartedAt = "2026-11-25T01:00:00Z";
  const latest = { id: batch.id, revision: "9007199254740997", execution_status: "running",
    execution: { first_started_at: firstStartedAt } };
  const conflict = Object.assign(new Error("发布条件已变化"), { status: 409, code: "publication_conflict",
    details: { batch_id: batch.id, current_revision: latest.revision, execution_status: "running" } });
  const api = {
    get: vi.fn().mockResolvedValueOnce({ ...batch, execution_status: "not_started" }).mockResolvedValue(latest),
    tasks: vi.fn().mockResolvedValue({ total: 1, items: [{
      id: "102", revision: "1", row_key: "a", media_account_id: "18", content: { description: "另一版本" },
    }] }),
    start: vi.fn().mockRejectedValueOnce(new TypeError("原请求未到达")).mockRejectedValueOnce(conflict),
  };
  const input = execution.buildBatchStart(batch, preview, "original", true);
  const command = { action: "start", input };
  await expect(execution.sendBatchCommand(api, batch.id, command)).rejects.toMatchObject({ outcomeUnknown: true });
  await expect(execution.sendBatchCommand(api, batch.id, command, { retry: true }))
    .rejects.toMatchObject({ outcomeUnknown: true, latest });
  expect(api.start.mock.calls).toEqual([[batch.id, input], [batch.id, input]]);
});

it.each(["cancelled", "stopping", "future_status", undefined])(
  "preserves an original start after a %s conflict and a failed second read",
  async executionStatus => {
    const current = { id: batch.id, revision: batch.revision, execution_status: "not_started",
      execution: { first_started_at: null } };
    const response = Object.assign(new Error("发布条件已变化"), { status: 409, code: "publication_conflict",
      details: { batch_id: batch.id, current_revision: "9007199254741000", execution_status: executionStatus } });
    const calls = [];
    const api = {
      get: vi.fn().mockImplementationOnce(async () => { calls.push("read before replay"); return current; })
        .mockImplementationOnce(async () => { calls.push("failed read after replay"); throw new TypeError("网络中断"); }),
      start: vi.fn(async () => { calls.push("same-key replay"); throw response; }),
    };
    const input = execution.buildBatchStart(batch, preview, "original", true);
    await expect(execution.sendBatchCommand(api, batch.id, { action: "start", input }, { retry: true }))
      .rejects.toMatchObject({ outcomeUnknown: true, latest: current });
    expect(api.start).toHaveBeenCalledExactlyOnceWith(batch.id, input);
    expect(calls).toEqual(["read before replay", "same-key replay", "failed read after replay"]);
  },
);

it("clears a structured replay rejection when Cloud explicitly reports the batch is still unstarted", async () => {
  const current = { id: batch.id, revision: batch.revision, execution_status: "not_started" };
  const response = Object.assign(new Error("发布条件已变化"), { status: 409, code: "publication_conflict",
    details: { batch_id: batch.id, current_revision: "9007199254741000", execution_status: "not_started" } });
  const api = {
    get: vi.fn().mockResolvedValueOnce(current).mockRejectedValueOnce(new TypeError("网络中断")),
    start: vi.fn().mockRejectedValue(response),
  };
  const input = execution.buildBatchStart(batch, preview, "original", true);
  await expect(execution.sendBatchCommand(api, batch.id, { action: "start", input }, { retry: true }))
    .rejects.toMatchObject({ batchRejection: true });
});

it("settles only the original intent when Cloud explicitly proves that another key started the batch", async () => {
  const latest = { id: batch.id, revision: "9007199254740997", execution_status: "running",
    execution: { first_started_at: "2026-11-25T01:00:00Z" } };
  const conflict = Object.assign(new Error("发布条件已变化"), { status: 409, code: "publication_conflict",
    details: { batch_id: batch.id, current_revision: latest.revision, execution_status: "running", start_request_applied: false } });
  const api = { get: vi.fn().mockResolvedValue(latest), start: vi.fn().mockRejectedValue(conflict) };
  const input = execution.buildBatchStart(batch, preview, "original", true);
  await expect(execution.sendBatchCommand(api, batch.id, { action: "start", input }, { retry: true }))
    .rejects.toMatchObject({ batchRejection: true, message: "批次已由另一请求开始，请核对正式任务", latest });
  expect(api.start).toHaveBeenCalledExactlyOnceWith(batch.id, input);
});

it("cannot clear a pending start from a receipt with the current revision instead of the original revision", async () => {
  const firstStartedAt = "2026-11-25T01:00:00Z";
  const latest = { id: batch.id, revision: "9007199254741010", execution: { first_started_at: firstStartedAt } };
  const api = {
    get: vi.fn().mockResolvedValue(latest),
    start: vi.fn().mockResolvedValue({ batch_id: batch.id, revision: latest.revision, execution_status: "running",
      first_started_at: firstStartedAt, task_ids: ["101"] }),
  };
  const input = execution.buildBatchStart(batch, preview, "original", true);
  await expect(execution.sendBatchCommand(api, batch.id, { action: "start", input }, { retry: true }))
    .rejects.toMatchObject({ outcomeUnknown: true, latest });
  expect(api.start).toHaveBeenCalledOnce();
});

it.each([401, 429])("retains the original resume after an uncertain ACK and a %i retry response", async status => {
  const input = { request_key: "original", expected_revision: "4", expected_pause_id: "15", acknowledged_exceptions: [] };
  const command = { action: "resume", input };
  const latest = { id: batch.id, revision: "5", execution: { phase: "running", pause: null } };
  const api = {
    get: vi.fn().mockResolvedValue(latest),
    resume: vi.fn().mockRejectedValueOnce(new TypeError("响应丢失"))
      .mockRejectedValueOnce(Object.assign(new Error("暂时不可用"), { status }))
      .mockResolvedValueOnce({ batch_id: batch.id, revision: "5", execution: { phase: "running" } }),
  };
  await expect(execution.sendBatchCommand(api, batch.id, command)).rejects.toMatchObject({ outcomeUnknown: true });
  await expect(execution.sendBatchCommand(api, batch.id, command, { retry: true }))
    .rejects.toMatchObject({ outcomeUnknown: true, latest });
  await expect(execution.sendBatchCommand(api, batch.id, command, { retry: true })).resolves.toMatchObject({ revision: "5" });
  expect(api.resume.mock.calls).toEqual([[batch.id, input], [batch.id, input], [batch.id, input]]);
});

it("only clears a replay conflict when Cloud reports its current revision", async () => {
  const command = { action: "resume", input: { request_key: "original", expected_revision: "4" } };
  const conflict = new Error("批次依据已变化");
  Object.assign(conflict, { status: 409, code: "publication_conflict", details: { current_revision: "7" } });
  const generic = Object.assign(new Error("客户端冲突"), { status: 409 });
  const api = { get: vi.fn().mockResolvedValue({ id: batch.id }), resume: vi.fn().mockRejectedValueOnce(generic).mockRejectedValueOnce(conflict) };
  await expect(execution.sendBatchCommand(api, batch.id, command, { retry: true })).rejects.toMatchObject({ outcomeUnknown: true });
  await expect(execution.sendBatchCommand(api, batch.id, command, { retry: true })).rejects.toMatchObject({ batchRejection: true });
});

it.each([
  ["publication_conflict", { issues: [{ row_key: "a", message: "检查后原成片已占用" }] }],
  ["publication_output_occupied", { issues: [{ row_key: "a", code: "output_occupied", message: "成片已占用" }] }],
  ["publication_batch_shortfall_changed", { shortfalls: [{ category_id: "9", target_count: 3, created_count: 0, shortfall: 3 }] }],
])("allows a new check after the unstarted batch rejects the original start with %s", async (code, details) => {
  const input = execution.buildBatchStart(batch, preview, "original", true);
  const current = { id: batch.id, revision: batch.revision, execution_status: "not_started", execution: { first_started_at: null } };
  const response = Object.assign(new Error("发布条件已变化"), { status: 409, code, details });
  const api = { get: vi.fn().mockResolvedValue(current),
    start: vi.fn().mockRejectedValueOnce(new TypeError("请求未到达")).mockRejectedValueOnce(response) };
  const command = { action: "start", input };
  await expect(execution.sendBatchCommand(api, batch.id, command)).rejects.toMatchObject({ outcomeUnknown: true });
  await expect(execution.sendBatchCommand(api, batch.id, command, { retry: true })).rejects.toMatchObject({ batchRejection: true });
  expect(api.start.mock.calls).toEqual([[batch.id, input], [batch.id, input]]);
});

it("keeps an unresolved start when a replay conflict only names another row", async () => {
  const input = execution.buildBatchStart(batch, preview, "original", true);
  const current = { id: batch.id, execution_status: "not_started", execution: { first_started_at: null } };
  const response = Object.assign(new Error("发布条件已变化"), { status: 409, code: "publication_conflict",
    details: { issues: [{ row_key: "b", message: "其他成片已占用" }] } });
  const api = { get: vi.fn().mockResolvedValue(current), start: vi.fn().mockRejectedValue(response) };
  await expect(execution.sendBatchCommand(api, batch.id, { action: "start", input }, { retry: true }))
    .rejects.toMatchObject({ outcomeUnknown: true });
});

it("recovers only its own persisted command across journal recreation", async () => {
  const data = new Map();
  const storage = {
    getItem: key => data.get(key),
    setItem: (key, value) => data.set(key, value),
    removeItem: key => data.delete(key),
  };
  const locks = { request: async (_name, mutate) => mutate() };
  const journal = (...scope) => execution.batchCommandJournal(...scope, batch.id, storage, locks);
  const scope = ["https://cloud.example.test", "desktop", "17"];
  const command = { action: "start", input: execution.buildBatchStart(batch, preview, "original", true) };
  await journal(...scope).save(command);
  expect(journal(...scope).read()).toEqual(command);
  expect(journal(scope[0], "cloud", scope[2]).read()).toBeNull();
  expect(journal(scope[0], "desktop", "18").read()).toBeNull();
  expect(journal("https://other.example.test", "desktop", scope[2]).read()).toBeNull();
  await journal(...scope).clear({ ...command, input: { ...command.input, request_key: "different" } });
  expect(journal(...scope).read()).toEqual(command);
  await journal(...scope).clear(command);
  expect(journal(...scope).read()).toBeNull();
});


it("returns from the original task to the same batch and filters", () => {
  const origin = { search: "原批次", execution_status: "paused", page: 3 };
  const scope = { status: "uncertain", page: 2, page_size: 20, media_account_id: "953" };
  const linked = execution.batchTaskLocation(batch.id, "502", origin, scope);
  expect(linked.name).toBe("Publish");
  expect(execution.batchTaskReturn({ ...linked.query, destination: "https://untrusted.example" })).toEqual({
    name: "PublishBatches", query: { view: "detail", id: batch.id, tab: "tasks",
      search: "原批次", execution_status: "paused", page: "3", task_status: "uncertain",
      task_page: "2", task_page_size: "20", task_media_account_id: "953" },
  });
});
