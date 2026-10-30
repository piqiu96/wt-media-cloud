export function prepareFailedRetry(items, ids) {
  const selected = new Set(ids)
  return items.map(item => selected.has(item.key) && item.status === 'failed'
    ? { ...item, status: 'pending', reason: '' }
    : item)
}

export function runningObservationStatus(actual, target) {
  if (actual === undefined) return 'unconfirmed'
  return actual === target ? 'success' : 'failed'
}

export function canSelectWindowForBatch(profile, isOwnDesktop, runningKnown) {
  return Boolean(isOwnDesktop && profile?.local_state === 'active' && profile.enabled === true
    && !profile.reserved_proxy_id && runningKnown)
}
