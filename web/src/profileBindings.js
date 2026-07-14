const profileFields = ['bit_profile_id', 'owner_user_id', 'name', 'seq', 'group_id', 'group_name', 'bit_status', 'bit_updated_at']

function safeProfile(profile) {
  return Object.fromEntries(profileFields.filter((key) => profile[key] !== undefined).map((key) => [key, profile[key]]))
}

export function createProfileBindingClient({ fetch = globalThis.fetch } = {}) {
  async function read(response) {
    response = await response
    const payload = await response.json()
    if (!response.ok) {
      const error = new Error(payload?.error?.message || '浏览器环境请求失败')
      error.code = payload?.error?.code || 'request_failed'
      throw error
    }
    return payload.data
  }

  async function write(path, body) {
    return read(await fetch(path, {
      method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body),
    }))
  }

  return {
    submit(snapshot) {
      return write('/api/v1/bit-browser/profile-scans', {
        owner_user_id: snapshot.owner_user_id,
        profiles: (snapshot.profiles || []).map(safeProfile),
      })
    },
    review(scanId) {
      return read(fetch(`/api/v1/bit-browser/profile-scans/${scanId}`, { credentials: 'include' }))
    },
    confirm(scanId) {
      return write(`/api/v1/bit-browser/profile-scans/${scanId}/confirm`, {})
    },
    listProfiles() {
      return read(fetch('/api/v1/browser-profiles', { credentials: 'include' }))
    },
  }
}
