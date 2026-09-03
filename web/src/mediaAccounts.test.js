import { describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

import { createMediaAccountClient } from './shared/api/mediaAccounts.js'

const __dirname = dirname(fileURLToPath(import.meta.url))

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
        check_items: [],
      }),
    }))
    expect(updated.original_cookie).toBeUndefined()
  })

  it('keeps B6 account page desktop boundaries explicit', () => {
    const source = readFileSync(resolve(__dirname, 'modules/accounts/pages/AccountsPage.vue'), 'utf8')

    expect(source).toContain('社媒账号')
    expect(source).toContain('新增账号')
    expect(source).toContain('>检查</t-button>')
    expect(source).toContain('从 Profile 读真实 Cookie')
    expect(source).toContain('>批量检查</t-button>')
    expect(source).toContain('v-if="isDesktop" variant="outline"')
    expect(source).toContain('请先勾选要检查的账号')
    expect(source).toContain('前往环境监测')
    expect(source).toContain('<t-form-item label="窗口">')
    expect(source).toContain('重试失败项')
    expect(source).toContain('retryFailedOnly')
    expect(source).toContain('打开窗口')
    expect(source).toContain('关闭窗口')
    expect(source).toContain('window-bit-id')
    expect(source).toContain('系统ID {{ profileForAccount')
    expect(source).toContain('不可执行：未绑定游戏')
    expect(source).toContain('Cloud Web 只展示 Cloud 已保存的账号与窗口绑定信息')
    expect(source).not.toContain('account.business_status === "draft"')
  })

  it('passes window binding fuzzy search to backend', async () => {
    const fetch = vi.fn(async () => response([]))
    const client = createMediaAccountClient({ fetch })

    await client.list({ profile: '运营' })

    expect(fetch).toHaveBeenCalledWith(
      '/api/v1/media-accounts?profile_search=%E8%BF%90%E8%90%A5',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('sends account name on create and update', async () => {
    const fetch = vi.fn(async () => response({ id: 'account-1' }))
    const client = createMediaAccountClient({ fetch })

    await client.create({ gameId: 'game-a', platform: 'bilibili', name: '测试昵称' })

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/media-accounts', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ game_id: 'game-a', platform: 'bilibili', name: '测试昵称' }),
    }))

    await client.update('account-1', { name: '新名' })

    expect(fetch).toHaveBeenNthCalledWith(2, '/api/v1/media-accounts/account-1', expect.objectContaining({
      method: 'PATCH',
      body: JSON.stringify({ name: '新名' }),
    }))
  })
})
