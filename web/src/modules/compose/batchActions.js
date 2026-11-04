import { isRetryableTask } from './retryPolicy.js'
export const TASK_STATUS_LABELS = { pending: '排队中', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
export function taskActionEligible(task, action, canRetry = false) {
  if (action === 'retry') return canRetry && isRetryableTask(task)
  return task.status === 'pending' || (task.status === 'running' && !task.cancel_requested)
}
export function batchProgress(counts = {}) {
  const total = Object.keys(TASK_STATUS_LABELS).reduce((sum, key) => sum + Number(counts[key] || 0), 0)
  const finished = Number(counts.success || 0) + Number(counts.failed || 0) + Number(counts.cancelled || 0)
  return { total, finished, percent: total ? Math.floor(finished * 100 / total) : 0 }
}
export function bulkResultMessage(result, action) {
  const accepted = Number(result.accepted || 0), skipped = Number(result.skipped || 0), failed = Number(result.failed || 0)
  return `${action === 'retry' ? '重试' : '取消'}请求已受理 ${accepted} 条，跳过 ${skipped} 条，失败 ${failed} 条`
}
export function taskActionSkipReason(task, action, canRetry = false) {
  if (taskActionEligible(task, action, canRetry)) return ''
  if (action === 'retry') {
    if (!canRetry) return '无操作权限'
    if (task.error_code === 'execution_uncertain') return '结果不明确，需人工核实'
    return '当前状态不可重试'
  }
  return task.cancel_requested ? '已请求取消' : '已结束'
}
export function selectionSkipSummary(tasks, action, canRetry) {
  const counts = {}
  for (const task of tasks) {
    const reason = taskActionSkipReason(task, action, canRetry)
    if (reason) counts[reason] = (counts[reason] || 0) + 1
  }
  return Object.entries(counts).map(([reason,count]) => ({reason,count}))
}
