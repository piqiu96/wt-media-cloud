const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const {planFixture}=require('./planFixtures.cjs');
const {pathToFileURL}=require('node:url');
const path = require('node:path');
const base = process.env.EXAMPLE_URL || 'http://127.0.0.1:8768';
let browser;
before(async () => { browser = await chromium.launch({ headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE }); });
after(async () => browser?.close());
async function setup(t, mutate, mutateDb) {
  const context=await browser.newContext({viewport:{width:1440,height:1000}});t.after(()=>context.close());
  const page=await context.newPage(),errors=[];page.setDefaultTimeout(8000);page.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));
  const root=pathToFileURL(path.resolve(__dirname,'..')+'/').href;
  const {seed}=await import(root+'model.js'),{createBatchStore}=await import(root+'../../publishing-preview.mjs');
  const db=seed();await planFixture(db);if(mutateDb)await mutateDb(db);
  const store=createBatchStore();await store.bootstrap(db);page.fixture=structuredClone(db);
  await page.route('**/__example/publishing/**',async route=>{try{const q=route.request(),body=q.method()==='GET'?null:q.postDataJSON();const result=q.method()==='GET'?await store.get():q.url().endsWith('/command')?await store.command(body):await store.save(body.data,body.revision);await route.fulfill({json:result});}catch(e){await route.fulfill({status:409,json:{error:e.message}});}});
  const saved={version:1,ui:{filters:{search:'',status:'all',platform:'',category:''},filterDraft:{search:'',status:'all',platform:'',category:''},pageNo:1,pageSize:10}};
  mutate?.(saved);await page.addInitScript(value=>{if(!sessionStorage.getItem('plan-list-fixture-loaded')){localStorage.setItem('wt-publishing-plan-v1',JSON.stringify(value));sessionStorage.setItem('plan-list-fixture-loaded','1');}},saved);
  await page.goto(base+'/example/operations/publishing/plans');await page.getByRole('heading',{name:'发布计划',exact:true}).waitFor();return page;
}
const rows = page => page.locator('.plan-list-panel .t-table__body tr');
const reset = page => page.getByRole('button', { name: '重置', exact: true }).click();
async function select(page, label, option) {
  await page.locator('label.wt-filter-field').filter({ hasText: label }).locator('.t-input__inner').click();
  await page.locator('.t-select-option:visible').filter({ hasText: option }).click();
}
async function shot(page, name) { if (process.env.SCREENSHOT_DIR) await page.screenshot({ path: path.join(process.env.SCREENSHOT_DIR, name) }); }
test('list removes pending scope including saved preferences and preserves query/reset semantics', async t => {
  const page = await setup(t, saved => { saved.ui.filters.needOnly = true; saved.ui.filterDraft.needOnly = true; });
  assert.equal(await rows(page).count(), 6);
  assert.equal(await page.getByText('待办范围', { exact: true }).count(), 0);
  const archived = JSON.parse(await page.evaluate(() => localStorage.getItem('wt-publishing-plan-v1')));
  assert.equal('needOnly' in archived.ui.filters, false);
  await page.getByRole('textbox', { name: '综合搜索', exact: true }).fill('PL-202611-002');
  assert.equal(await rows(page).count(), 6, 'draft search does not query automatically');
  await page.getByRole('button', { name: '查询', exact: true }).click();
  assert.equal(await rows(page).count(), 1); assert.match(await rows(page).innerText(), /PL-202611-002/);
  await reset(page); await select(page, '计划状态', '待执行');
  assert.equal(await rows(page).count(), 6); await page.getByRole('button', { name: '查询', exact: true }).click(); assert.equal(await rows(page).count(), 2);
  await reset(page);
  await select(page, '发布平台', 'B站'); await page.getByRole('button', { name: '查询', exact: true }).click();
  assert.equal(await rows(page).count(), page.fixture.plans.filter(p=>p.platformOrder.includes('B站')).length);
  await reset(page); await page.getByRole('textbox', { name: '综合搜索', exact: true }).fill('不存在的计划');
  await page.keyboard.press('Enter'); await page.getByText('没有符合条件的计划', { exact: true }).waitFor();
  await page.getByRole('button', { name: '重置筛选', exact: true }).click(); assert.equal(await rows(page).count(), 6);
  const saved = JSON.parse(await page.evaluate(() => localStorage.getItem('wt-publishing-plan-v1'))); assert.equal('plans' in saved,false,'list records live only in the shared store');
});
test('common pagination keeps totals, page size, refresh and detail return on the same filtered page', async t => {
  const page = await setup(t, undefined, async db => {
    const {confirmPlan}=await import(pathToFileURL(path.join(__dirname,'../relationships.js')));
    db.plans=[];db.batches=[];db.tasks=[];
    for(let i=0;i<25;i++)confirmPlan(db,{id:`PL-TEST-${String(i+1).padStart(3,'0')}`,name:`分页测试计划 ${i+1}`,confirmKey:'pagination-'+i,platformOrder:['B站'],targets:[{platform:'B站',category:'三角洲行动',count:1}],splits:[{name:'分页批次'+i,platform:'B站',targets:[{category:'三角洲行动',count:1}]}]});
  });
  await page.getByRole('textbox', { name: '综合搜索', exact: true }).fill('分页测试计划'); await page.getByRole('button', { name: '查询', exact: true }).click();
  assert.equal(await rows(page).count(), 10); assert.match(await page.locator('.plan-list-panel .t-pagination').innerText(), /共 25 条数据/);
  await page.locator('.plan-list-panel .t-pagination__btn-next').click(); assert.match(await rows(page).first().innerText(), /PL-TEST-011/);
  await page.reload(); await page.getByRole('heading', { name: '发布计划', exact: true }).waitFor(); assert.match(await rows(page).first().innerText(), /PL-TEST-011/);
  await rows(page).first().locator('[data-act="detail"]').first().click(); await page.getByRole('heading', { name: '发布计划详情', exact: true }).waitFor();
  await page.locator('.resource-detail-page__footer').getByRole('button', { name: '返回', exact: true }).click(); assert.equal(await page.getByRole('textbox', { name: '综合搜索', exact: true }).inputValue(), '分页测试计划'); assert.match(await rows(page).first().innerText(), /PL-TEST-011/);
  await page.locator('.plan-list-panel .t-pagination__select .t-input__inner').click();
  await page.locator('.t-select-option:visible').filter({ hasText: /20\s*条\/页/ }).click(); assert.equal(await rows(page).count(), 20);
  const saved = JSON.parse(await page.evaluate(() => localStorage.getItem('wt-publishing-plan-v1'))); assert.equal(saved.ui.pageSize, 20); assert.equal(saved.ui.pageNo, 1);
  await shot(page, 'plan-list-pagination.png');
});
test('shared list keeps original row actions and narrow table scrolling usable', async t => {
  const page = await setup(t);
  await shot(page, 'plan-list-desktop.png');
  const running = rows(page).filter({ hasText: 'PL-202611-005' });
  await running.getByRole('button', { name: '终止', exact: true }).click(); await page.locator('dialog.plan-modal[open]').waitFor();
  await page.locator('[data-act="close-modal"]').first().click();
  const execute = rows(page).filter({ hasText: 'PL-202611-002' }).getByRole('button', {name:'执行',exact:true});
  assert.equal(await execute.isDisabled(),true);
  await page.setViewportSize({ width: 390, height: 844 });
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), false);
  const table = page.locator('.plan-list-panel .t-table__content'); assert.ok(await table.evaluate(el => el.scrollWidth > el.clientWidth));
  await table.evaluate(el => el.scrollLeft = el.scrollWidth); await rows(page).first().getByRole('button', { name: '详情', exact: true }).waitFor();
  await shot(page, 'plan-list-mobile.png');
  await page.setViewportSize({ width: 1440, height: 1000 });
  assert.equal(await running.getByRole('button',{name:'编辑',exact:true}).count(),0);await rows(page).filter({hasText:'PL-202611-002'}).getByRole('button', { name: '编辑', exact: true }).click(); await page.getByRole('heading', { name: '编辑发布计划', exact: true }).waitFor();
});


