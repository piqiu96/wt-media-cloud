import { createApiClient } from './http.js'

const profileFields = [
  'bit_profile_id', 'bit_main_user_id', 'bit_user_id', 'bit_name', 'bit_seq',
  'bit_group_id', 'bit_group_name', 'bit_profile_status', 'bit_updated_at',
  'bit_remark', 'bit_proxy_type', 'bit_proxy_host', 'bit_proxy_port',
]

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

function normalizeProfile(profile) {
  if (!profile || typeof profile !== 'object') return profile
  return {
    ...profile,
    main_user_id: profile.bit_main_user_id ?? profile.main_user_id ?? '',
    profile_user_id: profile.bit_user_id ?? profile.profile_user_id ?? '',
    name: profile.bit_name ?? profile.name ?? '',
    seq: profile.bit_seq ?? profile.seq ?? 0,
    group_id: profile.bit_group_id ?? profile.group_id ?? '',
    group_name: profile.bit_group_name ?? profile.group_name ?? '',
    bit_status: profile.bit_profile_status ?? profile.bit_status ?? '',
    proxy_type: profile.bit_proxy_type ?? profile.proxy_type ?? '',
    proxy_host: profile.bit_proxy_host ?? profile.proxy_host ?? '',
    proxy_port: profile.bit_proxy_port ?? profile.proxy_port ?? 0,
    remark: profile.bit_remark ?? profile.remark ?? '',
    cloud_remark: profile.remark ?? profile.cloud_remark ?? '',
    business_status: profile.enabled === true ? 'enabled' : profile.enabled === false ? 'disabled' : profile.business_status,
    local_status: profile.local_state ?? profile.local_status ?? '',
  }
}

function normalizeSync(sync) {
  if (!sync || typeof sync !== 'object') return sync
  return {
    ...sync,
    main_user_id: sync.bit_main_user_id ?? sync.main_user_id ?? '',
    profiles: Array.isArray(sync.profiles) ? sync.profiles.map(normalizeProfile) : [],
  }
}

export function createProfileBindingClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    submit(snapshot) {
      return api.post('/browser-profiles/sync', {
        bit_main_user_id: snapshot.bit_main_user_id ?? snapshot.main_user_id,
        observed_at: snapshot.observed_at,
        profiles: (snapshot.profiles || []).map(safeProfile),
      }).then(normalizeSync)
    },
    review() {
      return api.get('/browser-profiles/sync').then(normalizeSync)
    },
    confirm(_scanId, adoptBitProfileIds = []) {
      return api.post('/browser-profiles/sync/confirm', { adopt_bit_profile_ids: adoptBitProfileIds }).then(normalizeSync)
    },
    reject() {
      return api.delete('/browser-profiles/sync')
    },
    listProfiles(filters = {}) {
      return api.get('/browser-profiles', {
        user_id: filters.user_id,
        id: filters.id,
        bit_name: filters.name ?? filters.bit_name,
        bit_group: filters.group ?? filters.bit_group,
        bit_profile_id: filters.bit_profile_id,
        remark: filters.remark,
        local_state: filters.local_status ?? filters.local_state,
        enabled: filters.business_status ? filters.business_status === 'enabled' : filters.enabled,
      }).then((rows) => Array.isArray(rows) ? rows.map(normalizeProfile) : rows)
    },
    updateProfile(id, config) {
      const body = {}
      if (config.cloud_remark !== undefined) body.remark = config.cloud_remark
      if (config.business_status !== undefined) body.enabled = config.business_status === 'enabled'
      return api.patch(`/browser-profiles/${id}`, body).then(normalizeProfile)
    },
    deleteProfile(id) {
      return api.delete(`/browser-profiles/${id}`)
    },
  }
}
