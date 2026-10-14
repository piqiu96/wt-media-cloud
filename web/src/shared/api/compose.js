import { createApiClient } from './http.js'
import { isDesktop } from '../../utils.js'

export function createComposeClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    listTemplates() { return api.get('/compose-templates') },
    listStrategies() { return api.get('/compose-strategies') },
    // 管理员创建策略时要选目标团队；高级运营的团队由服务端固定。
    listTeams() { return api.get('/operation-teams') },
    // 游戏列表用于展示映射和创建表单下拉，与素材/内容池页共用同一端点。
    listGames() { return api.get('/games') },
    getStrategy(id) { return api.get(`/compose-strategies/${id}`) },
    createStrategy(data) { return api.post('/compose-strategies', data) },
    updateStrategy(id, data) { return api.put(`/compose-strategies/${id}`, data) },
    copyStrategy(id, name) { return api.post(`/compose-strategies/${id}/copy`, { name }) },
    setStrategyStatus(id, status) { return api.patch(`/compose-strategies/${id}/status`, { status }) },
    setStrategySchedule(id, schedule) { return api.put(`/compose-strategies/${id}/schedule`, schedule) },
    runStrategyNow(id, requestKey) { return api.post(`/compose-strategies/${id}/run-now`, { request_key: requestKey }) },
    listStrategyRuns(id) { return api.get(`/compose-strategies/${id}/runs`) },
    // 批次结果归集（CHG-20261015-095）：按运行记录的计划时间取该批次逐素材任务
    // 与状态统计；schedule_slot 原样回传运行记录里的时间串。
    listStrategyRunTasks(id, scheduleSlot) { return api.get(`/compose-strategies/${id}/runs/tasks`, { schedule_slot: scheduleSlot }) },
    // 合成任务（CHG-20261013-082）：列表筛选（status / compose_strategy_id）由服务端读 query。
    createComposeTasks(data) { return api.post('/compose-tasks', data) },
    listComposeTasks(params) { return api.get('/compose-tasks', params) },
    getComposeTask(id) { return api.get(`/compose-tasks/${id}`) },
    cancelComposeTask(id) { return api.post(`/compose-tasks/${id}/cancel`) },
    // 执行记录与人工重试（CHG-20261013-089）：attempts 沿任务 scope 读，retry 只认
    // failed 终态；非重试错误码由服务端 409 compose_task_not_retryable 拒绝。
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
    // Desktop 前置（本机 Agent 必须可信）。挂在 compose client 上是因为路由在
    // /compose-outputs 下；下一次诚实的进度来源同样是 GET /file-transfer-tasks。
    async createOutputDownload(id) {
      if (isDesktop()) {
        const { ensureTrustedLocalAgent } = await import('../../apps/desktop/features/local-agent/init.js')
        await ensureTrustedLocalAgent()
      }
      return api.post(`/compose-outputs/${id}/downloads`, {})
    },
  }
}
