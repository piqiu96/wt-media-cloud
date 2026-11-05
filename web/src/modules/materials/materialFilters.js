// API bounds are start-inclusive/end-exclusive in the user's local calendar.
export function publishedRangeParams(range) {
  if (!Array.isArray(range) || !range[0] || !range[1]) return {}
  const midnight = value => {
    const [year, month, day] = String(value).split('-').map(Number)
    const date = new Date(year, month - 1, day)
    if (date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== day) throw new Error('请选择有效的发布时间')
    return date
  }
  const from = midnight(range[0]), before = midnight(range[1])
  before.setDate(before.getDate() + 1)
  if (from >= before) throw new Error('结束日期不能早于开始日期')
  return { published_from: from.toISOString(), published_before: before.toISOString() }
}
export function metricRangeParams(filters) {
  return Object.fromEntries(Object.entries(filters).filter(([,value]) => value !== null && value !== undefined && value !== ''))
}
export const emptyMetricRanges = () => ({ view_count_min: null, view_count_max: null, like_count_min: null, like_count_max: null, favorite_count_min: null, favorite_count_max: null })
export function metricLabel(row, key) {
  return row.metric_presence?.[key] === false || row[key] === null || row[key] === undefined ? '—' : Number(row[key]).toLocaleString('zh-CN')
}

// Source readiness and this device's download are separate facts; the badge
// presents the current actionable state without inventing local-file presence.
export function materialVideoState(row, presence = null) {
  if (row.download_status === 'downloaded' && ['present_current', 'present_elsewhere'].includes(presence?.presence)) return { label: '可用', tone: 'success' }
  if (row.download_status === 'downloading') return { label: '下载中', tone: 'info' }
  if (row.download_status === 'failed') return { label: '失败', tone: 'danger' }
  return ({ not_downloaded: { label: '待准备', tone: 'neutral' }, downloading: { label: '准备中', tone: 'info' }, ready: { label: '可用', tone: 'success' }, failed: { label: '失败', tone: 'danger' } })[row.video_status] || { label: '—', tone: 'neutral' }
}
