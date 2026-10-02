import { describe, expect, it, vi } from 'vitest'
import { createUsersClient } from './usersApi.js'

function ok(data) {
  return { ok: true, status: 200, json: async () => ({ errcode: 0, message: 'success', data, logid: 'test' }) }
}

describe('users api client', () => {
  it('maps filters and user/team mutations to the identity API', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ user: { id: 2 }, one_time_password: 'temporary-secret' }))
      .mockResolvedValueOnce(ok([{ id: 10, name: '火影组' }]))
      .mockResolvedValueOnce(ok([{ id: 'naruto', name: '火影忍者', status: 'enabled' }]))
      .mockResolvedValueOnce(ok(null))
    const client = createUsersClient({ fetch })

    await client.listUsers({ uid: 2, username: 'operator', role: 'operator', teamId: 10, status: 'enabled', gameId: 'game-a' })
    await client.createUser({ username: 'operator', password: 'temporary-secret', role: 'operator', teamId: 10, gameIds: ['game-a'] })
    await client.listTeams()
    await client.listGames({ keyword: '火影', status: 'enabled' })

    expect(fetch.mock.calls[0][0]).toContain('/users?uid=2&username=operator&role=operator&team_id=10&status=enabled&game_id=game-a')
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ username: 'operator', password: 'temporary-secret', role: 'operator', team_id: 10, game_ids: ['game-a'] })
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/operation-teams')
    expect(fetch.mock.calls[3][0]).toContain('/games?keyword=%E7%81%AB%E5%BD%B1&status=enabled')
  })

  it('returns the service error message for referenced team deletion', async () => {
    const fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: async () => ({ errcode: 20003, message: '对象仍有业务引用，不能删除', logid: 'test' }),
    })
    const client = createUsersClient({ fetch })
    await expect(client.deleteTeam(10)).rejects.toMatchObject({ message: '对象仍有业务引用，不能删除', errcode: 20003 })
  })

  it('loads game reference details from the dedicated endpoint', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ game_id: 'naruto', users: [], media_accounts: [] }))
    const client = createUsersClient({ fetch })

    await expect(client.getGameReferences('naruto')).resolves.toMatchObject({ game_id: 'naruto' })
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/games/naruto/references')
  })
})
