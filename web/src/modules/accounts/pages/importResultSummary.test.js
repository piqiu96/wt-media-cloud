import { describe, expect, it } from 'vitest'
import { importHistoryCounts, importResultSummary } from './importResultSummary.js'

describe('batch import result presentation', () => {
  const base = { total: 2, succeeded: 0, failed: 0, skipped: 0, running: 0, pending: 0, uncertain: 0 }
  it.each([
    [{ failed: 2 }, 'danger', '本轮接入未完成'],
    [{ succeeded: 1, failed: 1 }, 'warning', '接入完成，部分账号需处理'],
    [{ pending: 2 }, 'warning', '本轮接入已暂停'],
    [{ uncertain: 1, pending: 1 }, 'warning', '部分结果待确认'],
    [{ running: 1, pending: 1 }, 'warning', '部分结果待确认'],
    [{ succeeded: 2 }, 'success', '本轮接入完成'],
    [{ skipped: 2 }, 'info', '本轮未接入账号'],
    [{ total: 0 }, 'info', '本轮未接入账号'],
  ])('keeps the icon and title consistent with actual counts %j', (patch, tone, title) => {
    expect(importResultSummary({ ...base, ...patch })).toMatchObject({ tone, title })
  })
  it('shows running feedback while processing, rather than a premature success or failure', () => {
    expect(importResultSummary({ ...base, failed: 1, pending: 1 }, true)).toMatchObject({ tone: 'info', title: '正在接入账号' })
  })
})

describe('import history counts', () => {
  it('distinguishes failed and pending rows, and reconciles persisted running rows', () => {
    const counts = { succeeded: 3, failed: 2, pending: 1, uncertain: 2, running: 1, skipped: 4 }
    const result = importHistoryCounts(counts)
    expect(result).toEqual([
      { key: 'succeeded', label: '成功', tone: 'success', value: 3 },
      { key: 'failed', label: '未完成', tone: 'danger', value: 2 },
      { key: 'pending', label: '待处理', tone: 'neutral', value: 1 },
      { key: 'uncertain', label: '待确认', tone: 'warning', value: 3 },
      { key: 'skipped', label: '已跳过', tone: 'neutral', value: 4 },
    ])
    expect(result.reduce((total, item) => total + item.value, 0)).toBe(13)
  })
  it('omits empty statuses and tolerates missing counts', () => {
    expect(importHistoryCounts()).toEqual([])
    expect(importHistoryCounts({ succeeded: 2, failed: 0 })).toEqual([
      { key: 'succeeded', label: '成功', tone: 'success', value: 2 },
    ])
  })
})
