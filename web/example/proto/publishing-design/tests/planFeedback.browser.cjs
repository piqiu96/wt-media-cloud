const {requestBody}=require('./previewFixture.cjs');
const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {pathToFileURL}=require('node:url');
const path=require('node:path');
const {attachStore}=require('./previewFixture.cjs');
const source=path.resolve(__dirname,'..'),base=process.env.EXAMPLE_URL;
let browser,seed,defaults,confirmPlan;
before(async()=>{({seed,defaults}=await import(pathToFileURL(path.join(source,'model.js'))));({confirmPlan}=await import(pathToFileURL(path.join(source,'relationships.js'))));browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});});
after(()=>browser?.close());
async function pageFor(t){const context=await browser.newContext({viewport:{width:1440,height:1000}});const page=await context.newPage();page.setDefaultTimeout(7000);t.after(()=>context.close());return page;}
async function screenshot(p,name){if(process.env.SCREENSHOT_DIR)await p.screenshot({path:path.join(process.env.SCREENSHOT_DIR,name+'.png'),animations:'disabled'});}
test('plan scope selections preserve scroll, form nodes and the open multi-select across consecutive options',async t=>{
 const p=await pageFor(t),db=seed();db.plans=[];db.batches=[];db.tasks=[];await attachStore(p,db);
 await p.goto(base+'/example/operations/publishing/plans/new');await p.locator('#plan-goal-scope').waitFor();
 const select=p.locator('#plan-goal-scope label').filter({hasText:'发布平台'}).locator('.t-input__inner');
 await select.scrollIntoViewIfNeeded();await select.click();
 const original=await p.locator('#plan-goal-scope').elementHandle(),beforeY=await p.evaluate(()=>scrollY);
 await p.locator('.t-select-option:visible').filter({hasText:'B站'}).click();
 assert.equal(await original.evaluate(el=>el.isConnected),true,'select must not be unmounted on selection');
 assert.ok(Math.abs(await p.evaluate(()=>scrollY)-beforeY)<3,'selection must not move the document');
 await p.locator('.t-select-option:visible').filter({hasText:'小红书'}).click();
 assert.match(await p.locator('#plan-goal-scope').innerText(),/B站/);assert.match(await p.locator('#plan-goal-scope').innerText(),/小红书/);
 await p.keyboard.press('Escape');await screenshot(p,'plan-scope-stable');
});
async function batchEditor(t,{failSave=false}={}){
 const p=await pageFor(t),db=seed();db.plans=[];db.batches=[];db.tasks=[];
 const asset=db.assets.find(a=>a.id===139),category=asset.category;
 const plan=confirmPlan(db,{id:'PL-FEEDBACK',name:'反馈检查计划',confirmKey:'feedback',platformOrder:['B站'],targets:[{platform:'B站',category,count:1}],splits:[{name:'反馈批次',platform:'B站',targets:[{category,count:1}]}]});
 const batch=db.batches[0];batch.candidates=[{...defaults('B站',asset),platform:'B站',category,assetId:asset.id,accountId:db.accounts.find(a=>a.enabled&&a.platform==='B站'&&a.categories.includes(category)).id,declaration:'内容无需标注',contentApproved:true,coverApproved:true}];batch.draft={step:3,resourcesLocked:true};
 const store=await attachStore(p,db);let failed=false;
 if(failSave)await p.route('**/__example/publishing/snapshot',async r=>{if(r.request().method()==='PUT'&&!failed&&requestBody(r.request())?.data?.tasks.some(t=>t.batchId===batch.id)){failed=true;await r.fulfill({status:422,json:{error:'保存失败测试'}});}else await r.fallback();});
 await p.goto(base+`/example/operations/publishing/plans/${plan.id}`);await p.getByRole('button',{name:'去配置',exact:true}).click();await p.getByRole('heading',{name:'编辑发布批次',exact:true}).waitFor();if(await p.getByRole('button',{name:'预览检查',exact:true}).isVisible())await p.getByRole('button',{name:'预览检查',exact:true}).click();await p.getByRole('button',{name:'创建批次',exact:true}).waitFor();return {p,store,batch};
}
test('plan batch creation shows a success dialog only after saving and returns to its batch',async t=>{
 const {p,store,batch}=await batchEditor(t);await p.getByRole('button',{name:'创建批次',exact:true}).click();
 const dialog=p.getByRole('dialog').filter({hasText:'批次与任务已创建'});await dialog.waitFor();
 assert.equal(await p.locator('.batch-created-result').count(),0,'no success page');assert.equal(await p.getByText('成片已被同平台任务占用',{exact:true}).count(),0,'created tasks do not conflict with themselves');assert.equal((await store.get()).data.tasks.filter(t=>t.batchId===batch.id).length,1);
 await screenshot(p,'plan-batch-success-modal');await dialog.getByRole('button',{name:'查看批次',exact:true}).click();await p.getByRole('heading',{name:'发布批次详情',exact:true}).waitFor();assert.match(p.url(),/fromPlan=PL-FEEDBACK/);
});
test('failed creation save stays on the review page, retries once and never duplicates tasks',async t=>{
 const {p,store,batch}=await batchEditor(t,{failSave:true});await p.getByRole('button',{name:'创建批次',exact:true}).click();
 await p.getByRole('button',{name:'重试保存',exact:true}).waitFor();assert.equal(await p.getByRole('dialog').filter({hasText:'批次与任务已创建'}).count(),0);assert.equal((await store.get()).data.tasks.length,0);
 await p.getByRole('button',{name:'重试保存',exact:true}).click();const dialog=p.getByRole('dialog').filter({hasText:'批次与任务已创建'});await dialog.waitFor();assert.equal((await store.get()).data.tasks.filter(t=>t.batchId===batch.id).length,1);
 await screenshot(p,'plan-batch-retry-success');await dialog.getByRole('button',{name:'返回',exact:true}).click();await p.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();
});

