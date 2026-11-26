import { expect, it } from 'vitest';
import { batchStatus, batchListActions, batchActionLabel } from './batchPresentation.js';
it('keeps configuration distinct from execution and respects terminal state over stale phase', () => {
 expect(batchStatus({execution_status:'not_started',configuration_status:'ready'})).toBe('未开始');
 expect(batchStatus({execution_status:'running',execution:{phase:'awaiting_processing'}})).toBe('待处理');
 expect(batchStatus({execution_status:'paused',execution:{phase:'awaiting_processing'}})).toBe('已暂停');
 expect(batchStatus({execution_status:'ended',execution:{phase:'running'}})).toBe('已结束');
 expect(batchStatus({execution_status:'running',execution:{phase:'waiting_interval'}})).toBe('执行中');
 expect(batchStatus({execution_status:'unknown'})).toBe('状态待刷新');
});
it('always offers detail and only projects provider-permitted commands for the owner',()=>{
 const ended={execution_status:'ended',allowed_actions:['copy'],execution:{verification_required_count:0,task_counts:{failed:1}}};
 expect(batchListActions(ended,true)).toEqual(['detail','copy']);
 expect(batchListActions(ended,false)).toEqual(['detail']);
 const active={execution_status:'running',allowed_actions:['pause','stop'],execution:{verification_required_count:0,task_counts:{uncertain:1}}};
 expect(batchListActions(active,true)).toEqual(['process','detail','pause','stop']);
 expect(batchListActions(active,false)).toEqual(['detail']);
 expect(batchListActions({execution_status:'not_started',allowed_actions:['start','edit','stop']},true)).toEqual(['start','edit','detail','stop']);
 expect(batchActionLabel('stop',{execution_status:'not_started'})).toBe('终止');
 expect(batchActionLabel('stop',active)).toBe('终止发布');
});

it('keeps handed-off failures historical while retaining real unresolved work',()=>{
 const previous={execution_status:'running',handed_off_at:'2026-11-26T21:22:36+08:00',allowed_actions:[],execution:{phase:'awaiting_processing',verification_required_count:0,task_counts:{failed:1}}};
 expect(batchStatus(previous)).toBe('已推进');
 expect(batchListActions(previous,true)).toEqual(['detail']);
 const unresolved={...previous,execution:{...previous.execution,task_counts:{failed:1,uncertain:1}}};
 expect(batchStatus(unresolved)).toBe('待处理');
 expect(batchListActions(unresolved,true)).toEqual(['process','detail']);
 const verification={...previous,execution:{...previous.execution,verification_required_count:1}};
 expect(batchListActions(verification,true)).toEqual(['process','detail']);
 expect(batchStatus({...previous,execution_status:'ended'})).toBe('已结束');
});
