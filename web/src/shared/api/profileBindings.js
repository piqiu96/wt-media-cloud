import { createApiClient } from './http.js'

const profileFields = ['bit_profile_id', 'main_user_id', 'profile_user_id', 'name', 'seq', 'group_id', 'group_name', 'bit_status', 'bit_updated_at', 'remark', 'proxy_type', 'proxy_host', 'proxy_port']

function safeProfile(profile) {
  const normalized = { ...profile }
  if (normalized.bit_status === undefined && normalized.status !== undefined) {
    normalized.bit_status = String(normalized.status)
  }
  return Object.fromEntries(profileFields.filter((key) => normalized[key] !== undefined).map((key) => [key, normalized[key]]))
}

export function createProfileBindingClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    submit(snapshot, options = {}) {
      return api.post('/bit-browser/profile-scans', {
        main_user_id: snapshot.main_user_id,
        node_id: options.nodeId,
        profiles: (snapshot.profiles || []).map(safeProfile),
      })
    },
    review(scanId) {
      return api.get(`/bit-browser/profile-scans/${scanId}`)
    },
    confirm(scanId, options = {}) {
      return api.post(`/bit-browser/profile-scans/${scanId}/confirm`, { node_id: options.nodeId })
    },
    confirmMainIdentity(scanId) {
      return api.post(`/bit-browser/profile-scans/${scanId}/confirm-main-identity`, {})
    },
    // 「比特账号绑定」自助入口（CHG-20261002-074 阶段 3）：只有显式确认才带 `overwrite`，
    // 缺省（扫描确认等既有路径）不发送，服务端照旧把「账号不一致」当作 23002 拒绝。
    confirmMainIdentityDirect(mainUserId, options = {}) {
      const body = { main_user_id: mainUserId }
      if (options.overwrite) body.overwrite = true
      return api.post('/bit-browser/main-identity', body)
    },
    listProfiles() {
      return api.get('/browser-profiles')
    },
    createProfile(config, options = {}) {
      return api.post('/browser-profiles', { ...config, node_id: options.nodeId })
    },
    openProfile(id, options = {}) {
      return api.post(`/browser-profiles/${id}/open`, { node_id: options.nodeId })
    },
    closeProfile(id, options = {}) {
      return api.post(`/browser-profiles/${id}/close`, { node_id: options.nodeId })
    },
    updateProfile(id, config, options = {}) {
      return api.patch(`/browser-profiles/${id}`, { ...config, node_id: options.nodeId })
    },
    assignProfileOwner(id, userId) {
      return api.post(`/browser-profiles/${id}/assign-owner`, { user_id: Number(userId) })
    },
    deleteProfile(id) {
      return api.delete(`/browser-profiles/${id}`)
    },
  }
}
