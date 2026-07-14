function publicAccount(account) {
  if (!account || typeof account !== 'object') return account
  const { original_cookie, active_cookie, ...safe } = account
  return safe
}

export function createMediaAccountClient({ fetch = globalThis.fetch } = {}) {
  async function read(response) {
    const payload = await response.json()
    if (!response.ok) {
      const error = new Error(payload?.error?.message || '媒体账号请求失败')
      error.code = payload?.error?.code || 'request_failed'
      throw error
    }
    if (Array.isArray(payload.data)) return payload.data.map(publicAccount)
    return publicAccount(payload.data)
  }

  async function write(path, body, method = 'POST') {
    return read(await fetch(path, {
      method,
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }))
  }

  return {
    async list({ userId, gameId, platform, anyTags, allTags, excludeTags } = {}) {
      const query = new URLSearchParams()
      if (platform) query.set('platform', platform)
      if (userId) query.set('user_id', userId)
      if (gameId) query.set('game_id', gameId)
      if (anyTags?.length) query.set('any_tags', anyTags.join(','))
      if (allTags?.length) query.set('all_tags', allTags.join(','))
      if (excludeTags?.length) query.set('exclude_tags', excludeTags.join(','))
      const suffix = query.size ? `?${query}` : ''
      return read(await fetch(`/api/v1/media-accounts${suffix}`, { credentials: 'include' }))
    },

    async create({ userId, gameId, platform, originalCookie }) {
      const body = { game_id: gameId, platform }
      if (userId) body.user_id = userId
      if (originalCookie) body.original_cookie = originalCookie
      return write('/api/v1/media-accounts', body)
    },

    async update(accountId, { businessStatus, loginStatus }) {
      return write(`/api/v1/media-accounts/${accountId}`, {
        business_status: businessStatus,
        login_status: loginStatus,
      }, 'PATCH')
    },

    async identify(accountId, { platformAccountId, name, avatarUrl, loginStatus }) {
      return write(`/api/v1/media-accounts/${accountId}/identify`, {
        platform_account_id: platformAccountId,
        name,
        avatar_url: avatarUrl,
        login_status: loginStatus,
      })
    },

    async addTags(accountIds, tags) {
      return write('/api/v1/media-accounts/tags/add', { account_ids: accountIds, tags })
    },

    async removeTags(accountIds, tags) {
      return write('/api/v1/media-accounts/tags/remove', { account_ids: accountIds, tags })
    },
  }
}
