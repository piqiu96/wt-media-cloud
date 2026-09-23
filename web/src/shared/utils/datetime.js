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
