import { downloadedFileName } from '../transfer/downloadFacts.js'

export function outputTitle(output) { return String(output?.name || '').trim() || '—' }
export function canQueueOutputDownload(state) { return ['not_downloaded', 'failed', 'missing'].includes(state?.key) }
export function canDownloadOutput(output, state) { return output?.actions?.download === true && canQueueOutputDownload(state) }
export function outputDownloadState(tasks, outputId, fileFact, desktop) {
  const ownTasks = (Array.isArray(tasks) ? tasks : []).filter((task) => task.asset_type === 'composite_output' && Number(task.asset_id) === Number(outputId) && task.execution_scope === 'local_agent' && task.purpose === 'user_download')
  const latest = ownTasks[0], fileName = downloadedFileName(ownTasks, outputId), presence = fileName ? fileFact?.presence : null
  if (desktop && (presence === 'present_current' || presence === 'present_elsewhere')) return { key: 'downloaded', label: '已下载', tone: 'success', fileName, localFile: fileFact }
  if (latest?.status === 'pending' || latest?.status === 'running') return { key: 'downloading', label: '下载中', tone: 'info', fileName: null, localFile: null }
  if (latest?.status === 'failed') return { key: 'failed', label: '下载失败', tone: 'danger', reason: latest.error_message || '', fileName: null, localFile: null }
  if (fileName) {
    if (desktop && presence === 'absent') return { key: 'missing', label: '本机文件缺失', tone: 'warning', fileName, localFile: null }
    return { key: 'unverified', label: '本机未核验', tone: 'neutral', fileName: null, localFile: null }
  }
  return { key: 'not_downloaded', label: '未下载', tone: 'neutral', fileName: null, localFile: null }
}
export function outputVideoState(output, transfer = {}) {
  if (transfer.key === 'downloaded') return { key: 'ready', label: '可用', tone: 'success', reason: '已核验当前设备文件' }
  if (transfer.key === 'downloading') return { key: 'downloading', label: '下载中', tone: 'info', reason: '当前设备正在下载' }
  if (transfer.key === 'failed') return { key: 'failed', label: '失败', tone: 'danger', reason: transfer.reason || '本次下载失败，可查看下载中心' }
  if (output?.video?.status === 'ready') return { key: 'ready', label: '可用', tone: 'success', reason: output.video.reason || '' }
  if (output?.video?.status === 'failed') return { key: 'failed', label: '失败', tone: 'danger', reason: output.video.reason || '视频文件不可用' }
  return { key: 'unknown', label: '—', tone: 'neutral', reason: output?.video?.reason || '视频文件尚未核验' }
}
