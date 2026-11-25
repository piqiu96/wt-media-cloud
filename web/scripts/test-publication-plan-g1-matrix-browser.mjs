// Additional G1 checks against controlled HTTP facts; not real platform E2E or final UI acceptance.
import assert from "node:assert/strict";
import { chromium } from "playwright";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const errors = [],
  calls = [];
page.on("pageerror", (e) => errors.push(e.message));
const actor = {
  id: "9",
  role: "operator",
  team_id: "7",
  status: "enabled",
  username: "operator",
  category_ids: ["1", "2"],
};
const cats = [
    { id: "1", name: "游戏", enabled: true },
    { id: "2", name: "资讯", enabled: true },
  ],
  time = "2026-11-25T04:00:00Z";
let started = false;
let plan = {
  id: "3",
  revision: "1",
  name: "矩阵计划",
  owner_id: "9",
  date_start: "2026-11-25",
  date_end: "2026-11-26",
  note: "",
  status: "not_started",
  progress: "preparing",
  published_count: 0,
  batch_count: 2,
  processed_batch_count: 0,
  goals: cats.map((c) => ({
    platform: "baidu",
    category_id: c.id,
    target_count: 1,
  })),
  allowed_actions: ["edit", "cancel"],
};
let batches = [1, 2].map((i) => ({
  id: String(i),
  revision: "1",
  name: "配置批次" + i,
  plan_id: "3",
  plan_name: plan.name,
  operator_id: "9",
  platform: "baidu",
  targets: [{ category_id: String(i), target_count: 1 }],
  config: {
    accounts: [{ media_account_id: "11", limit: 2 }],
    output_ids: [String(40 + i)],
    manual_pairs: [],
    candidates: [
      {
        row_key: "r" + i,
        category_id: String(i),
        media_account_id: "11",
        composite_output_id: String(40 + i),
        input_revision: "1",
        field_revisions: {},
        content: {
          description: "发布内容" + i,
          cover_file_id: "31",
          portrait_cover_file_id: "32",
          declaration: "original",
          confirmed: true,
        },
      },
    ],
  },
  execution_status: "not_started",
  configuration_status: "incomplete",
  allowed_actions: ["edit", "stop"],
  candidate_count: 1,
  execution_setting: {
    concurrency_limit: 1,
    group_interval_seconds: 0,
    between_group_policy: "auto_continue",
    exception_task_limit: 1,
  },
  readiness: { ready: false, issues: [], shortfalls: [] },
  execution: {
    phase: "not_started",
    verification_required_count: 0,
    task_counts: {},
    target_count: 1,
    created_count: 0,
    published_count: 0,
    target_shortfall: 1,
  },
}));
let planPut = 0,
  batchPuts = 0;
const goalResults = () =>
  plan.goals.map((g) => ({
    ...g,
    published_count: 0,
    shortfall: g.target_count,
  }));
