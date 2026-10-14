import { describe, expect, it } from 'vitest'
import { diagnosticSummary, reviewReady, safeComposeError } from './uncertainReview.js'

describe('uncertain execution review', () => {
  it('shows only stored stage and safe error category in the diagnostic summary', () => {
    const summary = diagnosticSummary({
      attempt: 2,
      outcome: 'failed',
      stage: 'cutting',
      error_code: 'execution_uncertain',
      error_message: 'secret=https://storage.example/signed?token=abc',
    })
    expect(summary).toContain('第 2 次')
    expect(summary).toContain('结果不明确')
    expect(summary).not.toContain('storage.example')
    expect(summary).not.toContain('token=')
    expect(safeComposeError('storage_error')).toBe('存储暂时失败')
  })

  it('needs both declarations and a reviewable note without URLs', () => {
    const note = '核查原 Worker 节点进程已退出，数据库和对象存储均无已提交成片'
    expect(reviewReady({ oldExecutionStopped: true, noOutputCommitted: true, evidence: note })).toBe(true)
    expect(reviewReady({ oldExecutionStopped: false, noOutputCommitted: true, evidence: note })).toBe(false)
    expect(reviewReady({ oldExecutionStopped: true, noOutputCommitted: true, evidence: '太短' })).toBe(false)
    expect(reviewReady({ oldExecutionStopped: true, noOutputCommitted: true, evidence: `${note} https://signed.example/abc` })).toBe(false)
  })
})
