const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {attachStore}=require('./previewFixture.cjs');
const {planFixture}=require('./planFixtures.cjs');
const {pathToFileURL}=require('node:url');
const path=require('node:path');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
let browser;
before(async()=>{browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});});
after(async()=>browser?.close());
async function setup(t,path='/publishing/plans',viewport={width:1440,height:1000}){
 const context=await browser.newContext({viewport});t.after(()=>context.close());
 const page=await context.newPage(),errors=[];page.setDefaultTimeout(15000);
 page.on('pageerror',error=>errors.push(error.message));t.after(()=>assert.deepEqual(errors,[]));
 const {seed}=await import(pathToFileURL(require('node:path').join(__dirname,'../model.js')));const db=seed();await planFixture(db);page.store=await attachStore(page,db);
 await page.goto(base+'/example/operations'+path);await page.locator('.operations-nav').waitFor({state:'attached'});
 return page;
}
async function menu(page,name){await page.locator('.operations-nav-link').filter({hasText:name}).click();}
const planRows=page=>page.locator('.plan-list-panel .t-table__body tr');

test('same document shell covers plans, batches, templates and production without duplicate action handlers',async t=>{
 const page=await setup(t);await page.getByRole('heading',{name:'发布计划',exact:true}).waitFor();
 await page.evaluate(()=>window.__workbenchToken='same-document');
 for(const name of ['发布批次','封面模板','素材库','我的素材','合成策略','合成任务','成片管理','图片库','发布任务','发布计划','发布任务','发布计划']){
  await menu(page,name);await page.getByRole('heading',{name,exact:true}).waitFor();
  assert.equal(await page.evaluate(()=>window.__workbenchToken),'same-document');assert.equal(await page.locator('.operations-nav').count(),1);
  assert.equal(await page.locator('.operations-nav-link[aria-current="page"]').innerText(),name);
 }
 await page.locator('.plan-list-name').first().click();await page.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();
 const url=page.url();await page.reload();await page.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();assert.equal(page.url(),url);
 await page.getByRole('button',{name:'返回',exact:true}).click();await page.getByRole('heading',{name:'发布计划',exact:true}).waitFor();
});

test('independent raw IDs, default blue regular name links, deeper hover and minute timestamps',async t=>{
 const page=await setup(t);await page.locator('th').filter({hasText:'计划ID'}).waitFor();
 assert.match(await planRows(page).first().locator('td').first().innerText(),/^PL-/);
 const timestamp=await planRows(page).first().locator('td').nth(7).innerText();assert.match(timestamp,/\d{4}\/\d{1,2}\/\d{1,2} \d{2}:\d{2}$/);
 await menu(page,'发布批次');await page.getByRole('heading',{name:'发布批次',exact:true}).waitFor();
 assert.equal(await page.locator('th').first().innerText(),'批次ID');
 const first=page.locator('.t-table__body tr').first();assert.match(await first.locator('td').first().innerText(),/^\d+$/);
 const link=first.locator('.resource-name-link'),normal=await link.evaluate(el=>({color:getComputedStyle(el).color,weight:getComputedStyle(el).fontWeight}));
 assert.equal(normal.weight,'400');await link.hover();await page.waitForTimeout(180);assert.notEqual(await link.evaluate(el=>getComputedStyle(el).color),normal.color);
 await link.focus();await page.keyboard.press('Shift+Tab');await page.keyboard.press('Tab');assert.notEqual(await link.evaluate(el=>getComputedStyle(el).outlineStyle),'none');
 await menu(page,'发布任务');await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor();
 assert.equal(await page.locator('.task-title-link').first().evaluate(el=>getComputedStyle(el).color),normal.color);
 await menu(page,'封面模板');await page.getByRole('heading',{name:'封面模板',exact:true}).waitFor();assert.equal(await page.locator('th').first().innerText(),'模板ID');
});

test('new plan uses current user, date range occupies one field and recovery retains input',async t=>{
 const page=await setup(t);await page.getByRole('button',{name:'新建发布计划',exact:true}).click();await page.locator('#plan-basics').waitFor();
 assert.equal(await page.getByText('计划负责人',{exact:true}).count(),0);
 await page.locator('.plan-basic-name input').fill('统一表单回归');
 const inputs=page.locator('.plan-basic-period input');assert.equal(await inputs.count(),2);
 await page.clock.setFixedTime(new Date('2026-11-17T12:00:00+08:00'));
 await inputs.first().click();
 const month=page.locator('.t-date-picker__panel-content').first();
 await month.locator('td[type="current-month"]').filter({hasText:/^18$/}).click();
 await month.locator('td[type="current-month"]').filter({hasText:/^30$/}).click();
 await page.getByRole('heading',{name:'新建发布计划',exact:true}).click();
 const saved=JSON.parse(await page.evaluate(()=>localStorage.getItem('wt-publishing-plan-v1')));
 assert.equal(saved.ui.editor.ownerId,'chen');assert.equal(saved.ui.editor.createdBy,'chen');assert.equal(saved.ui.editor.dateStart,'2026-11-18');assert.equal(saved.ui.editor.dateEnd,'2026-11-30');
 await menu(page,'发布任务');await menu(page,'发布计划');await page.getByRole('button',{name:'新建发布计划',exact:true}).click();
 await page.getByRole('button',{name:'继续编辑',exact:true}).click();await page.locator('#plan-basics').waitFor();assert.equal(await page.locator('.plan-basic-name input').inputValue(),'统一表单回归');
 await page.reload();await page.locator('#plan-basics').waitFor();assert.equal(await page.locator('.plan-basic-name input').inputValue(),'统一表单回归');
});

