import assert from "node:assert/strict";
import { chromium } from "playwright";
import { mkdir } from "node:fs/promises";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const out = "/private/tmp/chg109-production-frontend";
await mkdir(out, { recursive: true });
const actor = {
  id: "9",
  role: "operator",
  team_id: "7",
  status: "enabled",
  category_ids: ["1"],
  username: "operator",
};
const goal = { platform: "baidu", category_id: "1", target_count: 2 };
function fixture() {
  return {
    plan: {
      id: "3",
      name: "周期目标计划",
      date_start: "2026-11-25",
      date_end: "2026-11-26",
      note: "",
      goals: [goal],
      revision: "1",
      status: "not_started",
      progress: "preparing",
      owner_id: "9",
      published_count: 0,
      goal_results: [{ ...goal, published_count: 0, shortfall: 2 }],
      batch_count: 2,
      processed_batch_count: 0,
      allowed_actions: ["edit", "cancel", "start"],
    },
    batches: [1, 2].map((i) => ({
      batch_id: String(i),
      row_key: "batch-" + i,
      configured: true,
      allowed_actions: ["edit", "stop"],
      batch: {
        id: String(i),
        name: "批次0" + i,
        platform: "baidu",
        plan_id: "3",
        plan_name: "周期目标计划",
        operator_id: "9",
        revision: "1",
        targets: [{ category_id: "1", target_count: 1 }],
        config: {
          candidates: [
            {
              row_key: "r" + i,
              category_id: "1",
              media_account_id: "9",
              composite_output_id: String(40 + i),
              content: { description: "内容" + i, confirmed: true },
            },
          ],
        },
        execution_status: "not_started",
        configuration_status: "ready",
        execution_setting: {
          concurrency_limit: 5,
          group_interval_seconds: 0,
          between_group_policy: "auto_continue",
          exception_task_limit: 3,
        },
        allowed_actions: ["edit", "stop"],
        readiness: { ready: true },
      },
    })),
    issues: [],
  };
}
let started = 0;
for (const role of (process.env.WT_PLAN_CREATE_ONLY ? [] : ["operator", "senior_operator", "admin"])) {
  const page = await browser.newPage({
    viewport: { width: role === "admin" ? 1024 : 1440, height: 1000 },
  });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  let d = fixture(),
    originalStart;
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request(),
      url = new URL(req.url()),
      p = url.pathname;
    const send = (data) =>
      route.fulfill({
        json: { errcode: 0, message: "success", data, logid: "fixture" },
      });
    if (p === "/api/v1/auth/me") return send({ ...actor, role });
    if (p === "/api/v1/categories")
      return send([{ id: "1", name: "三角洲行动", enabled: true }]);
    if (p === "/api/v1/publication-plan")
      return send({
        items: [
          {
            ...d.plan,
            allowed_actions: role === "admin" ? [] : d.plan.allowed_actions,
          },
        ],
        total: 1,
        page: 1,
        page_size: 10,
      });
    if (p === "/api/v1/publication-plan/3")
      return send({
        ...d,
        plan: {
          ...d.plan,
          allowed_actions: role === "admin" ? [] : d.plan.allowed_actions,
        },
        batches: d.batches.map((r) =>
          role === "admin"
            ? {
                ...r,
                allowed_actions: [],
                batch: { ...r.batch, allowed_actions: [] },
              }
            : r,
        ),
      });
    if (p === "/api/v1/media-accounts")
      return send({ items: [{ id: "9", name: "本人百家号" }], total: 1 });
    if (p === "/api/v1/publication/output")
      return send({
        items: [
          { composite_output_id: "41", name: "原成片一" },
          { composite_output_id: "42", name: "原成片二" },
        ],
        total: 2,
      });
    if (p.endsWith("/record")) return send({ items: [], total: 0 });
    if (role === "admin" && /^\/api\/v1\/publication-batch\/[12]$/.test(p))
      return route.fulfill({
        status: 404,
        json: {
          errcode: 18002,
          message: "模拟批次不可读",
          error: { code: "publication_not_found" },
        },
      });
    if (/^\/api\/v1\/publication-batch\/[12]$/.test(p))
      return send(d.batches[Number(p.at(-1)) - 1].batch);
    if (p.endsWith("/execution-preview")) {
      const id = p.split("/")[4],
        b = d.batches[Number(id) - 1].batch;
      return send({
        batch_id: b.id,
        revision: b.revision,
        can_start: true,
        rows: [{ row_key: "r" + id, ready: true }],
        shortfalls: [{ category_id: "1", shortfall: 0 }],
      });
    }
    if (p === "/api/v1/publication-plan/3/start") {
      started++;
      const input = req.postDataJSON();
      assert.equal(input.batch_confirmations.length, 2);
      assert.deepEqual(
        input.batch_confirmations.map((c) => c.included_row_keys),
        [["r1"], ["r2"]],
      );
      d.plan = {
        ...d.plan,
        revision: "2",
        status: "running",
        progress: "running",
        started_at: "2026-11-25T04:00:00Z",
        current_batch_id: "1",
        allowed_actions: ["pause", "cancel"],
      };
      d.batches.forEach((r) => {
        r.allowed_actions = [];
        r.batch.allowed_actions = [];
      });
      if (role === "operator" && !originalStart) {
        originalStart = input;
        return route.abort("failed");
      }
      if (originalStart) assert.deepEqual(input, originalStart);
      return send({
        plan_id: "3",
        revision: "2",
        first_started_at: d.plan.started_at,
        current_batch_id: "1",
        batch_confirmations: input.batch_confirmations,
        first_batch_receipt: {
          batch_id: "1",
          revision: "2",
          execution_status: "running",
          first_started_at: d.plan.started_at,
          task_ids: ["11"],
          created_count: 1,
          target_count: 1,
          creation_shortfall: 0,
        },
      });
    }
    return send({ items: [], total: 0 });
  });
  await page.goto(base + "/publish-plans");
  await page.getByRole("heading", { name: "发布计划", exact: true }).waitFor();
  await page.getByRole("button", { name: "周期目标计划", exact: true }).click();
  await page.getByRole("heading", { name: "发布计划详情" }).waitFor();
  await page.getByText("批次01", { exact: false }).first().waitFor();
  await page.screenshot({
    path: out + "/" + role + "-detail.png",
    fullPage: true,
  });
  if (role === "admin") {
    assert.equal(
      await page.getByRole("button", { name: "执行", exact: true }).count(),
      0,
    );
    assert.equal(
      await page.getByRole("button", { name: "编辑配置", exact: true }).count(),
      0,
    );
    await page
      .getByRole("button", { name: "详情", exact: true })
      .nth(1)
      .click();
    await page.getByText("模拟批次不可读", { exact: false }).waitFor();
    assert.notEqual(
      await page.locator(".content-area").evaluate((e) => e.style.overflow),
      "hidden",
    );
    await page.locator(".content-area").evaluate((e) => (e.scrollTop = 0));
    assert.equal(
      await page.locator(".content-area").evaluate((e) => e.scrollTop),
      0,
    );
  } else {
    await page
      .getByRole("button", { name: "详情", exact: true })
      .nth(1)
      .scrollIntoViewIfNeeded();
    const oldURL = page.url();
    const oldScroll = await page
      .locator(".content-area")
      .evaluate((e) => e.scrollTop);
    await page
      .getByRole("button", { name: "详情", exact: true })
      .nth(1)
      .click();
    await page.getByRole("dialog", { name: "发布批次详情" }).waitFor();
    assert.equal(
      await page.getByRole("dialog").getByText("正式发布任务 0 条").count(),
      1,
    );
    await page
      .getByRole("dialog")
      .locator(".t-drawer__body")
      .evaluate((e) => (e.scrollTop = 0));
    await page.getByRole("button", { name: "关闭发布批次详情" }).waitFor();
    const bounds = await page.getByRole("dialog").boundingBox();
    assert.ok(bounds.y >= 45 && bounds.y < 100);
    assert.ok(bounds.height <= 1000);
    await page.screenshot({ path: out + "/" + role + "-drawer.png" });
    await page.keyboard.press("Escape");
    await page.getByRole("dialog").waitFor({ state: "hidden" });
    assert.equal(page.url(), oldURL);
    console.log(
      "scroll",
      role,
      oldScroll,
      await page.locator(".content-area").evaluate((e) => e.scrollTop),
    );
    await page.waitForFunction(
      (y) =>
        Math.abs(document.querySelector(".content-area").scrollTop - y) < 2,
      oldScroll,
      { timeout: 2000 },
    );
    await page.getByRole("button", { name: "执行", exact: true }).click();
    await page.getByText("确认全部批次范围及目标缺口").click();
    await page.getByRole("button", { name: "确认执行", exact: true }).click();
    if (role === "operator")
      await page
        .getByRole("dialog")
        .getByRole("button", { name: "核对原计划请求", exact: true })
        .click();
    await page.getByRole("button", { name: "暂停计划", exact: true }).waitFor();
    assert.equal(
      await page.getByRole("button", { name: "编辑配置", exact: true }).count(),
      0,
    );
  }
  assert.deepEqual(errors, []);
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
    false,
  );
  await page.close();
}

