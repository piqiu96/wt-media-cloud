import { readFileSync } from 'node:fs'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { POLL_INTERVAL_MS, hasActiveTask, isActiveStatus, useTaskPolling } from './useTaskPolling.js'

const source = readFileSync(new URL('./useTaskPolling.js', import.meta.url), 'utf8')

// 状态自动刷新只服务「有任务在排队或执行」这一段窗口。这里量的是它会静默出错的几处：
// 重复 start 长出第二个 timer、慢接口把请求叠起来、卸载后还在跑。
describe('compose task polling', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  it('treats only pending and running as active', () => {
    expect(isActiveStatus('pending')).toBe(true)
    expect(isActiveStatus('running')).toBe(true)
    for (const status of ['success', 'failed', 'cancelled', '', undefined]) expect(isActiveStatus(status)).toBe(false)
  })

  it('finds an active task in the loaded rows', () => {
    expect(hasActiveTask([{ status: 'success' }, { status: 'running' }])).toBe(true)
    expect(hasActiveTask([{ status: 'success' }, { status: 'cancelled' }])).toBe(false)
    // 空列表与非数组（读取失败时 rows 可能是别的东西）都不能让定时器空转。
    expect(hasActiveTask([])).toBe(false)
    expect(hasActiveTask(null)).toBe(false)
  })

  it('runs on the interval only between start and stop', async () => {
    const run = vi.fn().mockResolvedValue()
    const polling = useTaskPolling(run)
    expect(polling.polling.value).toBe(false)

    polling.start()
    expect(polling.polling.value).toBe(true)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(run).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(run).toHaveBeenCalledTimes(2)

    polling.stop()
    expect(polling.polling.value).toBe(false)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3)
    expect(run).toHaveBeenCalledTimes(2)
  })

  it('keeps a single timer across repeated starts', async () => {
    const run = vi.fn().mockResolvedValue()
    const polling = useTaskPolling(run)
    polling.start()
    polling.start()
    polling.start()
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(run).toHaveBeenCalledTimes(1)
    polling.stop()
  })

  it('skips a tick while the previous read is still in flight', async () => {
    let release
    const run = vi.fn(() => new Promise((resolve) => { release = resolve }))
    const polling = useTaskPolling(run)
    polling.start()

    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3)
    expect(run).toHaveBeenCalledTimes(1)

    release()
    await vi.advanceTimersByTimeAsync(0)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(run).toHaveBeenCalledTimes(2)
    polling.stop()
  })

  // 读失败不能把轮询打死：下一次照样来。
  it('keeps polling after a failed read', async () => {
    const run = vi.fn().mockRejectedValue(new Error('boom'))
    const polling = useTaskPolling(run)
    polling.start()
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS)
    expect(run).toHaveBeenCalledTimes(2)
    polling.stop()
  })

  it('polls at the documented five-second rhythm and unmounts its timer', () => {
    expect(POLL_INTERVAL_MS).toBe(5000)
    expect(source).toContain('onUnmounted(stop)')
    expect(source).toContain('clearInterval(timer)')
  })
})
