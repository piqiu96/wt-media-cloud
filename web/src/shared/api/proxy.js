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
    listBindings(id) {
      return api.get(`/proxies/${id}/bindings`)
    },
    refreshDynamic(id) {
      return api.post(`/proxies/${id}/refresh`)
    },
    finishDynamicRefresh(id, extracted) {
      return api.post(`/proxies/${id}/refresh/result`, extracted)
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
    finishOperation(profileId, { action, proxy_id, outcome, message }) {
      return api.post(`/browser-profiles/${profileId}/proxy-operation/result`, { action, proxy_id, outcome, message })
    },
  }
}
