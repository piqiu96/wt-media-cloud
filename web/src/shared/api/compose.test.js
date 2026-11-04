import { describe, expect, it, vi } from 'vitest'
import { createComposeClient } from './compose.js'

function ok(data, status = 200) {
  return { ok: true, status, json: async () => ({ errcode: 0, message: 'success', data, logid: 'test' }) }
}

// 路径与 production/router.go 的六条策略路由 + 模板目录逐一比对（CHG-20261012-081 T-01
// 合同）。客户端只做路径映射，不改方法名与路径的对应关系；错一条就是打到 404 上。
describe('compose api client', () => {
  it('previews before confirming the reviewed result', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ selected_count: 2, estimated_output_count: 10, preview_token: 'x'.repeat(64) }))
    const client = createComposeClient({ fetch })
    const preview = await client.previewStrategyExecution(7)
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/execution-preview')
    expect(fetch.mock.calls[0][1].method).toBe('POST')
    await client.runStrategyNow(7, 'command-1', preview.preview_token)
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({ request_key: 'command-1', preview_token: 'x'.repeat(64) })
  })

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
    await client.runStrategyNow(7, 'request-001', 'a'.repeat(64))
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
    expect(fetch.mock.calls[7][1].body).toBe(JSON.stringify({ request_key: 'request-001', preview_token: 'a'.repeat(64) }))
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
    const fetch = vi.fn().mockResolvedValue(ok({items:[{ run_key: 'slot-1', skipped: [{ material_id: 8, reason: 'previously_run' }] }],total:1,page:1,page_size:20}))
    const client = createComposeClient({ fetch })
    await expect(client.listStrategyRuns(7)).resolves.toMatchObject({total:1})
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/runs')
    expect(fetch.mock.calls[0][1].method).toBe('GET')
  })

  it('reads a filtered history page with the server total', async () => {
    const fetch = vi.fn().mockResolvedValue(ok({ items: [{ run_key: 'run-1' }], total: 21, page: 2, page_size: 10 }))
    const client = createComposeClient({ fetch })
    await expect(client.listStrategyRuns(7, { page: 2, page_size: 10, trigger_type: 'immediate' })).resolves.toMatchObject({ total: 21, page: 2 })
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-strategies/7/runs?page=2&page_size=10&trigger_type=immediate')
  })

  it('reads batch metadata with its global counts and available actions', async () => {
    const data={batch:{id:7,counts:{pending:3,failed:2}},actions:{retry_count:2,cancel_count:3}}
    const fetch=vi.fn().mockResolvedValue(ok(data))
    const client=createComposeClient({fetch})
    await expect(client.getComposeTaskBatch(7)).resolves.toEqual(data)
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-task-batches/7')
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

    await client.createComposeTasks({ request_key:'batch-1', name:'新制作', strategy_id:5, operator_id:9, material_ids:[42], setting_overrides:[{key:'main',params:{end_seconds:20}}], remake_confirmed:true })
    await client.listComposeTasks({ status: 'pending', compose_strategy_id: '3' })
    await client.getComposeTask(9)
    await client.cancelComposeTask(9)

    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-tasks')
    expect(fetch.mock.calls[0][1].method).toBe('POST')
    expect(fetch.mock.calls[0][1].body).toBe(JSON.stringify({ request_key:'batch-1', name:'新制作', strategy_id:5, operator_id:9, material_ids:[42], setting_overrides:[{key:'main',params:{end_seconds:20}}], remake_confirmed:true }))
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

describe('compose batch operations', () => {
  it('forwards server pagination and exact batch context without reshaping totals', async () => {
    const data={items:[{id:1}],total:501,page:3,page_size:20}
    const fetch=vi.fn().mockResolvedValue(ok(data))
    const client=createComposeClient({fetch})
    await expect(client.listComposeTaskBatches({page:3,page_size:20,operator_id:9,search:'名字'})).resolves.toEqual(data)
    const url=new URL(fetch.mock.calls[0][0],'https://cloud.test')
    expect(url.pathname).toBe('/api/v1/compose-task-batches')
    expect(Object.fromEntries(url.searchParams)).toEqual({page:'3',page_size:'20',operator_id:'9',search:'名字'})
    await client.listComposeTasks({batch_id:7,status:'failed',page:2,page_size:10})
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-tasks?batch_id=7&status=failed&page=2&page_size=10')
  })
  it('keeps full batch and explicit task ID command scopes distinct', async () => {
    const fetch=vi.fn().mockResolvedValue(ok({accepted:1,skipped:1,failed:0,items:[{task_id:2,status:'accepted'},{task_id:3,status:'skipped',reason:'已结束'}]}))
    const client=createComposeClient({fetch})
    await client.batchRetryComposeTasks({batch_id:7})
    await client.batchCancelComposeTasks({task_ids:[2,3]})
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-tasks/batch-retry')
    expect(JSON.parse(fetch.mock.calls[0][1].body)).toEqual({batch_id:7})
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-tasks/batch-cancel')
    expect(JSON.parse(fetch.mock.calls[1][1].body)).toEqual({task_ids:[2,3]})
  })
  it('reads privileged disposition through attempts and retains protected recovery write', async () => {
    const fetch=vi.fn().mockResolvedValue(ok([{attempt:1,uncertain_review:{actor_user_id:9,evidence:'受权依据'}}]))
    const client=createComposeClient({fetch})
    await expect(client.listComposeTaskAttempts(5)).resolves.toMatchObject([{uncertain_review:{actor_user_id:9}}])
    await client.resolveComposeUncertain(5,{old_execution_stopped:true,no_output_committed:true,evidence:'受权依据'})
    expect(fetch.mock.calls[0][0]).toBe('/api/v1/compose-tasks/5/attempts')
    expect(fetch.mock.calls[1][0]).toBe('/api/v1/compose-tasks/5/uncertain-review')
    expect(fetch.mock.calls[1][1].method).toBe('POST')
    expect(client.listComposeUncertainReviews).toBeUndefined()
    expect(client.listStrategyMaterialStates).toBeUndefined()
  })
})
