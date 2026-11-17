const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {pathToFileURL}=require('node:url');
const path=require('node:path');
const {attachStore}=require('./previewFixture.cjs');
let browser,seed;
before(async()=>{
 ({seed}=await import(pathToFileURL(path.resolve(__dirname,'../model.js'))));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});
});
after(()=>browser?.close());
async function setup(t){
 const context=await browser.newContext({viewport:{width:1280,height:800}}),p=await context.newPage(),errors=[];
 t.after(()=>context.close());p.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));
 p.setDefaultTimeout(7000);await attachStore(p,seed());
 await p.goto(process.env.EXAMPLE_URL+'/example/operations/production/strategies');
 await p.getByRole('heading',{name:'合成策略',exact:true}).waitFor();return p;
}
async function clean(p){
 await p.waitForFunction(()=>CP.dialogs.length===0&&document.querySelectorAll('.production-overlay-host').length===0);
 assert.equal(await p.evaluate(()=>document.body.style.overflow),'');
}
test('production details and confirmations use their scoped drawer and dialog geometry',async t=>{
 const p=await setup(t);
 await p.getByRole('button',{name:'详情',exact:true}).first().click();
 const detail=p.getByRole('dialog',{name:'合成策略详情',exact:true});await detail.waitFor();
 const box=await detail.boundingBox();
 assert.ok(Math.abs(box.width-960)<1&&Math.abs(box.x+box.width-1280)<1&&Math.abs(box.height-800)<1,'complex detail retains the 960px right drawer');
 assert.ok(box.x>=216,'detail does not overlap the desktop menu');
 if(process.env.SCREENSHOT_DIR)await p.screenshot({path:path.join(process.env.SCREENSHOT_DIR,'production-scoped-detail.png')});
 await detail.getByRole('button',{name:'关闭合成策略详情',exact:true}).click();await clean(p);
 await p.getByRole('switch').first().click();
 const confirm=p.getByRole('dialog');await confirm.waitFor();const modal=await confirm.boundingBox();
 assert.ok(Math.abs(modal.width-560)<1&&Math.abs(modal.x+modal.width/2-640)<1,'ordinary confirmation retains the 560px centered dialog');
 await confirm.getByRole('button',{name:'取消',exact:true}).click();await clean(p);
});
test('nested discard keeps unsaved input and releases every production overlay host on close',async t=>{
 const p=await setup(t);await p.getByRole('button',{name:'新增策略',exact:true}).click();
 const editor=p.getByRole('dialog',{name:'新增合成策略',exact:true});await editor.waitFor();
 const input=editor.locator('[data-field="name"]');await input.fill('未保存的策略');
 await editor.getByRole('button',{name:'关闭新增合成策略',exact:true}).click();
 const discard=p.getByRole('dialog',{name:'放弃未保存的修改？',exact:true});await discard.waitFor();
 assert.equal(await p.locator('.production-overlay-host').count(),2);
 const bounds=await discard.boundingBox();assert.ok(Math.abs(bounds.width-560)<1);
 await discard.getByRole('button',{name:'继续编辑',exact:true}).click();await discard.waitFor({state:'hidden'});
 assert.equal(await input.inputValue(),'未保存的策略');assert.equal(await p.evaluate(()=>document.body.style.overflow),'hidden');
 await p.setViewportSize({width:390,height:844});const narrow=await editor.boundingBox();
 assert.ok(narrow.x>=0&&narrow.x+narrow.width<=391);
 await editor.getByRole('button',{name:'关闭新增合成策略',exact:true}).click();await discard.waitFor();
 await discard.getByRole('button',{name:'放弃修改',exact:true}).click();await clean(p);
});
