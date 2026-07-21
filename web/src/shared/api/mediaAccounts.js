import { createApiClient } from './http.js'

export function createMediaAccountClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  const sanitize = (account) => {
    if (!account || typeof account !== 'object') return account
    const { original_cookie: _originalCookie, active_cookie: _activeCookie, ...safe } = account
    return safe
  }
  return {
    list(params) {
      const query = params ? { ...params } : undefined
      if (query?.allTags !== undefined) {
        query.all_tags = query.allTags
        delete query.allTags
      }
      return api.get('/media-accounts', query).then((accounts) => Array.isArray(accounts) ? accounts.map(sanitize) : accounts)
    },
    create({ userId, gameId, platform, originalCookie }) {
      const body = { game_id: gameId, platform }
      if (userId) body.user_id = userId
      if (originalCookie) body.original_cookie = originalCookie
      return api.post('/media-accounts', body).then(sanitize)
    },
    update(accountId, { businessStatus, loginStatus }) {
      return api.patch(`/media-accounts/${accountId}`, {
        business_status: businessStatus,
        login_status: loginStatus,
      }).then(sanitize)
    },
    identify(accountId, { platformAccountId, name, avatarUrl, loginStatus }) {
      return api.post(`/media-accounts/${accountId}/identify`, {
        platform_account_id: platformAccountId,
        name,
        avatar_url: avatarUrl,
        login_status: loginStatus,
      }).then(sanitize)
    },
    bindProfile(accountId, browserProfileId) {
      return api.patch(`/media-accounts/${accountId}/profile`, { browser_profile_id: browserProfileId }).then(sanitize)
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
  }
}
