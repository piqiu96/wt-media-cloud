// Canonical shared-store records; UI localStorage contains creation buffers only.
const {pathToFileURL}=require('node:url');
const path=require('node:path');
const root=pathToFileURL(path.resolve(__dirname,'..')+'/').href;
exports.planFixture=async function(db){
  const {defaults,declarationOptions}=await import(root+'model.js');
  const {migrateRelationships,confirmPlan,createPublication,dispatchPlan,claimTask,setResult,cancelPlan}=await import(root+'relationships.js');
  migrateRelationships(db);db.plans=[];db.batches=[];db.tasks=[];
  let assetIndex=0;
  const assets=db.assets.filter(a=>a.ready!==false&&a.fileAvailable!==false);
  for(const [id,mode] of [['PL-202611-002','unconfigured'],['PL-202611-004','ready'],['PL-202611-005','running'],['PL-202611-003','gap'],['PL-202610-018','achieved'],['PL-202610-015','cancelled']]){
    const asset=assets[assetIndex++],platform='B站';
    const plan=confirmPlan(db,{id,name:'计划走查 '+mode,confirmKey:id,platformOrder:[platform],targets:[{platform,category:asset.category,count:1}],splits:[{name:'批次 '+mode,platform,targets:[{category:asset.category,count:1}]}]});
    Object.assign(plan,{ownerId:'chen',ownerName:'陈小雨',createdBy:'chen',dateStart:'2026-11-17',dateEnd:'2026-11-30',created:'2026-11-17T02:00:00Z',updated:'2026-11-17T02:30:00Z'});
    if(mode==='unconfigured')continue;
    const batch=db.batches.find(b=>b.planId===plan.id);
    createPublication(db,{name:batch.name,platform,planId:plan.id,batchId:batch.id,rows:[{...defaults(platform,asset),assetId:asset.id,platform,category:asset.category,accountId:db.accounts.find(a=>a.enabled&&a.platform===platform&&a.categories.includes(asset.category)).id,declaration:declarationOptions(platform)[0].value}]});
    if(mode==='ready')continue;
    dispatchPlan(db,plan);
    const task=db.tasks.find(t=>t.batchId===batch.id);
    if(mode==='running'){claimTask(db,task);setResult(task,'结果待确认');}
    else if(mode==='cancelled')cancelPlan(plan,db);
    else {setResult(task,mode==='achieved'?'已发布':'已放弃');batch.control='执行已结束';plan.position=plan.items.length;plan.control='执行已结束';}
  }
};
