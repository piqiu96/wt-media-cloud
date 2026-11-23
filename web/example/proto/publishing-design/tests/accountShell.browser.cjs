const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {attachStore}=require('./previewFixture.cjs');
const {pathToFileURL}=require('node:url');const path=require('node:path');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';let browser;
before(async()=>browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE}));
after(async()=>browser?.close());
async function setup(t,url='/example/operations/production/materials'){
 const context=await browser.newContext({viewport:{width:1440,height:1000}});t.after(()=>context.close());
 const page=await context.newPage();page.setDefaultTimeout(15000);const errors=[];
 page.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));
 const {seed}=await import(pathToFileURL(path.join(__dirname,'../model.js')));await attachStore(page,seed());
 await page.goto(base+url);await page.locator('.operations-nav').waitFor({state:'attached'});return page;
}
const heading=(p,name)=>p.getByRole('heading',{name,exact:true});
const menu=(p,name)=>p.locator('.operations-nav-link').filter({hasText:name}).click();
async function account(p){await p.getByRole('button',{name:'我的账号',exact:true}).click();await p.getByText('我的账号',{exact:true}).last().click();await heading(p,'我的账号').waitFor();}

test('account and local settings reuse the production/publishing document and one navigation',async t=>{
 const p=await setup(t);await heading(p,'素材库').waitFor();await p.evaluate(()=>window.__accountShellToken='same-shell');
 await account(p);assert.match(p.url(),/\/account\/profile$/);assert.equal(await p.locator('.identity strong').innerText(),'陈小雨');
 assert.equal(await p.locator('.operations-nav-link').filter({hasText:'我的账号'}).count(),0);
 for(const name of ['发布任务','发布批次','发布计划','桌面设置','素材库']){
  await menu(p,name);await heading(p,name).waitFor();assert.equal(await p.locator('.operations-nav').count(),1);assert.equal(await p.locator('.app-sidebar').count(),0);assert.equal(await p.locator('iframe').count(),0);assert.equal(await p.evaluate(()=>window.__accountShellToken),'same-shell');
 }
 await account(p);await p.reload();await heading(p,'我的账号').waitFor();assert.equal(await p.locator('.operations-nav').count(),1);
});

test('profile save updates shared account entry and actor switching uses the same identity',async t=>{
 const p=await setup(t,'/example/operations/account/profile');await p.getByRole('button',{name:'编辑',exact:true}).click();await p.locator('.profile-form input[type=text]').fill('运营昵称');await p.locator('.profile-form').getByRole('button',{name:'保存',exact:true}).click();
 assert.match(await p.getByRole('button',{name:'我的账号',exact:true}).innerText(),/运营昵称/);
 await menu(p,'发布任务');await heading(p,'发布任务').waitFor();await account(p);assert.equal(await p.locator('.identity strong').innerText(),'运营昵称');
 await p.getByRole('combobox',{name:'当前使用者',exact:true}).selectOption('林予安');assert.equal(await p.locator('.identity strong').innerText(),'林予安');
 await p.getByRole('combobox',{name:'当前使用者',exact:true}).selectOption('周予宁');await p.locator('.browser-card').waitFor({state:'detached'});assert.equal(await p.locator('.operations-nav-link').filter({hasText:'桌面设置'}).count(),0);
});

test('unsaved profile protects shared menu and browser Back',async t=>{
 const p=await setup(t);await heading(p,'素材库').waitFor();await account(p);await p.getByRole('button',{name:'编辑',exact:true}).click();await p.locator('.profile-form input[type=text]').fill('保留草稿');
 await menu(p,'发布任务');await p.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();assert.equal(await p.locator('.profile-form input[type=text]').inputValue(),'保留草稿');
 await p.goBack();await p.getByRole('dialog').getByRole('button',{name:'取消',exact:true}).click();assert.match(p.url(),/\/account\/profile$/);assert.equal(await p.locator('.profile-form input[type=text]').inputValue(),'保留草稿');
 await menu(p,'发布任务');await p.getByRole('dialog').getByRole('button',{name:'放弃修改',exact:true}).click();await heading(p,'发布任务').waitFor();
});

test('old review links redirect to one workbench and fixture control remains outside the business menu',async t=>{
 const p=await setup(t,'/example/proto/personal-settings/review.html#avatar');await p.locator('.avatar-list').waitFor();assert.match(p.url(),/\/account\/profile\?review=account/);assert.equal(await p.locator('iframe').count(),0);assert.equal(await p.locator('.operations-nav').count(),1);
 await p.getByLabel('原型状态',{exact:true}).click();await p.getByText('访问范围 · 管理员',{exact:true}).click();await p.locator('.browser-card').waitFor({state:'detached'});
 await p.getByLabel('原型状态',{exact:true}).click();await p.getByText('资料 · 默认只读',{exact:true}).click();await p.locator('.browser-card').waitFor();
 await p.goto(base+'/example/proto/personal-settings/?tab=log');await p.locator('.log-reader').waitFor();assert.match(p.url(),/\/desktop\/settings\?tab=log$/);assert.equal(await p.locator('.operations-nav').count(),1);
});


test('local settings stays in the sidebar footer while business navigation scrolls',async t=>{
 const p=await setup(t,'/example/operations/desktop/settings');
 await heading(p,'桌面设置').waitFor();
 assert.equal(await p.locator('.operations-nav nav .operations-settings-entry').count(),0);
 assert.equal(await p.locator('.operations-nav nav').getByText('桌面环境',{exact:true}).count(),0);
 const entry=p.locator('.operations-nav-footer .operations-settings-entry');
 assert.equal(await entry.getAttribute('aria-current'),'page');
 for(const height of [600,800,1000]){
  await p.setViewportSize({width:1280,height});
  const before=await entry.boundingBox();assert.equal(Math.round(before.y+before.height),height-16);
  const nav=p.locator('.operations-nav nav');
  await nav.evaluate(e=>{e.scrollTop=e.scrollHeight});
  const after=await entry.boundingBox();assert.equal(after.y,before.y);
  const navBox=await nav.boundingBox(),footer=await p.locator('.operations-nav-footer').boundingBox();
  assert(navBox.y+navBox.height<=footer.y);
 }
 await menu(p,'发布任务');await heading(p,'发布任务').waitFor();
 assert.equal(await entry.getAttribute('aria-current'),null);
 await p.getByRole('combobox',{name:'当前使用者',exact:true}).selectOption('周予宁');
 assert.equal(await p.locator('.operations-settings-entry').count(),0);
});
