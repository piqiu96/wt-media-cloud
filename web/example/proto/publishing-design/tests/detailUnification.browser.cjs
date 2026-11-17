const {requestBody}=require('./previewFixture.cjs');
const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const {planFixture}=require('./planFixtures.cjs');
const { pathToFileURL } = require('node:url');
const path = require('node:path');
const base = process.env.EXAMPLE_URL || 'http://127.0.0.1:8768';
let browser, seed;
before(async () => {
  ({ seed } = await import(pathToFileURL(path.join(__dirname, '../model.js'))));
  browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE });
});
after(async () => browser?.close());
async function pageFor(t, viewport = { width: 1440, height: 1000 }, mutate = () => {}) {
  const context = await browser.newContext({ viewport });
  t.after(() => context.close());
  const page = await context.newPage(), errors = [];
  page.on('pageerror', error => errors.push(error.message));
  t.after(() => assert.deepEqual(errors, []));
  const data = seed(); await mutate(data);
  const {createBatchStore}=await import(pathToFileURL(path.join(__dirname,'../../../publishing-preview.mjs')));
  const store=createBatchStore();await store.bootstrap(data);
  await page.route('**/__example/publishing/**',async route=>{
    try{const q=route.request(),body=q.method()==='GET'?null:requestBody(q);const result=q.method()==='GET'?await store.get():q.url().endsWith('/command')?await store.command(body):await store.save(body.data,body.revision);await route.fulfill({json:result});}
    catch(e){await route.fulfill({status:409,json:{error:e.message}});}
  });
  return page;
}
async function shot(page, name) {
  if (process.env.SCREENSHOT_DIR) {
    await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, name), fullPage: true });
    if (['01-task.png', '02-batch.png', '03-plan.png'].includes(name)) await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, name.replace('.png', '-viewport.png')) });
  }
}
async function overflow(page) {
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), false, 'page must not overflow horizontally');
}
test('three detail pages share readable regions and keep business operations in the footer', async t => {
  const page = await pageFor(t);
  await page.goto(base + '/example/?view=publishing#tasks?task=1001');
  const detail = page.locator('.resource-detail-page, .publishing-task-page');
  await detail.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor({ timeout: 5000 });
  const info = detail.getByRole('region', { name: '任务信息', exact: true });
  assert.match(await info.innerText(), /任务名称[\s\S]*三角洲实战精彩片段/);
  assert.match(await info.innerText(), /所属批次[\s\S]*小红书 · 首轮内容/);
  assert.deepEqual(await detail.locator('footer').getByRole('button').allTextContents(), ['返回'], 'task remains readonly');
  assert.equal(await detail.locator('nav').count(), 0, 'no second breadcrumb inside task');
  const widths = await info.locator('.t-descriptions__label').evaluateAll(nodes => nodes.map(el => el.getBoundingClientRect().width));
  assert.ok(widths.every(width => width <= 130), 'a full first row must not stretch subsequent labels');
  await shot(page, '01-task.png');
  await page.goto(base + '/example/?view=publishing#batches?batch=201');
  await detail.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor();
  for (const name of ['批次信息', '批次任务', '执行记录']) await detail.getByRole('region', { name, exact: true }).waitFor();
  const stats = detail.locator('[aria-label="执行情况"]');
  assert.match(await stats.innerText(), /发布成功率[\s\S]*0 \/ 4/);assert.equal(await stats.locator('.resource-progress').count(),1);assert.doesNotMatch(await stats.innerText(),/已处置进度|发布达成率/);
  await detail.locator('footer').getByRole('button', { name: '停止发布', exact: true }).waitFor();
  assert.equal(await detail.getByRole('region', { name: '批次快照', exact: true }).count(), 0, 'snapshot is not exposed as a separate detail section');
  await shot(page, '02-batch.png');
  await page.goto(base + '/example/proto/publishing-design/plan/');
  await page.locator('[data-act="detail"]').first().click();
  await detail.getByRole('heading', { name: '发布计划详情', exact: true }).waitFor();
  await detail.getByRole('region', { name: '目标完成情况', exact: true }).waitFor();
  await detail.getByRole('region', { name: '批次推进', exact: true }).waitFor();
  assert.equal(await detail.locator('footer [data-act=edit], footer [data-act=cancel]').count(), 0);
  assert.equal(await detail.locator('.resource-page-header button').count(), 0);
  await shot(page, '03-plan.png');
  await page.reload(); await detail.getByRole('heading', { name: '发布计划详情', exact: true }).waitFor();
  await detail.getByRole('button', { name: '返回', exact: true }).click();
  await page.locator('[data-act="new"]').waitFor();
});

