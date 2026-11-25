const {requestBody}=require('./previewFixture.cjs');
const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {planFixture}=require('./planFixtures.cjs');
const {pathToFileURL}=require('node:url'),path=require('node:path'),fs=require('node:fs');
const source=process.env.PUBLISHING_SOURCE||path.resolve(__dirname,'..');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
const shots=process.env.SCREENSHOT_DIR||'/private/tmp/wt-state-actions-20261116/screenshots';
let browser,seed,createBatchStore,defaults,coverSignature,migrateRelationships;
before(async()=>{
 ({seed,defaults}=await import(pathToFileURL(path.join(source,'model.js'))));
 ({createBatchStore}=await import(pathToFileURL(path.join(source,'../../publishing-preview.mjs'))));
 ({migrateRelationships}=await import(pathToFileURL(path.join(source,'relationships.js'))));
 ({coverSignature}=await import(pathToFileURL(path.join(source,'covers.js'))));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});fs.mkdirSync(shots,{recursive:true});
});
after(()=>browser?.close());
async function setup(t,mutate=()=>{}){
 const c=await browser.newContext({viewport:{width:1440,height:1000}});t.after(async()=>{fs.writeFileSync(path.join(shots,'debug-'+t.name.slice(0,12)+'.txt'),await p.locator('body').innerText());await p.screenshot({path:path.join(shots,'debug-'+t.name.slice(0,12)+'.png')});await c.close();});
 const p=await c.newPage();p.setDefaultTimeout(7000);const errors=[];p.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));
 const db=seed();await mutate(db);migrateRelationships(db);const store=createBatchStore();await store.bootstrap(db);
 await p.route('**/__example/publishing/**',async r=>{
  try {const q=r.request(),body=q.method()==='GET'?null:requestBody(q);let result;
   if(q.url().endsWith('/command'))result=await store.command(body);
   else if(q.method()==='GET')result=await store.get();
   else if(q.url().endsWith('/bootstrap'))result=await store.bootstrap(body.data);
   else result=await store.save(body.data,body.revision);
   await r.fulfill({json:result});
  }catch(e){await r.fulfill({status:409,json:{error:e.message}});}
 });
 return {p,store};
}
const list=p=>p.locator('.resource-list-panel:visible .t-table__body tr');
const footer=p=>p.locator('.resource-detail-page:visible .resource-detail-page__footer');
async function goto(p,type,id){await p.goto(`${base}/example/operations/publishing/${type}${id?'/'+id:''}`);await p.getByRole('heading',{name:{plans:'发布计划',batches:'发布批次',tasks:'发布任务'}[type]+(id?'详情':''),exact:true}).waitFor();}
async function shot(p,name){for(const close of await p.locator('.t-message__close:visible').all())await close.click();await p.waitForTimeout(450);await p.screenshot({path:path.join(shots,name+'.png')});}


test('plan list execution and each detail footer follow the approved state decisions',async t=>{
 const {p,store}=await setup(t,planFixture);await goto(p,'plans');assert.equal(await list(p).count(),6);assert.equal(await list(p).filter({hasText:'PL-202611-001'}).count(),0);
 const ready=list(p).filter({hasText:'PL-202611-002'}),execute=ready.getByRole('button',{name:'执行',exact:true});assert.equal(await execute.isDisabled(),true);
 await execute.locator('..').hover();await p.getByText('请先完成批次配置',{exact:true}).last().waitFor();await shot(p,'01-plan-list-execution-unavailable');
 const cases=[['PL-202611-005',['暂停计划']],['PL-202611-004',['启动计划']],['PL-202611-003',['确认结束']],['PL-202611-002',[]],['PL-202610-018',[]],['PL-202610-015',[]]];
 let n=2;
 for(const [id,actions] of cases){await goto(p,'plans',id);assert.deepEqual((await footer(p).getByRole('button').allTextContents()).map(s=>s.trim()),['返回',...actions]);await shot(p,String(n++).padStart(2,'0')+'-plan-detail-'+id);}
 await goto(p,'plans');
 await list(p).filter({hasText:'PL-202611-004'}).getByRole('button',{name:'执行',exact:true}).click();const dialog=p.locator('dialog.plan-modal[open]');await dialog.waitFor();assert.match(await dialog.innerText(),/启动/);assert.match(p.url(),/publishing\/plans$/);await shot(p,'08-plan-execution-confirmation');
 await dialog.getByRole('button',{name:'确认启动',exact:true}).click();await p.getByText('计划已更新',{exact:true}).waitFor();assert.match(p.url(),/publishing\/plans$/);
 assert.equal((await store.get()).data.plans.find(p=>p.id==='PL-202611-004').control,'执行中');
 await goto(p,'plans');await p.setViewportSize({width:390,height:844});assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);await shot(p,'09-plan-list-mobile');
});

