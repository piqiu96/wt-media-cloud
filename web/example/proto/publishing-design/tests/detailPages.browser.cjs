const {requestBody}=require('./previewFixture.cjs');
// Run against the example service, with Playwright available in NODE_PATH.
const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const { pathToFileURL } = require('node:url');
const path = require('node:path');
const base = process.env.EXAMPLE_URL || 'http://127.0.0.1:8768';
let browser, seed;
before(async () => {
  ({ seed } = await import(pathToFileURL(path.join(__dirname, '../model.js'))));
  browser = await chromium.launch({ headless: true, ...(process.env.CHROMIUM_EXECUTABLE ? { executablePath: process.env.CHROMIUM_EXECUTABLE } : {}) });
});
after(async () => browser?.close());
async function isolatedPage(t, viewport = { width: 1440, height: 1000 }) {
  const context = await browser.newContext({ viewport });
  t.after(() => context.close());
  const page = await context.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  t.after(() => assert.deepEqual(errors, [], 'details must not produce browser errors'));
  let snapshot = { revision: 1, data: seed() };
  // Keep all prototype persistence isolated from the reviewer's saved records.
  await page.route('**/__example/publishing/**', route => {
    if (route.request().method() !== 'GET') {
      const input = requestBody(route.request());
      snapshot = { revision: snapshot.revision + 1, data: input.data };
    }
    return route.fulfill({ contentType: 'application/json', body: JSON.stringify(snapshot) });
  });
  return page;
}
async function screenshot(page, name) {
  if (process.env.SCREENSHOT_DIR) await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, name) });
}
async function noDetailOverlay(page) {
  assert.equal(await page.locator('[role="dialog"]:visible, .t-drawer:visible').count(), 0);
}

test('task details use a standalone page, survive refresh and return to the filtered task list', async t => {
  const page = await isolatedPage(t);
  await page.goto(base + '/example/?view=publishing#tasks');
  await page.getByPlaceholder('搜索任务名称或 ID').fill('三角洲');
  await page.getByRole('button', { name: '查询', exact: true }).click();
  await page.getByRole('row').filter({ hasText: '1001' }).getByRole('button', { name: '详情', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor({ timeout: 3000 });
  await noDetailOverlay(page);
  assert.equal(new URL(page.url()).pathname, '/example/operations/publishing/tasks/1001');
  await screenshot(page, '01-task-detail.png');
  await page.getByRole('button', { name: '返回', exact: true }).click();
  assert.equal(await page.getByPlaceholder('搜索任务名称或 ID').inputValue(), '三角洲');
  assert.equal(new URL(page.url()).pathname, '/example/operations/publishing/tasks');
  await screenshot(page, '02-task-list-return.png');
  await page.goto(base + '/example/?view=publishing#tasks?task=1001');
  await page.reload();
  await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor();
});

test('batch details use the same return control and retain list search', async t => {
  const page = await isolatedPage(t);
  await page.goto(base + '/example/?view=publishing#batches');
  await page.getByPlaceholder('批次名称或编号').fill('首轮');
  await page.getByRole('button', { name: '查询', exact: true }).click();
  await page.getByRole('button', { name: '小红书 · 首轮内容', exact: true }).click();
  await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor({ timeout: 3000 });
  await noDetailOverlay(page);
  assert.equal(new URL(page.url()).pathname, '/example/operations/publishing/batches/201');
  const footer = page.locator('.resource-detail-page > footer');
  await footer.getByRole('button', { name: '停止发布', exact: true }).waitFor({ timeout: 1500 });
  assert.equal(await page.locator('.resource-section__header').getByRole('button', { name: '停止发布', exact: true }).count(), 0);
  const footerBox = await footer.boundingBox();
  assert.ok(footerBox && footerBox.y >= 900 && footerBox.y + footerBox.height <= 1001);
  await screenshot(page, '03-batch-detail.png');
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await screenshot(page, '07-batch-detail-lower.png');
  await page.evaluate(() => window.scrollTo(0, 0));
  await footer.getByRole('button', { name: '停止发布', exact: true }).click();
  const confirmation = page.locator('.t-dialog:visible');
  await confirmation.waitFor();
  assert.match(await confirmation.innerText(), /停止发布/);
  await confirmation.getByRole('button', { name: '返回', exact: true }).click();
  await confirmation.waitFor({ state: 'hidden' });
  await page.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByPlaceholder('批次名称或编号').waitFor();
  assert.equal(await page.getByPlaceholder('批次名称或编号').inputValue(), '首轮');
  assert.equal(new URL(page.url()).pathname, '/example/operations/publishing/batches');
  await screenshot(page, '04-batch-list-return.png');
});

test('cross-object details support browser history and return preserves the source context', async t => {
  const page = await isolatedPage(t);
  await page.goto(base + '/example/?view=publishing#batches?batch=201');
  await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor({ timeout: 3000 });
  await page.getByRole('row').filter({ hasText: 'T-1001' }).getByRole('button', { name: '详情', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor();
  await page.goBack();
  await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor();
  await page.goForward();
  await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor();
  await page.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor();
  assert.equal(new URL(page.url()).pathname, '/example/operations/publishing/batches/201');
  await page.goto(base + '/example/?view=publishing#tasks?task=1001');
  await page.getByRole('button', { name: '小红书 · 首轮内容', exact: true }).click();
  await page.getByRole('dialog',{name:'关联批次',exact:true}).waitFor();
  assert.match(page.url(), /tasks\/1001/);
  await page.getByRole('button', { name: '前往批次详情', exact: true }).click();
  await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor();
  await page.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByPlaceholder('批次名称或编号').waitFor();
});

test('both detail pages keep the return button visible on a narrow viewport', async t => {
  const page = await isolatedPage(t, { width: 390, height: 844 });
  for (const [hash, title, back, shot] of [
    ['tasks?task=1001', '发布任务详情', '返回', '05-task-mobile.png'],
    ['batches?batch=201', '发布批次详情', '返回', '06-batch-mobile.png'],
  ]) {
    await page.goto(base + '/example/?view=publishing#' + hash);
    await page.getByRole('heading', { name: title, exact: true }).waitFor({ timeout: 3000 });
    const box = await page.getByRole('button', { name: back, exact: true }).boundingBox();
    assert.ok(box && box.x >= 0 && box.x + box.width <= 390 && box.y > 700 && box.y + box.height <= 844);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    await screenshot(page, shot);
  }
});
