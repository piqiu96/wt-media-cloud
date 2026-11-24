const assert = require("node:assert/strict");
const { test } = require("node:test");
const path = require("node:path");
const { chromium } = require("playwright");
const { attachStore } = require("./previewFixture.cjs");

test("CHG108 reuses batch pages for startup, fixed groups, control and trusted results", async t => {
  const browser = await chromium.launch({ channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome", headless: true });
  t.after(() => browser.close());
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  page.setDefaultTimeout(12_000);
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  page.on("console", entry => { if (entry.type() === "error") console.error(entry.text()); });
  const { seed } = await import("../model.js");
  const data = seed();
  data.tasks = [];
  data.batches = [];
  data.plans = [];
  const store = await attachStore(page, data);
  const base = process.env.EXAMPLE_URL || "http://127.0.0.1:8768";
  const url = base + "/example/operations/publishing/batches?review=chg108";
  const shots = process.env.SCREENSHOT_DIR;
  const capture = name => shots && page.screenshot({ path: path.join(shots, `chg108-${name}.png`) });
  const button = (name, root = page) => root.getByRole("button", { name, exact: true });
  const detail = page.locator(".batch-detail");
  const dialog = page.getByRole("dialog");
  const row = name => page.getByRole("row").filter({ hasText: name });
  const open = async name => {
    await button(name).click();
    await detail.waitFor();
  };
  const back = async () => {
    await dialog.waitFor({ state: "hidden" });
    await page.getByRole("button", { name: "返回", description: "返回发布批次列表" }).click();
    await detail.waitFor({ state: "hidden" });
  };

  await page.goto(url);
  await button("新建批次").waitFor({ timeout: 5000 }).catch(async error => {
    console.error("preview diagnostics", errors, (await page.locator("body").innerText()).slice(0, 800));
    throw error;
  });
  await button("启动检查 · 保留目标缺口").waitFor();
  await capture("list");
  const opening = row("启动检查 · 保留目标缺口");
  assert.match(await opening.innerText(), /3 条候选 · 0 条任务/);
  await button("开始发布", opening).click();
  await dialog.getByText("本次发布", { exact: true }).waitFor();
  assert.equal(await button("确认开始发布", dialog).isEnabled(), false);
  assert.match(await dialog.innerText(), /文件不可取得/);
  await capture("startup-check");
  await dialog.getByText("确认按所选范围发布并保留缺口", { exact: true }).click();
  await button("确认开始发布", dialog).click();
  await dialog.waitFor({ state: "hidden" });
  const snapshot = (await store.get()).data;
  const started = snapshot.batches.find(batch => batch.previewSample === "start");
  assert.equal(started.targets[0].count, 4);
  assert.equal(snapshot.tasks.filter(task => task.batchId === started.id).length, 2);

  await open("分组执行 · 并发 5、可用 Profile 1");
  await detail.getByText("本组已安排 1 个独立账号，按可用 Profile 数成组执行", { exact: true }).waitFor();
  await capture("single-profile");
  await button("暂停发布").click();
  await button("确认暂停发布", dialog).click();
  await button("继续发布").waitFor();
  await button("继续发布").click();
  await button("确认继续发布", dialog).click();
  await button("暂停发布").waitFor();
  await back();

  await open("组间等待 · 审核尚未公开");
  await detail.getByText(/本组自动处理已结束。最早下一组/).waitFor();
  await capture("interval");
  await back();

  await open("异常暂停 · 保留人工现场");
  await detail.getByText(/本组处理结束，存在待处理异常/).waitFor();
  await capture("paused");
  await back();

  await open("终止收尾 · 原结果尚未知");
  await detail.getByText(/未领取任务已取消并保留成片占用/).waitFor();
  assert.equal(await button("结束批次").count(), 0);
  assert.equal(await button("继续发布").count(), 0);
  await capture("stopped-unknown");
  await back();

  await open("明确失败 · 可结束批次");
  await button("结束批次").click();
  await dialog.getByText(/保留 1 条明确失败及成片占用/).waitFor();
  await button("确认结束批次", dialog).click();
  await detail.getByText("已结束", { exact: true }).waitFor();
  assert.equal(await button("重试").count(), 0);
  await capture("finished-failure");
  await back();

  await open("可信公开 · 30 目标／28 任务／24 公开");
  await detail.getByText(/24 \/ 30/).waitFor();
  await capture("trusted-public");

  await page.setViewportSize({ width: 1024, height: 800 });
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1));
  await capture("narrow");
  await page.goto(url + "&role=admin");
  await open("分组执行 · 并发 5、可用 Profile 1");
  assert.equal(await button("暂停发布").count(), 0);
  assert.equal(await button("终止发布").count(), 0);
  await page.goto(url + "&role=operator");
  await button("普通运营 · 已保存未开始").waitFor();
  assert.equal(await button("分组执行 · 并发 5、可用 Profile 1").count(), 0);
  const own = row("普通运营 · 已保存未开始");
  await button("开始发布", own).click();
  await own.getByText("执行中", { exact: true }).waitFor();
  assert.equal(await dialog.count(), 0);
  assert.deepEqual(errors, []);
});
