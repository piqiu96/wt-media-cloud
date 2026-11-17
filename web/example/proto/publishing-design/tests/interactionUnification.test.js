import { describe, it, expect } from 'vitest';
import { seed } from '../model.js';
import { batchMetrics, isPendingTask, batchTaskActions } from '../batchLifecycle.js';
import { taskActionItems, canEditTask, canCancelTask } from '../taskExperience.js';

describe('shared task operations and batch decision metrics', () => {
  it('separates current failures from cancelled, abandoned and unresolved results', () => {
    const statuses = ['执行失败','已取消','已放弃','结果待确认','已发布','待执行'];
    const db = {tasks:statuses.map((status,id)=>({id,status,batchId:1}))};
    const batch = {id:1,targets:[{count:6}],firstStartedAt:'2026-11-16T00:00:00Z'};
    expect(batchMetrics(db,batch)).toMatchObject({total:6,target:6,published:1,failed:1,disposed:3,pending:3,achievement:17});
    db.tasks[0].status='已放弃';
    expect(batchMetrics(db,batch)).toMatchObject({failed:0,disposed:4,pending:2});
  });
  it('uses the same pending membership for the summary and task filter before and after execution', () => {
    const batch={id:1,targets:[]}, db={tasks:['待执行','执行失败','已取消','平台审核中','执行中'].map(status=>({status,batchId:1}))};
    for(const firstStartedAt of [undefined,'2026-11-16T00:00:00Z']) {
      batch.firstStartedAt=firstStartedAt;
      expect(batchMetrics(db,batch).pending).toBe(db.tasks.filter(t=>isPendingTask(t,batch)).length);
      expect(isPendingTask(db.tasks[0],batch)).toBe(Boolean(firstStartedAt));
    }
  });
  it('both entry points project batch policy without allowing unsafe cancellation or claiming', () => {
    const db=seed(), batch=db.batches[0], task=db.tasks.find(t=>t.batchId===batch.id);
    for(const status of ['待执行','排队中','执行中','等待人工','结果待确认','执行失败','已发布','已取消','已放弃']) {
      const t={...task,status,claimed:false}, items=taskActionItems(t,db);
      const legal=batchTaskActions(db,batch,t).filter(a=>a!=='cancel'||canCancelTask(t));
      expect(items.map(a=>a.key).sort()).toEqual(['detail',...(canEditTask(t,db)?['edit']:[]),...legal].sort());
      expect(items.filter(a=>a.primary).length).toBeLessThanOrEqual(1);
      const dangerIndex=items.findIndex(a=>a.danger);
      if(dangerIndex>=0) expect(items.slice(dangerIndex).every(a=>a.danger)).toBe(true);
    }
    const claimed={...task,status:'待执行',claimed:true};
    expect(taskActionItems(claimed,db).some(a=>['start','cancel','edit'].includes(a.key))).toBe(false);
  });
  it('keeps stopped batches read-only and never offers retry after stop', () => {
    const db=seed(), b=db.batches[0];b.stoppedAt='now';b.firstStartedAt='now';
    const t={...db.tasks[0],batchId:b.id,status:'执行失败',result:'确认未发布',claimed:false,attemptHistory:[{number:1,ended:'now'}]};
    expect(taskActionItems(t,db).some(a=>['retry','edit','start'].includes(a.key))).toBe(false);
  });
  it('exposes a specific disabled reason without adding redundant help to ordinary buttons', () => {
    const db=seed(), t={...db.tasks[0],batchId:null,status:'待执行',claimed:false,title:'',submittedAt:null,platformId:null,result:'未提交'};
    const items=taskActionItems(t,db), start=items.find(a=>a.key==='start');
    expect(start.disabled).toBe(true);expect(start.reason).toContain('标题');
    expect(items.find(a=>a.key==='detail').reason).toBe('');
    expect(items.find(a=>a.key==='cancel').label).toBe('终止');
  });
});
