// 挖掘策略的执行周期在库里是 `manual` / `daily 09:00` / `interval:30`，
// 列表、详情抽屉和内容池来源弹窗读的是同一份值，映射只在这里写一遍。
export function scheduleLabel(schedule) {
  if (!schedule || schedule === 'manual') return '手动执行'
  if (String(schedule).startsWith('daily ')) return `每天 ${String(schedule).slice(6)}`
  if (String(schedule).startsWith('interval:')) return `每 ${String(schedule).slice(9)} 分钟`
  return schedule
}
