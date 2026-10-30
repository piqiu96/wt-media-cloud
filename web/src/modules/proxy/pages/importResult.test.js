import { describe, expect, it } from 'vitest'
import { markImportUnknown, prepareFailedCheckRetry, reconcileImportItems } from './importResult.js'

describe('proxy import result recovery', () => {
  it('keeps an earlier uncertain row when retrying only a failed row', () => {
    const items = [
      { key: '1', status: 'unconfirmed', reason: '待核实' },
      { key: '2', status: 'pending', reason: '' },
      { key: '3', status: 'success', reason: '' },
    ]
    expect(reconcileImportItems(items, [{ line: 2, status: 'imported' }])).toEqual([
      items[0], { key: '2', status: 'success', reason: '' }, items[2],
    ])
  })

  it('keeps missing per-row responses and lost requests out of the retryable failure set', () => {
    const items = [
      { key: '1', status: 'pending' },
      { key: '2', status: 'skipped' },
    ]
    const missing = reconcileImportItems(items, [])
    const lost = markImportUnknown(items)
    for (const result of [missing, lost]) {
      expect(result[0].status).toBe('unconfirmed')
      expect(result[1].status).toBe('skipped')
      expect(result.some(item => item.status === 'failed')).toBe(false)
    }
  })

  it('retains successful check results when retrying only failures', () => {
    const items = [
      { key: 'a', status: 'success' },
      { key: 'b', status: 'failed', reason: '连接超时' },
      { key: 'c', status: 'unconfirmed', reason: '待核实' },
    ]
    expect(prepareFailedCheckRetry(items, ['b'])).toEqual([
      items[0], { key: 'b', status: 'pending', reason: '' }, items[2],
    ])
  })
})
