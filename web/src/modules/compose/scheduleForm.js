export const MATERIAL_SCOPE = 'creator_usages'

export function scheduleFormFor(strategy) {
  const schedule = strategy?.schedule || {}
  return {
    enabled: Boolean(schedule.enabled),
    mode: schedule.mode || 'manual',
    interval_minutes: schedule.interval_minutes || 30,
    daily_time: schedule.daily_time || '09:00',
    timezone: schedule.timezone || 'Asia/Shanghai',
    per_run_limit: schedule.per_run_limit || 10,
    daily_limit: schedule.daily_limit || 100,
  }
}

export function schedulePayload(form) {
  const errors = {}
  const mode = form.mode
  const perRun = Number(form.per_run_limit)
  const daily = Number(form.daily_limit)
  if (!String(form.timezone || '').trim()) errors.timezone = '请输入时区'
  if (!Number.isInteger(perRun) || perRun < 1 || perRun > 100) errors.per_run_limit = '请输入 1～100 的整数'
  if (!Number.isInteger(daily) || daily < perRun || daily > 1000) errors.daily_limit = '每日上限应不小于单次上限，且不超过 1000'
  const payload = {
    enabled: mode !== 'manual' && Boolean(form.enabled),
    mode,
    timezone: String(form.timezone || '').trim(),
    per_run_limit: perRun,
    daily_limit: daily,
    material_scope: MATERIAL_SCOPE,
  }
  if (mode === 'interval') {
    const minutes = Number(form.interval_minutes)
    if (!Number.isInteger(minutes) || minutes < 1 || minutes > 1440) errors.interval_minutes = '请输入 1～1440 的整数'
    payload.interval_minutes = minutes
  } else if (mode === 'daily') {
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(form.daily_time || '')) errors.daily_time = '请输入 HH:MM，例如 09:30'
    payload.daily_time = form.daily_time
  } else if (mode !== 'manual') {
    errors.mode = '请选择执行周期'
  }
  return { payload, errors }
}

export function scheduleLabel(schedule) {
  if (!schedule || schedule.mode === 'manual') return '手动执行'
  if (schedule.mode === 'interval') return `每 ${schedule.interval_minutes} 分钟`
  return `每天 ${schedule.daily_time}`
}