// Actual new-plan editing, shared SSE framing and unknown creation read-back.
{
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  let created,
    posts = 0,
    recommendCalls = 0;
  await page.route("**/api/v1/**", async (route) => {
    const q = route.request(),
      u = new URL(q.url()),
      p = u.pathname;
    const send = (data) =>
      route.fulfill({
        json: { errcode: 0, message: "success", data, logid: "newplan" },
      });
    if (p === "/api/v1/auth/me") return send(actor);
    if (p === "/api/v1/categories")
      return send([{ id: "1", name: "三角洲行动", enabled: true }]);
    if (p === "/api/v1/publication-plan/recommend") {
      recommendCalls++;
      const input = q.postDataJSON();
      assert.equal(input.goals[0].target_count, 4);
      assert.equal("resource_counts" in input, false);
      const result = {
        batches: [
          {
            row_key: "ai-first",
            name: "AI首批",
            platform: "baidu",
            group_order: 1,
            batch_order: 1,
            targets: [{ category_id: "1", target_count: 4 }],
          },
        ],
        resource_as_of: "2026-11-25T04:00:00Z",
        issues: [],
      };
      const frame = (e, data) =>
        "event: " + e + "\ndata: " + JSON.stringify(data) + "\n\n";
      return route.fulfill({
        contentType: "text/event-stream",
        body:
          frame("start", { total: 1 }) +
          frame("item_result", {
            row_key: "plan",
            input_revision: "1",
            result,
          }) +
          frame("done", { total: 1, success: 1, failed: 0 }),
      });
    }
    if (p === "/api/v1/publication-plan" && q.method() === "POST") {
      posts++;
      if (posts === 1)
        return route.fulfill({
          status: 400,
          json: {
            errcode: 18001,
            message: "首次配置明确无效",
            error: { code: "publication_invalid" },
          },
        });
      const input = q.postDataJSON();
      assert.equal(input.batches[0].name, "AI首批");
      created = {
        ...fixture(),
        plan: {
          ...fixture().plan,
          id: "13",
          name: input.name,
          goals: input.goals,
          goal_results: input.goals.map((g) => ({
            ...g,
            published_count: 0,
            shortfall: g.target_count,
          })),
          allowed_actions: ["edit", "cancel"],
        },
      };
      return route.abort("failed");
    }
    if (p === "/api/v1/publication-plan")
      return send({
        items: created ? [created.plan] : [],
        total: created ? 1 : 0,
      });
    if (p === "/api/v1/publication-plan/13") return send(created);
    if (p === "/api/v1/media-accounts")
      return send({ items: [{ id: "9", name: "本人百家号" }], total: 1 });
    if (p === "/api/v1/publication/output")
      return send({
        items: [
          { composite_output_id: "41", name: "原成片一" },
          { composite_output_id: "42", name: "原成片二" },
        ],
        total: 2,
      });
    if (p.endsWith("/record")) return send({ items: [], total: 0 });
    return send({ items: [], total: 0 });
  });
  await page.goto(base + "/publish-plans?view=create");
  await page.getByLabel("计划名称", { exact: true }).last().fill("关闭仍保留的计划");
  await page.getByRole("button", { name: "返回", exact: true }).click();
  await page.getByRole("button", { name: "确认离开", exact: true }).click();
  await page.getByRole("button", { name: "新建发布计划", exact: true }).click();
  let recovery = page.locator(".t-dialog").filter({ hasText: "发现未完成的发布计划配置" });
  await recovery.locator(".t-dialog__close").click();
  assert.equal(new URL(page.url()).searchParams.has("view"), false);
  await page.getByRole("button", { name: "新建发布计划", exact: true }).click();
  await page.getByRole("button", { name: "继续编辑", exact: true }).click();
  assert.equal(await page.getByLabel("计划名称", { exact: true }).last().inputValue(), "关闭仍保留的计划");
  await page.getByRole("button", { name: "返回", exact: true }).click();
  await page.getByRole("button", { name: "新建发布计划", exact: true }).click();
  await page.getByRole("button", { name: "重新开始", exact: true }).click();
  assert.equal(await page.getByLabel("计划名称", { exact: true }).last().inputValue(), "");
  await page.getByLabel("计划名称", { exact: true }).last().fill("新周期计划");
  await page.getByLabel("开始日期").fill("2026-11-25");
  await page.getByLabel("结束日期").fill("2026-11-26");
  await page.getByLabel("发布类目", { exact: true }).click();
  await page.getByText("三角洲行动", { exact: true }).last().click();
  await page.keyboard.press("Escape");
  await page
    .locator('.t-input-number[aria-label="三角洲行动目标数"] input')
    .first()
    .fill("4");
  await page
    .locator('.t-input-number[aria-label="三角洲行动目标数"] input')
    .first()
    .blur();
  await page.getByRole("button", { name: "AI 推荐拆批", exact: true }).click();
  await page.getByRole("button", { name: "采用建议", exact: true }).click();
  await page.getByRole("button", { name: "创建计划", exact: true }).click();
  await page.getByText("首次配置明确无效", { exact: false }).waitFor();
  assert.equal(
    await page.getByLabel("计划名称", { exact: true }).last().isEnabled(),
    true,
  );
  await page
    .getByLabel("计划名称", { exact: true })
    .last()
    .fill("修正后周期计划");
  await page.getByRole("button", { name: "创建计划", exact: true }).click();
  await page
    .getByRole("button", { name: "核对原创建请求", exact: true })
    .waitFor();
  await page
    .getByRole("button", { name: "核对原创建请求", exact: true })
    .click();
  await page.getByRole("heading", { name: "发布计划详情" }).waitFor();
  assert.equal(posts, 2);
  assert.equal(recommendCalls, 1);
  await page.screenshot({ path: out + "/create-recovered.png" });
  await page.close();
}
assert.equal(started, process.env.WT_PLAN_CREATE_ONLY ? 0 : 3);
await browser.close();
console.log(
  "CHG109 production plans: three roles, startup frozen scope and read-only drawer PASS",
);
