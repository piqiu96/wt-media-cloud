const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
const {pathToFileURL}=require('node:url');
const path=require('node:path');
const source=pathToFileURL(path.resolve(__dirname,'..')+'/').href;
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
let browser,model,relations,createBatchStore;
before(async()=>{
 model=await import(source+'model.js');relations=await import(source+'relationships.js');
 ({createBatchStore}=await import(source+'../../publishing-preview.mjs'));
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});
});
after(async()=>browser?.close());
async function scenario(t,mode){
 const {seed,defaults,declarationOptions}=model;
 const {migrateRelationships,confirmPlan,createPublication,dispatchPlan,setResult,claimTask,cancelBatchSafely}=relations;
 const db=migrateRelationships(seed());db.plans=[];db.batches=[];db.tasks=[];
 const plan=confirmPlan(db,{name:'收尾走查 '+mode,confirmKey:mode,platformOrder:['百家号'],targets:[{platform:'百家号',category:'三角洲行动',count:2}],splits:[0,1].map(i=>({name:'收尾批次'+i,platform:'百家号',targets:[{category:'三角洲行动',count:1}]}))});
 db.batches.forEach((batch,i)=>{
  const asset=db.assets.find(a=>a.id===128+i*2),account=db.accounts.find(a=>a.enabled&&a.platform===batch.platform&&a.categories.includes(asset.category));
  createPublication(db,{name:batch.name,platform:batch.platform,planId:plan.id,batchId:batch.id,rows:[{assetId:asset.id,platform:batch.platform,category:asset.category,accountId:account.id,...defaults(batch.platform,asset),declaration:declarationOptions(batch.platform)[0].value}]});
 });
 if(mode==='before-start-cancel')cancelBatchSafely(db,db.batches[0]);
 else dispatchPlan(db,plan);
 if(mode==='running'){claimTask(db,db.tasks[0]);setResult(db.tasks[0],'结果待确认');}
 else if(mode!=='before-start-cancel'){db.tasks.forEach(task=>setResult(task,'已放弃'));db.batches.forEach(batch=>batch.control='执行已结束');plan.position=plan.items.length;plan.control='执行已结束';}
 const store=createBatchStore();await store.bootstrap(db);
 const context=await browser.newContext({viewport:{width:1440,height:1000}});t.after(()=>context.close());
 const page=await context.newPage(),errors=[];page.setDefaultTimeout(7000);page.on('pageerror',e=>errors.push(e.message));
 t.after(()=>assert.deepEqual(errors,[]));
 await page.route('**/__example/publishing/**',async route=>{
  try{const request=route.request(),body=request.method()==='GET'?null:request.postDataJSON();
   const result=request.method()==='GET'?await store.get():request.url().endsWith('/command')?await store.command(body):await store.save(body.data,body.revision);
   await route.fulfill({json:result});
  }catch(e){await route.fulfill({status:409,json:{error:e.message}});}
 });
 return {page,store,id:plan.id};
}
const snapshot=async store=>(await store.get()).data;
const detail=page=>page.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();
for(const entry of ['list','detail'])test(`启动前终止首批后，从${entry==='list'?'列表':'详情'}确认启动后续批次并保留终止记录`,async t=>{
 const {page,store,id}=await scenario(t,'before-start-cancel');
 await page.goto(base+'/example/operations/publishing/plans'+(entry==='detail'?'/'+id:''));
 const execute=entry==='detail'?page.locator('.resource-detail-page__footer').getByRole('button',{name:'启动计划',exact:true}):page.locator('[data-act=execute]');
 await execute.click();const dialog=page.locator('dialog.plan-modal[open]');await dialog.waitFor();
 let saved=await snapshot(store);assert.equal(saved.plans[0].lockedAt,undefined);assert.equal(saved.tasks[1].status,'待执行');
 await dialog.getByRole('button',{name:'确认启动',exact:true}).click();await page.getByText('计划已更新',{exact:true}).waitFor();
 saved=await snapshot(store);assert.equal(saved.plans[0].position,1);assert.ok(saved.plans[0].lockedAt);assert.equal(saved.batches[0].control,'已取消');assert.deepEqual(saved.tasks.map(task=>task.status),['已取消','排队中']);
 await page.reload();await page.goto(base+'/example/operations/publishing/plans/'+id);await detail(page);
 assert.match(await page.locator('.resource-detail-page').innerText(),/已取消/);assert.equal((await snapshot(store)).plans[0].position,1);
});
test('运行中无编辑与补量，终止计划覆盖所有批次并保留核实入口',async t=>{
 const {page,store,id}=await scenario(t,'running');
 await page.goto(base+'/example/operations/publishing/plans');
 await page.locator('[data-act=detail]').first().waitFor();
 assert.equal(await page.locator('[data-act=edit]').count(),0);
 await page.locator('[data-act=detail]').first().click();await detail(page);
 assert.equal(await page.getByRole('button',{name:'创建补量批次',exact:true}).count(),0);
 await page.goto(base+'/example/operations/publishing/plans/'+id+'/edit');await detail(page);
 assert.equal(await page.locator('[data-field=name]').count(),0,'直接访问编辑地址仍进入只读详情');
 await page.goto(base+'/example/operations/publishing/plans');await page.locator('[data-act=cancel]').click();
 await page.locator('[data-modal-field=reason]').fill('整计划终止');
 await page.getByRole('button',{name:'确认终止计划',exact:true}).click();
 await page.waitForFunction(()=>document.body.innerText.includes('计划已更新'));
 const saved=await snapshot(store);
 assert.equal(saved.plans[0].control,'已取消');
 assert.ok(saved.batches.every(b=>b.stoppedAt));
 assert.equal(saved.tasks[0].status,'结果待确认');assert.equal(saved.tasks[1].status,'已取消');
 await page.goto(base+'/example/operations/publishing/plans/'+id);await detail(page);
 assert.match(await page.locator('.resource-detail-page').innerText(),/等待在途任务核实/);
 assert.ok(await page.getByRole('button',{name:'去处理',exact:true}).count());
 await page.reload();await detail(page);assert.equal((await snapshot(store)).tasks[0].status,'结果待确认');
});
test('全部执行及结果处置完成后，补量持久保存且原目标不变',async t=>{
 const {page,store,id}=await scenario(t,'makeup');
 await page.goto(base+'/example/operations/publishing/plans/'+id);await detail(page);
 await page.getByRole('button',{name:'创建补量批次',exact:true}).click();
 await page.locator('[data-makeup=name]').fill('目标补量');
 await page.getByRole('button',{name:'确认新增补量批次',exact:true}).click();
 await page.waitForFunction(()=>document.body.innerText.includes('补量批次待执行'));
 let saved=await snapshot(store);assert.equal(saved.plans[0].targets[0].count,2);assert.equal(saved.plans[0].items.length,3);
 assert.equal(saved.batches.at(-1).type,'makeup');assert.equal(saved.batches.at(-1).name,'目标补量');
 await page.reload();await detail(page);saved=await snapshot(store);
 assert.equal(saved.plans[0].items.length,3);assert.equal(saved.plans[0].targets[0].count,2);
 assert.equal(await page.getByRole('button',{name:'创建补量批次',exact:true}).count(),0);
});
test('全部执行结束后可说明缺口并确认结束，刷新仍保持只读',async t=>{
 const {page,store,id}=await scenario(t,'finish');
 await page.goto(base+'/example/operations/publishing/plans/'+id);await detail(page);
 await page.getByRole('button',{name:'确认结束',exact:true}).click();
 await page.locator('[data-modal-field=reason]').fill('本轮不再补量');
 await page.locator('[data-act=do-end-plan]').click();
 await page.waitForFunction(()=>document.body.innerText.includes('计划已更新'));
 const saved=await snapshot(store);assert.equal(saved.plans[0].endNote,'本轮不再补量');assert.ok(saved.plans[0].endedAt);
 await page.reload();await detail(page);
 assert.equal(await page.getByRole('button',{name:'创建补量批次',exact:true}).count(),0);
 assert.equal(await page.locator('footer [data-act=copy]').count(),0);
});
