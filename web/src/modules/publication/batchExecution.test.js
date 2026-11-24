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
    start: vi.fn(async (id, input) => { events.push(input); return { batch_id: id, revision: "6", first_started_at: "2026-11-25T01:00:00Z", task_ids: ["94"] }; }),
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

it("returns exact started tasks from Cloud after a lost start acknowledgement without reposting", async () => {
  const api = {
    get: vi.fn().mockResolvedValue({ id: batch.id, revision: "12", execution: { first_started_at: "2026-11-25T01:00:00Z" } }),
    tasks: vi.fn().mockResolvedValue({ total: 1, items: [{ id: "101", revision: "20", row_key: "a" }] }),
    start: vi.fn(),
  };
  const input = execution.buildBatchStart(batch, preview, "original", true);
  await expect(execution.sendBatchCommand(api, batch.id, { action: "start", input }, { retry: true }))
    .resolves.toEqual({ batch_id: batch.id, revision: "12", task_ids: ["101"], recovered: true });
  expect(api.start).not.toHaveBeenCalled();
});

it("recovers only its own persisted command across journal recreation", () => {
  const data = new Map();
  const storage = {
    getItem: key => data.get(key),
    setItem: (key, value) => data.set(key, value),
    removeItem: key => data.delete(key),
  };
  const journal = (...scope) => execution.batchCommandJournal(...scope, batch.id, storage);
  const scope = ["https://cloud.example.test", "desktop", "17"];
  const command = { action: "start", input: execution.buildBatchStart(batch, preview, "original", true) };
  journal(...scope).save(command);
  expect(journal(...scope).read()).toEqual(command);
  expect(journal(scope[0], "cloud", scope[2]).read()).toBeNull();
  expect(journal(scope[0], "desktop", "18").read()).toBeNull();
  expect(journal("https://other.example.test", "desktop", scope[2]).read()).toBeNull();
  journal(...scope).clear(command);
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
