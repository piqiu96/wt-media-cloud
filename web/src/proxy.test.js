import { describe, expect, it, vi } from 'vitest'

import { createProxyClient } from './shared/api/proxy.js'

const response = (data) => ({ ok: true, json: async () => ({ errcode: 0, data }) })

describe('proxy client', () => {
  it('uses the Cloud local-scan preview and confirm endpoints without sending proxy secrets', async () => {
    const fetch = vi.fn(async () => response({ scan_id: 'scan-1', changes: [] }))
    const client = createProxyClient({ fetch })

    await client.previewLocalScan('scan-1')
    await client.confirmLocalScan('scan-1', 'node-1')

    expect(fetch).toHaveBeenNthCalledWith(1, '/api/v1/proxies/local-scan/preview', expect.objectContaining({ method: 'POST', credentials: 'include' }))
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ scan_id: 'scan-1' })
    expect(fetch).toHaveBeenNthCalledWith(2, '/api/v1/proxies/local-scan/confirm', expect.objectContaining({ method: 'POST', credentials: 'include' }))
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ scan_id: 'scan-1', node_id: 'node-1' })
  })
})
