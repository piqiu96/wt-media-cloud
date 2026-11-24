// The production route runs against intercepted, synthetic Cloud contracts.
import assert from "node:assert/strict";
import { chromium } from "playwright";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const profile = await mkdtemp(join(tmpdir(), "wt-batch-execution-"));
let browser, page;
const now = "2026-11-25T02:00:00Z";
const id = "9007199254741003";
const owner = { id: "71", role: "operator", team_id: "3", category_ids: ["12"], status: "enabled", username: "test" };
let actor = owner;
const category = { id: "12", name: "合成走查类目", enabled: true };
const account = { id: "381", name: "走查账号", platform: "baidu", category_ids: ["12"], eligible: true };
const outputs = ["视频甲", "视频乙", "视频丙"].map((name, index) => ({
  id: index + 500, composite_output_id: String(index + 500), name,
  category_id: "12", team_id: "3", occupied: false, platforms: ["baidu"], video: { status: "ready" },
}));
const candidates = outputs.map((output, index) => ({
  category_id: "12", media_account_id: account.id, composite_output_id: output.composite_output_id,
  row_key: `row-${index}`, input_revision: "1", field_revisions: {}, order: index,
  content: { description: "发布文案 " + index },
}));
const newExecution = () => ({
  phase: "not_started", first_started_at: null, execution_group_no: null, group_task_ids: [],
  group_ended_at: null, next_group_at: null, pause: null, stopped_at: null, ended_at: null,
  target_count: 4, created_count: 0, creation_shortfall: null, published_count: 0, target_shortfall: 4, task_counts: {},
});
function prepared(batchID, name, value = {}) {
  return {
    id: batchID, revision: "4", name, platform: "baidu", operator_id: owner.id,
    targets: [{ category_id: "12", target_count: 4 }],
    config: { candidates, accounts: [{ media_account_id: account.id, limit: 4 }], output_ids: outputs.map(row => row.composite_output_id) },
    execution_setting: { concurrency_limit: 5, group_interval_seconds: 60, between_group_policy: "auto_continue", exception_task_limit: 3 },
    candidate_count: 3, execution_status: "not_started", configuration_status: "ready",
    allowed_actions: ["start", "edit", "stop"], readiness: { issues: [], shortfalls: [] },
    execution: newExecution(), updated_at: now, ...value,
  };
}
let batch = prepared(id, "受控批次");
let fresh;
let freshCreateCount = 0;
let tasks = [];
let lostStartRead = false;
let startCount = 0;
let expectedResume;
let expectedFinish;
let linkedFailedPage = 0;
const exceptions = [];
const calls = [];

