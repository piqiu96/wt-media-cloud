import { createApiClient } from './http.js'

// 合成策略 API（CHG-20261012-079）。七个命令的路径与信封由
// contracts/cloud-api/v1/content-production.openapi.yaml 冻结；expected_version
// 与 name 由调用方显式给出，客户端不代取、不代填。
export function createComposeClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    listTemplates() { return api.get('/compose-templates') },
    listStrategies() { return api.get('/compose-strategies') },
    getStrategy(id) { return api.get(`/compose-strategies/${id}`) },
    createStrategy(strategy) { return api.post('/compose-strategies', strategy) },
    updateStrategy(id, expectedVersion, strategy) {
      return api.put(`/compose-strategies/${id}`, { expected_version: expectedVersion, strategy })
    },
    copyStrategy(id, name) { return api.post(`/compose-strategies/${id}/copy`, { name }) },
    listVersions(id) { return api.get(`/compose-strategies/${id}/versions`) },
    listTeams() { return api.get('/operation-teams') },
    listGames() { return api.get('/games') },
  }
}
