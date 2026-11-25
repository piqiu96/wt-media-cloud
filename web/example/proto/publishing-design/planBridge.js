// Read-only page projections over the shared prototype records.
import { confirmPlan, batchTasks, planTasks, nextId, unresolved, dispatchPlan, advance, pausePlan, cancelPlan, planHasStarted, planBatchConfigured, awaitingPlanCheck } from './relationships.js';
import { batchState, isDisposed, trustedPublicTask } from './batchLifecycle.js';
import { publishingCategories, platforms, resourceStatus } from './model.js';
const copy=x=>JSON.parse(JSON.stringify(x));
const match=(rows,id)=>rows.find(x=>String(x.id)===String(id));
const lockedFields=['name','note','ownerId','ownerName','createdBy','dateStart','dateEnd','targets','platformOrder','items'];
export function protectPlanSnapshot(before, after) {
 for(const p of before.plans||[]){
  if(!planHasStarted(before,p))continue;
  const n=match(after.plans||[],p.id);
  if(p.planReview)for(const item of p.items){
   const old=match(before.batches,item.id),next=match(after.batches,item.id);
   if(!old||!next||['name','platform','targets','candidates','executionSettings','draft','planFrozenAt'].some(k=>JSON.stringify(old[k])!==JSON.stringify(next[k])))throw Error('计划开始后不能修改批次资源、内容或执行设置');
  }
  if(!n||lockedFields.some(k=>JSON.stringify(p[k])!==JSON.stringify(n[k]))||n.lockedAt!==p.lockedAt||n.endedAt!==p.endedAt||(p.control==='已取消'&&n.control!=='已取消'))throw Error('计划开始后不能编辑或恢复终止计划');
 }
}
export function planInventory(db) {
 const categories=[...new Set([...publishingCategories(db),...db.plans.flatMap(p=>(p.targets||[]).map(t=>t.category))])];
 return {categories,platforms:platforms.map(name=>{const accounts=db.accounts.filter(a=>a.enabled&&a.platform===name);const assets=db.assets.filter(a=>!resourceStatus(db,a,name));return {name,accounts:accounts.length,videos:assets.length,categories:categories.length,capacity:assets.length,cell:Object.fromEntries(categories.map(c=>[c,assets.filter(a=>a.category===c).length]))};})};
}
export function planViews(db) {
 return db.plans.map(p=>{
  const targets=p.targets||[],goals={};for(const t of targets){goals[t.platform]||={};goals[t.platform][t.category]=Number(t.count);}
  const batches=(p.items||[]).map((i,index)=>{const b=match(db.batches,i.id);if(!b)return null;const tasks=batchTasks(db,b);const published=tasks.filter(t=>p.planReview?trustedPublicTask(db,t):t.status==='已发布');const successCats={};for(const t of published)successCats[t.category]=(successCats[t.category]||0)+1;
   const settled=tasks.length&&!tasks.some(t=>(p.planReview&&awaitingPlanCheck(t))||['待执行','排队中','执行中','等待人工','结果待确认'].includes(t.status));
   const configured=p.planReview?planBatchConfigured(b):tasks.length>0;
   const operationalState=p.planReview?batchState(db,b):null;
   const phase=b.control==='已取消'?'cancelled':index<(p.position||0)||b.control==='执行已结束'?'ended':b.firstStartedAt?((p.planReview?operationalState==='已结束':settled)?'ended':'running'):configured?'ready':'draft';
   return {id:String(b.id),name:b.name,operationalState,platform:b.platform,cats:Object.fromEntries((b.targets||[]).map(t=>[t.category,t.count])),phase,success:published.length,candidateCount:b.candidates?.length||0,successCats,locked:!!(p.lockedAt||tasks.length||b.draft?.resourcesLocked),configured,executed:!!b.firstStartedAt,type:b.type||'base',tasks:tasks.filter(t=>['等待人工','结果待确认','执行失败','平台审核中','待补链接'].includes(t.status)).map(t=>({id:t.id,title:t.title,account:match(db.accounts,t.accountId)?.name||'账号已移除',status:p.planReview&&!unresolved(t)&&!awaitingPlanCheck(t)?'settled':'pending',type:t.status==='执行失败'?'failed':t.status==='等待人工'?'manual':'verify',description:awaitingPlanCheck(t)?'自动核验已停止，待人工核实':t.evidence||t.status,checkRequired:awaitingPlanCheck(t)})),accountCount:new Set(tasks.map(t=>t.accountId)).size};
  }).filter(Boolean);
  const target=targets.reduce((n,t)=>n+Number(t.count),0),success=batches.reduce((n,b)=>n+b.success,0);
  const awaitingPublic=(p.items||[]).some(i=>{const b=match(db.batches,i.id);return b&&batchTasks(db,b).some(t=>['平台审核中','待补链接'].includes(t.status));});
  const pending=planTasks(db,p).some(t=>unresolved(t)||(p.planReview&&(awaitingPlanCheck(t)||(t.status==='已发布'&&!trustedPublicTask(db,t))))),executionEnded=p.control==='执行已结束';
  const ended=!!p.endedAt||(!p.planReview&&executionEnded&&success>=target&&!awaitingPublic&&!pending);
  const view={id:String(p.id),name:p.name,note:p.note||'',owner:p.ownerName||'当前运营',ownerId:p.ownerId,createdBy:p.createdBy,dateStart:p.dateStart||'',dateEnd:p.dateEnd||'',created:p.created||'',updated:p.updated||'',platforms:p.platformOrder||Object.keys(goals),categories:[...new Set(targets.map(t=>t.category))],target,goals,batches,status:['已取消','执行已结束'].includes(p.control)?'ended':planHasStarted(db,p)?'running':'ready',result:p.control==='已取消'?'cancelled':p.control==='执行已结束'?(success>=target?'achieved':awaitingPublic?'pending':'unachieved'):'',control:p.control,position:p.position||0,lockedAt:p.lockedAt,logs:[...(p.designLogs||[]),...(p.log||[]).map(l=>[l.time?.slice(0,16).replace('T',' ')||'',l.type+(l.batchId?' · '+l.batchId:''),'系统'])]};
  view.status=p.control==='已取消'?(pending?'stopping':'ended'):ended?'ended':planHasStarted(db,p)?'running':'ready';
  view.result=p.control==='已取消'?'cancelled':ended?(p.endReason||(success>=target?'achieved':'unachieved')):executionEnded&&awaitingPublic?'pending':'';
  view.planReview=!!p.planReview;
  view.awaitingPublic=awaitingPublic;
  view.reviewBatchId=batches.find(b=>b.tasks.some(t=>t.type==='verify'&&t.status==='settled'))?.id;
  view.canSettle=executionEnded&&!p.endedAt&&(p.planReview?!pending&&!awaitingPublic:planTasks(db,p).every(isDisposed));
  if(p.legacySource){const old=p.legacySource;view.legacyReadOnly=true;view.status=old.status;view.result=old.result;view.control='历史只读';view.batches=view.batches.map(b=>{const source=match(old.batches,b.id);return source?{...b,phase:source.phase,success:source.success||0,successCats:source.successCats,executed:source.executed,locked:true,tasks:[]}:b;});}
  return view;
 });
}
export function savePlanViews(db, views, planReview = false) {
 for(const v of views){
  let p=match(db.plans,v.id);
  if(p?.legacySource)continue;
  if(p&&(planHasStarted(db,p)||p.control==='已取消'||p.endedAt)){
   const original=planViews(db).find(x=>String(x.id)===String(v.id));
   if(['name','note','ownerId','owner','createdBy','dateStart','dateEnd','goals','platforms','categories'].some(k=>JSON.stringify(v[k])!==JSON.stringify(original[k]))||JSON.stringify(v.batches.map(b=>[b.id,b.name,b.platform,b.cats]))!==JSON.stringify(original.batches.map(b=>[b.id,b.name,b.platform,b.cats])))throw Error('计划开始后不能编辑或追加批次');
   continue;
  }
  const targets=v.platforms.flatMap(platform=>v.categories.map(category=>({platform,category,count:Number(v.goals[platform]?.[category]||0)})).filter(t=>t.count>0));
  const splits=v.batches.map(b=>({id:match(db.batches,b.id)?.id||b.id,name:b.name,platform:b.platform,targets:Object.entries(b.cats).filter(([,n])=>n>0).map(([category,count])=>({category,count:Number(count)}))}));
  const structure=rows=>JSON.stringify(rows.map(b=>[String(b.id),b.name,b.platform,b.targets]));
  const current=p?(p.items||[]).map(i=>match(db.batches,i.id)).filter(Boolean):[];
  if(!p||(!planHasStarted(db,p)&&(JSON.stringify(p.targets)!==JSON.stringify(targets)||structure(current)!==structure(splits)))){
   p=confirmPlan(db,{id:p?.id||v.id,name:v.name,confirmKey:p?.confirmKey||v.id,revision:p?.revision||0,platformOrder:v.platforms,targets,splits});
  }
  for(const k of ['name','note','ownerId','createdBy','dateStart','dateEnd','created','updated'])if(p[k]!==v[k])p[k]=v[k];
  if(p.ownerName!==v.owner)p.ownerName=v.owner;
  if(planReview){
   p.planReview=true;
   for(const item of p.items){
    const batch=match(db.batches,item.id);
    Object.assign(batch,{planReview:true,executionReview:true,preparationOnly:true,ownerId:p.ownerId});
    if(batch.needsPreflight)batch.phase='待内容配置';
   }
  }
  // UI notes contain no execution facts; task results always come from db.tasks.
  const notes=v.logs.filter(l=>l[2]!=='系统');if(JSON.stringify(p.designLogs)!==JSON.stringify(notes))p.designLogs=copy(notes);
 }
}
export function importLegacyPlans(db, legacy) {
 if(legacy?.plans?.length){
  db.legacyPlanArchive||=[];
  for(const p of legacy.plans)if(!db.legacyPlanArchive.some(old=>JSON.stringify(old)===JSON.stringify(p)))db.legacyPlanArchive.push(copy(p));
  const incoming=legacy.plans.filter(p=>p.status!=='draft'&&!match(db.plans,p.id));
  savePlanViews(db,incoming);
  for(const old of incoming)if(old.status!=='ready'||old.batches.some(b=>b.executed||b.success>0||b.tasks?.length)){
   const p=match(db.plans,old.id);p.legacySource=copy(old);p.control=old.result==='cancelled'?'已取消':'执行已结束';
  }
 }
 db.planPageMigrated=true;
}
export function planAction(db,id,action,reason){
 const p=match(db.plans,id);if(!p)throw Error('计划不存在');
 if(action==='execute'||action==='resume')dispatchPlan(db,p);
 else if(action==='continue')advance(db,p,true);
 else if(action==='pause')pausePlan(p);
 else if(action==='cancel'){if(!reason?.trim())throw Error('请填写终止原因');cancelPlan(p,db);p.cancelReason=reason.trim();p.log.at(-1).reason=p.cancelReason;}
 else if(action==='makeup')throw Error('已启动计划不能新增批次');
 else if(action==='finish'){
  if(!planViews(db).find(view=>view.id===String(id))?.canSettle)throw Error('请等待全部批次执行完并处理未决结果后再结束');
  const actual=(platform,category)=>planTasks(db,p).filter(t=>(p.planReview?trustedPublicTask(db,t):t.status==='已发布')&&t.platform===platform&&t.category===category).length;
  const gap=p.targets.some(t=>actual(t.platform,t.category)<t.count);
  if(gap&&!reason?.trim())throw Error('请填写未达成原因');
  p.endedAt=new Date().toISOString();p.endReason=gap?'unachieved':'achieved';p.endNote=reason?.trim()||'';
  p.log.push({type:'确认结束计划',reason:p.endNote,time:p.endedAt});
 }else throw Error('不支持的计划操作');
}
