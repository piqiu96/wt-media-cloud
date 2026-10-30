// A missing window is absent from the fresh BitBrowser snapshot by definition.
// Resolve its display facts from the Cloud mirror retained before confirmation.
export function missingProfilesFromScan(scan, persistedProfiles) {
  const byID = new Map((persistedProfiles || []).map(profile => [String(profile.id), profile]))
  const byBitID = new Map((persistedProfiles || []).map(profile => [profile.bit_profile_id, profile]))
  return (scan?.diff || [])
    .filter(item => item.kind === 'missing')
    .map(item => byID.get(String(item.profile_id)) || byBitID.get(item.bit_profile_id) || {
      id: item.profile_id,
      bit_profile_id: item.bit_profile_id,
      bit_name: '已缺失窗口',
    })
}

// A sync receipt describes this operation, while the profile list also contains
// older issues. Keep the result dialog limited to rows touched by this scan.
export function pendingProfilesFromSync(scan, persistedProfiles, hasIssue) {
  const affectedIDs = new Set((scan?.diff || []).map(item => String(item.profile_id || '')).filter(Boolean))
  const affectedBitIDs = new Set((scan?.diff || []).map(item => String(item.bit_profile_id || '')).filter(Boolean))
  return (persistedProfiles || []).filter(profile =>
    (affectedIDs.has(String(profile.id)) || affectedBitIDs.has(String(profile.bit_profile_id))) && hasIssue(profile))
}

// Historical missing records remain actionable, but are not new scan changes.
export function historicalMissingProfilesFromSync(scan, persistedProfiles) {
  const affectedIDs = new Set((scan?.diff || []).map(item => String(item.profile_id || '')).filter(Boolean))
  const affectedBitIDs = new Set((scan?.diff || []).map(item => String(item.bit_profile_id || '')).filter(Boolean))
  return (persistedProfiles || []).filter(profile => profile.local_state === 'local_missing'
    && !affectedIDs.has(String(profile.id)) && !affectedBitIDs.has(String(profile.bit_profile_id)))
}

// An empty selection shows both current and missing windows; archive is not an
// operator-facing window state. A missing snapshot is not proof of deletion.
export function windowStateMatches(profile, selection = '') {
  if (profile?.local_state !== 'active' && profile?.local_state !== 'local_missing') return false
  if (selection === 'found') return profile.local_state === 'active'
  if (selection === 'missing') return profile.local_state === 'local_missing'
  return true
}

const changeLabels = {
  bit_main_user_id: '比特身份', bit_user_id: '环境 ID', bit_name: '窗口名称',
  bit_seq: '排序', bit_group_id: '分组 ID', bit_group_name: '分组名称',
  bit_remark: '比特备注', bit_proxy_type: '代理协议',
  bit_proxy_host: '代理地址', bit_proxy_port: '代理端口',
}

export function profileChangedValues(scan, persistedProfiles, bitProfileId) {
  const diff = (scan?.diff || []).find(item => item.kind === 'changed' && item.bit_profile_id === bitProfileId)
  if (!diff) return []
  const before = (persistedProfiles || []).find(profile => profile.bit_profile_id === bitProfileId) || {}
  const after = (scan?.profiles || []).find(profile => profile.bit_profile_id === bitProfileId) || {}
  return (diff.fields || []).map(field => ({
    field,
    label: changeLabels[field] || field,
    before: String(before[field] ?? '').trim() || '未填写',
    after: String(after[field] ?? '').trim() || '未填写',
  }))
}

export function profileTeamName(profile, currentUser, roster = [], selfId = '', teams = []) {
  const team = teams.find(item => String(item.id) === String(profile.team_id))
  if (team?.name) return team.name
  if (String(profile.user_id) === String(selfId) && String(currentUser?.team_id) === String(profile.team_id) && currentUser?.team_name) return currentUser.team_name
  const member = roster.find(user => String(user.team_id ?? '') === String(profile.team_id ?? '') && user.team_name)
  if (member?.team_name) return member.team_name
  return profile.team_id == null ? '-' : `团队 ${profile.team_id}`
}