const task = (id = "21") => ({
  id,
  revision: "7",
  batch_id: "1",
  operator_id: "9",
  category_id: "1",
  media_account_id: "11",
  composite_output_id: "41",
  platform: "baidu",
  platform_account_uid: "123",
  current_attempt_id: "70",
  platform_submission_id: "18001",
  status: "reviewing",
  allowed_actions: ["resolve"],
  content: { description: "可筛选任务" + id },
  resources: { account_name: "本人账号", output_name: "成片" + id },
});
await page.route("**/api/v1/**", async (route) => {
  const r = route.request(),
    u = new URL(r.url()),
    p = u.pathname,
    m = r.method();
  calls.push({
    p,
    m,
    q: Object.fromEntries(u.searchParams),
    body: m === "GET" ? null : r.postDataJSON(),
  });
  const send = (data) =>
    route.fulfill({
      json: { errcode: 0, message: "success", data, logid: "g1-matrix" },
    });
  if (p === "/api/v1/auth/me") return send(actor);
  if (p === "/api/v1/categories") return send(cats);
  if (p === "/api/v1/publication-plan" && m === "GET")
    return send({
      items: [{ ...plan, goal_results: goalResults() }],
      total: 1,
    });
  if (p === "/api/v1/publication-plan/3/structure" && m === "PUT") {
    const input = r.postDataJSON();
    planPut++;
    assert.equal(input.expected_revision, plan.revision);
    assert.deepEqual(
      input.batches.map((b) => [b.batch_id, b.row_key]),
      [
        ["1", "stable-1"],
        ["2", "stable-2"],
      ],
    );
    for (const g of input.goals)
      assert.equal(
        input.batches
          .filter((b) => b.platform === g.platform)
          .reduce(
            (n, b) =>
              n +
              (b.targets.find((t) => t.category_id === g.category_id)
                ?.target_count || 0),
            0,
          ),
        g.target_count,
      );
    plan = { ...plan, ...input, revision: String(Number(plan.revision) + 1) };
    for (const b of input.batches) {
      Object.assign(
        batches.find((x) => x.id === b.batch_id),
        { targets: b.targets, name: b.name },
      );
    }
    return send({ plan, batches: [], issues: [] });
  }
  if (p === "/api/v1/publication-plan/3")
    return send({
      plan: {
        ...plan,
        goal_results: goalResults(),
        ...(started
          ? {
              status: "running",
              progress: "waiting_verification",
              started_at: time,
              current_batch_id: "1",
              allowed_actions: [],
            }
          : {}),
      },
      batches: batches.map((b, i) => ({
        batch_id: b.id,
        row_key: "stable-" + b.id,
        group_order: 1,
        batch_order: i + 1,
        configured: b.configuration_status === "ready",
        allowed_actions: started ? [] : ["edit"],
        batch: b,
      })),
      issues: [],
    });
  if (p === "/api/v1/publication/account")
    return send({
      items: [
        {
          id: "11",
          name: "本人账号",
          eligible: true,
          platform: "baidu",
          category_ids: ["1", "2"],
        },
      ],
      total: 1,
    });
  if (p === "/api/v1/media-accounts")
    return send({
      items: [
        {
          id: "11",
          name: "本人账号",
          platform: "baidu",
          category_ids: ["1", "2"],
        },
      ],
      total: 1,
    });
  if (p === "/api/v1/publication/output")
    return send({
      items: [1, 2].map((i) => ({
        composite_output_id: String(40 + i),
        name: "本人素材" + i,
        category_id: String(i),
        team_id: "7",
        platforms: ["baidu"],
        occupied: false,
        video: { status: "ready" },
      })),
      total: 2,
    });
  if (p === "/api/v1/publication/resource")
    return send({
      as_of: time,
      account_count: 1,
      eligible_account_count: 1,
      output_count: 1,
      available_output_count: 1,
      unknown_output_count: 0,
      availability: "available",
      gaps: [],
    });
  if (p === "/api/v1/publication-batch/validate")
    return send({
      ready: true,
      issues: [],
      shortfalls: [],
      cover_resources: [],
    });
  const cm = p.match(/^\/api\/v1\/publication-batch\/([12])\/configuration$/);
  if (cm) {
    const input = r.postDataJSON(),
      b = batches.find((b) => b.id === cm[1]);
    batchPuts++;
    assert.equal(m, "PUT");
    assert.equal(input.expected_revision, b.revision);
    assert.deepEqual(input.targets, b.targets);
    Object.assign(b, input, {
      revision: String(Number(b.revision) + 1),
      configuration_status: "ready",
      readiness: { ready: true, issues: [], shortfalls: [] },
    });
    if (batches.every((b) => b.configuration_status === "ready"))
      plan.allowed_actions = ["edit", "start", "cancel"];
    return send(b);
  }
  const bm = p.match(/^\/api\/v1\/publication-batch\/([12])$/);
  if (bm) return send(batches.find((b) => b.id === bm[1]));
  if (p === "/api/v1/publication-batch/1/task")
    return send({
      items: [task(u.searchParams.get("page") === "2" ? "22" : "21")],
      total: 25,
    });
  if (/^\/api\/v1\/publication-batch\/[12]\/task$/.test(p))
    return send({ items: [], total: 0 });
  if (p.endsWith("/record")) {
    const second = u.searchParams.get("page") === "2";
    return send({
      items: [
        {
          id: second ? "102" : "101",
          created_at: second ? "2026-11-24T04:00:00Z" : time,
          event: "publication_batch.configure",
          actor_id: "9",
        },
      ],
      total: second || started ? 11 : 1,
    });
  }
  if (p === "/api/v1/publication/21" || p === "/api/v1/publication/22")
    return send(task(p.endsWith("/22") ? "22" : "21"));
  if (p.endsWith("/attempt")) return send([]);
  if (p === "/api/v1/publication") return send({ items: [task()], total: 1 });
  if (m !== "GET") throw Error("Unexpected mutation " + p);
  return send({ items: [], total: 0 });
});
const button = (name) => page.getByRole("button", { name, exact: true });
const number = (label) =>
  page.locator(`.t-input-number[aria-label="${label}"] input`).first();
