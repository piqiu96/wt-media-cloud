// 列表页统一的时间显示：精确到分钟，不带秒。
//
// 为什么用 hourCycle: 'h23' 而不是 hour12: false：
// 后者在部分实现下会把午夜渲染成 24:00，出现一个不存在的时刻。
//
// 为什么不带秒：带秒时最宽的时间戳是 2026/12/31 23:59:59（约 138px），
// 列宽压不到 150px 以内；去掉秒后约 118px，150px 定宽不会折行。
export function formatDateTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', {
    hourCycle: 'h23',
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 只要日期、不要时刻，给那些「精确到分钟」是噪声的格子用（素材库的「最近发布」）。
 *
 * 它与 formatDateTime 并存同样是列宽决定的：带上时刻后这一格会比它右边的「入库时间」
 * 还宽，而它说的是一个比入库时间次要得多的事实。两个函数，不是一个带开关的函数——
 * 开关的默认值会在两个调用点之间漂移。
 */
export function formatDate(value) {
  if (!value) return '-'
  return new Date(value).toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
  })
}
