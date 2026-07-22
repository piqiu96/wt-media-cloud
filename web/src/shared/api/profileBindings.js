import { createApiClient } from './http.js'

const profileFields = ['bit_profile_id', 'main_user_id', 'profile_user_id', 'name', 'seq', 'group_id', 'group_name', 'bit_status', 'bit_updated_at', 'remark', 'proxy_type', 'proxy_host', 'proxy_port']

function safeProfile(profile) {
  return Object.fromEntries(profileFields.filter((key) => profile[key] !== undefined).map((key) => [key, profile[key]]))
}

export function createProfileBindingClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    submit(snapshot) {
      return api.post('/bit-browser/profile-scans', {
        main_user_id: snapshot.main_user_id,
        profiles: (snapshot.profiles || []).map(safeProfile),
      })
    },
    review(scanId) {
      return api.get(`/bit-browser/profile-scans/${scanId}`)
    },
    confirm(scanId) {
      return api.post(`/bit-browser/profile-scans/${scanId}/confirm`, {})
    },
    confirmMainIdentity(scanId) {
      return api.post(`/bit-browser/profile-scans/${scanId}/confirm-main-identity`, {})
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
    deleteProfile(id) {
      return api.delete(`/browser-profiles/${id}`)
    },
  }
}
