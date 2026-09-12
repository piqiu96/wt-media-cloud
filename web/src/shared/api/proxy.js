import { createApiClient } from './http.js'

export function createProxyClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    list(params) {
      return api.get('/proxies', params)
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
    updateStatus(id, businessStatus) {
      return api.patch(`/proxies/${id}/status`, { business_status: businessStatus })
    },
    delete(id) {
      return api.delete(`/proxies/${id}`)
    },
    bulkImport(lines) {
      return api.post('/proxies/import', { lines })
    },
    previewImport(lines) {
      return api.post('/proxies/import/preview', { lines })
    },
    parseAddress(proxyAddress) {
      return api.post('/proxies/parse', { proxy_address: proxyAddress })
    },
    previewExtract(data) {
      return api.post('/proxies/extract-preview', data)
    },
    listBindings(id) {
      return api.get(`/proxies/${id}/bindings`)
    },
    refreshDynamic(id) {
      return api.post(`/proxies/${id}/refresh`)
    },
    check(id) {
      return api.post(`/proxies/${id}/check`)
    },
    backgroundCheck(id) {
      return api.post(`/proxies/${id}/check/background`)
    },
    setMaxProfileCount(proxyId, maxProfiles) {
      return api.post(`/proxies/${proxyId}/quota`, { max_profiles: maxProfiles })
    },
    assign(proxyId, profileId) {
      return api.post(`/proxies/${proxyId}/assign`, { profile_id: profileId })
    },
    recommend(profileIds) {
      return api.get('/proxies/recommendations', { profile_ids: profileIds.join(',') })
    },
    assignBatch(proxyId, profileIds) {
      return api.post(`/proxies/${proxyId}/assign-batch`, { profile_ids: profileIds })
    },
    unbind(proxyId, profileId) {
      return api.post(`/proxies/${proxyId}/unbind`, { profile_id: profileId })
    },
    previewLocalScan(scanId) {
      return api.post('/proxies/local-scan/preview', { scan_id: scanId })
    },
    confirmLocalScan(scanId, nodeId) {
      return api.post('/proxies/local-scan/confirm', { scan_id: scanId, node_id: nodeId })
    },
  }
}
