import { createApiClient } from './http.js'

export function createMediaAccountClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  const sanitize = (account) => {
    if (!account || typeof account !== 'object') return account
    const { original_cookie: _originalCookie, active_cookie: _activeCookie, before_cookie: _beforeCookie, cookie_update: _cookieUpdate, ...safe } = account
    return safe
  }
  return {
    listPage(params = {}) { return api.get('/media-accounts', params).then(value => ({...value,items:value.items.map(sanitize)})) },
    async list(params = {}) {
      const items = []
      let page = 1, total = 0
      do {
        const result = await api.get('/media-accounts', {...params,page,page_size:100})
        items.push(...result.items.map(sanitize)); total = result.total; page += 1
        if (!result.items.length) break
      } while (items.length < total)
      return items
    },
    create(input) {
      return api.post('/media-accounts', input).then(sanitize)
    },
    update(accountId, changes) {
      return api.patch(`/media-accounts/${accountId}`, changes).then(sanitize)
    },
    bindProfile(accountId, browserProfileId) {
      return api.patch(`/media-accounts/${accountId}/profile`, { browser_profile_id: browserProfileId }).then(sanitize)
    },
    unbindProfile(accountId) {
      return api.delete(`/media-accounts/${accountId}/profile`).then(sanitize)
    },
    check(accountId) {
      return api.post(`/media-accounts/${accountId}/check`, {})
    },
    submitCheckResult(accountId, input) {
      return api.post(`/media-accounts/${accountId}/check/result`, { ...input, check_items: input.check_items ?? [] }).then(sanitize)
    },
    get(id) { return api.get(`/media-accounts/${id}`).then(sanitize) },
    credentials(id) { return api.get(`/media-accounts/${id}/credentials`) },
    startCookieUpdate(id,input) { return api.post(`/media-accounts/${id}/cookie-updates`,input) },
    snapshotCookieUpdate(id,op,input) { return api.put(`/media-accounts/${id}/cookie-updates/${op}/snapshot`,input) },
    settleCookieUpdate(id,op,input) { return api.put(`/media-accounts/${id}/cookie-updates/${op}/result`,input) },
    getCookieUpdate(id,op) { return api.get(`/media-accounts/${id}/cookie-updates/${op}`) },
  }
}
