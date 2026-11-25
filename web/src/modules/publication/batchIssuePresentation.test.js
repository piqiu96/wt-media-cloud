import { describe, it, expect } from 'vitest';
import { presentBatchIssues, localContentIssue } from './batchIssuePresentation.js';
describe('batch issue presentation', () => {
 it('merges server and local failures without merging cover orientations or candidates', () => {
  const issues = [
   {row_key:'a',field:'cover_file_id',code:'cover_missing',message:'请选择所需规格的封面'},
   localContentIssue({field:'cover',message:'请选择发布封面'},'a'),
   localContentIssue({field:'portrait',message:'请选择竖版封面'},'a'),
   localContentIssue({field:'cover',message:'请选择发布封面'},'b'),
   {row_key:'a',field:'confirmed',code:'confirmation_required',message:'请确认当前内容'},
   localContentIssue({field:'confirmation',message:'请核对内容'},'a'),
   {row_key:'a',field:'composite_output_id',code:'output_occupied',message:'此成片已被占用'},
   {row_key:'a',field:'composite_output_id',code:'output_occupied',message:'该成片已被占用'},
  ];
  expect(presentBatchIssues(issues)).toHaveLength(5);
  expect(presentBatchIssues(issues).map(i=>i.message)).toContain('请选择竖版发布封面（3:4）');
 });
 it('preserves distinct unknown provider errors on the same field',()=>{
  const issues=[{field:'cover_file_id',code:'file_missing',message:'文件不存在'},{field:'cover_file_id',code:'file_denied',message:'无权访问'}];
  expect(presentBatchIssues(issues)).toEqual(issues);
 });
});

import { planRecordDescription, planRecordLabels } from './planPreparation.js';
it('describes real handoff receipts and tolerates missing history details',()=>{
 expect(planRecordLabels['publication_plan.handoff']).toBe('推进下一批');
 expect(planRecordDescription({event:'publication_plan.handoff',details:{result:{previous_batch_id:'3',current_batch_id:'4',decision:'automatic'}}})).toBe('批次 #3 → 批次 #4 · 自动推进');
 expect(planRecordDescription({event:'publication_plan.handoff'})).toBe('');
});