async function routeCloud(route) {
  const request = route.request();
  const url = new URL(request.url());
  const method = request.method();
  const input = request.postDataJSON();
  const endpoint = url.pathname.slice("/api/v1".length);
  const params = url.searchParams;
  calls.push({ method, endpoint, input, params });
  const send = data => route.fulfill({ json: { errcode: 0, data, message: "success", logid: "synthetic" } });
  if (endpoint === "/auth/me") return send(actor);
  if (endpoint === "/categories") return send([category]);
  if (endpoint === "/publication/account") return send({ items: [account], total: 1 });
  if (endpoint === "/media-accounts") return send({ items: [{ id: account.id, platform_name: account.name }], total: 1 });
  if (endpoint === "/publication/output") return send({ items: outputs, total: outputs.length });
  if (endpoint === "/publication/resource") return send({ availability: "available", available_output_count: 3, account_count: 1, eligible_account_count: 1 });
  if (endpoint === "/publication") return send({ items: tasks, total: tasks.length });
  if (endpoint === "/publication-batch/allocate") return send({ config: { candidates: [{ ...candidates[0] }] }, shortfalls: [] });
  if (endpoint === "/publication-batch/validate") return send({ ready: true, issues: [], shortfalls: [] });
  if (endpoint === "/publication-batch" && method === "POST") {
    freshCreateCount++;
    fresh = prepared("9007199254741004", input.name, input);
    fresh.execution.target_count = input.targets[0].target_count;
    return send(fresh);
  }
  if (endpoint === "/publication-batch") return send({ items: [batch, fresh].filter(Boolean), total: fresh ? 2 : 1 });

  if (endpoint.startsWith("/publication-batch/")) {
    const [, , batchID, action] = endpoint.split("/");
    const current = batchID === id ? batch : fresh;
    const control = () => ({ batch_id: current.id, revision: current.revision,
      execution_status: current.execution_status, execution: current.execution });
    const nextRevision = () => { current.revision = String(BigInt(current.revision) + 1n); };
    switch (action) {
      case undefined:
        if (current === batch && lostStartRead) return route.abort("failed");
        return send(current);
      case "execution-preview": {
        assert.equal(input.expected_revision, current.revision);
        const rows = input.row_keys.map(row_key => {
          const ready = row_key !== "row-2";
          return { row_key, ready, issues: ready ? [] : [{ message: "原成片已占用" }] };
        });
        const count = rows.filter(row => row.ready).length;
        const target = current.targets[0].target_count;
        return send({ batch_id: current.id, revision: current.revision, can_start: rows.every(row => row.ready), rows,
          shortfalls: [{ category_id: "12", target_count: target, created_count: count, shortfall: target - count }] });
      }
      case "start": {
        if (current === fresh) return route.fulfill({ status: 409, json: {
          errcode: 18003, message: "检查后原成片已占用", data: null,
          error: { code: "publication_conflict", details: { issues: [{ row_key: "row-0", message: "检查后原成片已占用" }] } },
        } });
        startCount++;
        assert.deepEqual(input.included_row_keys, ["row-0", "row-1"]);
        assert.deepEqual(input.excluded_row_keys, ["row-2"]);
        assert.equal(input.allow_shortfall, true);
        assert.deepEqual(input.expected_shortfalls, [{ category_id: "12", shortfall: 2 }]);
        assert.equal(typeof input.request_key, "string");
        tasks = candidates.slice(0, 2).map((row, index) => ({
          ...row, id: String(150 + index), revision: "6", batch_id: id, operator_id: owner.id, platform: "baidu",
          status: index ? "uncertain" : "running", execution_group_no: 1, created_at: now,
          execution_stage: index ? "connection_lost" : "preparing",
          resources: { account_name: account.name, output_name: outputs[index].name },
          allowed_actions: index ? ["resolve"] : [],
          ...(index && { execution_block: { message: "原尝试等待核实" } }),
        }));
        nextRevision();
        current.execution = { ...newExecution(), phase: "running", first_started_at: now, execution_group_no: 1,
          group_task_ids: ["150"], created_count: 2, creation_shortfall: 2,
          task_counts: { running: 1, uncertain: 1 } };
        current.execution_status = "running";
        current.allowed_actions = ["pause", "stop"];
        lostStartRead = true;
        return route.abort("failed");
      }
      case "task": {
        const status = params.get("status");
        const list = tasks.filter(item => !status || item.status === status);
        if (status === "failed") linkedFailedPage++;
        return send({ items: list, total: list.length, page: 1, page_size: 10 });
      }
      case "pause":
        assert.equal(input.reason, "复核原场景");
        nextRevision();
        current.execution_status = "paused";
        current.allowed_actions = ["resume", "stop"];
        current.execution.phase = "paused";
        current.execution.pause = {
          id: "77", kind: "manual", reason: input.reason, paused_at: now,
          exceptions: [{ task_id: "151", revision: "6", status: "uncertain" }],
        };
        return send(control());
      case "resume":
        expectedResume = input;
        nextRevision();
        current.execution_status = "running";
        current.allowed_actions = ["pause", "stop"];
        current.execution.phase = "waiting_interval";
        current.execution.pause = null;
        current.execution.next_group_at = "2026-11-25T02:01:00Z";
        return send(control());
      case "finish":
        expectedFinish = input;
        nextRevision();
        current.execution_status = "ended";
        current.allowed_actions = ["copy"];
        current.execution.phase = "ended";
        current.execution.ended_at = now;
        tasks = tasks.map(item => ({ ...item, allowed_actions: [] }));
        return send(control());
      case "stop":
        assert.equal(input.reason, "结束后续派发");
        nextRevision();
        current.execution_status = "cancelled";
        current.allowed_actions = [];
        current.execution.phase = "stopping";
        current.execution.stopped_at = now;
        return send({ ...control(), cancelled_unclaimed_count: 0, inflight_count: 1 });
      case "record": return send({ items: [], total: 0 });
    }
  }
  if (endpoint.startsWith("/publication/") && /^\/publication\/\d+(\/attempt)?$/.test(endpoint)) {
    if (endpoint.endsWith("/attempt")) return send([]);
    return send(tasks.find(item => endpoint === "/publication/" + item.id));
  }
  return send([]);
}

async function launch() {
  browser = await chromium.launchPersistentContext(profile, {
    channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome", headless: true, viewport: { width: 1440, height: 1000 },
  });
  page = browser.pages()[0] || await browser.newPage();
  page.setDefaultTimeout(10_000);
  page.on("pageerror", error => exceptions.push(error.message));
  await page.route("**/api/v1/**", routeCloud);
}

