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
      .mockResolvedValueOnce(ok([{ id: 1, type: 'game', name: '游戏.三角洲', enabled: true }]))
      .mockResolvedValueOnce(ok(null))
    const client = createUsersClient({ fetch })

    await client.listUsers({ uid: 2, username: 'operator', role: 'operator', teamId: 10, status: 'enabled', categoryId: 8 })
    await client.createUser({ username: 'operator', password: 'temporary-secret', role: 'operator', teamId: 10, categoryIds: [8] })
    await client.listTeams()
    await client.listCategories({ keyword: '火影', type: 'game', enabled: true, scope: 'all' })

    expect(fetch.mock.calls[0][0]).toContain('/users?uid=2&username=operator&role=operator&team_id=10&status=enabled&category_id=8')
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ username: 'operator', password: 'temporary-secret', role: 'operator', team_id: 10, category_ids: [8] })
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/operation-teams')
    expect(fetch.mock.calls[3][0]).toContain('/categories?keyword=%E7%81%AB%E5%BD%B1&type=game&enabled=true&scope=all')
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

  it('loads category reference details from the dedicated endpoint', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ category_id: 7, users: [], media_accounts: [] }))
    const client = createUsersClient({ fetch })

    await expect(client.getCategoryReferences(7)).resolves.toMatchObject({ category_id: 7 })
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/categories/7/references')
  })
})
