const {requestBody}=require('./previewFixture.cjs');
const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {pathToFileURL}=require('node:url'),path=require('node:path'),fs=require('node:fs');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
const shots=process.env.SCREENSHOT_DIR||'/private/tmp/wt-batch-content-after';
const source=process.env.PUBLISHING_SOURCE||path.resolve(__dirname,'..');
let browser,seed,defaults,coverSignature,defaultDocument,runBatchCommand;
before(async()=>{({seed,defaults}=await import(pathToFileURL(path.join(source,'model.js'))));({coverSignature,defaultDocument}=await import(pathToFileURL(path.join(source,'covers.js'))));({runBatchCommand}=await import(pathToFileURL(path.join(source,'batchLifecycle.js'))));browser=await chromium.launch({executablePath:process.env.CHROMIUM_EXECUTABLE,headless:true});fs.mkdirSync(shots,{recursive:true});});
after(async()=>browser?.close());
async function setup(t,{count=7,platform='B站',openDraft=true,mutate=()=>{}}={}){
 const c=await browser.newContext({viewport:{width:1440,height:1000}});t.after(()=>c.close());const p=await c.newPage();p.setDefaultTimeout(6000);
 const errors=[];p.on('pageerror',e=>errors.push(e.message));t.after(()=>assert.deepEqual(errors,[]));
 let data=seed(),revision=1;data.tasks=[];
 const assets=data.assets.filter(v=>v.ready!==false&&v.fileAvailable!==false).slice(0,count);
 const rows=assets.map((v,i)=>({...defaults(platform,v),platform,category:v.category,assetId:v.id,accountId:data.accounts.find(a=>a.platform===platform&&a.enabled&&a.categories.includes(v.category)).id,candidateId:'review-'+i,contentVersion:1,inputVersion:1,fieldVersions:{},ai:{suggestions:{}},contentApproved:false,coverApproved:false}));
 data.templates=[{id:9001,name:'走查横版模板',enabled:true,version:1,document:defaultDocument(1200,900)},{id:9002,name:'走查宽版模板',enabled:true,version:1,document:defaultDocument(1600,900)}];
 data.publishDraft={active:true,mode:'批量发布',step:2,name:'内容配置走查',platforms:[platform],categories:[...new Set(assets.map(v=>v.category))],rows,assets:assets.map(v=>v.id),targets:[...new Set(assets.map(v=>v.category))].map(category=>({category,count:assets.filter(v=>v.category===category).length})),resourcesLocked:true,creationKey:'batch-content-review'};
 rows.forEach(row=>{row.coverSignature=coverSignature(row);row.coverVariants={source:row.cover};});
 mutate(data);const writes=[];
 await p.route('**/__example/publishing/**',r=>{if(r.request().method()!=='GET'){const body=requestBody(r.request());if(r.request().url().endsWith('/command'))runBatchCommand(data,data.batches.find(b=>b.id===body.batchId),body.action,body.input);else data=body.data;revision++;writes.push(structuredClone(data));}return r.fulfill({json:{revision,data}});});
 await p.goto(base+'/example/?view=publishing#batches');if(openDraft){await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();await p.locator('.batch-content-workspace').waitFor();}return {p,writes};
}
const table=p=>p.locator('.batch-content-table');
const editor=p=>p.getByRole('dialog',{name:'编辑发布内容',exact:true});
async function editFirst(p){await table(p).getByRole('button',{name:'编辑',exact:true}).first().click();await editor(p).waitFor();await p.waitForTimeout(350);return editor(p);}
async function choose(p,label,text){await editor(p).getByLabel(label,{exact:true}).click();await p.locator('.t-select-option').filter({hasText:text}).last().click();}
async function shot(p,name){for(const close of await p.locator('.t-message__close').all())if(await close.isVisible())await close.click();await p.waitForTimeout(450);await p.screenshot({path:path.join(shots,name+'.png'),fullPage:false});}

