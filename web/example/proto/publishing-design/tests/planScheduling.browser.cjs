const {chromium}=require('playwright');const assert=require('assert/strict'),fs=require('fs');
const path=require('node:path');
const {pathToFileURL}=require('node:url');
const {test}=require('node:test');
const source=pathToFileURL(path.resolve(__dirname,'..')+'/').href;
const base=process.env.EXAMPLE_URL||'http://127.0.0.1:8768';
const output=process.env.SCREENSHOT_DIR;
if(output)fs.mkdirSync(output,{recursive:true});
const artifact=(name,value)=>{if(output)fs.writeFileSync(path.join(output,name),value);};
const shot=(page,name)=>output?page.screenshot({path:path.join(output,name)}):Promise.resolve();
test('automatic plan progression respects threshold, preparation and pause', {timeout:90000}, async()=>{
 const {seed,defaults,declarationOptions}=await import(source+'model.js');const {migrateRelationships,confirmPlan,createPublication,dispatchPlan,setResult}=await import(source+'relationships.js');const {createBatchStore}=await import(source+'../../publishing-preview.mjs');
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE});
 try{for(const mode of ['auto','threshold','paused','cancelled']){
  const db=migrateRelationships(seed());db.plans=[];db.batches=[];db.tasks=[];
  const p=confirmPlan(db,{name:'衔接验收 '+mode,confirmKey:mode,platformOrder:['百家号'],targets:[{platform:'百家号',category:'三角洲行动',count:2}],splits:[0,1].map(i=>({name:'衔接批次'+i,platform:'百家号',targets:[{category:'三角洲行动',count:1}]}))});
  db.batches.forEach((b,i)=>{const a=db.assets.find(a=>a.id===128+i*2);createPublication(db,{name:b.name,platform:b.platform,planId:p.id,batchId:b.id,rows:[{assetId:a.id,platform:b.platform,category:a.category,accountId:db.accounts.find(a=>a.enabled&&a.platform===b.platform&&a.categories.includes('三角洲行动')).id,...defaults(b.platform,a),declaration:declarationOptions(b.platform)[0].value}]});});
  dispatchPlan(db,p);setResult(db.tasks[0],mode==='threshold'?'执行失败':'平台审核中');if(mode==='paused')p.control='已暂停';if(mode==='cancelled')setResult(db.tasks[1],'已取消');
  const store=createBatchStore();await store.bootstrap(db);const ctx=await browser.newContext({viewport:{width:1440,height:1000}});const page=await ctx.newPage();page.setDefaultTimeout(7000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.route('**/__example/publishing/**',async r=>{try{const q=r.request(),v=q.method()==='GET'?await store.get():q.url().endsWith('/command')?await store.command(q.postDataJSON()):await store.save(q.postDataJSON().data,q.postDataJSON().revision);await r.fulfill({json:v});}catch(e){await r.fulfill({status:409,json:{error:e.message}});}});
  await page.goto(base+'/example/operations/publishing/plans/'+p.id);await page.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();await page.waitForTimeout(450);
  let saved=(await store.get()).data;
  if(mode==='auto'){assert.equal(saved.plans[0].position,1);assert.equal(saved.tasks[1].attempts,1);await page.reload();await page.getByRole('heading',{name:'发布计划详情',exact:true}).waitFor();assert.equal((await store.get()).data.tasks[1].attempts,1);}
  if(mode==='threshold'){assert.equal(saved.plans[0].control,'等待继续确认');await page.getByRole('button',{name:'确认进入下一批',exact:true}).click();await page.getByRole('button',{name:'确认继续',exact:true}).click();}
  if(mode==='paused'){assert.equal(saved.plans[0].position,0);assert.equal(saved.plans[0].control,'已暂停');assert.equal(saved.tasks[1].attempts,0);await page.getByRole('button',{name:'恢复计划',exact:true}).click();}
  if(mode==='cancelled'){assert.equal(saved.plans[0].control,'等待跳过确认');await page.getByRole('button',{name:'确认跳过当前批次',exact:true}).click();await page.getByRole('button',{name:'确认继续',exact:true}).click();}
  await page.waitForTimeout(500);saved=(await store.get()).data;
  if(['threshold','paused'].includes(mode)){assert.equal(saved.plans[0].position,1);assert.equal(saved.tasks[1].attempts,1);}
  if(mode==='cancelled'){assert.equal(saved.plans[0].control,'执行已结束');assert.equal(saved.tasks[1].attempts,0);}
  assert.deepEqual(errors,[]);await shot(page,'scheduling-'+mode+'.png');console.log('PASS scheduling '+mode);await ctx.close();
 }}finally{await browser.close();}
});