test('original material pictures and browser-generated covers persist together through the real HTTP service',async t=>{
 const http=require('node:http'),os=require('node:os'),fs=require('node:fs/promises'),{createHash}=require('node:crypto');
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'wt-real-cover-'));
 const {createPublishingMiddleware}=await import(pathToFileURL(path.join(source,'../../publishing-preview.mjs')));
 const middleware=createPublishingMiddleware({webRoot:root}),server=http.createServer((req,res)=>middleware(req,res,()=>res.end()));
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));const apiBase=`http://127.0.0.1:${server.address().port}`;const initial=seed();initial.tasks=[];initial.batches=[];initial.plans=[];await fetch(apiBase+'/__example/publishing/bootstrap',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({data:initial})});
 t.after(async()=>{await new Promise(resolve=>server.close(resolve));const hash=createHash('sha256').update(root+new URL(apiBase).host).digest('hex').slice(0,16);await fs.rm(path.join(os.tmpdir(),'wt-media-example',hash+'.json'),{force:true});await fs.rm(root,{recursive:true,force:true});});
 const p=await pageFor(t);
 await p.route('**/__example/publishing/**',async route=>{
  const request=route.request(),headers={'Content-Type':'application/json'};if(request.headers()['content-encoding'])headers['Content-Encoding']=request.headers()['content-encoding'];
  const response=await fetch(apiBase+new URL(request.url()).pathname,{method:request.method(),headers,...(request.method()!=='GET'?{body:request.postDataBuffer()}: {})});
  await route.fulfill({status:response.status,contentType:'application/json',body:Buffer.from(await response.arrayBuffer())});
 });
 await p.goto(base+'/example/operations/publishing/batches');await p.getByRole('heading',{name:'发布批次',exact:true}).waitFor();
 const result=await p.evaluate(async()=>{
  const root='/example/proto/publishing-design/';
  const {seed,defaults}=await import(root+'model.js'),{ensureCoverResults,defaultDocument,coverSpecs}=await import(root+'covers.js'),{createPublication}=await import(root+'relationships.js'),api=await import(root+'batchApi.js');
  const db=await api.loadBatchSnapshot();
  // A material photo. Noise makes this exercise actual PNG encoding,
  // rather than trivially compressible placeholder data or mocked cover output.
  const canvas=document.createElement('canvas');canvas.width=160;canvas.height=120;const ctx=canvas.getContext('2d'),pixels=ctx.createImageData(160,120);let n=123456789;
  for(let i=0;i<pixels.data.length;i+=4){n^=n<<13;n^=n>>>17;n^=n<<5;pixels.data[i]=n&255;pixels.data[i+1]=(n>>>8)&255;pixels.data[i+2]=(n>>>16)&255;pixels.data[i+3]=255;}
  ctx.putImageData(pixels,0,0);const original=canvas.toDataURL('image/png');
  const rows=[];for(const [i,asset]of db.assets.filter(a=>a.ready!==false&&a.fileAvailable!==false).slice(0,6).entries()){
   const row={...defaults('B站',asset),assetId:asset.id,platform:'B站',category:asset.category,accountId:db.accounts.find(a=>a.enabled&&a.platform==='B站'&&a.categories.includes(asset.category)).id,cover:original,coverSource:original,declaration:'内容无需标注',contentApproved:true,coverApproved:true};
   if(i%2){row.coverMode='模板合成';row.shortTitle='真实封面'+i;row.templateDocument=defaultDocument(1200,900,original);row.templateDocuments=Object.fromEntries(coverSpecs('B站').map(spec=>[spec.key,defaultDocument(spec.width,spec.height,original)]));}
   await ensureCoverResults(row);rows.push(row);
  }
  db.images=Array.from({length:2},(_,i)=>({id:'photo-'+i,url:original,enabled:true,uses:['现成封面','底图']}));
  const publication=createPublication(db,{platform:'B站',rows,name:'原图与生成封面保存回归'});
  const bytes=new Blob([JSON.stringify(db)]).size;await api.saveBatchSnapshot(db);const loaded=await api.loadBatchSnapshot();
  return {bytes,tasks:loaded.tasks.length,unchanged:JSON.stringify(loaded.tasks.map(t=>({cover:t.cover,source:t.coverSource,variants:t.coverVariants,snapshot:t.snapshot})))===JSON.stringify(publication.tasks.map(t=>({cover:t.cover,source:t.coverSource,variants:t.coverVariants,snapshot:t.snapshot}))),generated:loaded.tasks.filter(t=>t.coverMode==='模板合成').every(t=>t.coverVariants['4:3']&&t.coverVariants['16:9'])};
 });
 assert.ok(result.bytes>1024*1024);assert.equal(result.tasks,6);assert.equal(result.unchanged,true);assert.equal(result.generated,true);
});
