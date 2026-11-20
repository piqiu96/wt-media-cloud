import { expect, it } from 'vitest';
import { executionResult, executionTimeline } from './executionPresentation.js';
it('does not turn completion or an unknown result into a published result', () => {
  expect(executionResult({ status:'completed', stage:'finished' }).label).toBe('结果待确认');
  expect(executionResult({ status:'completed', result_status:'future-status' }).label).toBe('结果待确认');
});
it('keeps interruption and human waiting visible as warnings in the recorded process', () => {
  const result = executionTimeline({ status:'interrupted', steps:[{seq:1,at:'2026-11-18T07:00:00+08:00',report:{stage:'connection_lost'}}] });
  expect(result).toEqual([{key:'step-1', at:'2026-11-18T07:00:00+08:00', label:'连接中断，等待核实',tone:'warning'}]);
});
it('shows the confirmed preparation-only end once without inventing uploads', () => {
  const result = executionTimeline({ status:'completed', result_status:'failed', finished_at:'end', error_code:'PREPARATION_ONLY_NOT_SUBMITTED', steps:[{seq:1,at:'start',report:{stage:'ready'}},{seq:2,at:'end',report:{stage:'finished'}}] });
  expect(result.map(event => event.label)).toEqual(['资源及账号已核对','执行结束，未上传、未投稿']);
});
it('keeps the original uncertain result separate from a later human conclusion', () => {
  const attempt = { status:'interrupted', result_status:'uncertain', finished_at:'end', resolved_at:'later', resolution_note:JSON.stringify({conclusion:'failed',evidence:{reason:'人工确认未发布'}}) };
  expect(executionResult(attempt).label).toBe('已核实未发布');
  expect(executionTimeline(attempt).at(-1).label).toBe('结果尚未确认，请先核实，避免重复发布。');
});
it('keeps a recorded error visible even when no finish timestamp was saved', () => {
  const result = executionTimeline({status:'interrupted', heartbeat_at:'feedback-time', error_message:'资源无法读取', steps:[]});
  expect(result).toEqual([{key:'result',at:'feedback-time',label:'资源无法读取',tone:'warning'}]);
});
it('preserves a prototype step description instead of inventing a preparation step', () => {
  const result = executionTimeline({status:'running',steps:[{seq:1,at:'step-time',label:'上传 · 等待平台反馈'}]});
  expect(result[0].label).toBe('上传 · 等待平台反馈');
});
