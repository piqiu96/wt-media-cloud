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
      if (query?.businessStatus !== undefined) {
        query.business_status = query.businessStatus
        delete query.businessStatus
      }
      if (query?.loginStatus !== undefined) {
        query.login_status = query.loginStatus
        delete query.loginStatus
      }
      if (query?.allTags !== undefined) {
        query.all_tags = query.allTags
        delete query.allTags
      }
      if (query?.anyTags !== undefined) {
        query.any_tags = query.anyTags
        delete query.anyTags
      }
      if (query?.excludeTags !== undefined) {
        query.exclude_tags = query.excludeTags
        delete query.excludeTags
      }
      return api.get('/media-accounts', query).then((accounts) => Array.isArray(accounts) ? accounts.map(sanitize) : accounts)
    },
    create({ userId, gameId, platform, originalCookie, browserProfileId, remark, tags }) {
      const body = { game_id: gameId, platform }
      if (userId) body.user_id = userId
      if (originalCookie) body.original_cookie = originalCookie
      if (browserProfileId) body.browser_profile_id = browserProfileId
      if (remark) body.remark = remark
      if (tags?.length) body.tags = tags
      return api.post('/media-accounts', body).then(sanitize)
    },
    update(accountId, { businessStatus, loginStatus, remark }) {
      return api.patch(`/media-accounts/${accountId}`, {
        business_status: businessStatus,
        login_status: loginStatus,
        remark,
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
    unbindProfile(accountId) {
      return api.delete(`/media-accounts/${accountId}/profile`).then(sanitize)
    },
    check(accountId) {
      return api.post(`/media-accounts/${accountId}/check`, {})
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