test('four filters stay visible and the right action column stays fixed while the data scrolls', async t => {
  const page = await setup(t, saved => { saved.ui.advanced = false; });
  assert.deepEqual(await page.locator('.plan-list-panel .wt-filter-field__label').allTextContents(), ['综合搜索', '计划状态', '发布平台', '发布类目']);
  assert.equal(await page.getByRole('button', { name: '高级筛选', exact: false }).count(), 0);
  await select(page, '发布类目', '三角洲行动');
  await page.getByRole('button', { name: '查询', exact: true }).click();
  const saved = JSON.parse(await page.evaluate(() => localStorage.getItem('wt-publishing-plan-v1')));
  assert.ok(await rows(page).count() > 0);
  assert.equal(await rows(page).count(), page.fixture.plans.filter(p=>p.targets.some(t=>t.category==='三角洲行动')).length);
  await reset(page);
  for (const width of [1440, 1024, 390]) {
    await page.setViewportSize({ width, height: 1000 });
    const content = page.locator('.plan-list-panel .t-table__content');
    await content.evaluate(el => el.scrollLeft = 0);
    await page.waitForTimeout(120);
    const operation = rows(page).first().locator('td').last();
    const identity = rows(page).first().locator('td').nth(1);
    const before = await operation.boundingBox(), dataBefore = await identity.boundingBox();
    const container = await content.boundingBox();
    assert.ok(before.x >= container.x - 1 && before.x + before.width <= container.x + container.width + 1, 'actions visible before scrolling');
    assert.ok(await content.evaluate(el => el.scrollWidth > el.clientWidth));
    await shot(page, `plan-list-start-${width}.png`);
    await content.evaluate(el => el.scrollLeft = el.scrollWidth);
    await page.waitForTimeout(120);
    const after = await operation.boundingBox(), dataAfter = await identity.boundingBox();
    assert.ok(Math.abs(after.x - before.x) < 2, 'action column is fixed');
    assert.ok(dataAfter.x < dataBefore.x - 100, 'data columns scroll');
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), false);
    if (width === 390) await shot(page, 'plan-list-filters-mobile.png');
    await content.scrollIntoViewIfNeeded();
    await shot(page, `plan-list-fixed-${width}.png`);
    await rows(page).first().getByRole('button', { name: '详情', exact: true }).click();
    await page.getByRole('heading', { name: '发布计划详情', exact: true }).waitFor();
    await page.locator('.resource-detail-page__footer').getByRole('button', { name: '返回', exact: true }).click();
  }
});
