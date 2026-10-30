import { describe, expect, it, vi } from 'vitest'
import { createProfileRunningReader } from './profileRunningReader.js'

function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

function setup(overrides = {}) {
  const options = {
    scan: vi.fn().mockResolvedValue({ profiles: [] }),
    canRead: () => true,
    apply: vi.fn(),
    onError: vi.fn(),
    onReading: vi.fn(),
    ...overrides,
  }
  return { ...options, reader: createProfileRunningReader(options) }
}

describe('page-wide profile running-state reader', () => {
  it('shares a pending scan between automatic/manual refresh and window sync', async () => {
    const request = deferred()
    const { reader, scan, apply, onReading } = setup({ scan: vi.fn(() => request.promise) })
    const automatic = reader.read({ notifyError: true })
    const manual = reader.read({ notifyError: true })
    const sync = reader.read()
    expect(manual).toBe(automatic)
    expect(sync).toBe(automatic)
    await Promise.resolve()
    expect(scan).toHaveBeenCalledTimes(1)
    const snapshot = { profiles: [{ bit_profile_id: 'bit-1', status: '1' }] }
    request.resolve(snapshot)
    expect(await sync).toBe(snapshot)
    expect(apply).toHaveBeenCalledExactlyOnceWith(snapshot)
    expect(onReading.mock.calls).toEqual([[true], [false]])
  })

  it('reports an automatic/manual failure once, never retries, and permits a later manual read', async () => {
    const error = new Error('比特未连接')
    const scan = vi.fn().mockRejectedValueOnce(error).mockResolvedValueOnce({ profiles: [] })
    const { reader, onError, apply } = setup({ scan })
    const first = reader.read({ notifyError: true })
    const concurrent = reader.read({ notifyError: true })
    await expect(first).rejects.toBe(error)
    await expect(concurrent).rejects.toBe(error)
    expect(onError).toHaveBeenCalledExactlyOnceWith(error, { notifyError: true })
    expect(scan).toHaveBeenCalledTimes(1)
    expect(apply).not.toHaveBeenCalled()
    await reader.read({ notifyError: true })
    expect(scan).toHaveBeenCalledTimes(2)
  })

  it('does not scan outside the owner desktop scope', async () => {
    const { reader, scan, onReading } = setup({ canRead: () => false })
    expect(await reader.read({ notifyError: true })).toBeNull()
    expect(scan).not.toHaveBeenCalled()
    expect(onReading).not.toHaveBeenCalled()
  })

  it('drops late data after leaving the owner view and never scans two requests concurrently', async () => {
    const firstRequest = deferred()
    const snapshot = { profiles: [{ bit_profile_id: 'new', status: '0' }] }
    const { reader, scan, apply } = setup({ scan: vi.fn().mockImplementationOnce(() => firstRequest.promise).mockResolvedValueOnce(snapshot) })
    const first = reader.read()
    await Promise.resolve()
    reader.invalidate()
    const returned = reader.read()
    expect(scan).toHaveBeenCalledTimes(1)
    firstRequest.resolve({ profiles: [{ bit_profile_id: 'stale', status: '1' }] })
    expect(await first).toBeNull()
    expect(await returned).toBe(snapshot)
    expect(apply).toHaveBeenCalledExactlyOnceWith(snapshot)
    expect(scan).toHaveBeenCalledTimes(2)
  })

  it('does not apply data or surface errors from a view that has been left', async () => {
    const request = deferred()
    let own = true
    const { reader, apply, onError, scan } = setup({ canRead: () => own, scan: vi.fn(() => request.promise) })
    const pending = reader.read({ notifyError: true })
    await Promise.resolve()
    own = false
    reader.invalidate()
    request.reject(new Error('late failure'))
    expect(await pending).toBeNull()
    expect(apply).not.toHaveBeenCalled()
    expect(onError).not.toHaveBeenCalled()
    expect(scan).toHaveBeenCalledTimes(1)
  })
})
