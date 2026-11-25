import assert from "node:assert/strict";
import { chromium } from "playwright";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const page = await browser.newPage({ viewport: { width: 1024, height: 900 } });
const errors = [];
page.on("pageerror", (e) => errors.push(e.message));
const actor = {
  id: "9",
  role: "operator",
  team_id: "7",
  status: "enabled",
  category_ids: ["1", "2", "3", "4"],
  username: "operator",
};
const categories = ["A类目", "B类目", "C未知", "D失败"].map((name, i) => ({
  id: String(i + 1),
  name,
  enabled: true,
}));
let readScopes = [],
  saved,
  failedRead = true;
const original = {
  id: "8",
  name: "固定批次计划",
  revision: "1",
  status: "not_started",
  progress: "preparing",
  owner_id: "9",
  date_start: "2026-11-25",
  date_end: "2026-11-26",
  goals: [1, 2].map((i) => ({
    platform: "baidu",
    category_id: String(i),
    target_count: 1,
  })),
  allowed_actions: ["edit"],
  published_count: 0,
  goal_results: [],
  batch_count: 2,
};
const originalRows = [1, 2].map((i) => ({
  batch_id: String(i),
  row_key: "stable-" + i,
  group_order: 1,
  batch_order: i,
  configured: i === 2,
  batch: {
    id: String(i),
    name: i === 1 ? "已终止固定批" : "原已配置批",
    platform: "baidu",
    execution_status: i === 1 ? "cancelled" : "not_started",
    targets: [{ category_id: String(i), target_count: 1 }],
  },
}));
await page.route("**/api/v1/**", async (route) => {
  const req = route.request(),
    url = new URL(req.url()),
    p = url.pathname;
  const send = (data) =>
    route.fulfill({
      json: { errcode: 0, message: "success", data, logid: "editor" },
    });
  if (p === "/api/v1/auth/me") return send(actor);
  if (p === "/api/v1/categories") return send(categories);
  if (p === "/api/v1/publication/resource") {
    assert.equal(url.searchParams.has("owner"), false);
    assert.equal(url.searchParams.has("operator_id"), false);
    const id = url.searchParams.get("category_id");
    readScopes.push(id);
    if (id === "4" && failedRead)
      return route.fulfill({
        status: 503,
        json: { errcode: 18001, message: "盘点失败" },
      });
    return send({
      as_of: "2026-11-25T04:00:00Z",
      account_count: id === "2" ? 0 : 3,
      eligible_account_count: id === "2" ? 0 : 2,
      output_count: id === "1" ? 4 : 0,
      available_output_count: id === "1" ? 4 : 0,
      unknown_output_count: id === "3" ? 2 : 0,
      availability: id === "1" ? "available" : id === "3" ? "unknown" : "empty",
      gaps:
        id === "2"
          ? ["eligible_account_missing", "available_output_missing"]
          : id === "3"
            ? ["output_availability_unknown"]
            : [],
    });
  }
  if (p === "/api/v1/publication-plan/8")
    return send({ plan: original, batches: originalRows, issues: [] });
  if (p === "/api/v1/publication-plan/validate") {
    saved = req.postDataJSON();
    return send({ valid: true, issues: [] });
  }
  if (p === "/api/v1/publication-plan") return send({ items: [], total: 0 });
  return send({ items: [], total: 0 });
});
const button = (name) => page.getByRole("button", { name, exact: true });
const matrix = () =>
  page.getByRole("region", { name: "批次规划表，可横向滚动" });
const row = (i) => matrix().locator("tbody tr").nth(i);
const num = (label) =>
  page.locator(`.t-input-number[aria-label="${label}"] input`).first();
