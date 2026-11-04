import { describe, expect, it } from 'vitest'
import { batchProgress, bulkResultMessage, taskActionEligible } from './batchActions.js'
describe('batch action scope and outcome',()=>{
  it('keeps completion separate from success',()=>{
    expect(batchProgress({pending:2,running:1,success:2,failed:3,cancelled:2})).toEqual({total:10,finished:7,percent:70})
  })
  it('protects uncertain tasks and distinguishes requested stopping from terminal cancellation',()=>{
    expect(taskActionEligible({status:'failed',error_code:'execution_uncertain'},'retry',true)).toBe(false)
    expect(taskActionEligible({status:'cancelled'},'retry',false)).toBe(false)
    expect(taskActionEligible({status:'running',cancel_requested:true},'cancel')).toBe(false)
    expect(taskActionEligible({status:'running'},'cancel')).toBe(true)
  })
  it('reports partial request acceptance without claiming all tasks were cancelled',()=>{
    expect(bulkResultMessage({accepted:2,skipped:1,failed:1},'cancel')).toBe('取消请求已受理 2 条，跳过 1 条，失败 1 条')
  })
})