test('pending, failed, ended and long-content details stay readable at 390px', async t => {
  const page = await pageFor(t, { width: 390, height: 844 }, db => {
    db.tasks[0].title = '长任务名称用于检查内容换行'.repeat(8);
    db.tasks[0].description = '第一段正文包含较长的中文内容。'.repeat(15) + '\n第二段正文保留换行。';
    db.tasks[0].topics = '#长话题 '.repeat(25);
  });
  for (const [id, name] of [[1001, '04-task-long-mobile.png'], [1004, '05-task-pending-mobile.png'], [1008, '06-task-failed-mobile.png']]) {
    await page.goto(base + '/example/?view=publishing#tasks?task=' + id);
    await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor();
    if (id !== 1001) await page.locator('.task-detail > .resource-notice').waitFor();
    await overflow(page); await shot(page, name);
  }
  for (const id of [201, 202, 203]) {
    await page.goto(base + '/example/?view=publishing#batches?batch=' + id);
    await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor();
    await overflow(page);
    const footer = page.locator('.resource-detail-page > footer');
    if (await footer.count()) {
      const box = await footer.boundingBox(); assert.ok(box.x >= 0 && box.x + box.width <= 391 && box.y + box.height <= 845);
      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
      const history = await page.getByRole('region', { name: '执行记录', exact: true }).boundingBox(); assert.ok(history.y + history.height <= box.y, 'footer reserves room for execution history');
      assert.equal(await page.locator('.batch-audit').count(), 0);
      await page.evaluate(() => window.scrollTo(0, 0));
    }
    await shot(page, `07-batch-${id}-mobile.png`);
  }
  for (const [id, name] of [['PL-202611-005', '08-plan-pending-mobile.png'], ['PL-202610-018', '09-plan-ended-mobile.png']]) {
    await page.goto(base + '/example/proto/publishing-design/plan/#plans?plan=' + id);
    await page.getByRole('heading', { name: '发布计划详情', exact: true }).waitFor();
    await overflow(page); await shot(page, name);
  }
});

test('plan footer keeps decisions while the list owns cancellation and pre-start editing', async t => {
  const page = await pageFor(t, {width:1440,height:1000}, planFixture);
  const detail = page.locator('.resource-detail-page');
  const go=id=>page.goto(base+'/example/operations/publishing/plans/'+id);
  await go('PL-202611-004');
  await detail.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();
  assert.equal(await detail.locator('footer [data-act=edit], footer [data-act=cancel], footer [data-act=copy]').count(),0);
  await detail.getByRole('button',{name:'返回',exact:true}).click();
  await page.locator('[data-act=cancel][data-id=PL-202611-002]').click();
  await page.locator('dialog.plan-modal[open]').waitFor();await shot(page,'10-plan-cancel-confirm.png');
  await page.locator('dialog.plan-modal [data-act="close-modal"]').first().click();
  await page.locator('[data-act=edit][data-id=PL-202611-002]').click();
  const savedName=await page.locator('.plan-basic-name input').inputValue();
  await page.locator('.plan-basic-name input').fill('详情编辑恢复校验');
  await page.reload();await page.locator('.plan-basic-name input').waitFor();
  assert.equal(await page.locator('.plan-basic-name input').inputValue(),savedName,'editing does not create a new-record draft');
  await go('PL-202611-005');await detail.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();
  assert.equal(await detail.locator('footer [data-act=edit]').count(),0);
  await detail.getByRole('button',{name:'查看待处理批次',exact:true}).click();
  await page.getByRole('heading',{name:'发布批次详情',exact:true}).waitFor();await shot(page,'11-plan-workbench.png');
  await go('PL-202611-003');
  await detail.locator('footer').getByRole('button',{name:'创建补量批次',exact:true}).click();
  await page.locator('dialog.plan-drawer[open]').waitFor();assert.match(await page.locator('dialog.plan-drawer').innerText(),/创建补量批次/);
  await page.locator('dialog.plan-drawer [data-act="close-drawer"]').first().click();
  await detail.locator('footer').getByRole('button',{name:'确认结束',exact:true}).click();
  await page.locator('dialog.plan-modal[open]').waitFor();await page.locator('dialog.plan-modal [data-act="close-modal"]').first().click();
  await go('PL-202611-004');
  await detail.locator('footer').getByRole('button',{name:'启动计划',exact:true}).click();
  await page.locator('dialog.plan-modal[open]').waitFor();await shot(page,'12-plan-execute-confirm.png');
  await page.locator('dialog.plan-modal [data-act="close-modal"]').first().click();
  await detail.getByRole('button',{name:'返回',exact:true}).click();
  await page.goBack();await detail.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();assert.match(page.url(),/plans\/PL-202611-004/);
  await page.goForward();await page.locator('[data-act="new"]').waitFor();
  await go('missing');await page.getByText('计划不存在',{exact:true}).waitFor();
  await page.getByRole('button',{name:'返回',exact:true}).click();await page.locator('[data-act="new"]').waitFor();
});

