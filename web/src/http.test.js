import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApiClient } from './shared/api/http.js'

describe('createApiClient', () => {
  const originalWindow = globalThis.window

  afterEach(() => {
    if (originalWindow === undefined) {
      delete globalThis.window
    } else {
      globalThis.window = originalWindow
    }
  })

  it('accepts empty 204 responses for delete operations', async () => {
    const fetch = vi.fn(async () => ({ status: 204, ok: true }))
    const client = createApiClient({ fetchImpl: fetch })

    await expect(client.delete('/browser-profiles/profile-1')).resolves.toBeNull()

    expect(fetch).toHaveBeenCalledWith('/api/v1/browser-profiles/profile-1', {
      method: 'DELETE',
      credentials: 'include',
      headers: {},
    })
  })

  it('uses the local Cloud API origin when packaged Desktop runs outside the dev server', async () => {
    globalThis.window = {
      __WT_MEDIA_APP__: 'desktop',
      location: { protocol: 'tauri:', port: '', pathname: '/login', search: '' },
    }
    const fetch = vi.fn(async () => ({
      status: 200,
      ok: true,
      json: async () => ({ errcode: 0, data: { status: 'ok' } }),
    }))
    const client = createApiClient({ fetchImpl: fetch })

    await client.get('/auth/me')

    expect(fetch).toHaveBeenCalledWith('http://127.0.0.1:18080/api/v1/auth/me', expect.objectContaining({
      credentials: 'include',
    }))
  })

  it('keeps the relative API path for Desktop dev server proxy', async () => {
    globalThis.window = {
      __WT_MEDIA_APP__: 'desktop',
      location: { protocol: 'http:', port: '5174', pathname: '/login', search: '' },
    }
    const fetch = vi.fn(async () => ({
      status: 200,
      ok: true,
      json: async () => ({ errcode: 0, data: { status: 'ok' } }),
    }))
    const client = createApiClient({ fetchImpl: fetch })

    await client.get('/auth/me')

    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/me', expect.objectContaining({
      credentials: 'include',
    }))
  })

  // The backend answers an unauthenticated request with HTTP 401 *and* a
  // structured body whose errcode is non-zero (11001), so parseResponse throws on
  // it. The redirect therefore has to run before parseResponse — this test pins
  // that ordering, which is what keeps the 401 → login path reachable.
  it('redirects to login on a 401 with a structured error body', async () => {
    const location = { pathname: '/material-library', search: '', href: '' }
    globalThis.window = {
      __WT_MEDIA_APP__: 'desktop',
      location,
      localStorage: { getItem: () => null, setItem: () => {}, removeItem: () => {} },
    }
    const fetch = vi.fn(async () => ({
      status: 401,
      ok: false,
      json: async () => ({ errcode: 11001, message: '请先登录或凭证已过期', data: null, logid: 't-1' }),
    }))
    const client = createApiClient({ fetchImpl: fetch })

    await expect(client.get('/material-library')).rejects.toThrow()

    expect(location.href).toBe('/login?redirect=%2Fmaterial-library')
  })

  it('does not redirect for the login endpoint itself', async () => {
    const location = { pathname: '/login', search: '', href: '' }
    globalThis.window = {
      __WT_MEDIA_APP__: 'desktop',
      location,
      localStorage: { getItem: () => null, setItem: () => {}, removeItem: () => {} },
    }
    const fetch = vi.fn(async () => ({
      status: 401,
      ok: false,
      json: async () => ({ errcode: 11001, message: '请先登录或凭证已过期', data: null, logid: 't-2' }),
    }))
    const client = createApiClient({ fetchImpl: fetch })

    await expect(client.post('/auth/login', { username: 'u', password: 'p' })).rejects.toThrow()

    expect(location.href).toBe('')
  })
})