test('content list separates account and asset, applies filters, and requires explicit bulk selection',async t=>{
 const {p,writes}=await setup(t);assert.match(await table(p).locator('thead').innerText(),/发布内容[\s\S]*发布账号[\s\S]*发布成片[\s\S]*配置状态/);
 assert.equal(await table(p).locator('tbody tr').count(),7);assert.match(await table(p).locator('tbody tr').first().innerText(),/待完善[\s\S]*创作声明/);
 assert.equal(await p.locator('.batch-content-selection').count(),0);
 await p.getByLabel('搜索发布内容',{exact:true}).fill('火影');assert.equal(await table(p).locator('tbody tr').count(),7);
 await p.getByRole('button',{name:'查询',exact:true}).click();assert.equal(await table(p).locator('tbody tr').count(),3);
 await p.getByRole('button',{name:'重置',exact:true}).click();assert.equal(await table(p).locator('tbody tr').count(),7);
 await table(p).locator('tbody tr').first().locator('.t-checkbox').click();
 await p.getByRole('button',{name:'批量配置',exact:true}).click();const ai=p.getByRole('dialog',{name:'批量配置',exact:true});assert.match(await ai.innerText(),/已选 1 条/);
 await ai.getByText('AI 文案',{exact:true}).click();await ai.getByText('简介 / 正文',{exact:true}).click();await ai.getByRole('button',{name:'生成并审阅',exact:true}).click();await ai.locator('.bulk-config-result').waitFor();await shot(p,'09-ai-preview-dialog');
 assert.equal(await ai.isVisible(),true);await ai.getByRole('button',{name:'应用配置',exact:true}).click();await ai.waitFor({state:'hidden'});
 const rows=table(p).locator('tbody tr');assert.match(await rows.first().innerText(),/待完善/);assert.doesNotMatch(await rows.nth(1).innerText(),/AI 建议/);

 await p.locator('.resource-detail-page__footer').getByRole('button',{name:'保存草稿',exact:true}).click();
 assert.equal(writes.at(-1).publishDraft.rows[0].title,'三角洲实战精彩片段');assert.ok(writes.at(-1).publishDraft.rows[0].description);assert.equal(writes.at(-1).publishDraft.rows[1].description,'');
 await shot(p,'01-content-list');
});

test('single save validates and confirms content, guards discard, and preserves selection',async t=>{
 const {p,writes}=await setup(t,{count:3});await table(p).locator('tbody tr').first().locator('.t-checkbox').click();
 let d=await editFirst(p);assert.ok(Math.abs((await d.boundingBox()).width-960)<2);
 const title=await d.getByLabel('发布标题',{exact:true}).boundingBox(),preview=await d.locator('.batch-content-cover-preview').boundingBox();assert.ok(preview.y>title.y+title.height);assert.ok(Math.abs(preview.x-title.x)<2);assert.ok(Math.abs(preview.width-title.width)<2);assert.equal(await d.locator('.batch-content-source > div').count(),2);await shot(p,'02-edit-dialog-top');
 await d.getByLabel('发布标题',{exact:true}).fill('');await d.getByRole('button',{name:'保存',exact:true}).click();await d.getByRole('alert').filter({hasText:'标题'}).waitFor();assert.equal(await d.isVisible(),true);
 await d.getByLabel('发布标题',{exact:true}).fill('人工修改标题');await d.getByRole('button',{name:'保存',exact:true}).click();await d.getByRole('alert').filter({hasText:'创作声明'}).waitFor();
 await choose(p,'创作声明','内容无需标注');await d.getByRole('button',{name:'保存',exact:true}).click();await d.waitFor({state:'hidden'});assert.match(await table(p).locator('tbody tr').first().innerText(),/人工修改标题[\s\S]*已就绪/);assert.match(await p.locator('.batch-content-selection').innerText(),/1/);
 d=await editFirst(p);await d.getByLabel('简介 / 正文',{exact:true}).fill('未保存的简介');await d.getByRole('button',{name:'取消',exact:true}).click();const guard=p.locator('.t-dialog').filter({hasText:'放弃本条未保存修改？'});await guard.getByRole('button',{name:'取消',exact:true}).click();assert.equal(await d.getByLabel('简介 / 正文',{exact:true}).inputValue(),'未保存的简介');await d.getByRole('button',{name:'取消',exact:true}).click();await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await d.waitFor({state:'hidden'});
 await p.getByRole('button',{name:'保存草稿',exact:true}).click();const row=writes.at(-1).publishDraft.rows[0];assert.equal(row.title,'人工修改标题');assert.equal(row.description,'');assert.equal(row.contentApproved,true);assert.equal(writes.at(-1).tasks.length,0);await shot(p,'04-confirmed-list');
});

