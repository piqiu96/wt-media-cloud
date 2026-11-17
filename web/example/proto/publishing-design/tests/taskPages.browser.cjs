const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const {attachStore}=require('./previewFixture.cjs');
const { pathToFileURL } = require('node:url');
const path = require('node:path');
const base = process.env.EXAMPLE_URL || 'http://127.0.0.1:8768';
let browser, seed, declarationOptions;
before(async () => {
  ({ seed, declarationOptions } = await import(pathToFileURL(path.join(__dirname, '../model.js'))));
  browser = await chromium.launch({ executablePath:process.env.CHROMIUM_EXECUTABLE, headless: true });
});
after(async () => browser?.close());
async function setup(t, viewport = { width: 1440, height: 1000 }, mutate = () => {}) {
  const context = await browser.newContext({ viewport });
  t.after(() => context.close());
  const page = await context.newPage(), errors = [];
  page.on('pageerror', error => errors.push(error.message));
  t.after(() => assert.deepEqual(errors, []));
  const data = seed(); mutate(data);
  const writes = [];await attachStore(page,data,saved=>writes.push(saved));
  await page.goto(base + '/example/?view=publishing#tasks');
  await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
  return { page, writes, data };
}
async function shot(page, name) {
  if (process.env.SCREENSHOT_DIR) {
    // Wait for overlay exit transitions before recording the settled page.
    await page.waitForTimeout(350);
    await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, name), fullPage: true });
    await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, name.replace('.png', '-viewport.png')) });
  }
}
async function layout(page, title, labels) {
  const panel = page.locator('.publishing-task-page:visible');
  await panel.getByRole('heading', { name: title, exact: true }).waitFor({ timeout: 3000 });
  const footer = panel.locator('footer');
  assert.deepEqual(await footer.getByRole('button').allTextContents(), labels);
  assert.equal(await panel.locator('header button').count(), 0);
  assert.deepEqual(await panel.locator('h1').evaluate(el => ({ size: getComputedStyle(el).fontSize, weight: getComputedStyle(el).fontWeight })), { size: '24px', weight: '500' });
  const box = await footer.boundingBox();
  const viewport = page.viewportSize();
  assert.ok(box.y > viewport.height - 160 && Math.abs(box.y + box.height - viewport.height) <= 1);
  assert.ok(box.x >= 0 && box.x + box.width <= viewport.width + 1);
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
  return { panel, footer };
}
function editable(data) {
  const task = data.tasks[0];
  Object.assign(task, { batchId: null, planId: null, claimed: false, batchConfigLocked: false, batchStopped: false, declaration: declarationOptions(task.platform)[0].value, version: 1 });
  return task;
}
function draft(data) {
  const task = editable(data);
  data.quickDraft = { active: true, platform: task.platform, category: task.category, accountId: task.accountId, row: structuredClone(task), creationKey: 'isolated-page-draft', savedAt: '2026-11-14T10:00:00Z' };
}
async function openDraft(page) {
  await page.getByRole('button', { name: '快速发布', exact: true }).click();
  await page.getByRole('button', { name: '继续编辑', exact: true }).click();
}

test('new task uses the list title style and all page actions stay in the footer', async t => {
  const { page, writes, data } = await setup(t);
  await page.getByRole('button', { name: '快速发布', exact: true }).click();
  await shot(page, process.env.BASELINE ? '00-before-new.png' : '01-new.png');
  const { footer } = await layout(page, '快速发布', ['返回', '保存草稿', '创建任务']);
  await footer.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
  assert.ok(writes.every(saved => saved.tasks.length === data.tasks.length), 'empty return must not create a task');
});

test('new task footer preserves unsaved draft protection and resume', async t => {
  const { page, writes, data } = await setup(t, undefined, draft);
  await openDraft(page);
  const { footer } = await layout(page, '快速发布', ['返回', '保存草稿', '创建任务']);
  await page.locator('[data-field="title"] input').fill('保留本次新建配置');
  await footer.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByText('保存草稿后返回？', { exact: true }).waitFor();
  await page.getByRole('button', { name: '继续编辑', exact: true }).click();
  await footer.getByRole('button', { name: '保存草稿', exact: true }).click();
  await page.getByText('草稿已保存，不占用成片。', { exact: true }).first().waitFor();
  await footer.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
  assert.equal(writes.at(-1).tasks.length, data.tasks.length);
  assert.equal(writes.at(-1).quickDraft.row.title, '保留本次新建配置');
  await openDraft(page);
  assert.equal(await page.locator('[data-field="title"] input').inputValue(), '保留本次新建配置');
  await shot(page, '02-draft-restored.png');
});

