// 字节、速率、时长的显示。三个函数放在一起是因为它们回答同一个问题：
// 「这个机器的数怎么讲给人听」——素材库的体积、下载中心的进度与剩余时间。
//
// 共同的一条：**未知不是零**。体积未知显示 `-` 而不是 `0 B`（那会让「还没量过」
// 看起来像「空文件」）；剩余时间未知返回空串而不是 `0 秒`（那是「马上好」）。

/** 体积。0 与负数都当未知：`total_bytes: 0` 在传输里是「还没量」而不是「零字节」。 */
export function formatBytes(value) {
  const bytes = Number(value)
  if (!Number.isFinite(bytes) || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = bytes / 1024
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size.toFixed(1)} ${units[unit]}`
}

/** 速率。没有速率时返回空串 —— `- MB/s` 比不显示更糟。 */
export function formatByteRate(value) {
  const rate = Number(value)
  if (!Number.isFinite(rate) || rate <= 0) return ''
  return `${formatBytes(rate)}/s`
}

/** 剩余时间。未知或非正数返回空串，调用方据此决定整段不显示。 */
export function formatEta(seconds) {
  const value = Number(seconds)
  if (!Number.isFinite(value) || value <= 0) return ''
  if (value < 60) return `约 ${Math.ceil(value)} 秒`
  if (value < 3600) return `约 ${Math.ceil(value / 60)} 分钟`
  return `约 ${(value / 3600).toFixed(1)} 小时`
}