test('template text renders real covers, changed text invalidates confirmation, and precheck blocks creation',async t=>{
 const {p,writes}=await setup(t,{count:1});let d=await editFirst(p);
 await d.getByText('使用模板',{exact:true}).click();assert.equal(await d.getByLabel('封面主标题',{exact:true}).count(),1);
 await d.getByLabel('封面主标题',{exact:true}).fill('实战技巧');const sub=d.getByLabel('封面副标题',{exact:true});if(await sub.count())await sub.fill('进阶记录');
 await choose(p,'创作声明','内容无需标注');await d.getByRole('button',{name:'生成封面',exact:true}).click();await d.getByText('已生成，待确认',{exact:true}).waitFor();assert.match(await d.getByAltText('当前实际封面').getAttribute('src'),/^data:image\/png/);
 await shot(p,'05-template-preview');await d.getByLabel('封面主标题',{exact:true}).fill('更新封面标题');assert.match(await d.locator('.batch-content-cover-status').innerText(),/需生成封面/);
 await d.getByRole('button',{name:'保存',exact:true}).click();await d.getByRole('alert').filter({hasText:'封面'}).waitFor();assert.equal(await d.isVisible(),true);
 await d.getByRole('button',{name:'取消',exact:true}).click();await p.getByRole('button',{name:'放弃修改',exact:true}).click();await d.waitFor({state:'hidden'});
 await p.getByRole('button',{name:'预览检查',exact:true}).click();await p.getByRole('button',{name:'创建批次',exact:true}).waitFor();assert.equal(await p.getByRole('button',{name:'创建批次',exact:true}).isDisabled(),true);await shot(p,'06-precheck-blocked');
 await p.getByRole('button',{name:'返回内容配置',exact:true}).click();d=await editFirst(p);await d.getByText('使用模板',{exact:true}).click();await d.getByLabel('封面主标题',{exact:true}).fill('更新封面标题');const subtitle=d.getByLabel('封面副标题',{exact:true});if(await subtitle.count())await subtitle.fill('进阶记录');await choose(p,'创作声明','内容无需标注');await d.getByRole('button',{name:'生成封面',exact:true}).click();await d.getByText('已生成，待确认',{exact:true}).waitFor();await d.getByRole('button',{name:'保存',exact:true}).click();await d.waitFor({state:'hidden'});assert.match(await table(p).locator('tbody tr').first().innerText(),/已就绪/);
 await p.getByRole('button',{name:'预览检查',exact:true}).click();assert.equal(await p.getByRole('button',{name:'创建批次',exact:true}).isEnabled(),true);
 await p.getByRole('button',{name:'创建批次',exact:true}).click();await p.getByText('批次与任务已创建',{exact:true}).first().waitFor();assert.equal(writes.at(-1).tasks.length,1);assert.equal(writes.at(-1).tasks[0].status,'待执行');assert.equal(Object.keys(writes.at(-1).tasks[0].coverVariants).length,2);
});

test('pagination, cross-page selection and filters survive row editing; narrow modal keeps footer visible',async t=>{
 const {p}=await setup(t,{count:12,mutate:d=>d.publishDraft.rows.forEach(r=>r.declaration='内容无需标注')});
 assert.equal(await table(p).locator('tbody tr').count(),10);await table(p).locator('tbody tr').first().locator('.t-checkbox').click();
 await p.locator('.batch-content-table .t-pagination__number').filter({hasText:/^2$/}).click();assert.equal(await table(p).locator('tbody tr').count(),2);await table(p).locator('tbody tr').first().locator('.t-checkbox').click();assert.match(await p.locator('.batch-content-selection').innerText(),/2/);
 let d=await editFirst(p);await d.getByLabel('发布标题',{exact:true}).fill('第二页人工修改');await d.getByRole('button',{name:'保存',exact:true}).click();await d.waitFor({state:'hidden'});assert.equal(await table(p).locator('tbody tr').count(),2);assert.match(await table(p).locator('tbody tr').first().innerText(),/第二页人工修改/);assert.match(await p.locator('.batch-content-selection').innerText(),/2/);
 await p.getByLabel('搜索发布内容',{exact:true}).fill('火影');await p.getByRole('button',{name:'查询',exact:true}).click();d=await editFirst(p);await d.getByRole('button',{name:'保存',exact:true}).click();await d.waitFor({state:'hidden'});assert.equal(await p.getByLabel('搜索发布内容',{exact:true}).inputValue(),'火影');assert.equal(await table(p).locator('tbody tr').count(),6);
 await p.setViewportSize({width:390,height:844});assert.ok(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await shot(p,'07-content-list-mobile');d=await editFirst(p);const footer=d.locator('.t-dialog__footer');let box=await footer.boundingBox();assert.ok(box.y+box.height<=844);await d.locator('.t-dialog__body').evaluate(node=>node.scrollTop=node.scrollHeight);assert.ok(await d.getByRole('button',{name:'保存',exact:true}).isVisible());assert.ok(await p.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));await shot(p,'08-edit-dialog-mobile');
});


