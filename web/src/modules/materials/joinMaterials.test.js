import { describe, expect, it, vi } from 'vitest'
import { joinMaterials } from './joinMaterials.js'

describe('joining selected library materials', () => {
  it('skips existing or unavailable rows and keeps per-item failures for retry', async () => {
    const add = vi.fn(async id => { if (id === 4) throw new Error('没有权限') })
    const result = await joinMaterials([
      { id: 1, usage_status: 'active' }, { id: 2, status: 'delisted' },
      { id: 3, usage_status: 'removed', status: 'available' }, { id: 4, status: 'available' },
    ], add)
    expect(add.mock.calls.map(([id]) => id).sort()).toEqual([3, 4])
    expect(result.joined.map(r => r.id)).toEqual([3])
    expect(result.skipped.map(r => r.id)).toEqual([1, 2])
    expect(result.failed).toEqual([{ id: 4, status: 'available', reason: '没有权限' }])
  })
  it('does not submit the same ID twice and limits in-flight requests', async () => {
    let active = 0, max = 0
    const add = vi.fn(async () => { max = Math.max(max, ++active); await new Promise(resolve => setTimeout(resolve, 1)); active-- })
    const result = await joinMaterials([...Array.from({ length: 8 }, (_, id) => ({ id: id + 1 })), { id: 1 }], add)
    expect(add).toHaveBeenCalledTimes(8)
    expect(result.joined).toHaveLength(8)
    expect(max).toBeLessThanOrEqual(3)
  })
})
