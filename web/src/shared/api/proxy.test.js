import { describe, expect, it, vi } from 'vitest'
import { createProxyClient } from './proxy.js'

const response = data => ({ ok: true, json: async () => ({ errcode: 0, data }) })

describe('proxy list paging', () => {
  it('includes resources beyond the first 200 instead of hiding later pages', async () => {
    const fetch = vi.fn(async url => response(new URL(url, 'http://local').searchParams.get('offset') === '0'
      ? Array.from({ length: 200 }, (_, index) => ({ id: `p${index}` }))
      : [{ id: 'p200' }]))
    const rows = await createProxyClient({ fetch }).list({ enabled: true })
    expect(rows).toHaveLength(201)
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(fetch.mock.calls[1][0]).toContain('offset=200')
    expect(fetch.mock.calls[1][0]).toContain('enabled=true')
  })

  it('reports an old server that ignores offset instead of looping or duplicating rows', async () => {
    const fetch = vi.fn(async () => response(Array.from({ length: 200 }, (_, index) => ({ id: `p${index}` }))))
    await expect(createProxyClient({ fetch }).list()).rejects.toThrow('重复记录')
    expect(fetch).toHaveBeenCalledTimes(2)
  })
})
