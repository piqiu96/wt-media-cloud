import { afterEach, describe, expect, it, vi } from 'vitest'


// 登录页和侧栏读取同一原生版本；Web 和读取失败均不编造版本。
describe('resolveAppVersion', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.doUnmock('@tauri-apps/api/app')
    vi.resetModules()
  })

  it('does not display a version in Web', async () => {
    const { resolveAppVersion } = await import('./appVersion.js')
    expect(await resolveAppVersion()).toBe('')
  })

  it('returns the Tauri runtime version on desktop', async () => {
    vi.stubGlobal('window', { __TAURI_INTERNALS__: {} })
    vi.doMock('@tauri-apps/api/app', () => ({ getVersion: vi.fn().mockResolvedValue('9.9.9') }))
    const { resolveAppVersion } = await import('./appVersion.js')
    expect(await resolveAppVersion()).toBe('9.9.9')
  })

  it('does not invent a version when the desktop call fails', async () => {
    vi.stubGlobal('window', { __TAURI_INTERNALS__: {} })
    vi.doMock('@tauri-apps/api/app', () => ({ getVersion: vi.fn().mockRejectedValue(new Error('no tauri')) }))
    const { resolveAppVersion } = await import('./appVersion.js')
    expect(await resolveAppVersion()).toBe('')
  })
})
