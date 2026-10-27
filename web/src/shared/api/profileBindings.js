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
    // `observed_at` 是 Agent 实读 BitBrowser 的时刻，Cloud 用它判断这份 Profile
    // 列表是否还能证明本机现状。少了它，扫描会被当作过期事实拒绝。
    submit(snapshot) {
      return api.post('/bit-browser/profile-scans', {
        main_user_id: snapshot.main_user_id,
        observed_at: snapshot.observed_at,
        profiles: (snapshot.profiles || []).map(safeProfile),
      })
    },
    review(scanId) {
      return api.get(`/bit-browser/profile-scans/${scanId}`)
    },
    confirm(scanId, adoptBitProfileIds = []) {
      return api.post(`/bit-browser/profile-scans/${scanId}/confirm`, { adopt_bit_profile_ids: adoptBitProfileIds })
    },
    listProfiles(filters = {}) {
      return api.get('/browser-profiles', {
        user_id: filters.user_id,
        id: filters.id,
        name: filters.name,
        group: filters.group,
        bit_profile_id: filters.bit_profile_id,
        remark: filters.remark,
        local_status: filters.local_status,
        business_status: filters.business_status,
      })
    },
    createProfile(config) {
      return api.post('/browser-profiles', config)
    },
    openProfile(id) {
      return api.post(`/browser-profiles/${id}/open`, {})
    },
    closeProfile(id) {
      return api.post(`/browser-profiles/${id}/close`, {})
    },
    updateProfile(id, config) {
      return api.patch(`/browser-profiles/${id}`, config)
    },
    deleteProfile(id) {
      return api.delete(`/browser-profiles/${id}`)
    },
  }
}
