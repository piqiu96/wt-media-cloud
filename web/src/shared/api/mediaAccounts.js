import { createApiClient } from './http.js'

export function createMediaAccountClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  const sanitize = (account) => {
    if (!account || typeof account !== 'object') return account
    const { original_cookie: _originalCookie, active_cookie: _activeCookie, ...safe } = account
    return {
      ...safe,
      name: safe.platform_name ?? safe.name,
      avatar_url: safe.platform_avatar_url ?? safe.avatar_url,
      business_status: safe.enabled === true ? 'enabled' : safe.enabled === false ? 'disabled' : safe.business_status,
      login_status: safe.platform_login_status ?? safe.login_status,
      last_checked_at: safe.last_check_at ?? safe.last_checked_at,
      check_items: safe.last_check_items ?? safe.check_items,
      active_cookie_updated_at: safe.cookie_updated_at ?? safe.active_cookie_updated_at,
    }
  }
  return {
    list(params) {
      const query = params ? { ...params } : undefined
      if (query?.businessStatus !== undefined) {
        query.enabled = query.businessStatus === 'enabled'
        delete query.businessStatus
      }
      if (query?.loginStatus !== undefined) {
        query.platform_login_status = query.loginStatus
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
      if (query?.profile !== undefined) {
        query.profile_search = query.profile
        delete query.profile
      }
      if (query?.categoryIds !== undefined) {
        query.category_ids = query.categoryIds
        delete query.categoryIds
      }
      return api.get('/media-accounts', query).then((accounts) => Array.isArray(accounts) ? accounts.map(sanitize) : accounts)
    },
    create({ userId, categoryIds, name, platform, originalCookie, browserProfileId, remark, tags }) {
      const body = {}
      if (categoryIds !== undefined) body.category_ids = categoryIds
      body.platform = platform
      if (userId) body.user_id = userId
      if (name) body.platform_name = name
      if (originalCookie) body.original_cookie = originalCookie
      if (browserProfileId) body.browser_profile_id = browserProfileId
      if (remark) body.remark = remark
      if (tags?.length) body.tags = tags
      return api.post('/media-accounts', body).then(sanitize)
    },
    update(accountId, { businessStatus, loginStatus, remark, categoryIds, name }) {
      const body = {
        remark,
      }
      if (businessStatus !== undefined) body.enabled = businessStatus === 'enabled'
      if (loginStatus !== undefined) body.platform_login_status = loginStatus
      if (name !== undefined) body.platform_name = name
      if (categoryIds !== undefined) body.category_ids = categoryIds
      return api.patch(`/media-accounts/${accountId}`, body).then(sanitize)
    },
    identify(accountId, { platformAccountId, name, avatarUrl, loginStatus }) {
      return api.post(`/media-accounts/${accountId}/identify`, {
        platform_account_id: platformAccountId,
        platform_name: name,
        platform_avatar_url: avatarUrl,
        platform_login_status: loginStatus,
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
    submitCheckResult(accountId, { taskId, platformAccountId, name, avatarUrl, loginStatus, message, checkItems = [] }) {
      return api.post(`/media-accounts/${accountId}/check/result`, {
        operation_id: taskId,
        platform_account_id: platformAccountId,
        platform_name: name,
        platform_avatar_url: avatarUrl,
        platform_login_status: loginStatus,
        message,
        check_items: checkItems,
      }).then(sanitize)
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
    submitCookieReadResult(accountId, { taskId, cookies }) {
      return api.post(`/media-accounts/${accountId}/cookies/read-sync/result`, {
        operation_id: taskId,
        cookies,
      }).then(sanitize)
    },
    createAccountGroup({ name, filters }) {
      return api.post('/account-groups', { name, filters }).then(sanitize)
    },
    listAccountGroups() {
      return api.get('/account-groups')
    },
    listAccountsByGroup(groupId) {
      return api.get(`/account-groups/${groupId}/accounts`)
    },
    updateAccountGroup(groupId, { name, filters }) {
      return api.patch(`/account-groups/${groupId}`, { name, filters }).then(sanitize)
    },
    deleteAccountGroup(groupId) {
      return api.delete(`/account-groups/${groupId}`)
    },
  }
}
