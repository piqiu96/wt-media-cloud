import { createApiClient } from './http.js'
import { isDesktop } from '../../utils.js'

// 与 service.ComposeMaterialStateLimit 同一个数：服务端按 ID 个数校验，超限是整块 400。
const MATERIAL_STATE_CHUNK = 200

export function createComposeClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    listTemplates() { return api.get('/compose-templates') },
    listStrategies(filters = {}) { return api.get('/compose-strategies', { search: filters.search, category_id: filters.category_id, template_id: filters.template_id, status: filters.status }) },
    // 管理员创建策略时要选目标团队；高级运营的团队由服务端固定。
    listTeams() { return api.get('/operation-teams') },
    // 类目列表用于展示映射和筛选/创建表单下拉，与素材/内容池页共用同一端点。
    // 成片页要区分「名称字典（全量）」与「筛选项（仅启用）」，所以参数要能透传。
    listCategories(filters = {}) { return api.get('/categories', { keyword: filters.keyword, type: filters.type, enabled: filters.enabled, scope: filters.scope }) },
    // 可指派负责人目录（M4-I）：高级运营固定本团队，管理员必须给 team_id；
    // 策略表单带 category_id 收窄到负责该类目的人，任务/成片筛选只按团队。
    listComposeOperators(filters = {}) { return api.get('/compose-operators', { team_id: filters.team_id, category_id: filters.category_id }) },
    getStrategy(id) { return api.get(`/compose-strategies/${id}`) },
    createStrategy(data) { return api.post('/compose-strategies', data) },
    updateStrategy(id, data) { return api.put(`/compose-strategies/${id}`, data) },
    copyStrategy(id, name) { return api.post(`/compose-strategies/${id}/copy`, { name }) },
    setStrategyStatus(id, status) { return api.patch(`/compose-strategies/${id}/status`, { status }) },
    previewStrategyNextRun(schedule) { return api.post('/compose-strategies/next-run-preview', schedule) },
    runStrategyNow(id, requestKey) { return api.post(`/compose-strategies/${id}/run-now`, { request_key: requestKey }) },
    listStrategyRuns(id) { return api.get(`/compose-strategies/${id}/runs`) },
    listStrategyRunHistory(id, params) { return api.get(`/compose-strategies/${id}/runs/history`, params) },
    // 批次结果归集（CHG-20261015-095）：按运行记录的计划时间取该批次逐素材任务
    // 与状态统计；schedule_slot 原样回传运行记录里的时间串。
    listStrategyRunTasks(id, scheduleSlot) { return api.get(`/compose-strategies/${id}/runs/tasks`, { schedule_slot: scheduleSlot }) },
    // 素材在一条策略下的使用情况（创建抽屉的未使用过/已使用过分组）。服务端一次最多
    // 认 200 个 ID，超了整块 400，所以分片在这里做掉，调用方只给一份 ID 列表。
    async listStrategyMaterialStates(id, materialIds) {
      const ids = [...new Set(materialIds || [])]
      const states = []
      for (let start = 0; start < ids.length; start += MATERIAL_STATE_CHUNK) {
        const chunk = ids.slice(start, start + MATERIAL_STATE_CHUNK)
        const rows = await api.get(`/compose-strategies/${id}/material-states`, { material_ids: chunk.join(',') })
        if (Array.isArray(rows)) states.push(...rows)
      }
      return states
    },
    // 合成任务（CHG-20261013-082）：列表筛选（status / compose_strategy_id）由服务端读 query。
    createComposeTasks(data) { return api.post('/compose-tasks', data) },
    listComposeTasks(params) { return api.get('/compose-tasks', params) },
    getComposeTask(id) { return api.get(`/compose-tasks/${id}`) },
    cancelComposeTask(id) { return api.post(`/compose-tasks/${id}/cancel`) },
    // 执行记录与人工重试（CHG-20261013-089）：attempts 沿任务 scope 读，retry 受理已取消
    // 与除 execution_uncertain 以外的失败；其余状态由服务端 409 compose_task_not_retryable 拒绝。
    listComposeTaskAttempts(id) { return api.get(`/compose-tasks/${id}/attempts`) },
    retryComposeTask(id) { return api.post(`/compose-tasks/${id}/retry`) },
    listComposeUncertainReviews(id) { return api.get(`/compose-tasks/${id}/uncertain-review`) },
    resolveComposeUncertain(id, input) { return api.post(`/compose-tasks/${id}/uncertain-review`, input) },
    // 我的成片（CHG-20261013-084）：publish_status 筛选由服务端读 query；video-url
    // 是短时预签名，每次预览现取，客户端不做缓存。
    listComposeOutputs(params) { return api.get('/compose-outputs', params) },
    getComposeOutput(id) { return api.get(`/compose-outputs/${id}`) },
    archiveComposeOutput(id) { return api.post(`/compose-outputs/${id}/archive`) },
    getComposeOutputVideoUrl(id) { return api.get(`/compose-outputs/${id}/video-url`) },
    // 成片下载（CHG-20261013-085）：与素材 createDownload 同一个 202 语义与同一个
    // Desktop 前置（本机 Agent 必须持有执行凭证）。挂在 compose client 上是因为路由在
    // /compose-outputs 下；下一次诚实的进度来源同样是 GET /file-transfer-tasks。
    async createOutputDownload(id) {
      if (isDesktop()) {
        const { ensureAgentToken } = await import('../../apps/desktop/features/local-agent/init.js')
        await ensureAgentToken()
      }
      return api.post(`/compose-outputs/${id}/downloads`, {})
    },
  }
}
