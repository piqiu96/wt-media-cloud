import { createApiClient } from './http.js'

const api = createApiClient()

export function createMediaAccountClient() {
  return {
    list(params) {
      return api.get('/media-accounts', params)
    },
    create({ userId, gameId, platform, originalCookie }) {
      const body = { game_id: gameId, platform }
      if (userId) body.user_id = userId
      if (originalCookie) body.original_cookie = originalCookie
      return api.post('/media-accounts', body)
    },
    update(accountId, { businessStatus, loginStatus }) {
      return api.patch(`/media-accounts/${accountId}`, {
        business_status: businessStatus,
        login_status: loginStatus,
      })
    },
    identify(accountId, { platformAccountId, name, avatarUrl, loginStatus }) {
      return api.post(`/media-accounts/${accountId}/identify`, {
        platform_account_id: platformAccountId,
        name,
        avatar_url: avatarUrl,
        login_status: loginStatus,
      })
    },
    bindProfile(accountId, browserProfileId) {
      return api.patch(`/media-accounts/${accountId}/profile`, { browser_profile_id: browserProfileId })
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
