import { describe, expect, it } from 'vitest'
import { scheduleLabel } from './labels.js'

// 执行周期在库和快照里是 `manual` / `daily 09:00` / `interval:30` 这样的原始值，
// 列表、详情和来源弹窗读的是同一份，映射只能有一处。
describe('content pool labels', () => {
  it('maps the raw schedule values to the same wording everywhere', () => {
    expect(scheduleLabel('manual')).toBe('手动执行')
    expect(scheduleLabel('')).toBe('手动执行')
    expect(scheduleLabel(null)).toBe('手动执行')
    expect(scheduleLabel('daily 09:00')).toBe('每天 09:00')
    expect(scheduleLabel('interval:30')).toBe('每 30 分钟')
  })

  it('keeps an unknown value readable instead of blanking it', () => {
    expect(scheduleLabel('weekly mon')).toBe('weekly mon')
  })
})
