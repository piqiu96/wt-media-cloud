import { describe, expect, it, vi } from 'vitest'
import { createComposeClient } from './compose.js'

function ok(data, status = 200) {
  return { ok: true, status, json: async () => ({ errcode: 0, message: 'success', data, logid: 'test' }) }
}

// 路径与 production/router.go 的六条策略路由 + 模板目录逐一比对（CHG-20261012-081 T-01
// 合同）。客户端只做路径映射，不改方法名与路径的对应关系；错一条就是打到 404 上。
describe('compose api client', () => {
  it('maps the strategy commands to the contract paths', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ id: 7 }, 201))
      .mockResolvedValueOnce(ok({ id: 7 }))
      .mockResolvedValueOnce(ok({ id: 7 }))
      .mockResolvedValueOnce(ok({ id: 8 }, 201))
      .mockResolvedValueOnce(ok({ id: 7, status: 'inactive' }))
      .mockResolvedValueOnce(ok([]))
    const client = createComposeClient({ fetch })

    await client.listStrategies()
    await client.getStrategy(7)
    await client.createStrategy({ name: '切片' })
    await client.updateStrategy(7, { name: '切片' })
    await client.copyStrategy(7, '切片 副本')
    await client.setStrategyStatus(7, 'inactive')
    await client.listTemplates()
    await client.listTeams()

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-strategies/7')
    expect(fetch.mock.calls[1][1].method).toBe('GET')
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/compose-strategies')
    expect(fetch.mock.calls[2][1].method).toBe('POST')
    expect(fetch.mock.calls[2][1].body).toBe(JSON.stringify({ name: '切片' }))
    expect(fetch.mock.calls[3][0]).toBe('/api/v1/compose-strategies/7')
    expect(fetch.mock.calls[3][1].method).toBe('PUT')
    expect(fetch.mock.calls[3][1].body).toBe(JSON.stringify({ name: '切片' }))
    expect(fetch.mock.calls[4][0]).toBe('/api/v1/compose-strategies/7/copy')
    expect(fetch.mock.calls[4][1].method).toBe('POST')
    expect(fetch.mock.calls[4][1].body).toBe(JSON.stringify({ name: '切片 副本' }))
    expect(fetch.mock.calls[5][0]).toBe('/api/v1/compose-strategies/7/status')
    expect(fetch.mock.calls[5][1].method).toBe('PATCH')
    expect(fetch.mock.calls[5][1].body).toBe(JSON.stringify({ status: 'inactive' }))
    expect(fetch.mock.calls[6][0]).toBe('/api/v1/compose-templates')
    expect(fetch.mock.calls[7][0]).toBe('/api/v1/operation-teams')
  })

  // 页面用游戏列表做「游戏 ID → 名称」的展示映射和创建表单下拉，与素材/内容池页
  // 共用同一个 /games 端点。这里钉住方法存在且不带参数——状态过滤交给服务端语义，
  // 客户端不替它想象。
  it('lists games from the shared games endpoint', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([{ id: 'naruto', name: '火影' }]))
    const client = createComposeClient({ fetch })

    await expect(client.listGames()).resolves.toEqual([{ id: 'naruto', name: '火影' }])
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/games')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
  })
})

  // CHG-20261013-082：合成任务四条命令与 production/router.go 一一对应。列表的
  // 筛选参数（status / compose_strategy_id）由服务端读 query，客户端只负责拼 URL。
  it('maps the compose task commands to the contract paths', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok({ created: [], failed: [] }))
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ id: 9, status: 'pending' }))
      .mockResolvedValueOnce(ok({ id: 9, status: 'cancelled' }))
    const client = createComposeClient({ fetch })

    await client.createComposeTasks({ strategy_id: 5, items: [{ material_id: 42, request_key: 'rk-a' }] })
    await client.listComposeTasks({ status: 'pending', compose_strategy_id: '3' })
    await client.getComposeTask(9)
    await client.cancelComposeTask(9)

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-tasks')
    expect(fetch.mock.calls[0][1].method).toBe('POST')
    expect(fetch.mock.calls[0][1].body).toBe(JSON.stringify({ strategy_id: 5, items: [{ material_id: 42, request_key: 'rk-a' }] }))
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-tasks?status=pending&compose_strategy_id=3')
    expect(fetch.mock.calls[1][1].method).toBe('GET')
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/compose-tasks/9')
    expect(fetch.mock.calls[2][1].method).toBe('GET')
    expect(fetch.mock.calls[3][0]).toBe('/api/v1/compose-tasks/9/cancel')
    expect(fetch.mock.calls[3][1].method).toBe('POST')
  })

  // CHG-20261013-089：执行记录与人工重试两条命令与 production/router.go 一一对应。
  // retry 只认 failed 终态，非重试错误码由服务端 409 拒绝——客户端不带分类参数，
  // 分类规则钉在服务端，页面只按错误码藏入口。
  it('maps the attempts and retry commands to the contract paths', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ id: 9, status: 'pending' }))
    const client = createComposeClient({ fetch })

    await client.listComposeTaskAttempts(9)
    await client.retryComposeTask(9)

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-tasks/9/attempts')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-tasks/9/retry')
    expect(fetch.mock.calls[1][1].method).toBe('POST')
  })

  // CHG-20261013-084：成片四条命令与 production/router.go 一一对应。列表的
  // publish_status 筛选由服务端读 query；video-url 每次预览现取，客户端不做缓存。
  it('maps the compose output commands to the contract paths', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([]))
      .mockResolvedValueOnce(ok({ output: {}, trace: {} }))
      .mockResolvedValueOnce(ok({ id: 5, publish_status: 'archived' }))
      .mockResolvedValueOnce(ok({ url: 'https://signed.example/x' }))
      .mockResolvedValueOnce(ok({ id: 'transfer-1', status: 'pending' }, 202))
    const client = createComposeClient({ fetch })

    await client.listComposeOutputs({ publish_status: 'pending_publish' })
    await client.getComposeOutput(5)
    await client.archiveComposeOutput(5)
    await client.getComposeOutputVideoUrl(5)
    await client.createOutputDownload(5)

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-outputs?publish_status=pending_publish')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-outputs/5')
    expect(fetch.mock.calls[1][1].method).toBe('GET')
    expect(fetch.mock.calls[2][0]).toBe('/api/v1/compose-outputs/5/archive')
    expect(fetch.mock.calls[2][1].method).toBe('POST')
    expect(fetch.mock.calls[3][0]).toBe('/api/v1/compose-outputs/5/video-url')
    expect(fetch.mock.calls[3][1].method).toBe('GET')
    // CHG-20261013-085：成片下载与素材 createDownload 同一个 202 语义 —— 此刻还没有
    // 任何字节被下载，下一次诚实的进度来源是 GET /file-transfer-tasks。
    expect(fetch.mock.calls[4][0]).toBe('/api/v1/compose-outputs/5/downloads')
    expect(fetch.mock.calls[4][1].method).toBe('POST')
  })
