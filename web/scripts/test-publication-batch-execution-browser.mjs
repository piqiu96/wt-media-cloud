// The production route runs against intercepted, synthetic Cloud contracts.
import assert from "node:assert/strict";
import { chromium } from "playwright";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";

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
  verification_required_count: 0, target_count: 4, created_count: 0, creation_shortfall: null, published_count: 0, target_shortfall: 4, task_counts: {},
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
let startedInput, startReceipt, foreignOriginalInput;
let expectedResume;
let resumeReceipt;
let resumePosts = 0;
let expectedFinish;
let linkedFailedPage = 0;
const exceptions = [];
const review = process.env.WT_MEDIA_BATCH_REVIEW;
let hydrateGate, stoppingGate, stoppingReadStarted, lateTaskGate, lateSummaryGate;
let freshReread = false;
let reviewPreviews = 0;
let replayInput;
const isNavigationReview = ["changed-revision", "abandoned-navigation"].includes(review);
const isPublicResultReview = ["late-publication", "manual-publication"].includes(review);
const isDifferentStart = ["different-start", "different-start-rejected"].includes(review);
const rejectsOriginalStart = review === "different-start-rejected";
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
  if (endpoint === "/publication/output") {
    if (params.has("output_ids") && hydrateGate) await hydrateGate;
    return send({ items: outputs, total: outputs.length });
  }
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
        if (current === fresh && review === "changed-revision" && !freshReread) {
          freshReread = true;
          nextRevision();
          current.config.candidates[0].content.description = "另一标签页更新后的文案";
        }
        if (current === batch && stoppingGate) {
          stoppingReadStarted();
          await stoppingGate;
        }
        if (current === batch && lateSummaryGate) {
          const gate = lateSummaryGate;
          lateSummaryGate = null;
          await gate;
        }
        return send(current);
      case "execution-preview": {
        reviewPreviews++;
        assert.equal(input.expected_revision, current.revision);
        const rows = input.row_keys.map(row_key => {
          const ready = isNavigationReview || (row_key !== "row-2" && !(replayInput && row_key === "row-0"));
          return { row_key, ready, issues: ready ? [] : [{ message: "原成片已占用" }] };
        });
        const count = rows.filter(row => row.ready).length;
        const target = current.targets[0].target_count;
        return send({ batch_id: current.id, revision: current.revision, can_start: rows.every(row => row.ready), rows,
          shortfalls: [{ category_id: "12", target_count: target, created_count: count, shortfall: target - count }] });
      }
      case "start": {
        if (review === "rejected-replay") {
          if (!replayInput) {
            replayInput = input;
            return route.abort("failed");
          }
          assert.deepEqual(input, replayInput);
          return route.fulfill({ status: 409, json: {
            errcode: 18003, message: "发布条件已变化", data: null,
            error: { code: "publication_conflict", details: {
              issues: [{ row_key: "row-0", message: "检查后原成片已占用" }],
            } },
          } });
        }
        if (current === fresh || isNavigationReview) return route.fulfill({ status: 409, json: {
          errcode: 18003, message: "检查后原成片已占用", data: null,
          error: { code: "publication_conflict", details: { issues: [{ row_key: "row-0", message: "检查后原成片已占用" }] } },
        } });
        if (isDifferentStart && !foreignOriginalInput) {
          foreignOriginalInput = input;
          return route.abort("failed");
        }
        if (startReceipt) {
          if (input.request_key === startedInput.request_key) {
            assert.deepEqual(input, startedInput);
            return send(startReceipt);
          }
          assert.deepEqual(input, foreignOriginalInput);
          return route.fulfill({ status: 409, json: {
            errcode: 18003, message: "发布条件已变化，请重新检查", data: null,
            error: { code: "publication_conflict", details: {
              batch_id: current.id, current_revision: current.revision, execution_status: current.execution_status,
              ...(rejectsOriginalStart && { start_request_applied: false }),
            } },
          } });
        }
        startCount++;
        startedInput = input;
        assert.equal(input.expected_revision, current.revision);
        assert.deepEqual(input.included_row_keys, ["row-0", "row-1"]);
        assert.deepEqual(input.excluded_row_keys, ["row-2"]);
        assert.equal(input.allow_shortfall, true);
        assert.deepEqual(input.expected_shortfalls, [{ category_id: "12", shortfall: 2 }]);
        assert.equal(typeof input.request_key, "string");
        tasks = current.config.candidates.slice(0, 2).map((row, index) => ({
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
        startReceipt = { batch_id: current.id, revision: current.revision, execution_status: "running", first_started_at: now,
          task_ids: tasks.map(task => task.id), target_count: 4, created_count: 2, creation_shortfall: 2 };
        if (isDifferentStart) return send(startReceipt);
        lostStartRead = true;
        return route.abort("failed");
      }
      case "task": {
        if (isPublicResultReview) {
          const items = [...tasks];
          if (lateTaskGate) {
            const { entered, released } = lateTaskGate;
            lateTaskGate = null;
            entered();
            await released;
          }
          return send({ items, total: 21, page: Number(params.get("page")), page_size: Number(params.get("page_size")) });
        }
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
        resumePosts++;
        if (resumeReceipt) {
          assert.deepEqual(input, expectedResume);
          return resumePosts === 2 ? route.fulfill({ status: 429, json: {
            errcode: 99999, message: "请稍后再试", data: null,
          } }) : send(resumeReceipt);
        }
        expectedResume = input;
        nextRevision();
        current.execution_status = "running";
        current.allowed_actions = ["pause", "stop"];
        current.execution.phase = "waiting_interval";
        current.execution.pause = null;
        current.execution.next_group_at = "2026-11-25T02:01:00Z";
        resumeReceipt = control();
        return route.abort("failed");
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
        current.execution_status = "stopping";
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
async function createAndStart(name) {
  await page.goto(new URL("/publish-batches", base).href);
  await button("新建批次").click();
  await field("批次名称").fill(name);
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
  await heading(name).waitFor();
}
function startPosts() {
  return calls.filter(call => call.method === "POST" && call.endpoint.endsWith("/start")).length;
}
async function requestSettled() {
  await page.waitForLoadState("networkidle");
}
const regressions = {
  async "changed-revision"() {
    await createAndStart("启动前配置已变更");
    await requestSettled();
    assert.equal(freshCreateCount, 1);
    assert.equal(startPosts(), 0, "saving revision 4 must not authorize starting revision 5");
    assert.equal(reviewPreviews, 0);
  },
  async "abandoned-navigation"() {
    batch.targets[0].target_count = candidates.length;
    let release;
    hydrateGate = new Promise(resolve => { release = resolve; });
    await page.goto(new URL("/publish-batches", base).href);
    const hydrated = page.waitForRequest(request => new URL(request.url()).searchParams.has("output_ids"));
    await button("开始发布").click();
    await hydrated;
    await heading("受控批次").waitFor();
    await page.getByRole("button", { name: "返回", description: "返回列表" }).click();
    await heading("发布批次").waitFor();
    hydrateGate = null;
    release();
    await button("受控批次").click();
    await heading("受控批次").waitFor();
    await requestSettled();
    assert.equal(startPosts(), 0, "opening an ordinary detail must not reuse the abandoned start action");
    assert.equal(reviewPreviews, 0);
  },
  async "rejected-replay"() {
    await page.goto(detailURL);
    await button("开始发布").click();
    await dialog().getByText("确认按所选范围发布并保留目标缺口", { exact: true }).click();
    await button("确认开始发布", dialog()).click();
    await dialog().getByText("操作结果待确认，请核对原请求后继续", { exact: true }).waitFor();
    await button("核对原启动请求", dialog()).click();
    await dialog().getByText(/检查后原成片已占用/).waitFor({ timeout: 3000 });
    assert.equal(startPosts(), 2);
    await button("刷新批次", dialog()).click();
    await button("开始发布").click();
    await dialog().getByText(/视频甲.*原成片已占用/).waitFor();
    assert.equal(startPosts(), 2);
  },
  "different-start-rejected"() {
    return regressions["different-start"]();
  },
  async "different-start"() {
    await page.goto(detailURL);
    await button("开始发布").click();
    await dialog().getByText("确认按所选范围发布并保留目标缺口", { exact: true }).click();
    await button("确认开始发布", dialog()).click();
    await dialog().getByText("操作结果待确认，请核对原请求后继续", { exact: true }).waitFor();
    const stored = () => page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith("wt-media:publication-command")));
    const pending = await stored();
    assert.equal(pending.length, 1);

    // The server sees a saved edit and start from another session. The candidate
    // keys survive the edit, while the account, copy and settings have changed.
    batch.revision = "5";
    batch.config.candidates = batch.config.candidates.map((row, index) => index === 0
      ? { ...row, media_account_id: "382", content: { description: "另一会话已发布的配置" } }
      : row);
    batch.execution_setting = { ...batch.execution_setting, concurrency_limit: 3 };
    const other = await browser.newPage();
    await other.route("**/api/v1/**", routeCloud);
    await other.goto(detailURL);
    const result = await other.evaluate(async ({ batchID, input }) => {
      const response = await fetch("/api/v1/publication-batch/" + batchID + "/start", {
        method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input),
      });
      return response.json();
    }, { batchID: id, input: { ...foreignOriginalInput, request_key: "other-session-key", expected_revision: "5" } });
    assert.equal(result.data.revision, "6");
    await other.close();

    await button("核对原启动请求", dialog()).click();
    const message = rejectsOriginalStart ? "批次已由另一请求开始，请核对正式任务"
      : "批次已开始，Cloud 未确认原启动请求。请核对现有正式任务。";
    await dialog().getByText(message, { exact: true }).waitFor();
    await button("返回", dialog()).click();
    await dialog().waitFor({ state: "hidden" });
    await text("另一会话已发布的配置").waitFor();
    assert.deepEqual(await stored(), rejectsOriginalStart ? [] : pending, "only Cloud's explicit rejection can clear the original intent");
    if (rejectsOriginalStart) await button("暂停发布").waitFor();
    else await button("核对原启动请求").waitFor();
    assert.equal(await text("已核对原启动结果").count(), 0);
    assert.equal(startPosts(), 3);
    assert.equal(startCount, 1);
  },
  async "cross-tab"() {
    await page.goto(detailURL);
    const other = await browser.newPage();
    await other.route("**/api/v1/**", routeCloud);
    await other.goto(detailURL);
    const command = (tab, requestKey) => tab.evaluate(async ({ requestKey, id }) => {
      const { batchCommandJournal } = await import("/src/modules/publication/batchExecution.js");
      const journal = batchCommandJournal(location.origin, "cloud", "71", id);
      const operation = { action: "start", input: { request_key: requestKey, expected_revision: "4" } };
      try {
        await journal.save(operation);
        return "saved";
      } catch {
        await journal.clear(operation);
        return "blocked";
      }
    }, { requestKey, id });
    const key = ["wt-media:publication-command:v1", new URL(base).origin, "cloud", owner.id, id].join("|");
    await page.evaluate(async key => {
      await new Promise(acquired => {
        navigator.locks.request(key, () => new Promise(resolve => {
          window.releaseJournalLock = resolve;
          acquired();
        }));
      });
    }, key);
    const saves = Promise.all([command(page, "first"), command(other, "second")]);
    await page.bringToFront();
    let queued = 0;
    for (let attempt = 0; queued !== 2 && attempt < 30; attempt++) {
      queued = await page.evaluate(async key =>
        (await navigator.locks.query()).pending.filter(item => item.name === key).length, key);
      if (queued !== 2) await delay(100);
    }
    await page.evaluate(() => window.releaseJournalLock());
    const results = await saves;
    assert.equal(queued, 2, "both real tabs must wait for the shared command lock");
    assert.deepEqual([...results].sort(), ["blocked", "saved"]);
    const retainedKey = await other.evaluate(key => JSON.parse(localStorage.getItem(key)).input.request_key, key);
    assert.equal(retainedKey, results[0] === "saved" ? "first" : "second");
    await other.close();
  },
  async "failed-task-actions"() {
    const failed = {
      ...candidates[0], id: "150", revision: "6", batch_id: id, platform: "baidu", operator_id: owner.id,
      status: "failed", current_attempt_id: "90", execution_group_no: 1, created_at: now,
      can_cancel: false, allowed_actions: [],
      resources: { account_name: account.name, output_name: outputs[0].name },
    };
    tasks = [failed, {
      ...failed, ...candidates[1], id: "151", current_attempt_id: "91", status: "reviewing", allowed_actions: ["resolve"],
    }];
    for (const status of ["paused", "ended"]) {
      batch.execution_status = status;
      batch.allowed_actions = status === "paused" ? ["resume", "stop"] : ["copy"];
      batch.execution = {
        ...newExecution(), phase: status, first_started_at: now, created_count: 2,
        pause: status === "paused" ? { id: "3", kind: "manual", reason: "复核原尝试", paused_at: now, exceptions: [] } : null,
        ended_at: status === "ended" ? now : null, verification_required_count: 1, task_counts: { failed: 1, reviewing: 1 },
      };
      await page.goto(detailURL);
      const row = page.getByRole("row").filter({ hasText: "发布文案 0" });
      await row.getByRole("link", { name: "查看任务", exact: true }).click();
      await heading("发布任务详情").waitFor();
      await text("发布失败").waitFor();
      assert.equal(await button("重试").count(), 0, status + " batch denies retry");
      assert.equal(await button("放弃").count(), 0, status + " batch denies legacy abandon");
    }

    await page.getByRole("button", { name: "返回", description: "返回发布批次" }).click();
    await page.getByRole("link", { name: "补录结果", exact: true }).click();
    await heading("发布任务详情").waitFor();
    assert.equal(await button("补录结果").isEnabled(), true, "explicit resolution remains available in ended batches");

    const { batch_id, allowed_actions, ...standalone } = failed;
    tasks.push({ ...standalone, id: "152", content: { description: "独立失败任务" } });
    await page.goto(new URL("/publish", base).href);
    await heading("发布任务").waitFor();
    const standaloneRow = page.getByRole("row").filter({ hasText: "独立失败任务" });
    assert.equal(await button("重试", standaloneRow).isEnabled(), true, "standalone fail uses legacy fallback when the actions key is omitted");
    assert.equal(calls.filter(call => call.method === "POST" && call.endpoint.startsWith("/publication")).length, 0);
  },
  async "senior"() {
    actor = { ...owner, role: "senior_operator" };
    batch.execution_status = "paused";
    batch.allowed_actions = ["resume", "stop"];
    batch.execution = {
      ...newExecution(), phase: "paused", first_started_at: now, created_count: 1,
      task_counts: { uncertain: 1 }, pause: { id: "3", kind: "manual", reason: "合成任务核对", paused_at: now, exceptions: [] },
    };
    tasks = [{
      ...candidates[0], id: "150", revision: "6", batch_id: id, platform: "baidu", operator_id: owner.id,
      status: "uncertain", execution_group_no: 1, created_at: now, allowed_actions: ["resolve"],
      resources: { account_name: account.name, output_name: outputs[0].name },
    }];
    const link = detailURL + "&tab=tasks&task_status=uncertain&task_search=发布文案&task_page_size=20";
    await page.goto(link);
    await button("继续发布").waitFor();
    await button("终止发布").waitFor();
    await page.getByRole("link", { name: "核实结果", exact: true }).click();
    await heading("发布任务详情").waitFor();
    await button("核实结果").waitFor();
    await page.getByRole("button", { name: "返回", description: "返回发布批次" }).click();
    await button("继续发布").waitFor();
    const returned = new URL(page.url()).searchParams;
    assert.equal(returned.get("task_status"), "uncertain");
    assert.equal(returned.get("task_search"), "发布文案");
    assert.equal(returned.get("task_page_size"), "20");

    actor = { ...actor, id: "72" };
    batch.allowed_actions = [];
    await page.goto(link);
    await heading("受控批次").waitFor();
    await page.getByText("批次配置由所属运营处理", { exact: true }).waitFor();
    assert.equal(await button("继续发布").count(), 0);
    assert.equal(await button("终止发布").count(), 0);
    assert.equal(calls.filter(call => call.method === "POST" && call.endpoint.startsWith("/publication-batch/")).length, 0);
  },
  "manual-publication"() {
    return regressions["late-publication"](true);
  },
  async "late-publication"(publishBeforeManualSummary = false) {
    batch.targets[0].target_count = 60;
    batch.execution_status = "ended";
    batch.allowed_actions = ["copy"];
    batch.execution = {
      ...newExecution(), phase: "ended", first_started_at: now, ended_at: now, target_count: 60,
      created_count: 21, creation_shortfall: 39, target_shortfall: 60, verification_required_count: 1, task_counts: { reviewing: 1, cancelled: 20 },
    };
    tasks = [{
      ...candidates[0], id: "150", revision: "6", batch_id: id, platform: "baidu", operator_id: owner.id,
      status: "reviewing", execution_group_no: 1, created_at: now, allowed_actions: ["resolve"],
      resources: { account_name: account.name, output_name: outputs[0].name },
    }];
    const batchReads = () => calls.filter(call => call.endpoint === "/publication-batch/" + id);
    const taskReads = () => calls.filter(call => call.endpoint === "/publication-batch/" + id + "/task");
    const requestForBatch = () => page.waitForResponse(response => new URL(response.url()).pathname.endsWith("/publication-batch/" + id));
    const progress = page.getByRole("group", { name: "公开发布率", exact: true });
    const reviewing = page.getByText("待平台公开", { exact: true }).locator("..");
    await page.goto(detailURL + "&tab=tasks&task_search=发布文案&task_page=2&task_page_size=20");
    await progress.getByText(/0 \/ 60/).waitFor();
    await reviewing.getByText("1 条", { exact: true }).waitFor();
    const row = page.getByRole("row").filter({ hasText: "发布文案 0" });
    await row.getByText("平台审核中", { exact: true }).waitFor();
    assert.equal(taskReads().at(-1).params.get("page"), "2");

    // Hold a task read that started before the independent public check.
    // The ended summary can catch up while this response is still in flight.
    let entered, release;
    const taskEntered = new Promise(resolve => { entered = resolve; });
    lateTaskGate = { entered, released: new Promise(resolve => { release = resolve; }) };
    let releaseSummary;
    if (publishBeforeManualSummary) lateSummaryGate = new Promise(resolve => { releaseSummary = resolve; });
    const initialQuery = requestForBatch();
    await button("查询").click();
    await taskEntered;
    if (!publishBeforeManualSummary) await initialQuery;
    batch.execution = { ...batch.execution, published_count: 1, target_shortfall: 59, verification_required_count: 0, task_counts: { published: 1, cancelled: 20 } };
    tasks[0] = { ...tasks[0], status: "published", revision: "7", platform_work_id: "5900", allowed_actions: [] };
    if (publishBeforeManualSummary) {
      releaseSummary();
      await initialQuery;
    }
    await progress.getByText(/1 \/ 60/).waitFor({ timeout: 8000 });
    if (publishBeforeManualSummary) assert.equal(batchReads().length, 2, "the manual query must supply the terminal summary");
    await reviewing.getByText("0 条", { exact: true }).waitFor();
    assert.equal(taskReads().length, 2, "the final task read must not overlap the older one");
    release();
    await row.getByText("已发布", { exact: true }).waitFor();
    const settled = taskReads().at(-1).params;
    assert.equal(settled.get("search"), "发布文案");
    assert.equal(settled.get("page"), "2");
    assert.equal(settled.get("page_size"), "20");
    const counts = [batchReads().length, taskReads().length];
    await delay(5300);
    assert.deepEqual([batchReads().length, taskReads().length], counts, "ended batch with no reviews stops polling");

    const queried = requestForBatch();
    await button("查询").click();
    await queried;
    await row.getByText("已发布", { exact: true }).waitFor();
    assert.equal(taskReads().at(-1).params.get("page"), "2");
    const reset = Promise.all([requestForBatch(), page.waitForResponse(response =>
      new URL(response.url()).pathname.endsWith("/publication-batch/" + id + "/task"))]);
    await button("重置").click();
    await reset;
    assert.equal(taskReads().at(-1).params.get("search"), null);
    assert.equal(taskReads().at(-1).params.get("page"), "1");
    assert.equal(startPosts(), 0);
  },
  async "stopping"() {
    batch.execution_status = "stopping";
    batch.execution = { ...newExecution(), phase: "stopping", first_started_at: now, stopped_at: now };
    batch.allowed_actions = [];
    await page.goto(detailURL);
    await heading("受控批次").waitFor();
    const unknownLabelCount = await text("执行状态未知").count();
    const detailReads = () => calls.filter(call => call.endpoint === "/publication-batch/" + id).length;
    const initialReads = detailReads();
    let release;
    stoppingGate = new Promise(resolve => { release = resolve; });
    const polling = new Promise(resolve => { stoppingReadStarted = resolve; });
    await Promise.race([polling, delay(6500)]);
    assert.equal(detailReads(), initialReads + 1, "a stopping batch must continue reading state");
    await delay(5200);
    assert.equal(detailReads(), initialReads + 1, "a slow stopping read must not be superseded every five seconds");
    batch.execution_status = "cancelled";
    batch.execution = { ...batch.execution, phase: "ended", ended_at: now };
    stoppingGate = null;
    release();
    await text("结束时间").waitFor();
    assert.equal(unknownLabelCount, 0, "stopping is a known list and detail state");
    await delay(5200);
    assert.equal(detailReads(), initialReads + 1, "terminal state stops polling");
    await page.goto(new URL("/publish-batches?execution_status=stopping", base).href);
    await heading("发布批次").waitFor();
    assert.ok(calls.some(call => call.endpoint === "/publication-batch" && call.params.get("execution_status") === "stopping"));
  },
};
async function runBaseline() {
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
  batch.revision = "9"; // Later scheduler changes cannot alter the start receipt.
  await browser.close();
  await launch();
  await page.goto(detailURL);
  await button("核对原启动请求").click();
  await text("已核对原启动结果").waitFor();
  assert.equal(startPosts(), 2, "the original package is replayed only after explicit recovery");
  assert.equal(startCount, 1, "replay of the original receipt creates no additional tasks");
  assert.equal(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith("wt-media:publication-command")).length), 0);
  await text("本组已安排").waitFor();
  await text("公开发布率").waitFor();

  await button("暂停发布").click();
  await dialog().locator('[aria-label="暂停发布原因"] textarea, textarea[aria-label="暂停发布原因"]').fill("复核原场景");
  await button("确认暂停发布", dialog()).click();
  await button("继续发布").waitFor();
  await button("继续发布").click();
  await dialog().getByRole("link", { name: /任务 151.*结果待确认/ }).waitFor();
  await button("确认继续发布", dialog()).click();
  await dialog().getByText("操作结果待确认，请核对原请求后继续", { exact: true }).waitFor();
  const rateLimited = page.waitForResponse(response => response.url().endsWith("/resume") && response.status() === 429);
  await button("核对原继续请求", dialog()).click();
  await rateLimited;
  await dialog().getByText("操作结果待确认，请核对原请求后继续", { exact: true }).waitFor();
  await button("核对原继续请求", dialog()).click();
  await text("已继续发布").waitFor();
  await text("最早下一组").waitFor();
  assert.equal(resumePosts, 3);
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
  await text("终止收尾").first().waitFor();
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

  await createAndStart("保存成功后启动失败");
  await dialog().getByText(/检查后原成片已占用/).waitFor();
  assert.equal(freshCreateCount, 1);
  assert.equal(fresh.execution_status, "not_started");
  assert.equal(calls.filter(call => call.endpoint === "/publication-batch/" + fresh.id + "/start").length, 1);

  actor = { ...owner, id: "99", role: "admin" };
  await page.goto(detailURL);
  await heading("受控批次").waitFor();
  assert.equal(await button("开始发布").count(), 0);
  assert.equal(await button("暂停发布").count(), 0);
}
try {
  await launch();
  if (review) await regressions[review]();
  else await runBaseline();
  assert.deepEqual(exceptions, []);
  console.log("Publication batch execution browser checks passed" + (review ? ": " + review : ""));
} finally {
  await browser?.close();
  await rm(profile, { recursive: true, force: true });
}
