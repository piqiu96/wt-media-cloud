import { createApiClient } from './http.js'

const api = createApiClient()

const profileFields = ['bit_profile_id', 'main_user_id', 'profile_user_id', 'name', 'seq', 'group_id', 'group_name', 'bit_status', 'bit_updated_at', 'remark', 'proxy_type', 'proxy_host', 'proxy_port']

function safeProfile(profile) {
  return Object.fromEntries(profileFields.filter((key) => profile[key] !== undefined).map((key) => [key, profile[key]]))
}

export function createProfileBindingClient() {
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
    listProfiles() {
      return api.get('/browser-profiles')
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
