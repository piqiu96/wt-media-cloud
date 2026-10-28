import { describe, expect, it, vi } from 'vitest'

import { createProfileBindingClient } from './shared/api/profileBindings.js'

const response = (data) => ({ ok: true, json: async () => ({ errcode: 0, data }) })

describe('profile binding client', () => {
  it('submits the Agent snapshot with its observation time and nothing secret', async () => {
    const fetch = vi.fn(async () => response({ id: 'scan-1', status: 'ready', diff: [] }))
    const client = createProfileBindingClient({ fetch })
    const raw = { main_user_id: 'main-user-1', observed_at: '2026-10-23T10:00:00Z', profiles: [{ bit_profile_id: 'p1', main_user_id: 'main-user-1', profile_user_id: 'bit-user-1', name: '窗口', status: 1, proxy_type: 'socks5', proxy_host: '127.0.0.1', proxy_port: 1080, remark: '测试窗口', cookie: 'secret', proxyPassword: 'secret' }] }

    await client.submit(raw)

    const body = JSON.parse(fetch.mock.calls[0][1].body)
    expect(body).toEqual({
      bit_main_user_id: 'main-user-1',
      observed_at: '2026-10-23T10:00:00Z',
      profiles: [{ bit_profile_id: 'p1', bit_main_user_id: 'main-user-1', bit_user_id: 'bit-user-1', bit_name: '窗口', bit_profile_status: '1', bit_remark: '测试窗口', bit_proxy_type: 'socks5', bit_proxy_host: '127.0.0.1', bit_proxy_port: 1080 }],
    })
    expect(fetch.mock.calls[0][1].credentials).toBe('include')
  })

  it('confirms scans with only the selected windows and lists profiles with cookie credentials', async () => {
    const fetch = vi.fn(async () => response([]))
    const client = createProfileBindingClient({ fetch })

    await client.confirm('scan-1')
    await client.confirm('scan-1', ['bit-1'])
    await client.listProfiles()

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/browser-profiles/sync/confirm', expect.objectContaining({ method: 'POST', credentials: 'include' }))
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ adopt_bit_profile_ids: [] })
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ adopt_bit_profile_ids: ['bit-1'] })
    expect(fetch).toHaveBeenNthCalledWith(3, '/api/v1/browser-profiles', expect.objectContaining({ credentials: 'include' }))
  })

  it('uses the single sync, update, and delete surface', async () => {
    const fetch = vi.fn(async () => response({ data: {} }))
    const client = createProfileBindingClient({ fetch })

    await client.review()
    await client.updateProfile('profile-1', { remark: '新备注', enabled: false })
    await client.deleteProfile('profile-1')

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/browser-profiles/sync')
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ remark: '新备注', enabled: false })
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/browser-profiles/profile-1')
  })
})
