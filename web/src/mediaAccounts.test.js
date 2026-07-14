import { describe, expect, it, vi } from 'vitest'

import { createMediaAccountClient } from './mediaAccounts.js'

function response(data) {
  return { ok: true, json: async () => ({ data }) }
}

describe('media account client', () => {
  it('lists with cookie credentials and encoded tag filters', async () => {
    const fetch = vi.fn(async () => response([]))
    const client = createMediaAccountClient({ fetch })

    await client.list({ platform: 'douyin', allTags: ['launch', 'vip'] })

    expect(fetch).toHaveBeenCalledWith(
      '/api/v1/media-accounts?platform=douyin&all_tags=launch%2Cvip',
      { credentials: 'include' },
    )
  })

  it('creates an account without persisting or returning cookie fields', async () => {
    const fetch = vi.fn(async () => response({
      id: 'account-1',
      platform: 'douyin',
      original_cookie: 'server-must-not-return-this',
      active_cookie: 'server-must-not-return-this-either',
    }))
    const storage = { setItem: vi.fn(), getItem: vi.fn() }
    const client = createMediaAccountClient({ fetch, storage })

    const account = await client.create({ gameId: 'game-a', platform: 'douyin', originalCookie: 'import-secret' })

    expect(fetch).toHaveBeenCalledWith('/api/v1/media-accounts', expect.objectContaining({
      method: 'POST',
      credentials: 'include',
      body: JSON.stringify({ game_id: 'game-a', platform: 'douyin', original_cookie: 'import-secret' }),
    }))
    expect(account.original_cookie).toBeUndefined()
    expect(account.active_cookie).toBeUndefined()
    expect(storage.setItem).not.toHaveBeenCalled()
    expect(storage.getItem).not.toHaveBeenCalled()
  })

  it('adds and removes tags in bulk', async () => {
    const fetch = vi.fn(async () => response({ status: 'updated' }))
    const client = createMediaAccountClient({ fetch })

    await client.addTags(['account-1'], ['launch'])
    await client.removeTags(['account-1'], ['launch'])

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/media-accounts/tags/add', expect.objectContaining({ credentials: 'include' }))
    expect(fetch).toHaveBeenNthCalledWith(2, '/api/v1/media-accounts/tags/remove', expect.objectContaining({ credentials: 'include' }))
  })
})
