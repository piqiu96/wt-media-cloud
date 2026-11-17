// Run against the local example service with Playwright available in NODE_PATH.
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

async function isolatedPage(handler) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const page = await context.newPage();
  const writes = [];
  await page.addInitScript(() => localStorage.setItem('review-note', '保留我的评审记录'));
  await page.route('**/__example/publishing/**', async route => {
    if (route.request().method() !== 'GET') writes.push(route.request().postData());
    await handler(route);
  });
  return { context, page, writes };
}

test('failed initialization shows a retry without saving seed data, and reload restores saved records', async () => {
  let fail = true;
  const saved = seed();
  saved.tasks[0].title = '已有任务恢复验证';
  const { context, page, writes } = await isolatedPage(async route => {
    await route.fulfill({ status: fail ? 422 : 200, contentType: 'application/json', body: JSON.stringify(fail ? { error: '本地任务记录暂时不可读' } : { revision: 7, data: saved }) });
  });
  try {
    await page.goto(base + '/example/?view=publishing');
    await page.getByRole('heading', { name: '发布管理加载失败' }).waitFor({ timeout: 3000 });
    assert.match(await page.getByRole('alert').innerText(), /本地任务记录暂时不可读/);
    assert.equal(await page.getByText('正在加载任务…').count(), 0);
    assert.equal(writes.length, 0, 'a failed read must not write seed data');
    assert.equal(await page.evaluate(() => localStorage.getItem('review-note')), '保留我的评审记录');
    if (process.env.SCREENSHOT_DIR) await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, '02-loading-error.png') });
    fail = false;
    await page.getByRole('button', { name: '重新加载' }).click();
    await page.getByText('已有任务恢复验证', { exact: true }).waitFor({ timeout: 5000 });
    assert.equal(await page.getByText('正在加载任务…').count(), 0);
    if (process.env.SCREENSHOT_DIR) await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, '03-recovered-records.png') });
  } finally { await context.close(); }
});

test('an unresponsive snapshot stops loading and leaves existing data untouched', { timeout: 25000 }, async () => {
  const { context, page, writes } = await isolatedPage(() => new Promise(() => {}));
  try {
    await page.goto(base + '/example/?view=publishing');
    await page.getByRole('heading', { name: '发布管理加载失败' }).waitFor({ timeout: 18000 });
    assert.match(await page.getByRole('alert').innerText(), /超时/);
    assert.equal(writes.length, 0);
    assert.equal(await page.evaluate(() => localStorage.getItem('review-note')), '保留我的评审记录');
  } finally { await context.close(); }
});

test('browser storage failures are visible and do not overwrite existing records', async () => {
  const { context, page, writes } = await isolatedPage(route => route.fulfill({ contentType: 'application/json', body: 'null' }));
  try {
    await page.addInitScript(() => {
      Object.defineProperty(window, 'indexedDB', { value: { open() { throw new Error('浏览器禁止读取本地记录'); } } });
    });
    await page.goto(base + '/example/?view=publishing');
    await page.getByRole('heading', { name: '发布管理加载失败' }).waitFor({ timeout: 3000 });
    assert.match(await page.getByRole('alert').innerText(), /浏览器禁止读取本地记录/);
    assert.equal(writes.length, 0);
    assert.equal(await page.evaluate(() => localStorage.getItem('review-note')), '保留我的评审记录');
  } finally { await context.close(); }
});
