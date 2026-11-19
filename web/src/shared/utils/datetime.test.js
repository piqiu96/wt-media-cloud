import { describe, expect, it } from 'vitest'
import { formatDate, formatDateTime } from './datetime.js'

// 两个格式化函数的分工是列宽决定的，不是风格偏好：使用情况那一格在设计图里是
// 「最近发布 2025/9/20」，带上时刻后它会比它右边的「入库时间」还宽，而它描述的是
// 一个比入库时间次要得多的事实。
describe('date formatting', () => {
  it('lets an opted-in page display the same business time in different client timezones', () => {
    const previous = process.env.TZ
    try {
      for (const [timezone, local] of [['UTC', '2026/11/17 16:48'], ['Pacific/Auckland', '2026/11/18 05:48'], ['America/Los_Angeles', '2026/11/17 08:48']]) {
        process.env.TZ = timezone
        expect(formatDateTime('2026-11-17T16:48:00Z', { timeZone: 'Asia/Shanghai' })).toBe('2026/11/18 00:48')
        expect(formatDateTime('2026-11-17T16:48:00Z')).toBe(local)
      }
    } finally {
      if (previous === undefined) delete process.env.TZ
      else process.env.TZ = previous
    }
  })

  it('renders a day without a clock for the cells that only need the day', () => {
    expect(formatDate('2025-09-20T05:17:00Z')).toBe('2025/9/20')
  })

  it('keeps a missing value a dash, not an invalid date', () => {
    for (const value of ['', null, undefined, 0]) {
      expect(formatDate(value), String(value)).toBe('-')
    }
  })

  it('leaves the minute-precision formatter as it was', () => {
    expect(formatDateTime('2025-09-20T05:17:00Z')).toMatch(/^2025\/9\/20 \d{2}:\d{2}$/)
    expect(formatDateTime('')).toBe('-')
  })
})
