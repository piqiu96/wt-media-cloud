import { describe, expect, it, vi } from 'vitest'

import { createSessionClient } from './shared/api/session.js'

describe('session client', () => {
  it('uses the HttpOnly cookie and never persists credentials', async () => {
    const fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({ errcode: 0, data: { id: 1, username: 'admin', role: 'admin', status: 'enabled', team_id: null, team_name: '', game_ids: [] } }),
    }))
    const storage = { setItem: vi.fn(), getItem: vi.fn(), removeItem: vi.fn() }
    const client = createSessionClient({ fetch, storage })

    const user = await client.login('admin', 'a-long-initial-password')

    expect(user.username).toBe('admin')
    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({ credentials: 'include' }))
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.getItem).not.toHaveBeenCalled()
  })

  it('loads the current user with cookie credentials', async () => {
    const fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({ errcode: 0, data: { id: 1, username: 'admin', role: 'admin', status: 'enabled', team_id: null, team_name: '', game_ids: [] } }),
    }))
    const client = createSessionClient({ fetch, storage: {} })

    await client.me()

    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/me', expect.objectContaining({ credentials: 'include' }))
  })
})
