import { describe, expect, it } from 'vitest'
import { canSelectWindowForBatch, prepareFailedRetry, runningObservationStatus } from './batchProgress.js'

describe('foreground batch result recovery', () => {
  it('retains earlier successful and uncertain rows when retrying failures', () => {
    const items = [
      { key: 'a', status: 'success' },
      { key: 'b', status: 'unconfirmed', reason: '等待核实' },
      { key: 'c', status: 'failed', reason: '明确失败' },
    ]
    expect(prepareFailedRetry(items, ['c'])).toEqual([
      items[0], items[1], { key: 'c', status: 'pending', reason: '' },
    ])
    expect(items[2].status).toBe('failed')
  })

  it('does not turn an unreadable running state into a retryable failure', () => {
    expect(runningObservationStatus(undefined, true)).toBe('unconfirmed')
    expect(runningObservationStatus(true, true)).toBe('success')
    expect(runningObservationStatus(false, true)).toBe('failed')
  })

  it('selects only active owned windows whose running state is known', () => {
    const profile = { local_state: 'active', enabled: true, reserved_proxy_id: null }
    expect(canSelectWindowForBatch(profile, true, true)).toBe(true)
    expect(canSelectWindowForBatch(profile, true, false)).toBe(false)
    expect(canSelectWindowForBatch(profile, false, true)).toBe(false)
    expect(canSelectWindowForBatch({ ...profile, enabled: false }, true, true)).toBe(false)
    expect(canSelectWindowForBatch({ ...profile, local_state: 'local_missing' }, true, true)).toBe(false)
    expect(canSelectWindowForBatch({ ...profile, reserved_proxy_id: 42 }, true, true)).toBe(false)
  })
})