const button = (name, parent = page) => parent.getByRole("button", { name, exact: true });
const heading = name => page.getByRole("heading", { name, exact: true });
const dialog = () => page.getByRole("dialog");
const text = value => page.getByText(value, { exact: true });
const field = name => page.locator(`[aria-label="${name}"] input, input[aria-label="${name}"]`).first();
const detailURL = new URL("/publish-batches?search=受控&view=detail&id=" + id, base).href;
try {
  await launch();
  await page.goto(detailURL);
  await heading("受控批次").waitFor();
  await button("开始发布").click();
  await dialog().getByText(/原成片已占用/).waitFor();
  await dialog().getByText("目标 4 · 本次发布 2 · 缺口 2", { exact: true }).waitFor();
  assert.equal(await button("确认开始发布", dialog()).isEnabled(), false);
  await dialog().getByText("确认按所选范围发布并保留目标缺口", { exact: true }).click();
  await button("确认开始发布", dialog()).click();
  await dialog().getByText("操作结果待确认，请核对原请求后继续", { exact: true }).waitFor();
  const outstanding = await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith("wt-media:publication-command")));
  assert.equal(outstanding.length, 1);
  lostStartRead = false;
  await browser.close();
  await launch();
  await page.goto(detailURL);
  await button("核对原启动请求").click();
  await text("已核对原启动结果").waitFor();
  assert.equal(startCount, 1, "an observed started batch is read back without posting start again");
  await text("本组已安排").waitFor();
  await text("公开发布率").waitFor();

  await button("暂停发布").click();
  await dialog().locator('[aria-label="暂停发布原因"] textarea, textarea[aria-label="暂停发布原因"]').fill("复核原场景");
  await button("确认暂停发布", dialog()).click();
  await button("继续发布").waitFor();
  await button("继续发布").click();
  await dialog().getByRole("link", { name: /任务 151.*结果待确认/ }).waitFor();
  await button("确认继续发布", dialog()).click();
  await text("最早下一组").waitFor();
  assert.deepEqual(expectedResume.acknowledged_exceptions, [{ task_id: "151", revision: "6" }]);
  assert.equal(expectedResume.expected_pause_id, "77");

  await button("结果待确认 1").click();
  await page.getByRole("link", { name: "核实结果", exact: true }).click();
  await heading("发布任务详情").waitFor();
  await button("核实结果").waitFor();
  await page.getByRole("button", { name: "返回", description: "返回发布批次" }).click();
  await heading("受控批次").waitFor();
  assert.equal(new URL(page.url()).searchParams.get("search"), "受控");
  assert.equal(new URL(page.url()).searchParams.get("task_status"), "uncertain");

  await button("终止发布").click();
  await dialog().getByText(/未领取任务将取消并保留成片占用/).waitFor();
  await dialog().locator('[aria-label="终止发布原因"] textarea, textarea[aria-label="终止发布原因"]').fill("结束后续派发");
  await button("确认终止发布", dialog()).click();
  await text("终止收尾").waitFor();
  assert.equal(await button("继续发布").count(), 0);
  assert.equal(await button("结束批次").count(), 0);

  batch.execution_status = "running";
  batch.allowed_actions = ["finish"];
  batch.execution = { ...batch.execution, phase: "awaiting_processing", stopped_at: null, task_counts: { failed: 1 } };
  tasks = [{ ...tasks[0], status: "failed", revision: "25", allowed_actions: ["retry"] }];
  await page.reload();
  await button("结束批次").click();
  await dialog().getByText(/保留 1 条明确失败及成片占用/).waitFor();
  await dialog().locator('[aria-label="结束批次原因"] textarea, textarea[aria-label="结束批次原因"]').fill("保留明确失败记录");
  await button("确认结束批次", dialog()).click();
  await text("结束时间").waitFor();
  assert.deepEqual(expectedFinish.expected_failed_tasks, [{ task_id: "150", revision: "25" }]);
  assert.ok(linkedFailedPage > 0);

  await page.goto(new URL("/publish-batches", base).href);
  await button("新建批次").click();
  await field("批次名称").fill("保存成功后启动失败");
  await page.locator('[aria-label="目标类目"]').click();
  await page.locator(".t-select-option").getByText(category.name, { exact: true }).click();
  await button("添加类目").click();
  await button("载入可用资源").click();
  await button("按顺序自动配对").click();
  await text("已配对 1 条候选").waitFor();
  await button("下一步").click();
  await button("下一步").click();
  await field("异常任务上限").fill("3");
  await button("创建并发布").click();
  await heading("保存成功后启动失败").waitFor();
  await dialog().getByText(/检查后原成片已占用/).waitFor();
  assert.equal(freshCreateCount, 1);
  assert.equal(fresh.execution_status, "not_started");
  assert.equal(calls.filter(call => call.endpoint === "/publication-batch/" + fresh.id + "/start").length, 1);

  actor = { ...owner, id: "99", role: "admin" };
  await page.goto(detailURL);
  await heading("受控批次").waitFor();
  assert.equal(await button("开始发布").count(), 0);
  assert.equal(await button("暂停发布").count(), 0);
  assert.deepEqual(exceptions, []);
  console.log("Publication batch execution browser checks passed");
} finally {
  await browser?.close();
  await rm(profile, { recursive: true, force: true });
}
