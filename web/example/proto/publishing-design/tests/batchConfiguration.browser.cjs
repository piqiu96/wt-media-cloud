const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {pathToFileURL}=require('node:url'),path=require('node:path'),fs=require('node:fs');
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
const shots=process.env.SCREENSHOT_DIR||'/private/tmp/wt-batch-configuration-review';
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
 const reviewDocument=(width,height)=>{const doc=defaultDocument(width,height);doc.layers[0].y=height*0.12;return doc;};
 data.templates=[{id:9001,name:'走查横版模板',enabled:true,version:1,document:reviewDocument(1200,900)},{id:9002,name:'走查宽版模板',enabled:true,version:1,document:reviewDocument(1600,900)}];
 data.publishDraft={active:true,mode:'批量发布',step:2,name:'内容配置走查',platforms:[platform],categories:[...new Set(assets.map(v=>v.category))],rows,assets:assets.map(v=>v.id),targets:[...new Set(assets.map(v=>v.category))].map(category=>({category,count:assets.filter(v=>v.category===category).length})),resourcesLocked:true,creationKey:'batch-content-review'};
 rows.forEach(row=>{row.coverSignature=coverSignature(row);row.coverVariants={source:row.cover};});
 mutate(data);const writes=[];
 await p.route('**/__example/publishing/**',r=>{if(r.request().method()!=='GET'){const body=r.request().postDataJSON();if(r.request().url().endsWith('/command'))runBatchCommand(data,data.batches.find(b=>b.id===body.batchId),body.action,body.input);else data=body.data;revision++;writes.push(structuredClone(data));}return r.fulfill({json:{revision,data}});});
 await p.goto(base+'/example/?view=publishing#batches');if(openDraft){await p.getByRole('button',{name:'批量发布',exact:true}).click();await p.getByRole('button',{name:'继续编辑',exact:true}).click();await p.locator('.batch-content-workspace').waitFor();}return {p,writes};
}
const table=p=>p.locator('.batch-content-table');
const editor=p=>p.getByRole('dialog',{name:'编辑发布内容',exact:true});
async function editFirst(p){await table(p).getByRole('button',{name:'编辑',exact:true}).first().click();await editor(p).waitFor();await p.waitForTimeout(350);return editor(p);}
async function choose(p,label,text){await editor(p).getByLabel(label,{exact:true}).click();await p.locator('.t-select-option').filter({hasText:new RegExp('^'+text.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')+'$')}).last().click();}
async function shot(p,name){for(const close of await p.locator('.t-message__close').all())if(await close.isVisible())await close.click();await p.mouse.move(5,5);if(await p.locator('.t-message:visible').count())await p.locator('.t-message:visible').first().waitFor({state:'hidden',timeout:6000});if(await p.locator('.plan-scope .toast:visible').count())await p.locator('.plan-scope .toast:visible').waitFor({state:'hidden',timeout:6000});await p.waitForTimeout(450);await p.screenshot({path:path.join(shots,name+'.png'),fullPage:false});}

const config=p=>p.getByRole('dialog',{name:'批量配置',exact:true});
async function pick(p,d,label,text){await d.getByLabel(label,{exact:true}).click();await p.locator('.t-select-option:visible').filter({hasText:new RegExp('^'+text.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')+'$')}).last().click();}
async function openConfig(p){await p.getByRole('button',{name:'批量配置',exact:true}).click();await config(p).waitFor();return config(p);}
async function confirm(p){await p.getByRole('button',{name:'确认内容',exact:true}).click();const guard=p.locator('.t-dialog').filter({hasText:'确认 2 条内容'});await guard.getByRole('button',{name:'确认内容',exact:true}).click();await guard.waitFor({state:'hidden'});}

test('one batch configuration combines common settings and AI, preserves titles and marks confirmation explicitly',async t=>{
 const {p,writes}=await setup(t,{count:3,mutate:d=>d.publishDraft.rows.forEach(r=>{delete r.coverSignature;delete r.coverVariants;})});
 await table(p).locator('tbody tr').nth(0).locator('.t-checkbox').click();await table(p).locator('tbody tr').nth(1).locator('.t-checkbox').click();
 assert.equal(await p.getByRole('button',{name:'AI 生成',exact:true}).count(),0);assert.equal(await p.getByRole('button',{name:'批量编辑',exact:true}).count(),0);
 let d=await openConfig(p);assert.equal(await d.getByRole('checkbox',{checked:true}).count(),0);assert.equal(await d.getByRole('button',{name:'预览配置',exact:true}).isDisabled(),true);await shot(p,'01-batch-config-default');
 await d.getByText('公共设置',{exact:true}).click();await d.getByText('创作声明',{exact:true}).first().click();await pick(p,d,'批量创作声明','内容无需标注');
 await d.getByText('AI 文案',{exact:true}).click();await d.getByText('简介 / 正文',{exact:true}).click();await shot(p,'02-common-and-copy');
 await d.getByRole('button',{name:'生成并审阅',exact:true}).click();await d.locator('.bulk-config-result').first().waitFor({timeout:15000});assert.equal(await d.locator('.bulk-config-result').count(),2);
 assert.match(await table(p).locator('tbody tr').first().innerText(),/待完善/);await shot(p,'03-combined-preview');
 await d.getByRole('button',{name:'应用配置',exact:true}).click();await d.waitFor({state:'hidden'});
 assert.match(await table(p).locator('tbody tr').first().innerText(),/待确认/);assert.match(await table(p).locator('tbody tr').nth(2).innerText(),/待完善/);
 await shot(p,'17-list-pending-confirmation');await confirm(p);assert.match(await table(p).locator('tbody tr').first().innerText(),/已就绪/);assert.match(await table(p).locator('tbody tr').nth(1).innerText(),/已就绪/);
 await p.getByRole('button',{name:'保存草稿',exact:true}).click();const first=structuredClone(writes.at(-1).publishDraft.rows);
 assert.equal(first[0].title,'三角洲实战精彩片段');assert.ok(first[0].description);assert.equal(first[2].description,'');assert.equal(first[0].coverVariants,undefined);
 await shot(p,'04-list-three-states');
 d=await openConfig(p);assert.equal(await d.getByRole('checkbox',{checked:true}).count(),0);await d.getByText('封面设置',{exact:true}).click();
 await pick(p,d,'批量封面方式','使用已有图片');const image=(await import(pathToFileURL(path.join(source,'model.js')))).seed().images.find(i=>i.enabled&&i.uses.includes('现成封面'));await pick(p,d,'批量封面图片',image.name);
 assert.equal(await d.getByRole('button',{name:'生成封面',exact:true}).count(),0);await shot(p,'05-cover-only');
 await d.getByRole('button',{name:'预览配置',exact:true}).click();await d.locator('.bulk-config-result').first().waitFor({timeout:15000});assert.match(await table(p).locator('tbody tr').first().innerText(),/已就绪/);await shot(p,'06-native-cover-preview');
 await d.getByRole('button',{name:'取消',exact:true}).click();await d.waitFor({state:'hidden'});assert.match(await table(p).locator('tbody tr').first().innerText(),/已就绪/);
 d=await openConfig(p);await d.getByText('封面设置',{exact:true}).click();await pick(p,d,'批量封面方式','使用已有图片');await pick(p,d,'批量封面图片',image.name);
 await d.getByRole('button',{name:'预览配置',exact:true}).click();await d.locator('.bulk-config-result').first().waitFor({timeout:15000});await d.getByRole('button',{name:'应用配置',exact:true}).click();await d.waitFor({state:'hidden'});
 assert.match(await table(p).locator('tbody tr').first().innerText(),/待确认/);await p.getByRole('button',{name:'保存草稿',exact:true}).click();const second=writes.at(-1).publishDraft.rows;
 for(let i=0;i<2;i++){assert.equal(second[i].title,first[i].title);assert.equal(second[i].description,first[i].description);assert.equal(second[i].tags,first[i].tags);assert.equal(second[i].cover,image.src);assert.equal(second[i].contentApproved,false);}assert.deepEqual(second[2],first[2]);
 await confirm(p);await shot(p,'07-cover-updated-ready');
});

test('template previews show actual outputs without changing copy, including narrow layout',async t=>{
 const {p,writes}=await setup(t,{count:2,mutate:d=>d.publishDraft.rows.forEach(r=>r.declaration='内容无需标注')});
 await table(p).locator('thead .t-checkbox').click();const d=await openConfig(p);await d.getByText('封面设置',{exact:true}).click();await pick(p,d,'批量封面方式','使用模板合成');await pick(p,d,'批量封面模板 4:3','走查横版模板');await pick(p,d,'批量封面模板 16:9','走查宽版模板');assert.equal(await d.getByText('AI 补充封面文字',{exact:true}).count(),0);assert.ok(await d.getByRole('checkbox',{name:'AI 文案',exact:true}).isChecked());await shot(p,'08-template-settings');
 await d.getByRole('button',{name:'生成并审阅',exact:true}).click();await d.locator('.bulk-config-result').first().waitFor({timeout:15000});assert.equal(await d.locator('.bulk-config-error').count(),0);
 for(const item of await d.locator('.bulk-config-result').all()){const heading=item.locator('.bulk-config-result-heading');if(await heading.getAttribute('aria-expanded')!=='true')await heading.click();assert.equal(await item.locator('.bulk-config-covers img').count(),2);for(const image of await item.locator('.bulk-config-covers img').all()){await image.evaluate(image=>image.decode());assert.ok(await image.evaluate(image=>image.naturalWidth>0));}}
 await d.locator('.bulk-config-covers img').first().evaluate(image=>image.decode());assert.ok(await d.locator('.bulk-config-covers img').first().evaluate(image=>image.naturalWidth>0));await shot(p,'09-template-outputs');
 await p.setViewportSize({width:390,height:844});assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);const footer=await d.locator('.t-dialog__footer').boundingBox();assert.ok(footer.y+footer.height<=844);await shot(p,'10-config-mobile');
 await d.getByRole('button',{name:'应用配置',exact:true}).click();await d.waitFor({state:'hidden'});await p.setViewportSize({width:1440,height:1000});await p.getByRole('button',{name:'保存草稿',exact:true}).click();const rows=writes.at(-1).publishDraft.rows;
 assert.equal(rows[0].title,'三角洲实战精彩片段');assert.equal(rows[0].description,'');assert.ok(rows[0].shortTitle);assert.equal(rows[0].contentApproved,false);assert.ok(rows[0].coverVariants['16:9']);
});

test('partial failures can apply successful items and retry only the failed item',async t=>{
 const {p,writes}=await setup(t,{count:2,mutate:d=>{const rows=d.publishDraft.rows;d.assets.find(asset=>asset.id===rows[0].assetId).cover='https://example.test/flaky-cover.svg';rows[1].cover=d.assets[2].cover;rows[1].coverSource=rows[1].cover;}});
 let requests=0;await p.route('https://example.test/flaky-cover.svg',route=>{requests++;return requests===1?route.fulfill({status:503,headers:{'cache-control':'no-store'},body:'temporary image failure'}):route.fulfill({headers:{'cache-control':'no-store'},contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="900"><rect width="1200" height="900" fill="#284565"/></svg>'});});
 await table(p).locator('thead .t-checkbox').click();const d=await openConfig(p);await d.getByText('封面设置',{exact:true}).click();await shot(p,'19-source-cover');await d.getByRole('button',{name:'预览配置',exact:true}).click();await d.locator('.bulk-config-result').first().waitFor({timeout:15000});
 assert.equal(await d.locator('.bulk-config-error').count(),1);await shot(p,'11-partial-failure');await d.getByRole('button',{name:'应用配置',exact:true}).click();await p.getByRole('button',{name:'应用可用配置',exact:true}).click();await d.getByText('已应用',{exact:true}).waitFor();assert.equal(await d.isVisible(),true);assert.equal(await d.getByText('已应用',{exact:true}).count(),1);
 await d.getByRole('button',{name:'重试未成功项',exact:true}).click();await d.getByText('待应用',{exact:true}).waitFor();assert.equal(await d.locator('.bulk-config-error').count(),0);await shot(p,'12-retry-only-failed');await d.getByRole('button',{name:'应用配置',exact:true}).click();await d.waitFor({state:'hidden'});
 await p.getByRole('button',{name:'保存草稿',exact:true}).click();assert.equal(writes.at(-1).publishDraft.rows[0].cover,'https://example.test/flaky-cover.svg');assert.notEqual(writes.at(-1).publishDraft.rows[1].cover,'https://example.test/flaky-cover.svg');
});

test('plan selectors share one row, AI recommendation has one entry and owner stays hidden',async t=>{
 const {p}=await setup(t,{openDraft:false});await p.goto(base+'/example/operations/publishing/plans');await p.getByRole('button',{name:'新建发布计划',exact:true}).click();
 if(await p.getByRole('button',{name:'重新开始',exact:true}).isVisible().catch(()=>false))await p.getByRole('button',{name:'重新开始',exact:true}).click();
 await p.locator('#plan-goal-scope').waitFor();await p.getByPlaceholder('例如：新赛季内容矩阵 · 周度发布').fill('游戏内容发布计划');assert.equal(await p.getByText('计划负责人',{exact:true}).count(),0);
 const labels=p.locator('#plan-goal-scope label');const first=await labels.nth(0).boundingBox(),second=await labels.nth(1).boundingBox();assert.equal(first.y,second.y);assert.ok(second.x>first.x);
 async function scope(label,value){await labels.filter({hasText:label}).locator('.t-input').click();await p.locator('.t-select-option:visible').filter({hasText:new RegExp('^'+value+'$')}).click();if(!(await p.locator('.t-dialog').filter({hasText:'调整发布范围？'}).isVisible().catch(()=>false)))await p.keyboard.press('Escape');}
 await scope('发布平台','B站');await scope('发布平台','小红书');await scope('发布类目','三角洲行动');await scope('发布类目','火影忍者');
 await p.locator('[data-editor="target"]').fill('12');await p.locator('[data-editor="target"]').press('Tab');await p.locator('[data-act="generate-matrix"]').click();
 assert.equal(await p.getByRole('button',{name:'规则推荐',exact:true}).count(),0);assert.equal(await p.getByRole('button',{name:'重新推荐',exact:true}).count(),0);assert.equal(await p.getByRole('button',{name:'AI 推荐',exact:true}).count(),1);
 await p.getByRole('button',{name:'AI 推荐',exact:true}).click();if(await p.locator('[data-act="do-recommend"]').isVisible().catch(()=>false))await p.locator('[data-act="do-recommend"]').click();await p.locator('.batches-table tbody tr').first().waitFor();await p.locator('#plan-goal-scope').scrollIntoViewIfNeeded();await shot(p,'13-plan-scope-and-recommendation');await p.locator('.batches-table').scrollIntoViewIfNeeded();await shot(p,'16-plan-ai-recommendation');
 const stored=JSON.parse(await p.evaluate(()=>localStorage.getItem('wt-publishing-plan-v1')));assert.equal(stored.ui.editor.owner,'陈小雨');
 await scope('发布平台','小红书');const guard=p.locator('.t-dialog').filter({hasText:'调整发布范围？'});await guard.getByRole('button',{name:'确认调整',exact:true}).click();await p.locator('#plan-goal-scope').waitFor();assert.doesNotMatch(await p.locator('[data-matrix]').first().locator('xpath=ancestor::table').innerText(),/小红书/);await shot(p,'14-plan-range-change');
 await p.setViewportSize({width:390,height:844});assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);await shot(p,'15-plan-mobile');
});

test('confirmation blocks competing edits and rejects a version changed during image inspection',async t=>{
 const {p,writes}=await setup(t,{count:1,mutate:d=>d.publishDraft.rows.forEach(r=>r.declaration='内容无需标注')});
 await table(p).locator('thead .t-checkbox').click();
 await p.evaluate(()=>{window.__pendingImages=[];window.Image=class {naturalWidth=1200;naturalHeight=900;set src(value){window.__pendingImages.push(()=>this.onload());}};});
 await p.getByRole('button',{name:'确认内容',exact:true}).click();const guard=p.locator('.t-dialog').filter({hasText:'确认 1 条内容'});await guard.getByRole('button',{name:'确认内容',exact:true}).click();await guard.waitFor({state:'hidden'});await p.waitForFunction(()=>window.__pendingImages.length===1);
 assert.equal(await p.getByRole('button',{name:'批量配置',exact:true}).isDisabled(),true);
 await table(p).getByRole('button',{name:'编辑',exact:true}).click();assert.equal(await editor(p).count(),0);
 // Simulate an incoming data update during the pending asynchronous image read.
 await p.evaluate(()=>{const root=document.querySelector('.batch-content-workspace');root.__vueParentComponent.props.rows[0].visibility='仅自己可见';window.__pendingImages[0]();});
 await p.locator('.batch-content-workspace').getByText(/内容已发生变化，请检查后重新确认/).waitFor();assert.match(await table(p).locator('tbody tr').first().innerText(),/待确认/);
 await p.getByRole('button',{name:'保存草稿',exact:true}).click();assert.equal(writes.at(-1).publishDraft.rows[0].contentApproved,false);await shot(p,'18-confirm-version-protection');
});
