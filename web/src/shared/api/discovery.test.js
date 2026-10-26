import { describe, expect, it, vi } from 'vitest'
import { createDiscoveryClient } from './discovery.js'

function ok(data) {
  return { ok: true, status: 200, json: async () => ({ errcode: 0, message: 'success', data, logid: 'test' }) }
}

describe('discovery api client', () => {
  it('passes the game catalog scope and team filter through to the API', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok([]))
    const client = createDiscoveryClient({ fetch })

    await client.listStrategies({ teamId: 10, search: '热点' })
    await client.listGames({ scope: 'all' })
    await client.listTasks({ team_id: 10 })

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/discovery-strategies?search=%E7%83%AD%E7%82%B9&team_id=10')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/games?scope=all')
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/crawl-tasks?team_id=10')
  })
})
