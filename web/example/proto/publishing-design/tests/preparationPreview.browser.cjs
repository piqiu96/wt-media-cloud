const assert = require("node:assert/strict");
const { test } = require("node:test");
const path = require("node:path");
const { chromium } = require("playwright");
const { attachStore } = require("./previewFixture.cjs");

test("CHG107 preview saves the candidate batch and shows zero formal tasks", async t => {
  const browser = await chromium.launch({ channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome", headless: true });
  t.after(() => browser.close());
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const exceptions = [];
  page.on("pageerror", error => exceptions.push(error.message));
  const { seed } = await import("../model.js");
  const data = seed();
  data.tasks = [];
  data.batches = [];
  data.plans = [];
  const store = await attachStore(page, data);
  const base = process.env.EXAMPLE_URL || "http://127.0.0.1:8768";
  const screenshot = name => process.env.SCREENSHOT_DIR && page.screenshot({
    path: path.join(process.env.SCREENSHOT_DIR, `chg107-${name}.png`),
    fullPage: true,
  });
  const button = name => page.getByRole("button", { name, exact: true });
  await page.goto(base + "/example/operations/publishing/batches?review=chg107");
  await button("新建批次").click();
  await page.getByPlaceholder("请输入批次名称", { exact: true }).fill("CHG107 预览批次");
  await page.locator(".batch-category-chip").filter({ hasText: "三角洲行动" }).click();
  await page.locator(".batch-matching-table tbody tr").first().waitFor();
  await screenshot("resources");
  await button("下一步").click();
  await page.locator(".batch-content-workspace").waitFor();
  await screenshot("content");
  await button("下一步").click();
  await page.locator('[aria-label="异常任务上限"] input').fill("3");
  await screenshot("overview");
  await button("保存批次").click();
  await page.getByRole("heading", { name: "候选配置", exact: true }).waitFor();
  assert.equal(await page.getByText("正式发布任务", { exact: true }).count(), 1);
  assert.equal(await button("执行").count(), 0);
  await screenshot("detail");
  const { data: persisted } = await store.get();
  assert.equal(persisted.batches.length, 1);
  assert.ok(persisted.batches[0].candidates.length > 0);
  assert.equal(persisted.batches[0].executionSettings.exceptionLimit, 3);
  assert.equal(persisted.tasks.length, 0);
  assert.equal(new URL(page.url()).searchParams.get("review"), "chg107");
  assert.deepEqual(exceptions, []);
});
