import { createApiClient } from './http.js'

const api = createApiClient()

export function createProxyClient() {
  return {
    list(params) {
      return api.get('/proxies', params)
    },
    create(input) {
      return api.post('/proxies', input)
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
    check(id) {
      return api.post(`/proxies/${id}/check`)
    },
    backgroundCheck(id) {
      return api.post(`/proxies/${id}/check/background`)
    },
    setMaxProfileCount(proxyId, maxProfiles) {
      return api.post(`/proxies/${proxyId}/quota`, { max_profiles: maxProfiles })
    },
  }
}
