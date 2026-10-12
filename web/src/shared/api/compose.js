import { createApiClient } from './http.js'

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
    // 合成任务（CHG-20261013-082）：列表筛选（status / compose_strategy_id）由服务端读 query。
    createComposeTasks(data) { return api.post('/compose-tasks', data) },
    listComposeTasks(params) { return api.get('/compose-tasks', params) },
    getComposeTask(id) { return api.get(`/compose-tasks/${id}`) },
    cancelComposeTask(id) { return api.post(`/compose-tasks/${id}/cancel`) },
    // 我的成片（CHG-20261013-084）：publish_status 筛选由服务端读 query；video-url
    // 是短时预签名，每次预览现取，客户端不做缓存。
    listComposeOutputs(params) { return api.get('/compose-outputs', params) },
    getComposeOutput(id) { return api.get(`/compose-outputs/${id}`) },
    archiveComposeOutput(id) { return api.post(`/compose-outputs/${id}/archive`) },
    getComposeOutputVideoUrl(id) { return api.get(`/compose-outputs/${id}/video-url`) },
  }
}