test('existing batch reuses the list and modal while final save preserves its locked resources',async t=>{
 const {p,writes}=await setup(t,{count:2,openDraft:false,mutate:data=>{
  const rows=data.publishDraft.rows;data.publishDraft={active:false};
  data.batches=[{id:990,name:'待执行内容走查',platform:'B站',phase:'内容预检通过',control:'待执行',version:1,targets:rows.map(r=>({category:r.category,count:1})),taskIds:[5001,5002],log:[]}];
  data.tasks=rows.map((row,i)=>({...row,id:5001+i,batchId:990,status:'待执行',attempts:0,declaration:'内容无需标注'}));
 }});
 await p.locator('.batch-v12-list tbody tr').getByRole('button',{name:'编辑',exact:true}).click();await table(p).waitFor();
 const d=await editFirst(p);assert.match(await d.locator('.batch-content-source').innerText(),/游戏魔王嘟嘟[\s\S]*V-128/);
 await d.getByLabel('发布标题',{exact:true}).fill('批次已有任务的修改');await d.getByRole('button',{name:'保存',exact:true}).click();await d.waitFor({state:'hidden'});
 // The second task also passes the existing final-save guard after explicit confirmation.
 await table(p).locator('tbody tr').nth(1).getByRole('button',{name:'编辑',exact:true}).click();await editor(p).getByRole('button',{name:'保存',exact:true}).click();await editor(p).waitFor({state:'hidden'});
 await p.locator('.resource-detail-page__footer').getByRole('button',{name:'保存',exact:true}).click();await table(p).waitFor({state:'hidden'});
 const task=writes.at(-1).tasks.find(r=>r.id===5001);assert.equal(task.title,'批次已有任务的修改');assert.equal(task.assetId,128);assert.equal(task.accountId,11);assert.equal(task.batchId,990);assert.equal(task.status,'待执行');assert.equal(writes.at(-1).publishDraft.active,false);
});


test('selected platform edits and content confirmation leave unchecked rows untouched',async t=>{
 const {p,writes}=await setup(t,{count:3});await table(p).locator('tbody tr').first().locator('.t-checkbox').click();
 await p.getByRole('button',{name:'批量配置',exact:true}).click();const d=p.getByRole('dialog',{name:'批量配置',exact:true});assert.match(await d.innerText(),/已选 1 条/);
 assert.equal(await d.getByRole('button',{name:'预览配置',exact:true}).isDisabled(),true);await d.getByText('公共设置',{exact:true}).click();await d.getByText('创作声明',{exact:true}).first().click();
 await d.getByLabel('批量创作声明',{exact:true}).click();await p.locator('.t-select-option').filter({hasText:'内容无需标注'}).last().click();await d.getByRole('button',{name:'预览配置',exact:true}).click();await d.locator('.bulk-config-result').waitFor();await d.getByRole('button',{name:'应用配置',exact:true}).click();await d.waitFor({state:'hidden'});
 await p.getByRole('button',{name:'确认内容',exact:true}).click();const guard=p.locator('.t-dialog').filter({hasText:'确认 1 条内容'});await guard.getByRole('button',{name:'确认内容',exact:true}).click();await guard.waitFor({state:'hidden'});
 assert.match(await table(p).locator('tbody tr').first().innerText(),/已就绪/);assert.match(await table(p).locator('tbody tr').nth(1).innerText(),/待完善/);
 await p.getByRole('button',{name:'保存草稿',exact:true}).click();const saved=writes.at(-1).publishDraft.rows;assert.equal(saved[0].contentApproved,true);assert.equal(saved[0].coverApproved,true);assert.equal(saved[1].contentApproved,false);assert.equal(saved[1].declaration,'');
 await p.getByRole('button',{name:'预览检查',exact:true}).click();assert.equal(await table(p).locator('tbody tr').count(),3);await p.getByRole('button',{name:'查看待处理',exact:true}).click();assert.equal(await table(p).locator('tbody tr').count(),2);await p.getByRole('button',{name:'查看全部',exact:true}).click();assert.equal(await table(p).locator('tbody tr').count(),3);await shot(p,'11-review-mixed');
});

