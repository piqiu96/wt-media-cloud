// Address observations do not prove the write-only authentication fields.
export function classifyObservedAssignment(observed, target) {
  if (!observed || !target) return 'unconfirmed'
  const matches = String(observed.proxy_type || '').toLowerCase() === String(target.proxy_protocol || '').toLowerCase()
    && String(observed.proxy_host || '') === String(target.host || '')
    && Number(observed.proxy_port || 0) === Number(target.port || 0)
  return matches ? 'unconfirmed' : 'different'
}

export function isUncertainAssignmentError(error) {
  return error instanceof TypeError || [30008, 50000].includes(error?.errcode)
    || error?.code === '30008' || error?.code === 'RESULT_UNKNOWN'
    || /timeout|timed out|超时|服务器响应格式错误|服务器返回格式错误/i.test(String(error?.message || ''))
}
