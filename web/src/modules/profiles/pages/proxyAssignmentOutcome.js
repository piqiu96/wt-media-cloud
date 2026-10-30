// Address observations do not prove the write-only authentication fields.
export function classifyObservedAssignment(observed, target) {
  if (!observed || !target) return 'different'
  const matches = String(observed.proxy_type || '').toLowerCase() === String(target.protocol || '').toLowerCase()
    && String(observed.proxy_host || '') === String(target.host || '')
    && Number(observed.proxy_port || 0) === Number(target.port || 0)
  return matches ? 'matches' : 'different'
}
