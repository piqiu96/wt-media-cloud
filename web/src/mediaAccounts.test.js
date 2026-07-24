import { describe, expect, it, vi } from 'vitest'

import { createMediaAccountClient } from './shared/api/mediaAccounts.js'

function response(data) {
  return { ok: true, json: async () => ({ errcode: 0, data }) }
}

describe('media account client', () => {
  it('lists with cookie credentials and encoded tag filters', async () => {
    const fetch = vi.fn(async () => response([]))
    const client = createMediaAccountClient({ fetch })

    await client.list({ platform: 'douyin', allTags: ['launch', 'vip'] })

    expect(fetch).toHaveBeenCalledWith(
      '/api/v1/media-accounts?platform=douyin&all_tags=launch%2Cvip',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('maps ledger filters to backend query names', async () => {
    const fetch = vi.fn(async () => response([]))
    const client = createMediaAccountClient({ fetch })

    await client.list({
      search: '重点',
      businessStatus: 'enabled',
      loginStatus: 'unknown',
      anyTags: ['launch'],
      excludeTags: ['retired'],
    })

    expect(fetch).toHaveBeenCalledWith(
      '/api/v1/media-accounts?search=%E9%87%8D%E7%82%B9&business_status=enabled&login_status=unknown&any_tags=launch&exclude_tags=retired',
      expect.objectContaining({ credentials: 'include' }),
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

  it('binds and unbinds a confirmed browser profile', async () => {
    const fetch = vi.fn(async () => response({ id: 'account-1', browser_profile_id: 'profile-1' }))
    const client = createMediaAccountClient({ fetch })

    await client.bindProfile('account-1', 'profile-1')
    await client.unbindProfile('account-1')

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/media-accounts/account-1/profile', expect.objectContaining({
      method: 'PATCH', credentials: 'include', body: JSON.stringify({ browser_profile_id: 'profile-1' }),
    }))
    expect(fetch).toHaveBeenNthCalledWith(2, '/api/v1/media-accounts/account-1/profile', expect.objectContaining({
      method: 'DELETE', credentials: 'include',
    }))
  })

  it('starts local account check and submits safe result facts', async () => {
    const fetch = vi.fn(async () => response({ task_id: 'task-1' }))
    const client = createMediaAccountClient({ fetch })

    await client.check('account-1', { nodeId: 'node-1' })

    expect(fetch).toHaveBeenCalledWith('/api/v1/media-accounts/account-1/check', expect.objectContaining({
      method: 'POST',
      credentials: 'include',
      body: JSON.stringify({ node_id: 'node-1' }),
    }))

    fetch.mockResolvedValueOnce(response({
      id: 'account-1',
      platform_account_id: '123',
      original_cookie: 'server-must-not-return-this',
    }))
    const updated = await client.submitCheckResult('account-1', {
      taskId: 'task-1',
      platformAccountId: '123',
      name: '',
      avatarUrl: '',
      loginStatus: 'normal',
      message: 'ok',
    })

    expect(fetch).toHaveBeenLastCalledWith('/api/v1/media-accounts/account-1/check/result', expect.objectContaining({
      method: 'POST',
      credentials: 'include',
      body: JSON.stringify({
        task_id: 'task-1',
        platform_account_id: '123',
        name: '',
        avatar_url: '',
        login_status: 'normal',
        message: 'ok',
      }),
    }))
    expect(updated.original_cookie).toBeUndefined()
  })
})
