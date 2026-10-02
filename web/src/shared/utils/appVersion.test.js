import { afterEach, describe, expect, it, vi } from 'vitest'

const webVersion = (await import('../../../package.json')).default.version

// 版本号必须两处同源：登录页顶栏与后台左上角品牌共用 resolveAppVersion，
// 不同场景（Tauri 运行时 / 浏览器预览）给出各自真实版本，不能写死。
describe('resolveAppVersion', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.doUnmock('@tauri-apps/api/app')
    vi.resetModules()
  })

  it('returns the Web package version in a browser preview', async () => {
    const { resolveAppVersion } = await import('./appVersion.js')
    expect(await resolveAppVersion()).toBe(webVersion)
  })

  it('returns the Tauri runtime version on desktop', async () => {
    vi.stubGlobal('window', { __TAURI_INTERNALS__: {} })
    vi.doMock('@tauri-apps/api/app', () => ({ getVersion: vi.fn().mockResolvedValue('9.9.9') }))
    const { resolveAppVersion } = await import('./appVersion.js')
    expect(await resolveAppVersion()).toBe('9.9.9')
  })

  it('falls back to the Web version when the desktop version call fails', async () => {
    vi.stubGlobal('window', { __TAURI_INTERNALS__: {} })
    vi.doMock('@tauri-apps/api/app', () => ({ getVersion: vi.fn().mockRejectedValue(new Error('no tauri')) }))
    const { resolveAppVersion } = await import('./appVersion.js')
    expect(await resolveAppVersion()).toBe(webVersion)
  })
})
