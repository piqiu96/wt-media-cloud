import { describe, expect, it, vi } from 'vitest'
import { createComposeClient } from './compose.js'

function ok(data, status = 200) {
  return { ok: true, status, json: async () => ({ errcode: 0, message: 'success', data, logid: 'test' }) }
}

const strategy = {
  team_id: 3,
  game_id: 'naruto',
  name: '火影每日高光',
  content_type: '短视频',
  template_key: 'basic_cut',
  template_version: '1.0',
  parameters: { video_ratio: '16:9', subtitle: false, bgm: false, outro: false },
  allowed_overrides: ['video_ratio'],
  status: 'active',
}

describe('compose api client', () => {
  it('maps the strategy commands to the frozen paths', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([{ key: 'basic_cut', version: '1.0' }]))
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ id: 1 }))
      .mockResolvedValueOnce(ok({ id: 2 }, 201))
      .mockResolvedValueOnce(ok({ id: 1 }))
      .mockResolvedValueOnce(ok({ id: 3 }, 201))
      .mockResolvedValueOnce(ok([]))
    const client = createComposeClient({ fetch })

    await client.listTemplates()
    await client.listStrategies()
    await client.getStrategy(1)
    await client.createStrategy(strategy)
    await client.updateStrategy(1, 4, strategy)
    await client.copyStrategy(1, '火影每日高光 副本')
    await client.listVersions(1)

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-templates')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-strategies')
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/compose-strategies/1')
    expect(fetch.mock.calls[3][0]).toBe('/api/v1/compose-strategies')
    expect(fetch.mock.calls[3][1].method).toBe('POST')
    expect(fetch.mock.calls[4][0]).toBe('/api/v1/compose-strategies/1')
    expect(fetch.mock.calls[4][1].method).toBe('PUT')
    expect(fetch.mock.calls[5][0]).toBe('/api/v1/compose-strategies/1/copy')
    expect(fetch.mock.calls[5][1].method).toBe('POST')
    expect(fetch.mock.calls[6][0]).toBe('/api/v1/compose-strategies/1/versions')
  })

  // PUT 的信封是合同冻结的：{expected_version, strategy}。期望版本由调用方显式传入，
  // 客户端持有的行可能已经过期 —— 把「从哪个版本算期望」藏进客户端就是替页面决定
  // 了并发语义。
  it('wraps the update body in the frozen envelope', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ id: 1 }))
    const client = createComposeClient({ fetch })

    await client.updateStrategy(1, 4, strategy)

    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ expected_version: 4, strategy })
  })

  // 服务端 DisallowUnknownFields 且合同写明 additionalProperties: false：
  // copy 只发 {name}，多带一个字段就是具名的 15201 拒绝。
  it('sends only the name when copying', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ id: 3 }, 201))
    const client = createComposeClient({ fetch })

    await client.copyStrategy(1, '副本')

    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({ name: '副本' })
  })

  // 列表范围由服务端从会话推导（普通运营只见启用的、范围外的直接不返回）。
  // 任何查询串都会被安静丢弃 —— 客户端不替服务端想象筛选参数。
  it('lists strategies without any query parameters', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([]))
    const client = createComposeClient({ fetch })

    await client.listStrategies()

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies')
  })

  // 管理员建策略时需要选团队；团队目录对所有登录用户开放（identity ListTeams），
  // 与策略命令共用一个客户端，避免共享页面去 import Cloud 管理页的 usersApi。
  it('reads the team catalog for the admin picker', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([{ id: 3, name: '一组' }]))
    const client = createComposeClient({ fetch })

    await client.listTeams()

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/operation-teams')
  })

  // 游戏目录同上（identity ListGames，全部登录用户可读）：页面要游戏名做展示，
  // 管理员要全量启用游戏做选择，一条命令两处都够。
  it('reads the game catalog', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([{ id: 'naruto', name: '火影', status: 'enabled' }]))
    const client = createComposeClient({ fetch })

    await client.listGames()

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/games')
  })
})
