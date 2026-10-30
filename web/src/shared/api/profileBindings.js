import { createApiClient } from './http.js'

const profileFields = [
  'bit_profile_id', 'bit_main_user_id', 'bit_user_id', 'bit_name', 'bit_seq',
  'bit_group_id', 'bit_group_name', 'bit_profile_status', 'bit_updated_at',
  'bit_remark', 'bit_proxy_type', 'bit_proxy_host', 'bit_proxy_port',
]

// Local Agent snapshots still carry readable local names. This is the single
// translation boundary into the Cloud contract's bit_* mirror fields.
function safeProfile(profile) {
  const wire = {
    bit_profile_id: profile.bit_profile_id,
    bit_main_user_id: profile.bit_main_user_id ?? profile.main_user_id,
    bit_user_id: profile.bit_user_id ?? profile.profile_user_id,
    bit_name: profile.bit_name ?? profile.name,
    bit_seq: profile.bit_seq ?? profile.seq,
    bit_group_id: profile.bit_group_id ?? profile.group_id,
    bit_group_name: profile.bit_group_name ?? profile.group_name,
    bit_profile_status: profile.bit_profile_status ?? profile.bit_status ?? profile.status,
    bit_updated_at: profile.bit_updated_at,
    bit_remark: profile.bit_remark ?? profile.remark,
    bit_proxy_type: profile.bit_proxy_type ?? profile.proxy_type,
    bit_proxy_host: profile.bit_proxy_host ?? profile.proxy_host,
    bit_proxy_port: profile.bit_proxy_port ?? profile.proxy_port,
  }
  if (wire.bit_profile_status !== undefined && wire.bit_profile_status !== null) wire.bit_profile_status = String(wire.bit_profile_status)
  return Object.fromEntries(profileFields.filter((key) => wire[key] !== undefined).map((key) => [key, wire[key]]))
}

export function createProfileBindingClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    submit(snapshot) {
      return api.post('/browser-profiles/sync', {
        bit_main_user_id: snapshot.bit_main_user_id ?? snapshot.main_user_id,
        observed_at: snapshot.observed_at,
        profiles: (snapshot.profiles || []).map(safeProfile),
      })
    },
    review() {
      return api.get('/browser-profiles/sync')
    },
    confirm(syncId, adoptBitProfileIds = []) {
      return api.post('/browser-profiles/sync/confirm', { sync_id: syncId, adopt_bit_profile_ids: adoptBitProfileIds })
    },
    reject() {
      return api.delete('/browser-profiles/sync')
    },
    listProfiles(filters = {}) {
      return api.get('/browser-profiles', filters)
    },
    updateProfile(id, changes) {
      return api.patch(`/browser-profiles/${id}`, changes)
    },
    deleteProfile(id) {
      return api.delete(`/browser-profiles/${id}`)
    },
  }
}
