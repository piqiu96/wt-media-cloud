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
      .mockResolvedValueOnce(ok({ id: 7, status: 'disabled' }))
      .mockResolvedValueOnce(ok({ next_run_at: '2026-10-17T02:00:00Z' }))
      .mockResolvedValueOnce(ok({ run_key: 'r', created_count: 1 }))
      .mockResolvedValueOnce(ok([]))
    const client = createComposeClient({ fetch })

    await client.listStrategies()
    await client.getStrategy(7)
    await client.createStrategy({ name: '切片' })
    await client.updateStrategy(7, { name: '切片' })
    await client.copyStrategy(7, '切片 副本')
    await client.setStrategyStatus(7, 'disabled')
    await client.previewStrategyNextRun({ mode: 'interval', interval_minutes: 30, timezone: 'Asia/Shanghai' })
    await client.runStrategyNow(7, 'request-001')
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
    expect(fetch.mock.calls[5][1].body).toBe(JSON.stringify({ status: 'disabled' }))
    expect(fetch.mock.calls[6][0]).toBe('/api/v1/compose-strategies/next-run-preview')
    expect(fetch.mock.calls[6][1].method).toBe('POST')
    expect(fetch.mock.calls[6][1].body).toBe(JSON.stringify({ mode: 'interval', interval_minutes: 30, timezone: 'Asia/Shanghai' }))
    expect(fetch.mock.calls[7][0]).toBe('/api/v1/compose-strategies/7/run-now')
    expect(fetch.mock.calls[7][1].method).toBe('POST')
    expect(fetch.mock.calls[7][1].body).toBe(JSON.stringify({ request_key: 'request-001' }))
    expect(fetch.mock.calls[8][0]).toBe('/api/v1/compose-templates')
    expect(fetch.mock.calls[9][0]).toBe('/api/v1/operation-teams')
  })

  // 页面用类目列表做「类目 ID → 名称」的展示映射和筛选/创建表单下拉，与素材/内容池页
  // 共用同一个 /categories 端点。成片页需要「全量名称 + 仅启用选项」两种读法，所以参数必须透传。
  it('lists categories from the shared categories endpoint with its filters', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([{ id: 1, type: 'game', name: '游戏.三角洲', enabled: true }]))
    const client = createComposeClient({ fetch })

    await expect(client.listCategories()).resolves.toEqual([{ id: 1, type: 'game', name: '游戏.三角洲', enabled: true }])
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/categories')
    expect(fetch.mock.calls[0][1].method).toBe('GET')

    await client.listCategories({ scope: 'all', enabled: true, type: 'game' })
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/categories?type=game&enabled=true&scope=all')
  })

  it('reads strategy schedule run history', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([{ run_key: 'slot-1', skipped: [{ material_id: 8, reason: 'already_produced' }] }]))
    const client = createComposeClient({ fetch })
    await expect(client.listStrategyRuns(7)).resolves.toHaveLength(1)
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/runs')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
  })

  it('reads a filtered history page with the server total', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ items: [{ run_key: 'run-1' }], total: 21, page: 2, page_size: 10 }))
    const client = createComposeClient({ fetch })
    await expect(client.listStrategyRunHistory(7, { page: 2, page_size: 10, trigger_type: 'manual' })).resolves.toMatchObject({ total: 21, page: 2 })
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/runs/history?page=2&page_size=10&trigger_type=manual')
  })

  it('reads a run batch with its schedule slot as the query', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ run: { run_key: 'slot-1' }, tasks: [{ id: 1, status: 'failed' }], counts: { success: 0, failed: 1 } }))
    const client = createComposeClient({ fetch })
    await expect(client.listStrategyRunTasks(7, '2026-10-15T01:00:00Z')).resolves.toMatchObject({ counts: { failed: 1 } })
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/runs/tasks?schedule_slot=2026-10-15T01%3A00%3A00Z')
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

// 素材使用情况（创建抽屉的未使用过/已使用过分组）：服务端一次最多认 200 个 ID，超了整块
// 400，所以分片必须在客户端做掉 —— 留给调用方就是留一个「素材多了就整块不可用」的坑。
describe('compose material states', () => {
  it('reads one query for a small candidate list', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([{ material_id: 42, success_count: 1 }]))
    const client = createComposeClient({ fetch })
    await expect(client.listStrategyMaterialStates(7, [42, 43])).resolves.toHaveLength(1)
    expect(fetch.mock.calls).toHaveLength(1)
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/material-states?material_ids=42%2C43')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
  })

  it('splits more than 200 ids and merges the answers', async () => {
    const ids = Array.from({ length: 450 }, (_, index) => index + 1)
    const fetch = vi.fn()
      .mockResolvedValueOnce(ok([{ material_id: 1 }]))
      .mockResolvedValueOnce(ok([{ material_id: 201 }]))
      .mockResolvedValueOnce(ok([{ material_id: 401 }]))
    const client = createComposeClient({ fetch })
    await expect(client.listStrategyMaterialStates(7, ids)).resolves.toHaveLength(3)
    expect(fetch.mock.calls).toHaveLength(3)
    const counts = fetch.mock.calls.map((call) => call[0].split('material_ids=')[1].split('%2C'))
    expect(counts.map((list) => list.length)).toEqual([200, 200, 50])
    expect(counts[0][0]).toBe('1')
    expect(counts[1][0]).toBe('201')
    expect(counts[2][49]).toBe('450')
  })

  // 同一个 ID 出现两次只该问一次：重复只浪费额度，还会让「每个请求的 ID 都有一条」这个
  // 契约看起来被违反了。
  it('asks about a repeated id once', async () => {
    const fetch = vi.fn().mockResolvedValue(ok([]))
    const client = createComposeClient({ fetch })
    await client.listStrategyMaterialStates(7, [42, 42, 43])
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/material-states?material_ids=42%2C43')
  })
})