test('editing only saves content, returns safely and leaves new-task draft untouched', async t => {
  const { page, writes } = await setup(t, undefined, data => {
    editable(data);
    data.quickDraft = { active: true, creationKey: 'unrelated-new-draft', savedAt: '2026-11-14T10:00:00Z', row: { title: '原新建草稿' } };
  });
  const row = page.getByRole('row').filter({ hasText: '1001' });
  await row.getByRole('button', { name: '编辑', exact: true }).click();
  let { footer } = await layout(page, '编辑发布内容', ['返回', '保存']);
  assert.equal(await page.locator('[data-field="resources"] .t-select input:not(:disabled)').count(), 0);
  const input = page.locator('[data-field="title"] input');
  const original = await input.inputValue();
  await input.fill('放弃这一版标题');
  await footer.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('button', { name: '继续编辑', exact: true }).click();
  assert.equal(await input.inputValue(), '放弃这一版标题');
  await footer.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('button', { name: '放弃修改', exact: true }).click();
  await row.getByRole('button', { name: '编辑', exact: true }).click();
  assert.equal(await input.inputValue(), original);
  ({ footer } = await layout(page, '编辑发布内容', ['返回', '保存']));
  await input.fill('已保存的发布标题');
  await shot(page, '03-edit.png');
  await footer.getByRole('button', { name: '保存', exact: true }).click();
  await page.getByRole('heading', { name: '发布任务', exact: true }).waitFor();
  const updated = writes.at(-1).tasks.find(task => task.id === 1001);
  assert.equal(updated.title, '已保存的发布标题');
  assert.equal(updated.status, '待执行');
  assert.equal(updated.version, 2);
  assert.deepEqual(writes.at(-1).quickDraft, { active: true, creationKey: 'unrelated-new-draft', savedAt: '2026-11-14T10:00:00Z', row: { title: '原新建草稿' } });
});

test('readonly detail footer returns to filtered and scrolled list without unsafe actions', async t => {
  const { page, writes, data } = await setup(t, undefined, data => {
    data.tasks.push({ ...structuredClone(data.tasks.at(-1)), id: 1011 });
  });
  assert.equal(await page.locator('.task-list-table .t-table__body tr').count(), 10);
  assert.equal(await page.locator('.task-list-table .t-table__body td:nth-child(5) small').count(), 0);
  await page.getByPlaceholder('搜索任务名称或 ID').fill('1005');
  await page.getByRole('button', { name: '查询', exact: true }).click();
  await page.getByRole('button', { name: '详情', exact: true }).click();
  const { panel, footer } = await layout(page, '发布任务详情', ['返回']);
  assert.equal(await panel.getByRole('button', { name: /执行|重试|复制|编辑|取消/ }).count(), 0);
  await shot(page, '04-detail.png');
  await footer.getByRole('button', { name: '返回', exact: true }).click();
  assert.equal(await page.getByPlaceholder('搜索任务名称或 ID').inputValue(), '1005');
  assert.equal(await page.getByRole('row').count(), 2);
  assert.ok(writes.every(saved => saved.tasks.length === data.tasks.length));
  await page.getByRole('button', { name: '重置', exact: true }).click();
  await page.locator('.t-pagination__number').filter({ hasText: /^2$/ }).click();
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  const scroll = await page.evaluate(() => window.scrollY);
  await page.getByRole('row').filter({ hasText: '1011' }).getByRole('button', { name: '详情', exact: true }).click();
  await layout(page, '发布任务详情', ['返回']);
  await page.locator('.publishing-task-page footer').getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('row').filter({ hasText: '1011' }).waitFor();
  assert.equal(await page.locator('.task-list-table .t-pagination__number.t-is-current').innerText(), '2');
  await page.waitForTimeout(100);
  assert.ok(Math.abs(await page.evaluate(() => window.scrollY) - scroll) <= 2);
});

test('narrow new, edit and detail keep footer visible and reserve final content space', async t => {
  const { page } = await setup(t, { width: 390, height: 844 }, draft);
  await openDraft(page);
  for (const [mode, title, buttons] of [['new', '快速发布', ['返回', '保存草稿', '创建任务']], ['edit', '编辑发布内容', ['返回', '保存']], ['detail', '发布任务详情', ['返回']]]) {
    if (mode === 'edit') {
      await page.getByRole('row').filter({ hasText: '1001' }).getByRole('button', { name: '编辑', exact: true }).click();
    } else if (mode === 'detail') await page.getByRole('row').filter({ hasText: '1001' }).getByRole('button', { name: '详情', exact: true }).click();
    const { panel, footer } = await layout(page, title, buttons);
    await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    const contentBox = await panel.locator('.resource-detail-page__content').boundingBox();
    const footerBox = await footer.boundingBox();
    assert.ok(contentBox.y + contentBox.height <= footerBox.y - 10, 'last content must not be obscured');
    await shot(page, `05-mobile-${mode}.png`);
    await footer.getByRole('button', { name: '返回', exact: true }).click();
  }
});

