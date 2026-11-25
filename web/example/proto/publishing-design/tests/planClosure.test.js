import { describe, it, expect } from 'vitest';
import { seed, defaults, declarationOptions } from '../model.js';
import { migrateRelationships, confirmPlan, createPublication, dispatchPlan, claimTask, setResult, syncPlan, cancelBatchSafely } from '../relationships.js';
import { planViews, savePlanViews, planAction } from '../planBridge.js';
import { createBatchStore } from '../../../publishing-preview.mjs';

function setup(count=2) {
  const db=migrateRelationships(seed());db.plans=[];db.batches=[];db.tasks=[];
  const p=confirmPlan(db,{name:'计划收尾',confirmKey:'closure',platformOrder:['百家号'],targets:[{platform:'百家号',category:'三角洲行动',count}],splits:Array.from({length:count},(_,i)=>({name:'批次'+i,platform:'百家号',targets:[{category:'三角洲行动',count:1}]}))});
  db.batches.forEach((b,i)=>{const asset=db.assets.find(a=>a.id===128+i*2);createPublication(db,{name:b.name,platform:b.platform,planId:p.id,batchId:b.id,rows:[{assetId:asset.id,platform:b.platform,category:asset.category,accountId:db.accounts.find(a=>a.enabled&&a.platform===b.platform&&a.categories.includes(asset.category)).id,...defaults(b.platform,asset),declaration:declarationOptions(b.platform)[0].value}]});});
  return {db,p};
}
function completed(db,p) {
  db.tasks.forEach(t=>setResult(t,'已放弃'));
  db.batches.forEach(b=>b.control='执行已结束');
  p.position=p.items.length;p.control='执行已结束';
}
describe('计划执行锁定与目标收尾',()=>{
  it('启动前终止首批后，保留终止结果并从后续可执行批次启动',()=>{
    const {db,p}=setup();cancelBatchSafely(db,db.batches[0]);
    expect(p.lockedAt).toBeUndefined();dispatchPlan(db,p);
    expect(p.position).toBe(1);expect(p.lockedAt).toBeTruthy();
    expect(db.batches[0].control).toBe('已取消');expect(db.tasks[0].status).toBe('已取消');
    expect(db.tasks[1].status).toBe('排队中');expect(p.control).toBe('执行中');
  });
  it('终止首批仍有未决结果时，不跳过也不锁定计划',()=>{
    const {db,p}=setup();cancelBatchSafely(db,db.batches[0]);setResult(db.tasks[0],'结果待确认');
    expect(()=>dispatchPlan(db,p)).toThrow();
    expect(p.position).toBe(0);expect(p.lockedAt).toBeUndefined();expect(db.tasks[1].status).toBe('待执行');
  });
  it('后续批次未配置时启动失败，不提前移动计划位置',()=>{
    const {db,p}=setup();cancelBatchSafely(db,db.batches[0]);db.batches[1].phase='待资源配置';
    expect(()=>dispatchPlan(db,p)).toThrow(/未就绪/);
    expect(p.position).toBe(0);expect(p.lockedAt).toBeUndefined();
  });
  it('连续终止的前序批次都保留，全部终止时不启动或锁定计划',()=>{
    const {db,p}=setup(3);db.batches.slice(0,2).forEach(batch=>cancelBatchSafely(db,batch));dispatchPlan(db,p);
    expect(p.position).toBe(2);expect(db.tasks.map(task=>task.status)).toEqual(['已取消','已取消','排队中']);
    const all=setup();all.db.batches.forEach(batch=>cancelBatchSafely(all.db,batch));
    expect(()=>dispatchPlan(all.db,all.p)).toThrow();expect(all.p.lockedAt).toBeUndefined();expect(all.p.position).toBe(0);
  });
  it('首次启动后拒绝计划编辑，包括名称与直接保存',()=>{
    const {db,p}=setup();dispatchPlan(db,p);
    const views=planViews(db);views[0].name='绕过入口修改';
    expect(()=>savePlanViews(db,views)).toThrow(/开始|锁定|编辑/);
    expect(p.name).toBe('计划收尾');
  });
  it('全部批次执行结束后依然拒绝追加，保留原目标和批次',()=>{
    const {db,p}=setup();dispatchPlan(db,p);completed(db,p);
    const input={platform:'百家号',name:'补量',cats:{'三角洲行动':2}};
    expect(()=>planAction(db,p.id,'makeup',input)).toThrow(/不能新增/);
    expect(p.targets[0].count).toBe(2);expect(p.items).toHaveLength(2);
  });
  it('确认结束后只读，不再补量，审核中不允许提前补量',()=>{
    const {db,p}=setup();dispatchPlan(db,p);completed(db,p);
    setResult(db.tasks[0],'平台审核中');
    expect(()=>planAction(db,p.id,'makeup',{platform:'百家号',name:'补量',cats:{'三角洲行动':1}})).toThrow();
    setResult(db.tasks[0],'已放弃');planAction(db,p.id,'finish','不再补量');
    expect(planViews(db)[0].status).toBe('ended');expect(p.endReason).toBe('unachieved');
    expect(()=>planAction(db,p.id,'makeup',{platform:'百家号',name:'补量',cats:{'三角洲行动':1}})).toThrow();
  });
  it('终止计划取消全部未开始任务，未知结果保持可核实且不再派发',()=>{
    const {db,p}=setup();dispatchPlan(db,p);claimTask(db,db.tasks[0]);setResult(db.tasks[0],'结果待确认');
    planAction(db,p.id,'cancel','停止整计划');
    expect(db.tasks.map(t=>t.status)).toEqual(['结果待确认','已取消']);
    expect(db.batches.every(b=>b.stoppedAt)).toBe(true);
    expect(planViews(db)[0].status).toBe('stopping');expect(claimTask(db,db.tasks[1])).toBe(false);
    setResult(db.tasks[0],'平台审核中');expect(planViews(db)[0].status).toBe('ended');
  });
  it('单独停止当前批次不终止计划，明确结果满足条件时后续继续',()=>{
    const {db,p}=setup();dispatchPlan(db,p);setResult(db.tasks[0],'平台审核中');
    db.batches[0].stoppedAt=new Date().toISOString();syncPlan(db,p);
    expect(p.position).toBe(1);expect(db.tasks[1].status).toBe('排队中');expect(db.batches[1].stoppedAt).toBeUndefined();
  });
  it('预览服务的终止计划命令原子保存，旧快照不能恢复停止标记',async()=>{
    const {db,p}=setup();dispatchPlan(db,p);const store=createBatchStore();await store.bootstrap(db);
    const result=await store.command({action:'planOperation',input:{planId:p.id,action:'cancel',value:'停止'},revision:1,key:'stop-plan'});
    expect(result.data.tasks.every(t=>t.status==='已取消')).toBe(true);
    const tampered=structuredClone(result.data);delete tampered.batches[0].stoppedAt;
    await expect(store.save(tampered,result.revision)).rejects.toThrow();
  });
});
