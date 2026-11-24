const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const path = require('node:path');
const { attachStore } = require('./previewFixture.cjs');

const base = process.env.EXAMPLE_URL;
const plans = '/example/operations/publishing/plans';
const planID = id => 'PL-109-' + id;
const route = (id, role = '') => base + plans + (id ? '/' + planID(id) : '') + '?review=chg109' + (role ? '&role=' + role : '');
let browser, seed, addPlanReviewScenarios, migrateRelationships;
before(async () => {
  ({ seed } = await import('../model.js'));
  ({ migrateRelationships } = await import('../relationships.js'));
  ({ addPlanReviewScenarios } = await import('../scenarios.js'));
  browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE });
});
after(() => browser?.close());

async function open(t, id, role) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' });
  const page = await context.newPage(), errors = [];
  page.on('pageerror', error => errors.push(error.message));
  t.after(async () => { await context.close(); assert.deepEqual(errors, []); });
  page.setDefaultTimeout(9000);
  const db = migrateRelationships(seed());
  addPlanReviewScenarios(db);
  const store = await attachStore(page, db);
  await page.goto(route(id, role));
  await page.getByRole('heading', { name: id ? '发布计划详情' : '发布计划', exact: true }).waitFor();
  return { page, store };
}
async function screenshot(page, name) {
  if (process.env.SCREENSHOT_DIR) await page.screenshot({
    path: path.join(process.env.SCREENSHOT_DIR, 'chg109-' + name + '.png'), animations: 'disabled',
  });
}
const button = (page, name) => page.getByRole('button', { name, exact: true });
const batchRows = page => page.locator('#batch-area tbody tr');
async function loadPlan(page, id) {
  await page.goto(route(id));
  await page.getByText(planID(id), { exact: true }).first().waitFor();
}

test('configures from the original batch, freezes all batch input on explicit plan startup, and previews read-only', async t => {
  const { page, store } = await open(t, 'PREPARE');
  assert.ok(await button(page, '执行').isDisabled());
  assert.match(await page.locator('.plan-scope').innerText(), /全部待执行批次的配置：批次 02/);
  await screenshot(page, 'prepare');
  await button(batchRows(page).nth(1), '去配置').click();
  await page.waitForURL(/publishing\/batches\/PL-109-PREPARE-B2.*review=chg109/);
  await page.getByRole('heading', { name: "编辑发布批次", exact: true }).waitFor();
  assert.equal(await button(page, '开始发布').count(), 0);
  await page.locator('button[aria-description=返回发布计划]').click();
  await page.waitForURL(route('PREPARE'));

  await loadPlan(page, 'READY');
  assert.equal((await store.get()).data.tasks.filter(task => String(task.batchId).startsWith(planID('READY'))).length, 0);
  await button(page, '执行').click();
  const confirmation = page.getByRole('dialog').filter({ hasText: '确认启动发布计划' });
  await confirmation.waitFor();
  assert.match(await confirmation.innerText(), /冻结计划目标、批次顺序及全部批次/);
  await screenshot(page, 'startup');
  await button(confirmation, '确认启动').click();
  await page.getByText('按批次顺序自动发布', { exact: true }).waitFor();
  const stored = (await store.get()).data;
  const first = stored.batches.find(row => row.id === planID('READY') + '-B1');
  const future = stored.batches.find(row => row.id === planID('READY') + '-B2');
  assert.ok(first.firstStartedAt);
  assert.equal(first.planFrozenAt, future.planFrozenAt);
  assert.equal(future.firstStartedAt, undefined);
  assert.equal(stored.tasks.filter(task => task.batchId === first.id).length, 1);
  assert.equal(stored.tasks.filter(task => task.batchId === future.id).length, 0);
  assert.deepEqual(await batchRows(page).nth(1).getByRole('button').allTextContents(), ['详情']);

  const trigger = button(batchRows(page).nth(1), '详情');
  await trigger.scrollIntoViewIfNeeded();
  await trigger.focus();
  await screenshot(page, 'batch-progression-table');
  const before = await page.evaluate(() => scrollY);
  await trigger.click();
  const drawer = page.getByRole('dialog', { name: '发布批次详情', exact: true });
  await drawer.waitFor();
  assert.match(await drawer.innerText(), /批次 02 · 下一批/);
  assert.equal(await drawer.getByRole('button', { name: /去配置|编辑配置|开始发布|终止发布/ }).count(), 0);
  await screenshot(page, 'readonly-batch');
  await page.keyboard.press('Escape');
  await drawer.waitFor({ state: 'hidden' });
  assert.equal(page.url(), route('READY'));
  await page.waitForFunction(el => document.activeElement === el, await trigger.elementHandle());
  assert.ok(Math.abs(await page.evaluate(() => scrollY) - before) < 3);
});

