import { afterEach, describe, expect, it, vi } from 'vitest'
import { handOverCloudVideo } from './cloudVideo.js'

// 地址在这里是**假造**的：真实地址是凭据，不进测试、不进日志，这里只需看它被原样
// 交出去。
const SIGNED =
  'https://s3.oss.longyanyue.cn/data/dev/materials/30/a.mp4?X-Amz-Signature=stub'

/**
 * 装一个假的 `window`（`environment: 'node'` 下没有它）并把每次调用按顺序记进一条
 * 流水——这两条分支的规矩有一半在顺序上：浏览器端要先开标签页再取地址，桌面端
 * 一次都不预留。
 */
function installWindow({ reserve = null } = {}) {
  const calls = []
  globalThis.window = {
    open: (...args) => {
      calls.push(['open', ...args])
      return reserve
    },
  }
  return calls
}

/** 预留出来的标签页：`opener` 从非 null 起，`replace`/`close` 都记进同一条流水。 */
function fakeTab(calls) {
  return {
    opener: 'kept',
    location: { replace: (url) => calls.push(['replace', url]) },
    close: () => calls.push(['close']),
  }
}

/** 取地址也进流水，否则「取址在开窗之后」这条无从判断。 */
function fakeClient(calls, { url = SIGNED, fail = null } = {}) {
  return {
    getVideoUrl: vi.fn(async () => {
      calls.push(['fetch'])
      if (fail) throw fail
      return { url }
    }),
  }
}

afterEach(() => {
  delete globalThis.window
})

describe('cloud video hand-over', () => {
  // 桌面端不预留标签页：WKWebView 里开不出来，返回值恒为 null。地址取到之后交给壳，
  // 由壳送给系统浏览器。
  it('hands the address to the shell on desktop, without reserving a tab', async () => {
    const calls = installWindow()
    const client = fakeClient(calls)
    await handOverCloudVideo({
      client,
      materialId: 30,
      desktop: true,
      reportError: () => {},
    })
    expect(calls).toEqual([['fetch'], ['open', SIGNED, '_blank', 'noopener']])
    expect(client.getVideoUrl).toHaveBeenCalledWith(30)
  })

  // 壳对应用内开窗一律是拒绝，`window.open` 的返回值因此永远是 null。把它读成
  // 「被拦」就是此前那个误报，这条钉住它不再回来。
  it('does not read the shell refusing the in-app window as a block', async () => {
    const calls = installWindow()
    const reportError = vi.fn()
    await handOverCloudVideo({
      client: fakeClient(calls),
      materialId: 30,
      desktop: true,
      reportError,
    })
    expect(reportError).not.toHaveBeenCalled()
  })

  // 浏览器端反过来：跨过 await 就不算用户手势，标签页必须**先**开出来。
  it('reserves the tab before asking for the address, then navigates it', async () => {
    const calls = []
    const tab = fakeTab(calls)
    globalThis.window = {
      open: (...args) => {
        calls.push(['open', ...args])
        return tab
      },
    }
    await handOverCloudVideo({
      client: fakeClient(calls),
      materialId: 30,
      desktop: false,
      reportError: () => {},
    })
    expect(calls).toEqual([['open', '', '_blank'], ['fetch'], ['replace', SIGNED]])
    expect(tab.opener).toBe(null)
  })

  // 浏览器里连同步开窗都被拦时，地址一定也用不上：那次签名请求不发。
  it('reports a blocked tab in a browser without spending the request', async () => {
    const calls = installWindow()
    const client = fakeClient(calls)
    const reportError = vi.fn()
    await handOverCloudVideo({ client, materialId: 30, desktop: false, reportError })
    expect(reportError).toHaveBeenCalledWith('浏览器拦截了新标签页，请允许弹出窗口后重试')
    expect(client.getVideoUrl).not.toHaveBeenCalled()
  })

  it('closes the reserved tab when the address cannot be fetched', async () => {
    const calls = []
    const tab = fakeTab(calls)
    globalThis.window = {
      open: (...args) => {
        calls.push(['open', ...args])
        return tab
      },
    }
    const reportError = vi.fn()
    await handOverCloudVideo({
      client: fakeClient(calls, { fail: new Error('网络错误') }),
      materialId: 30,
      desktop: false,
      reportError,
    })
    expect(calls).toEqual([['open', '', '_blank'], ['fetch'], ['close']])
    expect(reportError).toHaveBeenCalledWith('网络错误')
  })

  it('treats an empty address as a failure', async () => {
    const calls = installWindow()
    const reportError = vi.fn()
    await handOverCloudVideo({
      client: fakeClient(calls, { url: '' }),
      materialId: 30,
      desktop: true,
      reportError,
    })
    expect(calls).toEqual([['fetch']])
    expect(reportError).toHaveBeenCalledWith('云端视频地址为空')
  })
})