async function setNumber(label, v) {
  await num(label).fill(String(v));
  await num(label).blur();
}
async function choose(name, close = true) {
  await page.getByLabel("发布类目", { exact: true }).click();
  await page
    .locator(".t-select-option")
    .getByText(name, { exact: true })
    .click();
  if (close) await page.keyboard.press("Escape");
}
async function checkedArrangement() {
  const response = page.waitForResponse(
    (r) => new URL(r.url()).pathname === "/api/v1/publication-plan/validate",
  );
  await button("创建计划").click();
  await response;
  await page.getByRole("dialog", {name:"确认创建发布计划",exact:true}).getByRole("button",{name:"取消",exact:true}).click();
  return structuredClone(saved);
}
try {
  await page.goto(base + "/publish-plans?view=create");
  await page.locator(".plan-editor-section").last().waitFor();
  const sections = await page
    .locator(".plan-editor-section")
    .evaluateAll(async (elements) => {
      await new Promise((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(resolve)),
      );
      return elements.map((element) => {
        const bounds = element.getBoundingClientRect(),
          style = getComputedStyle(element);
        return {
          top: bounds.top,
          bottom: bounds.bottom,
          marginTop: style.marginTop,
          marginBottom: style.marginBottom,
        };
      });
    });
  assert.equal(sections.length, 4);
  for (let index = 1; index < sections.length; index++) {
    const gap = sections[index].top - sections[index - 1].bottom;
    assert.ok(
      Math.abs(gap - 18) < 1,
      `Editor card gap must be the prototype's 18px, without collapsed or doubled spacing (${gap})`,
    );
    assert.equal(sections[index].marginTop, "0px");
    assert.equal(sections[index - 1].marginBottom, "0px");
  }
  await button("创建计划").click();
  const missingName = page.getByText("请填写计划名称（最多100字）", { exact: true });
  await missingName.waitFor();
  await page
    .getByLabel("计划名称", { exact: true })
    .last()
    .fill("手工目标矩阵");
  await missingName.waitFor({ state: "hidden" });
  await page.getByPlaceholder("开始日期", { exact: true }).fill("2026-11-25");
  await page.getByPlaceholder("结束日期", { exact: true }).fill("2026-11-26");
  await page.getByPlaceholder("结束日期", { exact: true }).press("Enter");
  await page.getByRole("heading", { name: "基础信息", exact: true }).click();
  await choose("A类目");
  await choose("B类目");
  await button("按目标生成一批").click();
  await row(0).waitFor();
  await row(0).getByRole("button", { name: "拆分", exact: true }).click();
  assert.equal(await matrix().locator("tbody tr").count(), 2);
  assert.equal(await num("批次 01A类目数量").inputValue(), "1");
  assert.equal(await num("批次 01B类目数量").inputValue(), "0");
  assert.equal(await num("批次 01 · 拆分B类目数量").inputValue(), "1");
  const beforeMerge = await checkedArrangement();
  await row(0).getByRole("button", { name: "合并下一批", exact: true }).click();
  assert.equal(await matrix().locator("tbody tr").count(), 1);
  const merged = await checkedArrangement();
  assert.equal(merged.batches[0].row_key, beforeMerge.batches[0].row_key);
  assert.deepEqual(merged.batches[0].targets, [
    { category_id: "1", target_count: 1 },
    { category_id: "2", target_count: 1 },
  ]);
  await row(0).getByRole("button", { name: "拆分", exact: true }).click();
  const splitAgain = await checkedArrangement();
  assert.equal(splitAgain.batches[0].row_key, beforeMerge.batches[0].row_key);
  await row(1).getByRole("button", { name: "上移", exact: true }).click();
  assert.equal(
    await page
      .locator('[aria-label="批次1名称"] input,input[aria-label="批次1名称"]')
      .first()
      .inputValue(),
    "批次 01 · 拆分",
  );
  await row(0).getByRole("button", { name: "下移", exact: true }).click();
  const restoredOrder = await checkedArrangement();
  assert.deepEqual(
    restoredOrder.batches.map((b) => b.row_key),
    splitAgain.batches.map((b) => b.row_key),
  );
  await choose("C未知");
  await choose("D失败");
  const inventory = page.getByRole("region", {
    name: "本人资源盘点，可横向滚动",
  });
  await inventory
    .getByText("盘点读取失败，请重新盘点", { exact: true })
    .waitFor();
  assert.match(await inventory.innerText(), /暂无可用账号/);
  assert.match(await inventory.innerText(), /部分成片状态未知/);
  assert.match(await inventory.innerText(), /2026/);
  assert.equal(await page.getByText("容量未知", { exact: true }).count(), 1);
  await setNumber("A类目目标数", 6);
  await inventory
    .getByText("素材缺口 2 条（目标保持 6 条）", { exact: true })
    .waitFor();
  await setNumber("A类目目标数", 1);
  await button("新增批次").click();
  await setNumber("批次 03C未知数量", 1);
  await setNumber("批次 03D失败数量", 1);
  await checkedArrangement();
  assert.deepEqual(
    saved.goals.map((g) => g.target_count),
    [1, 1, 1, 1],
  );
  assert.deepEqual(
    saved.batches.map((b) => b.targets),
    [
      [{ category_id: "1", target_count: 1 }],
      [{ category_id: "2", target_count: 1 }],
      [
        { category_id: "3", target_count: 1 },
        { category_id: "4", target_count: 1 },
      ],
    ],
  );
  assert.equal(new Set(saved.batches.map((b) => b.row_key)).size, 3);
  failedRead = false;
  await button("重新盘点").click();
  await inventory
    .getByText("盘点读取失败，请重新盘点", { exact: true })
    .waitFor({ state: "hidden" });
  // Removing a target explicitly prompts; all other goal cells remain unchanged.
  await choose("D失败", false);
  await button("确认移除").click();
  assert.equal(await num("批次 03D失败数量").count(), 0);
  assert.equal(await num("批次 03C未知数量").inputValue(), "1");
  await page.screenshot({
    path: "/private/tmp/chg109-production-frontend/editor-matrix-1024.png",
    fullPage: true,
  });
  assert.equal(
    readScopes.every((id) => ["1", "2", "3", "4"].includes(id)),
    true,
  );
  // Fresh editor keeps cancelled rows anchored; existing configuration needs explicit approval.
  await button("返回").click();
  await button("确认离开").click();
  await page.goto(base + "/publish-plans?view=edit&id=8");
  await row(0).waitFor();
  assert.equal(
    await page
      .locator('[aria-label="批次1名称"] input, input[aria-label="批次1名称"]')
      .first()
      .isDisabled(),
    true,
  );
  assert.equal(
    await row(1)
      .getByRole("button", { name: "上移", exact: true })
      .isDisabled(),
    true,
  );
  assert.equal(
    await row(0)
      .getByRole("button", { name: "删除", exact: true })
      .isDisabled(),
    true,
  );
  await setNumber("原已配置批B类目数量", 2);
  await page.getByText("调整已配置批次？", { exact: true }).waitFor();
  await page
    .locator(".resource-feedback-dialog")
    .getByRole("button", { name: "取消", exact: true })
    .click();
  await page
    .getByText("调整已配置批次？", { exact: true })
    .waitFor({ state: "hidden" });
  assert.equal(await num("原已配置批B类目数量").inputValue(), "1");
  await setNumber("原已配置批B类目数量", 2);
  await button("确认调整").click();
  assert.equal(await num("原已配置批B类目数量").inputValue(), "2");
  await choose("A类目");
  await page
    .getByText("已终止批次的目标范围不可移除", { exact: true })
    .waitFor();
  assert.equal(await num("A类目目标数").inputValue(), "1");
  // Exercise the shared component's old default contract and the new opt-in anchor controls.
  await page.evaluate(async () => {
    const source = await (
      await fetch("/src/shared/ui/resource/ResourceReorderActions.vue")
    ).text();
    const vueURL = source.match(/from\s+["']([^"']*\/vue\.js[^"']*)["']/)?.[1];
    if (!vueURL) throw Error("Vue runtime import missing");
    const { createApp, h, reactive } = await import(vueURL);
    const { default: Reorder } =
      await import("/src/shared/ui/resource/ResourceReorderActions.vue");
    const node = document.createElement("div");
    node.id = "reorder-compatibility-probe";
    node.style.cssText =
      "position:fixed;left:260px;top:60px;z-index:10000;background:white;padding:16px";
    document.body.append(node);
    const values = reactive({
      itemKey: "stable",
      scope: "compatibility",
      index: 1,
      count: 3,
    });
    const events = [];
    const app = createApp({
      render: () =>
        h(Reorder, {
          ...values,
          onMove: (d) => events.push(d),
          onRemove: () => events.push("remove"),
        }),
    });
    app.mount(node);
    window.__reorderCompatibility = { values, events, app, node };
  });
  const probe = page.locator("#reorder-compatibility-probe");
  assert.equal(
    await probe.getByRole("button", { name: "上移", exact: true }).isEnabled(),
    true,
  );
  assert.equal(
    await probe.getByRole("button", { name: "下移", exact: true }).isEnabled(),
    true,
  );
  await probe.getByRole("button", { name: "上移", exact: true }).click();
  await probe.getByRole("button", { name: "下移", exact: true }).click();
  assert.deepEqual(
    await page.evaluate(() => window.__reorderCompatibility.events),
    [-1, 1],
  );
  await page.evaluate(() =>
    Object.assign(window.__reorderCompatibility.values, {
      moveUpDisabled: true,
      moveDownDisabled: true,
      removeDisabled: true,
    }),
  );
  assert.equal(
    await probe.getByRole("button", { name: "上移", exact: true }).isDisabled(),
    true,
  );
  assert.equal(
    await probe.getByRole("button", { name: "下移", exact: true }).isDisabled(),
    true,
  );
  await page.evaluate(() => {
    window.__reorderCompatibility.app.unmount();
    window.__reorderCompatibility.node.remove();
    delete window.__reorderCompatibility;
  });
  assert.deepEqual(errors, []);
  console.log(
    "Plan editor: manual allocation, mixed one-item split, per-cell conservation, four inventory states, fixed rows and configured adjustment guard PASS",
  );
} finally {
  await browser.close();
}