test('shows real plan gates and sends stopped verification to the original task', async t => {
  const { page, store } = await open(t, '10');
  assert.match(await batchRows(page).first().innerText(), /9 \/ 10/);
  assert.equal(await button(page, '确认进入下一批').count(), 0);
  await screenshot(page, 'auto-10');
  for (const [id, label, action] of [
    ['20', '当前批次失败率超过 10%', '确认进入下一批'],
    ['ZERO', '当前批次没有可计算的发布结果', '确认跳过当前批次'],
    ['PENDING', '有待处理任务', '去处理'],
    ['PAUSED', '计划已暂停', '恢复计划'],
    ['DUPLICATE', '与批次 02 · 下一批第1行使用同平台成片', '执行'],
  ]) {
    await loadPlan(page, id);
    await page.getByText(label, { exact: false }).first().waitFor();
    assert.ok(await button(page, action).first().isVisible());
    if (id === 'DUPLICATE') assert.ok(await button(page, action).isDisabled());
    await screenshot(page, id.toLowerCase());
  }
  await loadPlan(page, 'CHECK');
  await page.getByText('自动核验已停止，待人工核实', { exact: true }).waitFor();
  assert.equal(await button(page, '确认进入下一批').count(), 0);
  await screenshot(page, 'stopped-check');
  await button(page, '去处理').first().click();
  await page.waitForURL(/publishing\/batches\/PL-109-CHECK-B1.*fromPlan=PL-109-CHECK/);
  await button(page, '核实结果').click();
  const dialog = page.getByRole('dialog', { name: '核实发布结果', exact: true });
  await dialog.waitFor();
  await screenshot(page, 'original-verification');
  await button(dialog, '取消').click();
  await page.locator('button[aria-description=返回发布计划]').click();
  await page.waitForURL(route('CHECK'));
  assert.equal((await store.get()).data.plans.find(plan => plan.id === planID('CHECK')).position, 0);

  await loadPlan(page, 'PUBLIC');
  await page.getByText('本轮执行已结束，等待平台公开或核实', { exact: true }).waitFor();
  assert.equal(await button(page, '确认结束').count(), 0);
  await screenshot(page, 'pending-public');
  await loadPlan(page, 'FINISH');
  assert.equal(await button(page, '创建补量批次').count(), 0);
  await button(page, '确认结束').click();
  const close = page.getByRole('dialog').filter({ hasText: '确认结束发布计划' });
  await close.locator('textarea').fill('合成演示已核对为未公开，按缺口收尾');
  await screenshot(page, 'finish-shortfall');
  await button(close, '确认结束').click();
  await page.getByText('未达成结束', { exact: true }).first().waitFor();
});

test('preserves operator and administrator scope in the existing plan list', async t => {
  const { page } = await open(t, null, 'operator');
  await page.getByText('普通运营 · 本人已配置计划', { exact: true }).waitFor();
  assert.ok(await button(page, '新建发布计划').isVisible());
  assert.equal(await page.getByText('首启前 · 全部批次已配置', { exact: true }).count(), 0);
  await screenshot(page, 'operator-list');
  await page.goto(route('READY', 'admin'));
  await page.getByRole('heading', { name: '发布计划详情', exact: true }).waitFor();
  assert.equal(await button(page, '执行').count(), 0);
  assert.deepEqual(await batchRows(page).nth(1).getByRole('button').allTextContents(), ['详情']);
  await page.goto(route(null, 'admin'));
  await page.getByRole('heading', { name: '发布计划', exact: true }).waitFor();
  assert.equal(await button(page, '新建发布计划').count(), 0);
  await page.setViewportSize({ width: 1024, height: 900 });
  await screenshot(page, 'admin-list-1024');
});
