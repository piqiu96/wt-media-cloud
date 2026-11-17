import { describe, it, expect, vi, afterEach } from 'vitest';
import { seed, defaults, declarationOptions } from '../model.js';
import { createPublication, claimTask, setResult } from '../relationships.js';
import { runBatchCommand, protectBatchSnapshot } from '../batchLifecycle.js';
const fixture = (interval = 10) => {
  const db = seed(); db.tasks = []; db.batches = []; db.plans = [];
  const rows = db.assets.slice(0, 2).map(asset => ({...defaults('B站', asset), platform:'B站', assetId:asset.id, category:asset.category, accountId:db.accounts.find(a=>a.platform==='B站'&&a.enabled&&a.categories.includes(asset.category)).id, declaration:declarationOptions('B站')[0].value}));
  const result = createPublication(db, {platform:'B站', name:'间隔测试', rows, taskIntervalSeconds:interval});
  return {db, ...result};
};
afterEach(()=>vi.useRealTimers());
describe('batch execution settings',()=>{
  it('persists the selected interval on the created batch without starting tasks',()=>{
    const {batch,tasks}=fixture(12);expect(batch.rules.taskIntervalSeconds).toBe(12);expect(tasks.map(t=>t.status)).toEqual(['待执行','待执行']);
  });
  it.each([-1,1.5,NaN,Infinity])('rejects an invalid interval %s before creating records',value=>{
    expect(()=>fixture(value)).toThrow(/间隔/);
  });
  it('keeps the existing concurrent-account policy when interval is zero',()=>{
    const {db,batch,tasks}=fixture(0);runBatchCommand(db,batch,'execute',{},()=>[]);
    const other=db.accounts.find(a=>a.platform==='B站'&&a.enabled&&a.id!==tasks[0].accountId);tasks[1].accountId=other.id;
    expect(claimTask(db,tasks[0])).toBe(true);expect(claimTask(db,tasks[1])).toBe(true);
  });
  it('serializes different accounts and waits until the exact completion plus interval boundary',()=>{
    vi.useFakeTimers();vi.setSystemTime(new Date('2026-11-16T00:00:00Z'));
    const {db,batch,tasks}=fixture();runBatchCommand(db,batch,'execute',{},()=>[]);
    expect(claimTask(db,tasks[1])).toBe(false);expect(claimTask(db,tasks[0])).toBe(true);
    expect(claimTask(db,tasks[1])).toBe(false);setResult(tasks[0],'平台审核中');
    expect(claimTask(db,tasks[1],Date.now()+9999)).toBe(false);
    expect(claimTask(db,tasks[1],Date.now()+10000)).toBe(true);
  });
  it.each(['等待人工','结果待确认'])('does not bypass %s even after the interval',status=>{
    const {db,batch,tasks}=fixture();runBatchCommand(db,batch,'execute',{},()=>[]);claimTask(db,tasks[0]);setResult(tasks[0],status);
    expect(claimTask(db,tasks[1],Date.now()+3600000)).toBe(false);
  });
  it('preserves interval enforcement after snapshot restore',()=>{
    vi.useFakeTimers();vi.setSystemTime(new Date('2026-11-16T00:00:00Z'));
    const {db,batch,tasks}=fixture(15);runBatchCommand(db,batch,'execute',{},()=>[]);claimTask(db,tasks[0]);setResult(tasks[0],'已发布');
    const restored=JSON.parse(JSON.stringify(db));expect(claimTask(restored,restored.tasks[1],Date.now()+14999)).toBe(false);expect(claimTask(restored,restored.tasks[1],Date.now()+15000)).toBe(true);
  });
  it('prevents a frozen interval from being changed by snapshot saves',()=>{
    const {db,batch}=fixture(15);runBatchCommand(db,batch,'execute',{},()=>[]);const after=JSON.parse(JSON.stringify(db));after.batches[0].rules.taskIntervalSeconds=0;
    expect(()=>protectBatchSnapshot(db,after)).toThrow(/发布设置/);
  });
  it('late publication receipts do not restart the completed execution interval',()=>{
    vi.useFakeTimers();vi.setSystemTime(new Date('2026-11-16T00:00:00Z'));
    const {db,batch,tasks}=fixture(10);runBatchCommand(db,batch,'execute',{},()=>[]);claimTask(db,tasks[0]);setResult(tasks[0],'平台审核中');const ended=tasks[0].executionEndedAt;
    vi.advanceTimersByTime(11000);setResult(tasks[0],'已发布');expect(tasks[0].executionEndedAt).toBe(ended);expect(claimTask(db,tasks[1])).toBe(true);
  });
  it('a fresh execution clears the prior execution end and starts a fresh wait',()=>{
    vi.useFakeTimers();vi.setSystemTime(new Date('2026-11-16T00:00:00Z'));
    const {db,batch,tasks}=fixture(10);runBatchCommand(db,batch,'execute',{},()=>[]);claimTask(db,tasks[0]);setResult(tasks[0],'执行失败');const ended=tasks[0].executionEndedAt;
    vi.advanceTimersByTime(20000);tasks[0].status='排队中';expect(claimTask(db,tasks[0])).toBe(true);expect(tasks[0].executionEndedAt).toBeUndefined();setResult(tasks[0],'平台审核中');expect(tasks[0].executionEndedAt).not.toBe(ended);expect(claimTask(db,tasks[1])).toBe(false);vi.advanceTimersByTime(10000);expect(claimTask(db,tasks[1])).toBe(true);
  });

});
