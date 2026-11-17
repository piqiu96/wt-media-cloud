const {requestBody}=require('./previewFixture.cjs');
const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {attachStore}=require('./previewFixture.cjs');
const {pathToFileURL}=require('node:url'),path=require('node:path'),fs=require('node:fs');
const source=process.env.PUBLISHING_SOURCE||path.resolve(__dirname,'..');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
const shots=process.env.SCREENSHOT_DIR||'/private/tmp/wt-plan-batch-drag-review';
let browser,seed;
before(async()=>{({seed}=await import(pathToFileURL(path.join(source,'model.js'))));browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});fs.mkdirSync(shots,{recursive:true});});
after(()=>browser?.close());
const rows=p=>p.locator('[data-plan-batch-row]');
const handle=(p,id)=>p.locator(`[data-plan-batch-handle="${id}"]`);
const row=(p,id)=>p.locator(`[data-plan-batch-row="${id}"]`);
async function ids(p){return rows(p).evaluateAll(rows=>rows.map(row=>row.dataset.planBatchRow));}
async function saved(p){const {planViews}=await import(pathToFileURL(path.join(source,'planBridge.js')));return {plans:planViews((await p.store.get()).data)};}
function orderSaved(p,id,expected){return p.waitForResponse(response=>{const request=response.request();if(!response.url().endsWith('/publishing/snapshot')||request.method()!=='PUT'||!response.ok())return false;const plan=requestBody(request)?.data?.plans.find(plan=>String(plan.id)===String(id));return JSON.stringify(plan?.items.map(item=>String(item.id)))===JSON.stringify(expected);});}
async function shot(p,name){await p.mouse.move(5,5);await p.locator('.batches-table').scrollIntoViewIfNeeded();await p.screenshot({path:path.join(shots,name+'.png')});}
async function setup(t,{locked=false}={}){
 const context=await browser.newContext({viewport:{width:1440,height:1000}});t.after(()=>context.close());const p=await context.newPage();p.setDefaultTimeout(7000);
 const errors=[];p.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));
 const data=seed();data.plans=[];data.batches=[];data.tasks=[];
 const {confirmPlan}=await import(pathToFileURL(path.join(source,'relationships.js')));
 const canonical=confirmPlan(data,{id:'PL-DRAG',name:'拖拽批次检查',confirmKey:'drag',platformOrder:['B站'],targets:[{platform:'B站',category:'三角洲行动',count:5}],splits:Array.from({length:5},(_,i)=>({name:'批次 '+(i+1),platform:'B站',targets:[{category:'三角洲行动',count:1}]}))});Object.assign(canonical,{ownerId:'chen',ownerName:'陈小雨',createdBy:'chen',dateStart:'2026-11-17',dateEnd:'2026-11-30'});
 if(locked)data.batches[2].draft={resourcesLocked:true};
 p.store=await attachStore(p,data);
 const plan=(await saved(p)).plans[0];
 await p.goto(base+'/example/operations/publishing/plans/'+plan.id+'/edit');await rows(p).first().waitFor();
 return {p,plan:structuredClone(plan)};
}

test('batch drag moves to its target, preserves data and saves the order across reload',async t=>{
 const {p,plan}=await setup(t),before=plan.batches.map(batch=>batch.id);
 assert.equal(await p.locator('[data-act="move-batch"]').count(),0);
 assert.equal(await handle(p,before[0]).getAttribute('draggable'),'true');
 await shot(p,'01-unlocked-handles');await handle(p,before[0]).dragTo(row(p,before[4]));
 const expected=[...before.slice(1),before[0]];assert.deepEqual(await ids(p),expected);
 let s=await saved(p);assert.deepEqual(s.plans.find(x=>x.id===plan.id).batches,plan.batches);await shot(p,'02-reordered');
 const persisted=orderSaved(p,plan.id,expected);await p.getByRole('button',{name:'保存',exact:true}).click();await persisted;await p.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();
 s=await saved(p);assert.deepEqual(s.plans.find(x=>x.id===plan.id).batches.map(batch=>batch.id),expected);
 await p.reload();await p.goto(base+'/example/operations/publishing/plans/'+plan.id+'/edit');await rows(p).first().waitFor();assert.deepEqual(await ids(p),expected);
});

test('locked rows cannot start a drag or be crossed, while their editable neighbors can reorder',async t=>{
 const {p,plan}=await setup(t,{locked:true}),before=plan.batches.map(batch=>batch.id),locked=handle(p,before[2]);
 assert.equal(await locked.isDisabled(),true);assert.equal(await locked.getAttribute('draggable'),'false');
 assert.equal(await locked.evaluate(el=>{const event=new DragEvent('dragstart',{bubbles:true,cancelable:true,dataTransfer:new DataTransfer()});el.dispatchEvent(event);return event.defaultPrevented;}),true);
 assert.equal(await p.locator('.is-batch-dragging').count(),0);await shot(p,'03-locked-boundary');
 await handle(p,before[4]).dragTo(row(p,before[0]));assert.deepEqual(await ids(p),before);
 await handle(p,before[0]).dragTo(row(p,before[2]));assert.deepEqual(await ids(p),before);
 await handle(p,before[4]).dragTo(row(p,before[3]));const expected=[before[0],before[1],before[2],before[4],before[3]];assert.deepEqual(await ids(p),expected);
 const persisted=orderSaved(p,plan.id,expected);await p.getByRole('button',{name:'保存',exact:true}).click();await persisted;await p.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();
 const s=await saved(p),result=s.plans.find(x=>x.id===plan.id);assert.deepEqual(result.batches[2],plan.batches[2]);assert.deepEqual(result.batches.map(batch=>batch.id),expected);
});

test('drag feedback shows the destination and Escape cancels without changing order or dirty state',async t=>{
 const {p,plan}=await setup(t),before=plan.batches.map(batch=>batch.id);
 await handle(p,before[0]).scrollIntoViewIfNeeded();const a=await handle(p,before[0]).boundingBox(),b=await row(p,before[1]).boundingBox();
 await p.mouse.move(a.x+a.width/2,a.y+a.height/2);await p.mouse.down();await p.mouse.move(a.x+a.width/2+12,a.y+a.height/2,{steps:4});await p.mouse.move(b.x+55,b.y+b.height/2,{steps:8});
 await row(p,before[1]).locator('xpath=self::*[contains(@class,"is-drop-after")]').waitFor();
 await p.screenshot({path:path.join(shots,'04-drag-destination.png')});await p.keyboard.press('Escape');await p.mouse.up();
 assert.deepEqual(await ids(p),before);assert.equal(await p.locator('.is-batch-dragging,.is-drop-before,.is-drop-after').count(),0);await p.locator('.editor-footer [data-act=detail]').click();await p.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();assert.equal(await p.locator('dialog.plan-modal[open]').count(),0);
});

test('one handle supports keyboard sorting, preserves focus and respects locked boundaries',async t=>{
 const {p,plan}=await setup(t,{locked:true}),before=plan.batches.map(batch=>batch.id),moving=before[3];
 await handle(p,moving).focus();await p.keyboard.press('ArrowUp');assert.deepEqual(await ids(p),before);
 await p.keyboard.press('ArrowDown');assert.deepEqual(await ids(p),[before[0],before[1],before[2],before[4],before[3]]);
 assert.equal(await handle(p,moving).evaluate(el=>el===document.activeElement),true);
 await p.keyboard.press('ArrowDown');assert.deepEqual(await ids(p),[before[0],before[1],before[2],before[4],before[3]]);await shot(p,'05-keyboard-order');
});
