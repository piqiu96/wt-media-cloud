const {requestBody}=require('./previewFixture.cjs');
const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {pathToFileURL}=require('node:url'),path=require('node:path'),fs=require('node:fs');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
const shots=process.env.SCREENSHOT_DIR||'/tmp/wt-batch-inline-after';
let browser,seed;
before(async()=>{({seed}=await import(pathToFileURL(path.resolve(__dirname,'../model.js'))));browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true});fs.mkdirSync(shots,{recursive:true});});
after(async()=>browser?.close());
async function setup(t,mutate=()=>{}){
 const context=await browser.newContext({viewport:{width:1440,height:1000}});t.after(()=>context.close());const p=await context.newPage();p.setDefaultTimeout(5000);
 const errors=[];p.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));
 let data=seed(),revision=1;mutate(data);const writes=[];
 await p.route('**/__example/publishing/**',r=>{if(r.request().method()!=='GET'){data=requestBody(r.request()).data;revision++;writes.push(structuredClone(data));}return r.fulfill({json:{revision,data}});});
 await p.goto(base+'/example/?view=publishing#batches');await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByPlaceholder('请输入批次名称',{exact:true}).fill('同页资源配置');return {p,writes};
}
const quantity=p=>p.locator('.batch-goal-field .t-input-number input');
const categoryQuantity=p=>p.locator('.batch-target-table .t-input-number input').first();
async function fillQuantity(input,value){await input.fill(String(value));await input.press('Tab');}
async function choose(p){await p.getByPlaceholder('选择一个平台',{exact:true}).click();await p.locator('.t-select-option').filter({hasText:'小红书'}).click();await p.locator('.batch-category-chip').filter({hasText:'三角洲行动'}).click();await p.locator('.batch-target-table').waitFor();}
async function advance(p){
 await p.locator('.batch-matching-table tbody tr').first().waitFor();
 assert.equal(await p.getByRole('button',{name:'匹配账号与成片',exact:true}).count(),0);
 assert.equal(await p.getByRole('button',{name:'上一步',exact:true}).count(),0);
 assert.equal(await p.getByRole('button',{name:'确认资源',exact:true}).isEnabled(),true);
 assert.equal(await p.locator('[data-section="batch-scope"] input[placeholder="请输入批次名称"]').isVisible(),true);
 assert.equal(await p.locator('.batch-target-table').isVisible(),true);
 assert.match(await p.locator('.batch-target-table thead').innerText(),/可用账号[\s\S]*可用成片/);
}
async function shot(p,name){await p.waitForTimeout(200);await p.screenshot({path:path.join(shots,name+'.png'),fullPage:true});}
test('scope and categories share one page; blank advisory target allows matching',async t=>{const {p}=await setup(t);await choose(p);assert.equal(await quantity(p).inputValue(),'');assert.equal(await p.getByRole('button',{name:'下一步：配置类目数量',exact:true}).count(),0);assert.equal(await p.locator('[data-section="batch-scope"] input[placeholder="请输入批次名称"]').isVisible(),true);await shot(p,'01-no-target');await advance(p);await shot(p,'02-matched-without-target');});
test('unreachable overall quantity recommends available resources and does not block',async t=>{const {p}=await setup(t);await choose(p);await fillQuantity(quantity(p),9999);await p.getByRole('button',{name:'采用系统推荐',exact:true}).click();let count=Number(await categoryQuantity(p).inputValue());assert.ok(count>0&&count<9999);assert.match(await p.locator('.batch-total-line').innerText(),/目标 9999 条/);await shot(p,'03-unreachable-target');await advance(p);await p.getByRole('button',{name:'确认资源',exact:true}).click();await p.getByRole('button',{name:'进入内容配置',exact:true}).click();await p.locator('.batch-content-workspace').waitFor();await shot(p,'06-content-with-unreachable-target');});
test('category totals below and above advisory target remain editable and can advance',async t=>{const {p}=await setup(t);await choose(p);await fillQuantity(quantity(p),1);await fillQuantity(categoryQuantity(p),2);assert.match(await p.locator('.batch-total-line').innerText(),/超出目标 1 条/);await advance(p);await fillQuantity(quantity(p),10);assert.equal(await categoryQuantity(p).inputValue(),'2','changing advisory target must preserve manual category quantities');assert.match(await p.locator('.batch-total-line').innerText(),/比目标少 8 条/);await p.setViewportSize({width:390,height:844});await p.waitForTimeout(350);assert.ok(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await shot(p,'04-manual-quantity-mobile');await advance(p);});
test('advisory target survives draft recovery; unavailable resources remain guarded',async t=>{const {p,writes}=await setup(t,d=>{d.accounts.forEach(a=>a.enabled=false);});await choose(p);await fillQuantity(quantity(p),20);await p.getByRole('button',{name:'保存草稿',exact:true}).click();assert.equal(writes.at(-1).publishDraft.desiredQuantity,20);await p.locator('.resource-detail-page__footer').getByRole('button',{name:'返回',exact:true}).click();await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();assert.equal(await quantity(p).inputValue(),'20');assert.equal(await p.getByRole('button',{name:'确认资源',exact:true}).isDisabled(),true);assert.equal(await p.locator('.batch-matching-table tbody tr').count(),0);await shot(p,'05-resource-guard');});

test('batch name is required before confirming resources and only quantity changes rebuild matching',async t=>{
 const {p}=await setup(t);await choose(p);await advance(p);
 const name=p.getByPlaceholder('请输入批次名称',{exact:true});await name.fill('');
 await p.getByRole('button',{name:'确认资源',exact:true}).click();
 await p.getByRole('alert').filter({hasText:'请输入批次名称'}).waitFor();
 assert.equal(await p.getByRole('button',{name:'进入内容配置',exact:true}).count(),0);
 await name.fill('正式发布批次');await fillQuantity(categoryQuantity(p),1);
 assert.equal(await p.locator('.batch-matching-table tbody tr').count(),1);
 await p.getByRole('button',{name:'确认资源',exact:true}).click();
 await p.getByRole('button',{name:'进入内容配置',exact:true}).waitFor();await shot(p,'07-required-name-fixed');
});

test('automatic matching preserves a compatible manual account and reacts to pool changes',async t=>{
 const {p}=await setup(t,d=>{const original=d.accounts.find(a=>a.platform==='小红书'&&a.categories.includes('三角洲行动'));d.accounts.push({...structuredClone(original),id:999,name:'备选发布账号'});});
 await choose(p);await fillQuantity(categoryQuantity(p),2);
 const selected=p.locator('.batch-matching-table tbody tr').first().locator('.t-select');
 await selected.click();await p.locator('.t-select-option').filter({hasText:'备选发布账号'}).click();
 await fillQuantity(categoryQuantity(p),1);assert.match(await selected.locator('input').inputValue(),/备选发布账号/);
 await p.getByRole('button',{name:'选择三角洲行动发布账号',exact:true}).click();
 const dialog=p.getByRole('dialog').filter({hasText:'选择发布账号 · 三角洲行动'});
 await dialog.locator('.t-checkbox').filter({hasText:'备选发布账号'}).click();await dialog.getByRole('button',{name:'确定选择',exact:true}).click();
 assert.doesNotMatch(await selected.locator('input').inputValue(),/备选发布账号/);await shot(p,'08-pool-rematched');
});

test('category count opens account selection; cancellation, independent shared pools and draft recovery work',async t=>{
 const {p,writes}=await setup(t,d=>{d.tasks=[];const a=d.accounts.find(a=>a.platform==='小红书');d.accounts=[{...a,id:777,name:'跨类目发布账号',categories:['三角洲行动','火影忍者'],enabled:true,loginValid:true}];});
 await choose(p);await p.locator('.batch-category-chip').filter({hasText:'火影忍者'}).click();
 const input=p.locator('.batch-target-table tbody tr').filter({hasText:'火影忍者'}).locator('.t-input-number input');await fillQuantity(input,1);
 assert.equal(await p.locator('.batch-account-pool').count(),0);
 const count=p.getByRole('button',{name:'选择三角洲行动发布账号',exact:true});await count.click();
 let dialog=p.getByRole('dialog').filter({hasText:'选择发布账号 · 三角洲行动'});
 await dialog.getByRole('button',{name:'清空',exact:true}).click();await dialog.getByRole('button',{name:'取消',exact:true}).click();assert.match(await count.innerText(),/1 个/);
 await count.click();dialog=p.getByRole('dialog').filter({hasText:'选择发布账号 · 三角洲行动'});await dialog.getByRole('button',{name:'清空',exact:true}).click();await dialog.getByRole('button',{name:'确定选择',exact:true}).click();
 assert.match(await count.innerText(),/0 个/);assert.match(await p.getByRole('button',{name:'选择火影忍者发布账号',exact:true}).innerText(),/1 个/);
 await p.getByRole('button',{name:'保存草稿',exact:true}).click();assert.deepEqual(writes.at(-1).publishDraft.categoryAccountIds['三角洲行动'],[]);
 await p.locator('.resource-detail-page__footer').getByRole('button',{name:'返回',exact:true}).click();await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();assert.match(await count.innerText(),/0 个/);
 await p.getByRole('button',{name:'选择火影忍者发布账号',exact:true}).click();await p.waitForTimeout(3200);await shot(p,'10-category-accounts-desktop');await p.setViewportSize({width:390,height:844});await p.waitForTimeout(350);assert.ok(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await shot(p,'09-category-accounts-mobile');
});
