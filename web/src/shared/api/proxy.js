import { createApiClient } from './http.js'

export function createProxyClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    async list(params = {}) {
      const size = 200
      const result = []
      const seen = new Set()
      for (let offset = 0; offset <= 10000; offset += size) {
        const page = await api.get('/proxies', { ...params, limit: size, offset })
        const rows = Array.isArray(page) ? page : page?.list
        if (!Array.isArray(rows)) throw new Error('代理列表返回格式错误')
        for (const row of rows) {
          if (seen.has(row.id)) throw new Error('代理分页返回了重复记录，请更新服务后重试')
          seen.add(row.id)
          result.push(row)
        }
        if (rows.length < size) return result
      }
      throw new Error('代理列表超过 10000 条，请缩小筛选范围')
    },
    get(id) {
      return api.get(`/proxies/${id}`)
    },
    create(data) {
      return api.post('/proxies', data)
    },
    update(id, data) {
      return api.patch(`/proxies/${id}`, data)
    },
    delete(id) {
      return api.delete(`/proxies/${id}`)
    },
    bulkImport(teamId, rows) {
      return api.post('/proxies/import', { team_id: teamId, rows })
    },
    previewImport(teamId, lines) {
      return api.post('/proxies/import/preview', { team_id: teamId, lines })
    },
    parseAddress(proxyAddress) {
      return api.post('/proxies/parse', { proxy_address: proxyAddress })
    },
    extractionConfig(id) {
      return api.get(`/proxies/${id}/extraction-config`)
    },
    listBindings(id) {
      return api.get(`/proxies/${id}/bindings`)
    },
    refreshDynamic(id, data) {
      return api.post(`/proxies/${id}/refresh`, data)
    },
    check(id) {
      return api.post(`/proxies/${id}/check`)
    },
    updateMaxProfiles(proxyId, maxProfiles) {
      return api.patch(`/proxies/${proxyId}`, { max_profiles: maxProfiles })
    },
    recommend(profileIds) {
      return api.get('/proxies/recommendations', { profile_ids: profileIds.join(',') })
    },
    operate(profileId, { action, proxy_id }) {
      return api.post(`/browser-profiles/${profileId}/proxy-operation`, { action, proxy_id })
    },
    currentOperation(profileId) { return api.get(`/browser-profiles/${profileId}/proxy-operation`) },
    finishOperation(profileId, { operation_id, action, proxy_id, outcome, message }) {
      return api.post(`/browser-profiles/${profileId}/proxy-operation/result`, { operation_id, action, proxy_id, outcome, message })
    },
  }
}
