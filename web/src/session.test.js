import { describe, expect, it, vi } from 'vitest'

import { createSessionClient } from './shared/api/session.js'

describe('session client', () => {
  it('uses the HttpOnly cookie and never persists credentials', async () => {
    const fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({ errcode: 0, data: { id: 'user-1', username: 'tech', role: 'technician', status: 'enabled', game_ids: [] } }),
    }))
    const storage = { setItem: vi.fn(), getItem: vi.fn(), removeItem: vi.fn() }
    const client = createSessionClient({ fetch, storage })

    const user = await client.login('tech', 'a-long-initial-password')

    expect(user.username).toBe('tech')
    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({ credentials: 'include' }))
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.getItem).not.toHaveBeenCalled()
  })

  it('loads the current user with cookie credentials', async () => {
    const fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({ errcode: 0, data: { id: 'user-1', username: 'tech', role: 'technician', status: 'enabled', game_ids: [] } }),
    }))
    const client = createSessionClient({ fetch, storage: {} })

    await client.me()

    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/me', expect.objectContaining({ credentials: 'include' }))
  })
})
