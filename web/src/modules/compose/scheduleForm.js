export function scheduleLabel(schedule) {
  if (!schedule || schedule.mode === 'manual') return '手动执行'
  if (schedule.mode === 'interval') return `每 ${schedule.interval_minutes} 分钟`
  return `每天 ${schedule.daily_time}`
}