test('common example exercises the real shared detail and responsive descriptions', async t => {
  const page = await pageFor(t, { width: 1000, height: 900 });
  await page.goto(base + '/example/#components');
  await page.getByRole('button', { name: '独立详情', exact: true }).last().click();
  const detail = page.locator('.resource-detail-page');
  await detail.getByRole('heading', { name: '资源详情示例', exact: true }).waitFor();
  await detail.locator('.resource-descriptions[data-columns="2"]').waitFor();
  await shot(page, '13-common-detail.png');
  await page.setViewportSize({ width: 390, height: 844 });
  await detail.locator('.resource-descriptions[data-columns="1"]').waitFor();
  await overflow(page);
  await detail.getByRole('button', { name: '返回', exact: true }).click();
  await page.getByRole('button', { name: '独立详情', exact: true }).first().waitFor();
});

test('desktop states retain task outcomes and one success rate over the original batch target', async t => {
  const page = await pageFor(t);
  for (const [id, name] of [[1004, '14-task-pending.png'], [1008, '15-task-failed.png'], [1007, '16-task-published.png']]) {
    await page.goto(base + '/example/?view=publishing#tasks?task=' + id);
    await page.getByRole('heading', { name: '发布任务详情', exact: true }).waitFor();
    assert.deepEqual(await page.locator('.publishing-task-page footer').getByRole('button').allTextContents(), ['返回']);
    await shot(page, name);
  }
  await page.goto(base + '/example/?view=publishing#batches?batch=203');
  await page.getByRole('heading', { name: '发布批次详情', exact: true }).waitFor();
  const stats=page.locator('[aria-label="执行情况"]');
  const progress=(await stats.innerText()).replace(/\s+/g,' ');
  assert.equal(await stats.locator('.resource-progress').count(),1);assert.match(progress,/发布成功率[\s\S]*0 \/ 2/);assert.match(progress,/已发布[\s\S]*0/);assert.doesNotMatch(progress,/已处置进度|发布达成率/);
  await shot(page, '17-batch-ended.png');
  assert.equal(await page.locator('.batch-audit').count(), 0);
  await page.getByRole('region', { name: '执行记录', exact: true }).scrollIntoViewIfNeeded();
  await shot(page, '18-batch-history.png');
  for (const [id, name] of [['PL-202611-004', '19-plan-ready-next.png'], ['PL-202611-003', '20-plan-gap.png'], ['PL-202610-018', '21-plan-ended.png']]) {
    await page.goto(base + '/example/proto/publishing-design/plan/#plans?plan=' + id);
    await page.getByRole('heading', { name: '发布计划详情', exact: true }).waitFor();
    await overflow(page); await shot(page, name);
  }
});
