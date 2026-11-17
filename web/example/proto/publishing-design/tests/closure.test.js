import { describe, it, expect } from 'vitest';
import { seed, defaults, declarationOptions } from '../model.js';
import { batchMetrics } from '../batchLifecycle.js';
import { allowedActions } from '../taskExperience.js';
import { migrateRelationships, confirmPlan, createPublication, dispatchPlan, setResult, syncPlan, nextId, planImpact } from '../relationships.js';
const fresh=()=>{const db=migrateRelationships(seed());db.tasks=[];db.batches=[];db.plans=[];return db;};
const content=(db,id)=>{const a=db.assets.find(a=>a.id===id);return {platform:'百家号',assetId:id,category:a.category,accountId:db.accounts.find(a=>a.enabled&&a.platform==='百家号'&&a.categories.includes('三角洲行动')).id,...defaults('百家号',a),declaration:declarationOptions('百家号')[0].value};};
const setup=()=>{const db=fresh();const p=confirmPlan(db,{name:'双批',confirmKey:'closure',platformOrder:['百家号'],targets:[{platform:'百家号',category:'三角洲行动',count:2}],splits:[0,1].map(i=>({name:'批'+i,platform:'百家号',targets:[{category:'三角洲行动',count:1}]}))});return {db,p};};
const configure=(db,p,index)=>{const b=db.batches.find(b=>b.id===p.items[index].id);return createPublication(db,{platform:b.platform,name:b.name,batchId:b.id,planId:p.id,rows:[content(db,128+index*2)]});};
describe('publishing closure regressions',()=>{
 it('cancelled tasks do not count as published progress',()=>{const db=fresh(),b={id:1,targets:[{count:2}]};db.tasks=[{batchId:1,status:'已取消'},{batchId:1,status:'已放弃'}];expect(batchMetrics(db,b).progress).toBe(0);});
 it('partial creation retains target as progress denominator',()=>{const db=fresh(),b={id:1,targets:[{count:30}]};db.tasks=Array.from({length:28},()=>({batchId:1,status:'已发布'}));expect(batchMetrics(db,b).progress).toBe(93);});
 it('independent reviewing tasks can supplement a public result',()=>{const db=fresh();expect(allowedActions({id:1,status:'平台审核中'},db)).toContain('supplement');});
 it('waits for configuration and resumes automatically once the next batch is ready',()=>{const {db,p}=setup();const first=configure(db,p,0);dispatchPlan(db,p);setResult(first.tasks[0],'平台审核中');try{syncPlan(db,p)}catch{}expect(p.control).toBe('等待配置');configure(db,p,1);syncPlan(db,p);expect(p.position).toBe(1);expect(p.control).toBe('执行中');expect(db.tasks[1].status).toBe('排队中');syncPlan(db,p);expect(db.tasks[1].attempts).toBe(1);});
 it('string plan IDs do not break formal task ID allocation',()=>{const db=fresh();db.plans.push({id:'PL-123'});expect(Number.isFinite(nextId(db))).toBe(true);});
});

import { planViews, savePlanViews, importLegacyPlans, planInventory } from '../planBridge.js';
import { createBatchStore } from '../../../publishing-preview.mjs';
import { prototypePeople } from '../prototypeSession.js';
import { ownershipIssue } from '../plan/ownership.js';
it('saves a legal merge of unconfigured plan skeletons',async()=>{const {db}=setup();const store=createBatchStore();await store.bootstrap(db);const views=planViews(db);views[0].batches[0].cats['三角洲行动']=2;views[0].batches.splice(1,1);savePlanViews(db,views);await expect(store.save(db,1)).resolves.toBeTruthy();});
it('the default owner can create a plan for an available publishing category',()=>{const db=fresh(),actor=prototypePeople[0];const category=planInventory(db).categories.find(c=>c==='三角洲行动');expect(ownershipIssue({owner:actor.name,ownerId:actor.id,categories:[category]},null,actor,prototypePeople)).toBe('');});
it('a fully cancelled next batch waits for explicit skip instead of stalling as running',()=>{const {db,p}=setup();const a=configure(db,p,0),b=configure(db,p,1);dispatchPlan(db,p);b.tasks.forEach(t=>setResult(t,'已取消'));setResult(a.tasks[0],'平台审核中');syncPlan(db,p);expect(p.position).toBe(1);expect(p.control).toBe('等待跳过确认');expect(b.tasks[0].attempts).toBe(0);});
it('legacy finished plans preserve their reported results and cannot be restarted',()=>{const db=fresh();const old={id:'PL-old',name:'旧计划',status:'ended',result:'achieved',owner:'旧运营',dateStart:'2026-01-01',dateEnd:'2026-01-02',platforms:['百家号'],categories:['三角洲行动'],goals:{百家号:{三角洲行动:2}},logs:[],batches:[{id:'BT-old',name:'旧批次',platform:'百家号',cats:{三角洲行动:2},phase:'ended',success:2,executed:true,tasks:[]}]};importLegacyPlans(db,{plans:[old]});const view=planViews(db)[0];expect(view.status).toBe('ended');expect(view.batches[0].success).toBe(2);expect(()=>dispatchPlan(db,db.plans[0])).toThrow();expect(db.tasks).toHaveLength(0);});
it('archives a second browser legacy record even after the first migration',()=>{const db=fresh();db.planPageMigrated=true;const old={id:'draft-2',status:'draft',name:'另一个浏览器'};importLegacyPlans(db,{plans:[old]});importLegacyPlans(db,{plans:[old]});expect(db.legacyPlanArchive).toEqual([old]);});

it('completed dispatch with reviewing tasks keeps the public goal pending',()=>{const {db,p}=setup();const first=configure(db,p,0),second=configure(db,p,1);dispatchPlan(db,p);setResult(first.tasks[0],'平台审核中');syncPlan(db,p);setResult(second.tasks[0],'平台审核中');syncPlan(db,p);expect(p.control).toBe('执行已结束');expect(planViews(db)[0].result).toBe('pending');});

it('a plan with a started batch is in progress even when its legacy control still says planning',()=>{
 const {db,p}=setup();const first=configure(db,p,0);first.batch.firstStartedAt='2026-11-13T18:35:28.814Z';first.tasks[0].status='执行中';
 const before=JSON.stringify(db);const view=planViews(db)[0];
 expect(view.status).toBe('running');expect(view.batches[0].phase).toBe('running');
 expect(JSON.stringify(db)).toBe(before);
});
it('a stale planning flag does not allow changing structure after a batch has started',()=>{
 const {db,p}=setup();configure(db,p,0).batch.firstStartedAt='2026-11-13T18:35:28.814Z';
 const draft={...p,splits:db.batches.map(b=>({id:b.id,name:b.name,platform:b.platform,targets:b.targets}))};
 expect(()=>planImpact(db,p,draft)).toThrow('首次启动后结构永久锁定');
});

it('saving a started legacy plan projection cannot reapply its structure',()=>{
 const {db,p}=setup();configure(db,p,0).batch.firstStartedAt='2026-11-13T18:35:28.814Z';
 const views=planViews(db),order=p.items.map(i=>i.id);views[0].batches.reverse();
 expect(()=>savePlanViews(db,views)).toThrow(/开始|锁定/);expect(p.items.map(i=>i.id)).toEqual(order);
});
