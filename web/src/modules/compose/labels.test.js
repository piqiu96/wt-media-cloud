import { describe, expect, it } from 'vitest'
import { TRIGGER_TYPE_OPTIONS, triggerLabel } from './labels.js'

describe('compose trigger labels', () => {
  it('uses the same wording for task triggers and output generation', () => {
    expect(triggerLabel('manual')).toBe('人工创建')
    expect(triggerLabel('scheduled')).toBe('策略自动')
    expect(triggerLabel('')).toBe('—')
    expect(triggerLabel(undefined)).toBe('—')
  })

  it('keeps the filter order used by every compose page', () => {
    expect(TRIGGER_TYPE_OPTIONS).toEqual([
      { value: 'manual', label: '人工创建' },
      { value: 'scheduled', label: '策略自动' },
    ])
  })
})
