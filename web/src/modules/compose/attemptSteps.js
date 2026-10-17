// 执行记录里的「这一步做了什么」。步骤本身由云端在执行时逐条追加，这里只负责把它讲给人听：
// 怎么排、这一条有什么成果数字。中文标签由服务端下发（服务端还会顺手丢掉不认识的步骤），
// 前端不再维护第二份 key→中文 的映射 —— 两边各有一份就会各错一份。
import { formatBytes } from '../../shared/utils/units.js'

/** 步骤列表。老响应没有这个字段，缺了就是空，不是错误。 */
export function attemptSteps(attempt) {
  const steps = attempt?.steps
  return Array.isArray(steps) ? steps.filter((step) => step && typeof step === 'object') : []
}

// 未知不是零：云端没记体积/时长/摘要时整段不显示，而不是显示 0 B（那是「空文件」）。
function finiteNumber(value) {
  if (value === undefined || value === null || value === '') return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

/** 这一步的成果数字，拼成一行；什么都没有就返回空串，调用方据此不显示这一格。 */
export function stepMetricsText(step) {
  const metrics = step?.metrics || {}
  const parts = []
  const size = formatBytes(metrics.size_bytes)
  if (size !== '-') parts.push(`文件 ${size}`)
  const duration = finiteNumber(metrics.output_duration_ms)
  if (duration !== null && duration > 0) parts.push(`成片时长 ${(duration / 1000).toFixed(1)} 秒`)
  const outputID = finiteNumber(metrics.output_id)
  if (outputID !== null) parts.push(`成片编号 ${outputID}`)
  // 摘要只显示前 8 位：它的用处是「两次是不是同一个文件」，不是给人抄。
  const digest = String(metrics.sha256 || '')
  if (digest) parts.push(`sha256 ${digest.slice(0, 8)}…`)
  return parts.join(' · ')
}
