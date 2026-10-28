import { createApiClient } from './http.js'

export function createMediaAccountClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  const sanitize = (account) => {
    if (!account || typeof account !== 'object') return account
    const { original_cookie: _originalCookie, active_cookie: _activeCookie, ...safe } = account
    return safe
  }
  const listResult = (value) => Array.isArray(value) ? value.map(sanitize) : sanitize(value)
  return {
    list(params) {
      return api.get('/media-accounts', params).then(listResult)
    },
    create(input) {
      return api.post('/media-accounts', input).then(sanitize)
    },
    update(accountId, changes) {
      return api.patch(`/media-accounts/${accountId}`, changes).then(sanitize)
    },
    identify(accountId, input) {
      return api.post(`/media-accounts/${accountId}/identify`, input).then(sanitize)
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
    addTags(accountIds, tags) {
      return api.post('/media-accounts/tags/add', { account_ids: accountIds, tags })
    },
    removeTags(accountIds, tags) {
      return api.post('/media-accounts/tags/remove', { account_ids: accountIds, tags })
    },
    fetchCookies(accountId) {
      return api.get(`/media-accounts/${accountId}/cookies`)
    },
    startCookieReadSync(accountId) {
      return api.post(`/media-accounts/${accountId}/cookies/read-sync`, {})
    },
    submitCookieReadResult(accountId, input) {
      return api.post(`/media-accounts/${accountId}/cookies/read-sync/result`, input).then(sanitize)
    },
    createAccountGroup(input) {
      return api.post('/account-groups', input).then(sanitize)
    },
    listAccountGroups() {
      return api.get('/account-groups')
    },
    listAccountsByGroup(groupId) {
      return api.get(`/account-groups/${groupId}/accounts`)
    },
    updateAccountGroup(groupId, changes) {
      return api.patch(`/account-groups/${groupId}`, changes).then(sanitize)
    },
    deleteAccountGroup(groupId) {
      return api.delete(`/account-groups/${groupId}`)
    },
  }
}
