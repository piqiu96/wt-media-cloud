const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {pathToFileURL}=require('node:url');
const path=require('node:path');
const {attachStore}=require('./previewFixture.cjs');
const {planFixture}=require('./planFixtures.cjs');
const root=path.resolve(__dirname,'..'),base=process.env.EXAMPLE_URL;
let browser,seed,confirmPlan;
before(async()=>{
 ({seed}=await import(pathToFileURL(path.join(root,'model.js'))));
 ({confirmPlan}=await import(pathToFileURL(path.join(root,'relationships.js'))));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});
});
after(()=>browser?.close());
async function pageFor(t){
 const context=await browser.newContext({viewport:{width:1440,height:1000}}),page=await context.newPage(),errors=[];
 page.on('pageerror',error=>errors.push(error.message));t.after(()=>assert.deepEqual(errors,[]));
 page.setDefaultTimeout(7000);t.after(()=>context.close());return page;
}
async function screenshot(page,name){if(process.env.SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.SCREENSHOT_DIR,name+'.png'),animations:'disabled'});}
for(const [planId,label]of [['PL-202611-002','unconfigured'],['PL-202611-004','ready'],['PL-202611-005','running']]){
 test(`plan batch details stay read-only in context: ${label}`,async t=>{
  const p=await pageFor(t),db=seed();await planFixture(db);
  const store=await attachStore(p,db);
  await p.goto(base+'/example/operations/publishing/plans/'+planId);
  const row=p.locator('#batch-area tbody tr').first(),trigger=row.getByRole('button',{name:'详情',exact:true});
  await trigger.scrollIntoViewIfNeeded();await trigger.focus();
  const href=p.url(),beforeY=await p.evaluate(()=>scrollY),beforeData=(await store.get()).data;
  await trigger.click();
  const dialog=p.getByRole('dialog',{name:'发布批次详情',exact:true});await dialog.waitFor();await p.waitForTimeout(250);await screenshot(p,'plan-batch-preview-'+label);
  assert.equal(p.url(),href,'detail preview must not navigate');
  assert.equal(await dialog.evaluate(el=>el.contains(document.activeElement)),true,'opening moves focus inside the drawer');
  assert.match(await dialog.innerText(),new RegExp('批次 '+label));
  assert.match(await dialog.innerText(),label==='unconfigured'?/待配置/:label==='ready'?/待执行/:/待处理/);
  assert.equal(await dialog.locator('input:not([readonly]):not([disabled]), textarea:not([readonly]):not([disabled])').count(),label==='unconfigured'?0:1,'only started task lists expose search');
  for(const title of label==='unconfigured'?['批次信息','候选配置','操作记录']:['批次信息','执行情况','批次任务','执行记录'])assert.ok(await dialog.getByRole('heading',{name:new RegExp('^'+title)}).isVisible(),title+' should be visible');
  await p.waitForTimeout(250);const box=await dialog.boundingBox();assert.ok(Math.abs(box.x-216)<=1&&Math.abs(box.y-72)<=1&&Math.abs(box.width-1224)<=1&&Math.abs(box.height-928)<=1,'drawer only fills the content area below the header and beside the menu');
  assert.equal(await dialog.getByRole('button',{name:/^(去配置|执行|编辑|前往批次详情)$/}).count(),0);
  const afterData=(await store.get()).data;
  assert.deepEqual(afterData.batches,beforeData.batches,'reading does not mutate batches');
  assert.deepEqual(afterData.tasks,beforeData.tasks,'reading does not mutate tasks');
  await screenshot(p,'plan-batch-preview-'+label);
  await p.keyboard.press('PageUp');
  assert.ok(Math.abs(await p.evaluate(()=>scrollY)-beforeY)<3,'background remains locked while reading the drawer');
  await p.keyboard.press('Escape');await dialog.waitFor({state:'hidden'});
  assert.equal(p.url(),href);assert.ok(Math.abs(await p.evaluate(()=>scrollY)-beforeY)<3);
  await p.waitForFunction(el=>el===document.activeElement,await trigger.elementHandle());
  assert.equal(await trigger.evaluate(el=>el===document.activeElement),true,'close restores the detail trigger');
  if(label==='ready'){
   await p.setViewportSize({width:390,height:844});await trigger.click();await dialog.waitFor();
   await p.waitForTimeout(400);assert.equal(await dialog.evaluate(el=>el.contains(document.activeElement)),true);
   assert.ok(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
   await p.getByRole('button',{name:'展开菜单',exact:true}).click();
   const navigation=p.getByRole('dialog',{name:'内容运营导航',exact:true});await navigation.waitFor();
   await p.keyboard.press('Tab');const first=await p.evaluate(()=>document.activeElement.outerHTML);
   await p.keyboard.press('Tab');const second=await p.evaluate(()=>document.activeElement.outerHTML);
   assert.notEqual(first,second,'native navigation keeps its own keyboard sequence');
   assert.equal(await navigation.evaluate(el=>el.contains(document.activeElement)),true);
   await p.keyboard.press('Escape');await navigation.waitFor({state:'hidden'});assert.ok(await dialog.isVisible(),'Escape closes only the navigation above the drawer');
   await dialog.getByRole('heading',{name:label==='unconfigured'?'操作记录':'执行记录',exact:false}).scrollIntoViewIfNeeded();
   await screenshot(p,'plan-batch-preview-mobile-records');
   await dialog.getByRole('button',{name:'关闭发布批次详情',exact:true}).click();await dialog.waitFor({state:'hidden'});
   await p.waitForFunction(el=>el===document.activeElement,await trigger.elementHandle());
  }
  if(label==='unconfigured'){
   await row.getByRole('button',{name:'去配置',exact:true}).click();
   await p.getByRole('heading',{name:'编辑发布批次',exact:true}).waitFor();
   await p.getByRole('button',{name:'保存',exact:true}).click();
   await p.getByText('批次配置已保存',{exact:true}).waitFor();
   await p.getByRole('button',{name:'返回',exact:true}).click();
   await p.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();assert.equal(p.url(),href);
  }else if(label==='running'){
   await row.getByRole('button',{name:'去处理',exact:true}).click();
   await p.getByRole('heading',{name:'发布批次详情',exact:true}).waitFor();assert.match(p.url(),/pending=1/);
   await p.getByRole('button',{name:'返回',exact:true}).click();
   await p.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();assert.equal(p.url(),href);
  }
 });
}
test('plan resource gap uses the common notice with table spacing and accurate guidance at two widths',async t=>{
 const p=await pageFor(t),db=seed();db.plans=[];db.batches=[];db.tasks=[];db.accounts=[];db.assets=[];
 const plan=confirmPlan(db,{id:'PL-GAP-CONTEXT',name:'资源缺口计划',confirmKey:'gap-context',platformOrder:['B站'],targets:[{platform:'B站',category:'游戏攻略',count:30}],splits:[{name:'资源缺口批次',platform:'B站',targets:[{category:'游戏攻略',count:30}]}]});
 await attachStore(p,db);await p.goto(base+'/example/operations/publishing/plans/'+plan.id);
 await p.locator('#batch-area').getByRole('button',{name:'去配置',exact:true}).click();
 await p.getByRole('heading',{name:'编辑发布批次',exact:true}).waitFor();
 const table=p.locator('.batch-target-container'),section=p.locator('.batch-section').filter({has:table}),notice=section.getByRole('note');
 await notice.waitFor();assert.match(await notice.innerText(),/目标 30 条.*可匹配 0 条.*缺口 30 条/);
 assert.doesNotMatch(await notice.innerText(),/调整.*数量|最终创建需要明确确认/);
 assert.match(await notice.innerText(),/调整账号选择/);
 for(const width of [1440,768]){
  await p.setViewportSize({width,height:1000});await notice.scrollIntoViewIfNeeded();
  const a=await table.boundingBox(),b=await notice.boundingBox(),c=await section.boundingBox();
  assert.ok(Math.abs(b.y-(a.y+a.height)-16)<=1,'notice must have space below the table');
  assert.ok(Math.abs(a.x-b.x)<=1&&Math.abs(a.width-b.width)<=1,'notice aligns with the table');
  assert.ok(b.x+b.width<=c.x+c.width&&b.y+b.height<c.y+c.height,'notice stays inside the section padding');
  await screenshot(p,'plan-resource-gap-'+width);
 }
});

test('shared upper area stays fixed across publishing, tools and production pages',async t=>{
 const p=await pageFor(t),db=seed();await planFixture(db);await attachStore(p,db);
 const ready=db.batches.find(b=>b.planId==='PL-202611-004');
 for(const route of ['publishing/plans','publishing/batches','publishing/tasks','tools/images','tools/templates','production/materials','production/outputs','publishing/plans/PL-202611-004','publishing/plans/new','publishing/batches/'+ready.id,'publishing/tasks/'+db.tasks.find(t=>t.batchId===ready.id).id]){
  await p.goto(base+'/example/operations/'+route);
  const header=p.locator('.publish-main:visible .breadcrumbs:visible, .publish-main:visible .production-context:visible').first();await header.waitFor();
  await p.evaluate(()=>{document.body.style.minHeight='2400px';window.scrollTo(0,0)});
  const before=await header.boundingBox();await p.evaluate(()=>window.scrollTo(0,400));const after=await header.boundingBox();
  assert.ok(Math.abs(before.y)<=1&&Math.abs(after.y)<=1,route+' upper area must remain fixed');
  assert.ok(Math.abs(after.x-216)<=1&&Math.abs(after.height-72)<=1,route+' uses the shared region boundary '+JSON.stringify(after));
 }
 await screenshot(p,'operations-fixed-header');
 await p.setViewportSize({width:390,height:844});await p.evaluate(()=>window.scrollTo(0,500));
 const menu=p.getByRole('button',{name:'展开菜单',exact:true});await menu.waitFor();
 const button=await menu.boundingBox();assert.ok(button.y>=0&&button.y<56,'mobile menu stays in the upper fixed area');
 await menu.click();await p.getByRole('dialog',{name:'内容运营导航',exact:true}).waitFor();
});