test('creation check persists interval through draft recovery and creates pending tasks',async t=>{
 const {p,writes}=await setup(t,{count:2,mutate:d=>d.publishDraft.rows.forEach(r=>{r.declaration='内容无需标注';r.contentApproved=true;r.coverApproved=true;})});
 await p.getByRole('button',{name:'预览检查',exact:true}).click();const interval=p.locator('[aria-label="任务间隔"] input');await interval.fill('17');await interval.press('Tab');await p.getByRole('button',{name:'保存草稿',exact:true}).click();assert.equal(writes.at(-1).publishDraft.taskIntervalSeconds,17);
 await p.locator('.resource-detail-page__footer').getByRole('button',{name:'返回',exact:true}).click();await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();assert.equal(await interval.inputValue(),'17');assert.equal(await p.getByRole('button',{name:'创建批次',exact:true}).isEnabled(),true);await shot(p,'12-review-ready');
 await p.getByRole('button',{name:'创建批次',exact:true}).click();await p.getByText('批次与任务已创建',{exact:true}).waitFor();const saved=writes.at(-1);assert.equal(saved.tasks.length,2);assert.ok(saved.tasks.every(t=>t.status==='待执行'));assert.equal(saved.batches.at(-1).rules.taskIntervalSeconds,17);await shot(p,'13-created');
});

test('resource assets are viewable, categories reorder by drag and keyboard, and confirmation leads to content',async t=>{
 const {p,writes}=await setup(t,{openDraft:false,mutate:d=>{d.publishDraft={active:false};}});await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByPlaceholder('请输入批次名称',{exact:true}).fill('资源配置走查');
 await p.getByPlaceholder('选择一个平台',{exact:true}).click();await p.locator('.t-select-option').filter({hasText:'B站'}).last().click();for(const name of ['三角洲行动','火影忍者'])await p.locator('.batch-category-chip').filter({hasText:name}).click();
 const target=p.locator('.batch-target-table');await target.locator('tbody tr').nth(1).waitFor();await p.getByRole('button',{name:'查看三角洲行动可用成片',exact:true}).click();const d=p.getByRole('dialog',{name:'三角洲行动 · 可用成片',exact:true});await d.waitFor();assert.ok(await d.locator('tbody tr').count()>0);assert.doesNotMatch(await d.locator('tbody').innerText(),/火影忍者/);await shot(p,'14-assets-dialog');await d.locator('.t-dialog__close').click();
 const before=await target.locator('tbody tr').first().innerText();await p.getByRole('button',{name:'调整火影忍者优先级',exact:true}).dragTo(target.locator('tbody tr').first());assert.match(await target.locator('tbody tr').first().innerText(),/火影忍者/);await p.getByRole('button',{name:'调整火影忍者优先级',exact:true}).press('ArrowDown');assert.equal(await target.locator('tbody tr').first().innerText(),before);await shot(p,'15-resource-config');
 await p.getByRole('button',{name:'确认资源',exact:true}).click();const confirm=p.getByRole('dialog',{name:'确认资源',exact:true});await confirm.waitFor();assert.equal(await p.getByRole('button',{name:'进入内容配置',exact:true}).count(),1);await shot(p,'16-resource-confirm-dialog');await confirm.getByRole('button',{name:'继续调整',exact:true}).click();await confirm.waitFor({state:'hidden'});assert.ok(await target.isVisible());
 await p.getByRole('button',{name:'确认资源',exact:true}).click();await confirm.getByRole('button',{name:'进入内容配置',exact:true}).click();await p.locator('.batch-content-workspace').waitFor();await p.getByRole('button',{name:'保存草稿',exact:true}).click();assert.equal(writes.at(-1).tasks.length,0);assert.equal(writes.at(-1).publishDraft.resourcesLocked,true);assert.ok(writes.at(-1).publishDraft.rows.length>0);
});