test('batch pages retain their existing shared header and navigation', async t => {
  const { page } = await setup(t);
  await page.goto(base + '/example/?view=publishing#batches?batch=201');
  await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor();
  assert.equal(await page.locator('.publishing-task-page').count(), 0);
  assert.equal(await page.getByRole('button', { name: '返回', exact: true }).isVisible(), true);
});

for (const mode of ['new', 'edit']) test(`AI fill in ${mode} has pending protection and a transient toast without overwriting manual content`, async t => {
  const { page } = await setup(t, undefined, draft);
  if (mode === 'new') await openDraft(page);
  else await page.getByRole('row').filter({ hasText: '1001' }).getByRole('button', { name: '编辑', exact: true }).click();
  const panel = page.locator('.publishing-task-page:visible');
  const input = panel.locator('[data-field="title"] input');
  await input.fill('人工确认的发布标题');
  const heightBefore = (await panel.boundingBox()).height;
  const ai = panel.getByRole('button', { name: 'AI 填写', exact: true });
  await ai.evaluate(button => { for (let i = 0; i < 8; i++) button.dispatchEvent(new MouseEvent('click', { bubbles: true })); });
  assert.equal(await ai.getAttribute('aria-busy'), 'true');
  assert.equal(await ai.isEnabled(), false);
  assert.equal(await panel.locator('.quick-publish .t-alert').count(), 0);
  await shot(page, `06-ai-${mode}-loading.png`);
  const message = page.locator('.t-message').filter({ hasText: '暂时无法获取 AI 建议' });
  await message.waitFor({ state: 'visible' });
  assert.equal(await message.count(), 1, 'rapid clicks must produce one feedback message');
  assert.equal(await input.inputValue(), '人工确认的发布标题');
  assert.equal(await panel.locator('.quick-publish .t-alert').count(), 0);
  assert.ok(Math.abs((await panel.boundingBox()).height - heightBefore) <= 1, 'feedback must not shift form content');
  await shot(page, `07-ai-${mode}-toast.png`);
  await message.waitFor({ state: 'hidden', timeout: 5000 });
  assert.equal(await ai.isEnabled(), true);
  assert.equal(await ai.getAttribute('aria-busy'), 'false');
});

test('four task views share spacing and type hierarchy while the table header has a distinct surface', async t => {
  const { page } = await setup(t, undefined, draft);
  const style = await page.locator('.task-list-table th').first().evaluate(el => ({ background: getComputedStyle(el).backgroundColor, border: getComputedStyle(el).borderBottomColor, weight: getComputedStyle(el).fontWeight }));
  assert.notEqual(style.background, style.border);
  assert.notEqual(style.background, 'rgba(0, 0, 0, 0)');
  assert.equal(style.weight, '500');
  assert.equal(await page.locator('.publishing-task-list .resource-stat-grid').evaluate(el => getComputedStyle(el).marginBottom), '16px');
  await shot(page, '08-list-review.png');
  await openDraft(page);
  assert.equal(await page.locator('.quick-block').last().evaluate(el => getComputedStyle(el).marginTop), '16px');
  assert.deepEqual(await page.locator('.quick-section-heading h3').first().evaluate(el => ({ size: getComputedStyle(el).fontSize, weight: getComputedStyle(el).fontWeight, line: getComputedStyle(el).lineHeight })), { size: '16px', weight: '500', line: '24px' });
  await shot(page, '09-new-review.png');
  await page.locator('.publishing-task-page footer').getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('row').filter({ hasText: '1001' }).getByRole('button', { name: '编辑', exact: true }).click();
  await shot(page, '10-edit-review.png');
  await page.locator('.publishing-task-page footer').getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('row').filter({ hasText: '1001' }).getByRole('button', { name: '详情', exact: true }).click();
  assert.equal(await page.locator('.task-detail').evaluate(el => getComputedStyle(el).gap), '16px');
  const sectionHeading = page.locator('.task-detail .resource-section__header h3').first();
  assert.deepEqual(await sectionHeading.evaluate(el => ({ border: getComputedStyle(el).borderLeftWidth, weight: getComputedStyle(el).fontWeight, markWidth: getComputedStyle(el, '::before').width, markHeight: getComputedStyle(el, '::before').height })), { border: '0px', weight: '500', markWidth: '3px', markHeight: '14px' });
  await shot(page, '11-detail-review.png');
});