test('plan legacy recovery, restart and copy never add a formal row before creation',async t=>{
 const {p,store}=await setup(t,planFixture);
 await p.addInitScript(()=>localStorage.setItem('wt-publishing-plan-v1',JSON.stringify({version:1,ui:{recoveryEditor:{mode:'create',sourceId:'',name:'待续编计划',platforms:[],categories:[],target:0,goals:{},batches:[]}}})));
 await goto(p,'plans');
 await p.getByRole('button',{name:'新建发布计划',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).waitFor();await shot(p,'10-plan-recovery');
 await p.getByRole('button',{name:'继续编辑',exact:true}).click();await p.getByRole('heading',{name:'新建发布计划',exact:true}).waitFor();assert.equal(await p.getByPlaceholder('例如：新赛季内容矩阵 · 周度发布',{exact:true}).inputValue(),'待续编计划');
 await p.getByPlaceholder('例如：新赛季内容矩阵 · 周度发布',{exact:true}).fill('本轮续编');await p.getByRole('button',{name:'保存草稿',exact:true}).click();await p.getByRole('heading',{name:'发布计划',exact:true}).waitFor();assert.equal(await list(p).count(),6);
 await list(p).filter({hasText:'PL-202610-018'}).getByRole('button',{name:'复制',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();assert.equal(await p.getByPlaceholder('例如：新赛季内容矩阵 · 周度发布',{exact:true}).inputValue(),'本轮续编');
 await p.getByRole('button',{name:'保存草稿',exact:true}).click();await p.getByRole('button',{name:'新建发布计划',exact:true}).click();await p.getByRole('button',{name:'重新开始',exact:true}).click();assert.equal(await p.getByPlaceholder('例如：新赛季内容矩阵 · 周度发布',{exact:true}).inputValue(),'');
 assert.equal((await store.get()).data.plans.length,6);
});
function batchFixture(d){
 const original=d.batches[0],task=d.tasks[0];d.batches=[];d.tasks=[];d.plans=[];
 for(const [id,status] of [[301,'待执行'],[302,'排队中'],[303,'结果待确认'],[304,'已发布'],[305,'已取消']]){
  const b={...structuredClone(original),id,name:'状态走查-'+id,planId:null,phase:'内容预检通过',control:id===305?'已取消':'待执行',targets:[{category:task.category,count:1}],version:1,created:'2026-11-16T08:00:00Z'};delete b.firstStartedAt;delete b.stoppedAt;delete b.frozenSnapshot;
  d.batches.push(b);d.tasks.push({...structuredClone(task),id:id+1000,batchId:id,planId:null,status,attempts:[301,305].includes(id)?0:1,claimed:false,result:id===303?'未知':id===304?'已公开':'未提交',attemptHistory:[],batchConfigLocked:false,batchStopped:false});
 }
 d.batches.push({id:306,name:'个人历史草稿',platform:original.platform,targets:[{category:task.category,count:1}],phase:'待资源配置',control:'待配置',candidates:[]});
 d.batches.push({id:307,name:'计划批次骨架',planId:999,platform:original.platform,targets:[{category:task.category,count:1}],phase:'待资源配置',control:'待配置',candidates:[]});
 d.plans=[{id:999,name:'计划骨架走查',items:[{kind:'batch',id:307}],position:0,control:'规划中',log:[]}];
 d.publishDraft={active:true,mode:'批量发布',step:0,name:'当前批次续编',platforms:[],categories:[],targets:[],rows:[],assets:[]};
}
test('formal batch states and detail footers are consistent; copy keeps the creation buffer isolated',async t=>{
 const {p,store}=await setup(t,batchFixture);await goto(p,'batches');assert.equal(await list(p).count(),5);assert.equal(await p.getByText('个人历史草稿',{exact:true}).count(),0);await shot(p,'11-batch-formal-list');
 let n=12;
 for(const [id,buttons] of [[301,['返回','执行']],[302,['返回','停止发布']],[303,['返回']],[304,['返回']],[305,['返回']]]){await goto(p,'batches',id);assert.deepEqual((await footer(p).getByRole('button').allTextContents()).map(s=>s.trim()),buttons);await shot(p,String(n++).padStart(2,'0')+'-batch-detail-'+id);}
 await goto(p,'batches');assert.deepEqual((await list(p).filter({hasText:'状态走查-301'}).locator('td:last-child button').allTextContents()).map(s=>s.trim()),['执行','编辑','详情','终止']);
 await list(p).filter({hasText:'状态走查-304'}).getByRole('button',{name:'复制',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();assert.equal(await p.getByPlaceholder('请输入批次名称',{exact:true}).inputValue(),'当前批次续编');
 await footer(p).getByRole('button',{name:'返回',exact:true}).click();await list(p).filter({hasText:'状态走查-304'}).getByRole('button',{name:'复制',exact:true}).click();await p.getByRole('button',{name:'重新开始',exact:true}).click();assert.equal(await p.getByPlaceholder('请输入批次名称',{exact:true}).inputValue(),'状态走查-304（副本）');await shot(p,'17-batch-copy-configuration');
 await footer(p).getByRole('button',{name:'保存草稿',exact:true}).click();const snap=await store.get();assert.equal(snap.data.batches.length,7);assert.equal(snap.data.tasks.length,5);assert.equal(snap.data.publishDraft.sourceBatchId,304);assert.ok(snap.data.publishDraft.rows.every(r=>!r.attempts&&!r.attemptHistory&&!r.contentApproved));
});

test('legacy personal draft URLs use the same recovery prompt; plan skeletons stay configurable',async t=>{
 const {p,store}=await setup(t,batchFixture);await p.goto(base+'/example/operations/publishing/batches/306');
 await p.getByRole('button',{name:'继续编辑',exact:true}).waitFor();await shot(p,'18-old-draft-url-recovery');await p.getByRole('button',{name:'继续编辑',exact:true}).click();assert.equal(await p.getByPlaceholder('请输入批次名称',{exact:true}).inputValue(),'当前批次续编');
 await footer(p).getByRole('button',{name:'返回',exact:true}).click();await p.goto(base+'/example/operations/publishing/batches/307');await p.getByPlaceholder('请输入批次名称',{exact:true}).waitFor();assert.equal(await p.getByPlaceholder('请输入批次名称',{exact:true}).inputValue(),'计划批次骨架');assert.equal((await store.get()).data.publishDraft.name,'当前批次续编');await shot(p,'19-plan-batch-skeleton-configuration');
});

test('supplement uses concise shared actions and preserves actual URL/date validation',async t=>{
 const {p,store}=await setup(t);await goto(p,'tasks');const row=list(p).filter({hasText:'1006'});await row.getByRole('button',{name:'补录结果',exact:true}).click();const d=p.getByRole('dialog',{name:'补录发布结果',exact:true});await d.waitFor();await d.getByRole('button',{name:'确认补录',exact:true}).click();await d.locator('.t-alert').waitFor();assert.match(await d.locator('.t-alert').innerText(),/实际发布时间/);await shot(p,'20-supplement-validation');
 const dt=d.getByPlaceholder('请选择日期和时间');await dt.fill('2026-11-15 10:30');await dt.press('Enter');await d.getByRole('button',{name:'确认补录',exact:true}).click();await d.locator('.t-alert').filter({hasText:'链接'}).waitFor();
 await d.getByRole('button',{name:'取消',exact:true}).click();await d.waitFor({state:'hidden'});await row.getByRole('button',{name:'补录结果',exact:true}).click();await d.waitFor();await d.getByPlaceholder('请选择日期和时间').fill('2026-11-15 10:30');await d.getByPlaceholder('请选择日期和时间').press('Enter');await d.getByPlaceholder('具体作品的 HTTPS 链接').fill('https://www.bilibili.com/video/BV123abc');await shot(p,'21-supplement-filled');await p.setViewportSize({width:390,height:844});await shot(p,'23-supplement-mobile');assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);await p.setViewportSize({width:1440,height:1000});await d.getByRole('button',{name:'确认补录',exact:true}).click();await d.waitFor({state:'hidden'});
 const result=(await store.get()).data.tasks.find(t=>t.id===1006);assert.equal(result.status,'已发布');assert.ok(result.publicAt);assert.equal(result.workUrl,'https://www.bilibili.com/video/BV123abc');
 await goto(p,'batches',202);const taskrow=p.locator('.resource-detail-page:visible .t-table__body tr').filter({hasText:'1006'});assert.equal(await taskrow.getByRole('button',{name:'补录结果',exact:true}).count(),0);
});

test('recovered copy creates one formal batch, clears the buffer and preserves source tracking',async t=>{
 const {p,store}=await setup(t,d=>{
  d.plans=[];
  const asset=d.assets.find(a=>a.id===139),platform='B站';
  const row={...defaults(platform,asset),platform,category:asset.category,assetId:asset.id,accountId:d.accounts.find(a=>a.platform===platform&&a.enabled&&a.categories.includes(asset.category)).id,candidateId:'source-tracking',contentVersion:1,inputVersion:1,fieldVersions:{},ai:{suggestions:{}},declaration:'内容无需标注',contentApproved:true,coverApproved:true};
  row.coverSignature=coverSignature(row);row.coverVariants={source:row.cover};
  d.publishDraft={active:true,mode:'批量发布',step:3,name:'来源追溯副本',platforms:[platform],categories:[asset.category],rows:[row],assets:[asset.id],targets:[{category:asset.category,count:1}],resourcesLocked:true,creationKey:'source-tracking-create',sourceBatchId:203,sourcePlanId:999,taskIntervalSeconds:0};
 });
 await goto(p,'batches');const before=(await store.get()).data;
 await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();
 await p.getByRole('button',{name:'创建批次',exact:true}).click();await p.getByText('批次与任务已创建',{exact:true}).waitFor();
 const after=(await store.get()).data;assert.equal(after.batches.length,before.batches.length+1);assert.equal(after.tasks.length,before.tasks.length+1);assert.equal(after.publishDraft.active,false);
 const batch=after.batches.find(b=>b.name==='来源追溯副本');assert.equal(batch.sourceBatchId,203);assert.equal(batch.sourcePlanId,999);assert.equal(batch.firstStartedAt,undefined);
 await p.getByRole('button',{name:'查看批次',exact:true}).click();await p.getByRole('heading',{name:'发布批次详情',exact:true}).waitFor();await footer(p).getByRole('button',{name:'返回',exact:true}).click();
 const row=list(p).filter({hasText:'来源追溯副本'});assert.match(await row.innerText(),/复制自/);await shot(p,'22-copy-created-source-tracking');
});

test('all ten task detail states remain read only with a return footer',async t=>{
 const {p}=await setup(t);for(let id=1001;id<=1010;id++){
  await goto(p,'tasks',id);assert.deepEqual((await footer(p).getByRole('button').allTextContents()).map(s=>s.trim()),['返回']);
  await shot(p,String(id-977).padStart(2,'0')+'-task-detail-'+id);
 }
});

test('stopping plan keeps a read only detail footer',async t=>{
 const {p}=await setup(t,async db=>{await planFixture(db);const {cancelPlan}=await import(pathToFileURL(path.join(source,'relationships.js')));cancelPlan(db.plans.find(p=>p.id==='PL-202611-005'),db);});
 await goto(p,'plans','PL-202611-005');assert.deepEqual((await footer(p).getByRole('button').allTextContents()).map(s=>s.trim()),['返回']);await shot(p,'34-plan-stopping-detail');
});
