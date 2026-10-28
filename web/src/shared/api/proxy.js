import { createApiClient } from './http.js'

function normalizeProxy(proxy) {
  if (!proxy || typeof proxy !== 'object') return proxy
  return {
    ...proxy,
    business_status: proxy.enabled === true ? 'active' : proxy.enabled === false ? 'paused' : proxy.business_status,
    proxy_protocol: proxy.protocol ?? proxy.proxy_protocol,
    max_profile_count: proxy.max_profiles ?? proxy.max_profile_count,
  }
}

function normalizeProxies(rows) {
  return Array.isArray(rows) ? rows.map(normalizeProxy) : rows
}

export function createProxyClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    list(params) {
      return api.get('/proxies', params).then(normalizeProxies)
    },
    get(id) {
      return api.get(`/proxies/${id}`).then(normalizeProxy)
    },
    create(data) {
      return api.post('/proxies', data).then(normalizeProxy)
    },
    update(id, data) {
      return api.patch(`/proxies/${id}`, data).then(normalizeProxy)
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
    previewExtract(data) {
      return api.post('/proxies/extract-preview', data)
    },
    listBindings(id) {
      return api.get(`/proxies/${id}/bindings`)
    },
    refreshDynamic(id) {
      return api.post(`/proxies/${id}/refresh`).then(normalizeProxy)
    },
    check(id) {
      return api.post(`/proxies/${id}/check`).then(normalizeProxy)
    },
    setMaxProfileCount(proxyId, maxProfiles) {
      return api.patch(`/proxies/${proxyId}`, { max_profiles: maxProfiles }).then(normalizeProxy)
    },
    recommend(profileIds) {
      return api.get('/proxies/recommendations', { profile_ids: profileIds.join(',') }).then(normalizeProxies)
    },
    operate(profileId, { action, proxy_id }) {
      return api.post(`/browser-profiles/${profileId}/proxy-operation`, { action, proxy_id })
    },
    finishOperation(profileId, { action, proxy_id, outcome, message }) {
      return api.post(`/browser-profiles/${profileId}/proxy-operation/result`, { action, proxy_id, outcome, message })
    },
  }
}
