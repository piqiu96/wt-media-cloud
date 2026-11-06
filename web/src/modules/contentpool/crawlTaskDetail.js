// 任务统计包含入池、转素材等不同维度，不把它们相加为内容总数。
export function taskContents(task) {
  return (task.results || []).filter(item => item.platform_content_id || item.title)
}

export function taskResultSummary(task) {
  if (task.status === 'pending') return '等待执行。'
  const stats = task.stats || {}
  const parts = [task.task_type === 'retry_failed_task'
    ? `本次处理 ${Number(stats.found || 0).toLocaleString()} 项`
    : `${task.status === 'running' ? '已发现' : '发现'} ${Number(stats.found || 0).toLocaleString()} 条`]
  const contents = taskContents(task)
  const pending = contents.filter(item => item.processing_status === 'pending').length
  if (stats.added > 0) parts.push(`新增入池 ${Number(stats.added).toLocaleString()} 条`)
  if (stats.auto_materialized > 0) parts.push(`已转素材 ${Number(stats.auto_materialized).toLocaleString()} 条`)
  if (pending > 0) parts.push(`待审核 ${pending.toLocaleString()} 条`)
  if (stats.duplicate > 0) parts.push(`重复 ${Number(stats.duplicate).toLocaleString()} 条`)
  if (stats.failed > 0) parts.push(`失败 ${Number(stats.failed).toLocaleString()} 项`)
  if (task.status === 'running') parts.unshift('执行中')
  return `${parts.join('，')}。`
}

// 只展示服务端已记录的事件，不根据统计反推抓取、去重等执行步骤。
export function taskExecutionEvents(task) {
  return [
    { label: '创建任务', time: task.created_at },
    { label: '开始执行', time: task.started_at },
    { label: '执行结束', time: task.finished_at },
  ].filter(event => event.time)
}

export function taskExceptions(task) {
  const items = (task.results || [])
    .filter(item => ['failed', 'material_failed'].includes(item.processing_status))
    .map(item => ({
      stage: item.processing_status === 'material_failed' ? '转素材失败' : '内容处理失败',
      reason: item.failure_reason || '未记录失败原因',
      scope: item.title || item.platform_content_id || item.failure_key || '—',
    }))
  if (task.error && !items.some(item => item.reason === task.error)) {
    items.unshift({ stage: '执行异常', reason: task.error, scope: '任务' })
  }
  if (!items.length && (task.stats?.failed > 0 || ['failed', 'partial_success'].includes(task.status))) {
    items.push({ stage: '执行异常', reason: '未记录失败原因', scope: '任务' })
  }
  return items
}

