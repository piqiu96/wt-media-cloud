export function importItemPresentation(item, running = false) {
  if (item.status === 'skipped') return { label: '已跳过', tone: 'neutral', message: '本行暂不接入，可恢复处理。' }
  if (item.status === 'succeeded' && item.same_identity_account_id) return { label: '已存在', tone: 'success', message: '账号已存在，本窗口已接入。' }
  if (item.status === 'failed' && item.step === 'manual') return { label: '待人工登录', tone: 'warning', message: '请打开窗口完成人工登录，再检查并同步。' }
  if (item.status === 'uncertain' || item.status === 'running' && !running) return { label: '待确认', tone: 'warning', message: '请先核实本行结果，再继续接入。' }
  const labels = { pending: '待处理', running: '执行中', succeeded: '已接入', failed: '未完成' }
  const tone = { running: 'info', succeeded: 'success', failed: 'danger' }[item.status] || 'neutral'
  const fallback = item.creation_unavailable ? '无法新建窗口，可选择空闲窗口或稍后重试。' : item.step === 'blocked' ? '当前窗口不可用于接入，请使用新窗口。' : item.status === 'failed' ? '上次接入未完成，请重试。' : '等待接入'
  const message = !item.message || ['已恢复', '已跳过'].includes(item.message) ? fallback : item.message
  return { label: labels[item.status] || '待处理', tone, message }
}

export function importItemActions(item, { busy = false, blocked = false } = {}) {
  const actions = []
  const add = (key, label, disabled = busy) => actions.push({ key, label, disabled })
  if (item.status === 'succeeded') {
    if (item.account_id) add('view', '查看账号')
  } else if (item.status === 'skipped') {
    add('restore', '恢复处理')

  } else {
    const manual = item.status === 'failed' && item.step === 'manual'
    if (manual && item.bit_profile_id) add('open', '打开窗口')
    if (['running', 'uncertain'].includes(item.status) || manual) add('reconcile', manual ? '检查并同步' : '核实结果')
    if (item.status === 'failed' && !manual) {
      add('retry', item.step === 'blocked' ? '使用新窗口' : '重试', busy || blocked)
      if (item.creation_unavailable) add('reuse', '选择空闲窗口', busy || blocked)
    }
    if (!manual && item.bit_profile_id && !['pending'].includes(item.status)) add('open', '打开窗口')
    if (['failed', 'pending'].includes(item.status)) add('skip', '跳过此行')
  }
  return actions.map((action, index) => ({ ...action, primary: index === 0 && !['skip', 'restore'].includes(action.key) }))
}
