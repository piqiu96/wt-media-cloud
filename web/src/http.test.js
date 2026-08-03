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
})
