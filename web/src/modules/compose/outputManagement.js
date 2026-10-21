import { downloadedFileName } from '../transfer/downloadFacts.js'

export function outputTitle(output, materialTitle, strategyName) {
  const material = String(materialTitle || '').trim() || `素材 ${output.material_id}`
  const strategy = String(strategyName || '').trim() || `策略 ${output.compose_strategy_id}`
  return `${material} · ${strategy}`
}

export function generationLabel(triggerType) {
  if (triggerType === 'manual') return '人工'
  if (triggerType === 'scheduled') return '定时'
  return '—'
}

export function canQueueOutputDownload(state) {
  return ['not_downloaded', 'failed', 'missing'].includes(state?.key)
}

export function outputDownloadState(tasks, outputId, fileFact, desktop) {
  const ownTasks = (Array.isArray(tasks) ? tasks : []).filter((task) => (
    task.asset_type === 'composite_output'
    && Number(task.asset_id) === Number(outputId)
    && task.execution_scope === 'local_agent'
    && task.purpose === 'user_download'
  ))
  const latest = ownTasks[0]
  const fileName = downloadedFileName(ownTasks, outputId)
  const presence = fileName ? fileFact?.presence : null
  if (desktop && (presence === 'present_current' || presence === 'present_elsewhere')) {
    return { key: 'downloaded', label: '已下载', tone: 'success', fileName, localFile: fileFact }
  }
  if (latest?.status === 'pending' || latest?.status === 'running') {
    return { key: 'downloading', label: '下载中', tone: 'info', fileName: null, localFile: null }
  }
  if (latest?.status === 'failed') {
    return { key: 'failed', label: '下载失败', tone: 'danger', fileName: null, localFile: null }
  }
  if (fileName) {
    if (desktop && presence === 'absent') return { key: 'missing', label: '本机文件缺失', tone: 'warning', fileName, localFile: null }
    return { key: 'unverified', label: '下载记录完成 · 本机未核验', tone: 'neutral', fileName: null, localFile: null }
  }
  return { key: 'not_downloaded', label: '未下载', tone: 'neutral', fileName: null, localFile: null }
}
