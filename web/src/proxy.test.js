import { describe, expect, it, vi } from 'vitest'

import { createProxyClient } from './shared/api/proxy.js'

const response = (data) => ({ ok: true, json: async () => ({ errcode: 0, data }) })

describe('proxy client', () => {
  it('starts each profile proxy operation and reports its outcome', async () => {
    const fetch = vi.fn(async () => response({ agent_required: true, profile_id: 'profile-1', proxy_id: 'proxy-1' }))
    const client = createProxyClient({ fetch })

    await client.recommend(['profile-1', 'profile-2'])
    await client.operate('profile-1', { action: 'apply', proxy_id: 'proxy-1' })
    await client.finishOperation('profile-1', { action: 'apply', proxy_id: 'proxy-1', outcome: 'success', message: '' })

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/proxies/recommendations?profile_ids=profile-1%2Cprofile-2')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/browser-profiles/profile-1/proxy-operation')
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ action: 'apply', proxy_id: 'proxy-1' })
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/browser-profiles/profile-1/proxy-operation/result')
    expect(JSON.parse(fetch.mock.calls[2][1].body)).toEqual({ action: 'apply', proxy_id: 'proxy-1', outcome: 'success', message: '' })
  })

  it('uses Cloud-only parse, extraction preview, and binding-detail endpoints', async () => {
    const fetch = vi.fn(async () => response({}))
    const client = createProxyClient({ fetch })

    await client.parseAddress('socks5://user:pass@203.0.113.9:1080')
    await client.previewExtract({ extract_url: 'https://provider.example/extract', protocol: 'socks5' })
    await client.listBindings('proxy-1')

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/proxies/parse', expect.objectContaining({ method: 'POST' }))
    expect(fetch).toHaveBeenNthCalledWith(2, '/api/v1/proxies/extract-preview', expect.objectContaining({ method: 'POST' }))
    expect(fetch).toHaveBeenNthCalledWith(3, '/api/v1/proxies/proxy-1/bindings', expect.objectContaining({ method: 'GET' }))
  })
})
