import { describe, expect, it } from 'vitest'
import { attemptSteps, stepMetricsText } from './attemptSteps.js'

describe('attempt steps', () => {
  it('reads the steps the server recorded, in the order they happened', () => {
    expect(attemptSteps({ steps: [{ step: 'prepare_input' }, { step: 'cut' }] }).map((step) => step.step))
      .toEqual(['prepare_input', 'cut'])
  })

  // 这一列是后加的：更早那几次执行（以及任何老响应）只有 `steps` 缺席，不是出错。
  it('treats a missing or malformed field as no steps', () => {
    expect(attemptSteps({ attempt: 1 })).toEqual([])
    expect(attemptSteps({ steps: null })).toEqual([])
    expect(attemptSteps({ steps: {} })).toEqual([])
    expect(attemptSteps({ steps: 'cut' })).toEqual([])
    expect(attemptSteps(undefined)).toEqual([])
    expect(attemptSteps({ steps: [null, 'cut', { step: 'cut' }] })).toHaveLength(1)
  })
})

describe('step metrics text', () => {
  it('names the size, the output duration, the output id and a shortened digest', () => {
    const text = stepMetricsText({ metrics: { size_bytes: 12 * 1024 * 1024, output_duration_ms: 15000, output_id: 55, sha256: '0f34abcd99ee' } })
    expect(text).toContain('文件 12.0 MB')
    expect(text).toContain('成片时长 15.0 秒')
    expect(text).toContain('成片编号 55')
    expect(text).toContain('sha256 0f34abcd…')
  })

  // 未知不是零：没量到的体积画 0 B 会看着像「空文件」。
  it('never shows an unmeasured value as a zero', () => {
    expect(stepMetricsText({ metrics: {} })).toBe('')
    expect(stepMetricsText({})).toBe('')
    expect(stepMetricsText({ metrics: { size_bytes: 0, output_duration_ms: 0 } })).toBe('')
  })

  it('keeps the figures it does have when the others are missing', () => {
    expect(stepMetricsText({ metrics: { size_bytes: 1024 } })).toBe('文件 1.0 KB')
    expect(stepMetricsText({ metrics: { output_id: 7 } })).toBe('成片编号 7')
  })

  it('shows the digest as a fingerprint, not as a copyable value', () => {
    expect(stepMetricsText({ metrics: { sha256: 'a'.repeat(64) } })).toBe(`sha256 ${'a'.repeat(8)}…`)
  })
})
