import { describe, expect, it } from 'vitest'
import { taskContents, taskExecutionEvents, taskExceptions, taskResultSummary } from './crawlTaskDetail.js'

describe('mining task detail facts', () => {
  it('explains retry output without treating it as newly discovered or newly added content', () => {
    expect(taskResultSummary({ task_type: 'retry_failed_task', status: 'success', stats: { found: 3, added: 0, auto_materialized: 3 } }))
      .toBe('本次处理 3 项，已转素材 3 条。')
  })

  it('does not double count pending material failures as content awaiting review', () => {
    const task = { status: 'partial_success', stats: { found: 2, added: 2, pending: 2, failed: 1 }, results: [
      { platform_content_id: '1', processing_status: 'pending' },
      { platform_content_id: '2', processing_status: 'material_failed' },
    ] }
    expect(taskResultSummary(task)).toBe('发现 2 条，新增入池 2 条，待审核 1 条，失败 1 项。')
  })

  it('keeps failed requests in execution records instead of displaying them as unnamed content', () => {
    const task = { results: [
      { platform_content_id: '1', title: '作品', processing_status: 'auto_materialized' },
      { failure_key: '关键词', processing_status: 'failed', failure_reason: '请求超时' },
    ] }
    expect(taskContents(task)).toHaveLength(1)
    expect(taskExceptions(task)).toEqual([{ stage: '内容处理失败', reason: '请求超时', scope: '关键词' }])
  })

  it('never invents process steps, a finish event, or per-item exception timestamps', () => {
    const task = { created_at: '2026-11-06T01:00:00Z', started_at: '2026-11-06T01:01:00Z', updated_at: '2026-11-06T01:02:00Z', status: 'running', stats: { found: 3 } }
    expect(taskExecutionEvents(task)).toEqual([
      { label: '创建任务', time: task.created_at }, { label: '开始执行', time: task.started_at },
    ])
    expect(taskExceptions({ results: [{ processing_status: 'failed' }] })[0]).not.toHaveProperty('time')
    expect(taskResultSummary(task)).toBe('执行中，已发现 3 条。')
    expect(taskResultSummary({ status: 'pending' })).toBe('等待执行。')
  })

  it('does not invent a platform failure cause or drop the overall task error', () => {
    expect(taskExceptions({ status: 'failed' })[0].reason).toBe('未记录失败原因')
    expect(taskExceptions({ error: '执行中断', results: [{ processing_status: 'material_failed', failure_reason: '素材保存失败' }] })).toHaveLength(2)
    expect(taskExceptions({ error: '执行中断', results: [{ processing_status: 'failed', failure_reason: '执行中断' }] })).toHaveLength(1)
  })

  it('keeps duplicate results in the summary without adding overlapping counts to found content', () => {
    expect(taskResultSummary({ status: 'success', stats: { found: 4, added: 3, auto_materialized: 3, duplicate: 1 } }))
      .toBe('发现 4 条，新增入池 3 条，已转素材 3 条，重复 1 条。')
  })
})