const region = () =>
  page.getByRole("region", { name: "计划目标与批次表，可横向滚动" }).last();
const planRow = (i) => region().locator("tbody tr").nth(i);
async function select(label, value) {
  const field = page.locator("label").filter({ hasText: label }).first();
  await field.locator(".t-select-input").click();
  await page
    .locator(".t-select-option")
    .getByText(value, { exact: true })
    .click();
  await page.keyboard.press("Escape");
}
try {
  await page.goto(base + "/publish-plans");
  await page.locator("tr").filter({ hasText: "矩阵计划" }).waitFor();
  await page
    .locator("label")
    .filter({ hasText: "综合搜索" })
    .locator("input")
    .first()
    .fill("筛选计划");
  await select("计划状态", "已结束");
  await select("发布平台", "百家号");
  await select("发布类目", "游戏");
  const filterResponse = page.waitForResponse(
    (r) =>
      new URL(r.url()).pathname === "/api/v1/publication-plan" &&
      new URL(r.url()).searchParams.get("search") === "筛选计划",
  );
  await button("查询").click();
  await filterResponse;
  await page.waitForURL(/search=/);
  const filtered = calls
    .filter((c) => c.p === "/api/v1/publication-plan")
    .at(-1);
  assert.deepEqual(filtered.q, {
    search: "筛选计划",
    status: "ended",
    platform: "baidu",
    category_id: "1",
    page: "1",
    page_size: "10",
  });
  const resetResponse = page.waitForResponse(
    (r) =>
      new URL(r.url()).pathname === "/api/v1/publication-plan" &&
      !new URL(r.url()).searchParams.has("search"),
  );
  await button("重置").click();
  await resetResponse;
  await page.waitForURL((url) => !url.searchParams.has("search"));
  assert.equal(
    calls.filter((c) => c.p === "/api/v1/publication-plan").at(-1).q.search,
    undefined,
  );
  await page
    .locator("tr")
    .filter({ hasText: "矩阵计划" })
    .getByRole("button", { name: "编辑", exact: true })
    .click();
  await page
    .getByLabel("计划名称", { exact: true })
    .last()
    .fill("结构保存计划");
  await number("游戏目标数").fill("2");
  await number("游戏目标数").blur();
  await button("保存修改").click();
  await page
    .getByText("批次分目标须与每格计划目标一致", { exact: true })
    .waitFor();
  assert.equal(planPut, 0);
  await number("配置批次1游戏数量").fill("2");
  await number("配置批次1游戏数量").blur();
  await button("保存修改").click();
  await page
    .getByRole("heading", { name: "发布计划详情", exact: true })
    .waitFor();
  assert.equal(planPut, 1);
  // Both inherited batch configurations use the original 107 page and save without a start/task call.
  for (let i = 0; i < 2; i++) {
    await planRow(i)
      .getByRole("button", { name: "编辑配置", exact: true })
      .click();
    assert.equal(
      await number((i === 0 ? "游戏" : "资讯") + "发布目标").isDisabled(),
      true,
    );
    await button("下一步").click();
    await button("下一步").click();
    assert.equal(
      await button("开始发布").count(),
      0,
      "linked plan batch cannot offer independent start",
    );
    await button("保存并返回计划").click();
    await page
      .getByRole("heading", { name: "发布计划详情", exact: true })
      .waitFor();
  }
  assert.equal(batchPuts, 2);
  assert.equal(await button("执行").count(), 1);
  assert.ok(
    batches.every(
      (b) =>
        b.execution.created_count === 0 && b.execution_status === "not_started",
    ),
  );
  assert.equal(
    calls.filter((c) => c.m === "POST" && /\/start$|\/publication$/.test(c.p))
      .length,
    0,
  );
  // Read-only drawer shares formal task filters, pagination and history with original batch detail.
  started = true;
  batches.forEach((b) => {
    b.execution_status = "ended";
    b.allowed_actions = [];
    b.execution = {
      ...b.execution,
      phase: "ended",
      first_started_at: time,
      ended_at: time,
      created_count: 25,
      verification_required_count: 1,
      task_counts: { reviewing: 1 },
    };
  });
  await page.goto(
    base + "/publish-plans?view=detail&id=3&search=keep&page=2&page_size=20",
  );
  assert.equal(await button("编辑配置").count(), 0);
  assert.equal(await button("编辑").count(), 0);
  await planRow(0)
    .getByRole("button", { name: "详情", exact: true })
    .scrollIntoViewIfNeeded();
  const scroll = await page
      .locator(".content-area")
      .evaluate((e) => e.scrollTop),
    url = page.url();
  await planRow(0).getByRole("button", { name: "详情", exact: true }).click();
  const drawer = page.getByRole("dialog", { name: "发布批次详情" });
  await drawer.waitFor();
  await drawer
    .locator('[aria-label="搜索任务"] input, input[aria-label="搜索任务"]')
    .first()
    .fill("可筛选任务");
  await drawer.getByRole("button", { name: "查询", exact: true }).click();
  await page.waitForResponse(
    (r) =>
      new URL(r.url()).pathname.endsWith("/1/task") &&
      new URL(r.url()).searchParams.get("search") === "可筛选任务",
  );
  await drawer
    .locator(".batch-tab .t-pagination")
    .getByText("2", { exact: true })
    .click();
  await drawer.getByText("可筛选任务22", { exact: true }).waitFor();
  assert.equal(calls.filter((c) => c.p.endsWith("/1/task")).at(-1).q.page, "2");
  await drawer.getByRole("button", { name: "重置", exact: true }).click();
  await drawer.getByText("可筛选任务21", { exact: true }).waitFor();
  assert.equal(
    calls.filter((c) => c.p.endsWith("/1/task")).at(-1).q.search,
    undefined,
  );
  const recordSection = drawer.getByRole("region", { name: "操作记录" });
  await recordSection
    .locator(".t-pagination")
    .getByText("2", { exact: true })
    .click();
  await page.waitForResponse(
    (r) =>
      new URL(r.url()).pathname.endsWith("/1/record") &&
      new URL(r.url()).searchParams.get("page") === "2",
  );
  await page
    .getByRole("button", { name: "关闭发布批次详情", exact: true })
    .click();
  await drawer.waitFor({ state: "hidden" });
  assert.equal(page.url(), url);
  await page.waitForFunction(
    (y) => Math.abs(document.querySelector(".content-area").scrollTop - y) < 2,
    scroll,
  );
  await planRow(0).getByRole("button", { name: "去处理", exact: true }).click();
  await page.waitForURL(
    (u) =>
      u.pathname === "/publish-batches" &&
      u.searchParams.get("from_plan") === "3",
  );
  const bq = new URL(page.url()).searchParams;
  assert.equal(bq.get("from_plan"), "3");
  assert.equal(bq.get("plan_search"), "keep");
  assert.equal(bq.get("plan_page"), "2");
  assert.equal(bq.get("plan_page_size"), "20");
  assert.equal(Number(bq.get("plan_scroll")), scroll);
  await page
    .getByRole("link", { name: "补录结果", exact: true })
    .first()
    .click();
  await page
    .getByRole("heading", { name: "发布任务详情", exact: true })
    .waitFor();
  await button("返回").click();
  await page.getByRole("heading", { name: "配置批次1", exact: true }).waitFor();
  await button("返回").click();
  await page
    .getByRole("heading", { name: "发布计划详情", exact: true })
    .waitFor();
  await page.waitForFunction(
    (y) => Math.abs(document.querySelector(".content-area").scrollTop - y) < 2,
    scroll,
  );
  assert.equal(new URL(page.url()).searchParams.get("search"), "keep");
  assert.equal(new URL(page.url()).searchParams.get("page"), "2");
  assert.deepEqual(errors, []);
  console.log(
    "G1 plan matrix: filter/reset, guarded structure PUT, two inherited config saves without tasks, freeze, formal drawer filter/page/records, task-batch-plan return with scroll PASS",
  );
} finally {
  await browser.close();
}
