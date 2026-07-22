import { describe, expect, it, vi } from 'vitest'

import { createProfileBindingClient } from './shared/api/profileBindings.js'

const response = (data) => ({ ok: true, json: async () => ({ errcode: 0, data }) })

describe('profile binding client', () => {
  it('submits only normalized secret-free snapshot fields', async () => {
    const fetch = vi.fn(async () => response({ id: 'scan-1', status: 'ready', diff: [] }))
    const client = createProfileBindingClient({ fetch })
    const raw = { main_user_id: 'main-user-1', profiles: [{ bit_profile_id: 'p1', main_user_id: 'main-user-1', profile_user_id: 'bit-user-1', name: '窗口', cookie: 'secret', proxyPassword: 'secret' }] }

    await client.submit(raw)

    const body = JSON.parse(fetch.mock.calls[0][1].body)
    expect(body).toEqual({
      main_user_id: 'main-user-1',
      profiles: [{ bit_profile_id: 'p1', main_user_id: 'main-user-1', profile_user_id: 'bit-user-1', name: '窗口' }],
    })
    expect(fetch.mock.calls[0][1].credentials).toBe('include')
  })

  it('confirms scans and lists profiles with cookie credentials', async () => {
    const fetch = vi.fn(async () => response([]))
    const client = createProfileBindingClient({ fetch })

    await client.confirm('scan-1')
    await client.listProfiles()

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/bit-browser/profile-scans/scan-1/confirm', expect.objectContaining({ method: 'POST', credentials: 'include' }))
    expect(fetch).toHaveBeenNthCalledWith(2, '/api/v1/browser-profiles', expect.objectContaining({ credentials: 'include' }))
  })

  it('sends node id for local sensitive profile entries', async () => {
    const fetch = vi.fn(async () => response({ task_id: 'task-1' }))
    const client = createProfileBindingClient({ fetch })

    await client.createProfile({ name: '窗口一' }, { nodeId: 'node-1' })
    await client.openProfile('profile-1', { nodeId: 'node-1' })

    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ name: '窗口一', node_id: 'node-1' })
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ node_id: 'node-1' })
  })
})