test('hidden ownership follows the current creator and recovery preserves that identity',async t=>{
 const page=await setup(t);await page.getByRole('combobox',{name:'当前使用者',exact:true}).selectOption('林予安');
 await page.getByRole('button',{name:'新建发布计划',exact:true}).click();await page.locator('.plan-basic-name input').fill('身份续编检查');
 assert.equal(await page.getByText('计划负责人',{exact:true}).count(),0);
 const saved=JSON.parse(await page.evaluate(()=>localStorage.getItem('wt-publishing-plan-v1')));assert.equal(saved.ui.editor.ownerId,'lin');assert.equal(saved.ui.editor.createdBy,'lin');
 await page.getByRole('button',{name:'保存草稿',exact:true}).click();await page.getByRole('button',{name:'新建发布计划',exact:true}).click();await page.getByRole('button',{name:'继续编辑',exact:true}).click();
 assert.equal(await page.locator('.plan-basic-name input').inputValue(),'身份续编检查');
});

test('dirty existing edit blocks menu and Back until explicitly discarded, without replacing historical owner',async t=>{
 const page=await setup(t);await page.getByRole('heading',{name:'发布计划',exact:true}).waitFor();
 const original=(await page.store.get()).data.plans.find(p=>p.id==='PL-202611-002');
 await planRows(page).filter({hasText:original.id}).locator('[data-act="edit"]').click();await page.locator('#plan-basics').waitFor();
 assert.match(page.url(),new RegExp(original.id+'/edit$'));assert.equal(await page.getByText('计划负责人',{exact:true}).count(),0);
 await page.locator('.plan-basic-name input').fill('未保存的编辑');await menu(page,'发布任务');
 await page.getByRole('dialog').getByRole('button',{name:'继续编辑',exact:true}).click();assert.equal(await page.locator('.plan-basic-name input').inputValue(),'未保存的编辑');
 await page.goBack();await page.getByRole('dialog').getByRole('button',{name:'继续编辑',exact:true}).click();assert.match(page.url(),/\/edit$/);
 await menu(page,'发布任务');await page.getByRole('dialog').getByRole('button',{name:'放弃修改',exact:true}).click();await page.getByRole('heading',{name:'发布任务',exact:true}).waitFor();
 const saved=(await page.store.get()).data.plans.find(p=>p.id===original.id);assert.equal(saved.name,original.name);assert.equal(saved.ownerId,original.ownerId);
});

test('mobile navigation stays accessible and shared plan footer spans the content area',async t=>{
 const page=await setup(t,'/publishing/plans',{width:390,height:844});await page.getByRole('heading',{name:'发布计划',exact:true}).waitFor();
 await page.getByRole('button',{name:'展开菜单',exact:true}).click();await page.getByRole('dialog',{name:'内容运营导航',exact:true}).waitFor();
 await menu(page,'发布批次');await page.getByRole('heading',{name:'发布批次',exact:true}).waitFor();
 await page.getByRole('button',{name:'展开菜单',exact:true}).click();await menu(page,'发布计划');await page.getByRole('button',{name:'新建发布计划',exact:true}).click();
 await page.locator('.editor-footer').waitFor();const box=await page.locator('.editor-footer').boundingBox();assert.equal(Math.round(box.x),0);assert.equal(Math.round(box.width),390);
});

test('accepted Back across modules preserves destination URL and Forward restores plan list',async t=>{
 const page=await setup(t,'/production/materials');await page.getByRole('heading',{name:'素材库',exact:true}).waitFor();
 await menu(page,'发布计划');await planRows(page).filter({hasText:'PL-202611-002'}).locator('[data-act="edit"]').click();await page.locator('.plan-basic-name input').fill('返回保护回归');
 await page.evaluate(()=>history.go(-2));await page.getByRole('dialog').getByRole('button',{name:'放弃修改',exact:true}).click();
 await page.getByRole('heading',{name:'素材库',exact:true}).waitFor();assert.match(page.url(),/\/production\/materials$/);
 await page.goForward();await page.getByRole('heading',{name:'发布计划',exact:true}).waitFor();assert.match(page.url(),/\/publishing\/plans$/);
});
