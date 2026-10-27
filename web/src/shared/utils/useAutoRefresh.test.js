import { readFileSync } from 'node:fs'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AUTO_REFRESH_INTERVAL_MS, useAutoRefresh } from './useAutoRefresh.js'

const source = readFileSync(new URL('./useAutoRefresh.js', import.meta.url), 'utf8')

// 定时刷新会静默出错的几处：间隔不是 10 秒、重复 start 长出第二个 timer、慢接口把
// 请求叠起来、用户正在操作时把列表换掉、卸载后还在跑。
describe('list auto refresh', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('refreshes every ten seconds by default', async () => {
    expect(AUTO_REFRESH_INTERVAL_MS).toBe(10000)
    const run = vi.fn().mockResolvedValue()
    const refresh = useAutoRefresh(run)
    refresh.start()
    await vi.advanceTimersByTimeAsync(9999)
    expect(run).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(run).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(10000)
    expect(run).toHaveBeenCalledTimes(2)
    refresh.stop()
  })

  it('stops when stopped and keeps a single timer', async () => {
    const run = vi.fn().mockResolvedValue()
    const refresh = useAutoRefresh(run)
    refresh.start()
    refresh.start()
    await vi.advanceTimersByTimeAsync(10000)
    expect(run).toHaveBeenCalledTimes(1)
    refresh.stop()
    await vi.advanceTimersByTimeAsync(30000)
    expect(run).toHaveBeenCalledTimes(1)
  })

  it('skips a tick while the previous read is still in flight', async () => {
    let release
    const run = vi.fn(() => new Promise((resolve) => { release = resolve }))
    const refresh = useAutoRefresh(run)
    refresh.start()
    await vi.advanceTimersByTimeAsync(30000)
    expect(run).toHaveBeenCalledTimes(1)
    release()
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(10000)
    expect(run).toHaveBeenCalledTimes(2)
    refresh.stop()
  })

  it('skips while the page says it should', async () => {
    let skip = true
    const run = vi.fn().mockResolvedValue()
    const refresh = useAutoRefresh(run, { shouldSkip: () => skip })
    refresh.start()
    await vi.advanceTimersByTimeAsync(20000)
    expect(run).not.toHaveBeenCalled()
    skip = false
    await vi.advanceTimersByTimeAsync(10000)
    expect(run).toHaveBeenCalledTimes(1)
    refresh.stop()
  })

  it('skips while the tab is hidden', async () => {
    const run = vi.fn().mockResolvedValue()
    const refresh = useAutoRefresh(run)
    vi.stubGlobal('document', { hidden: true })
    refresh.start()
    await vi.advanceTimersByTimeAsync(20000)
    expect(run).not.toHaveBeenCalled()
    vi.stubGlobal('document', { hidden: false })
    await vi.advanceTimersByTimeAsync(10000)
    expect(run).toHaveBeenCalledTimes(1)
    refresh.stop()
    vi.unstubAllGlobals()
  })

  // 刷新失败保持上一次读数：下一拍照样来，页面不该因为一次读失败停摆。
  it('keeps refreshing after a failed read', async () => {
    const run = vi.fn().mockRejectedValue(new Error('boom'))
    const refresh = useAutoRefresh(run)
    refresh.start()
    await vi.advanceTimersByTimeAsync(20000)
    expect(run).toHaveBeenCalledTimes(2)
    refresh.stop()
  })

  it('starts on mount and stops on unmount', () => {
    expect(source).toContain('onMounted(start)')
    expect(source).toContain('onUnmounted(stop)')
    expect(source).toContain('clearInterval(timer)')
  })
})
