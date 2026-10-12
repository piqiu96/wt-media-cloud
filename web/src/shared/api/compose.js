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
  }
}
