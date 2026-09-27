// 一行下载任务要显示的全部东西，一次算完。
//
// 从模板里抽出来有两个理由，都不是洁癖：
//
// 1. 模板里 `progressOf(task)` 会被调用三次（判种类、取百分比、判完成），而它每次都
//    重算同一件事；更要紧的是**三个判断分开写就会漂开** —— 判成 determinate 的那一支
//    改了条件，另一支不同步改，就会出现「按 0% 画了一条确定的进度条」这类组合。
// 2. 一张行表能被单测钉住。模板字符串不能。
import { formatByteRate, formatBytes, formatEta } from '../../shared/utils/units.js'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { canCancel, canOpenFile, canRetry, isTerminal, progressOf, taskState } from './downloadFacts.js'
import { taskErrorLabel } from './downloadErrors.js'

/** 「已传 / 总长」。总长未知时只说已传了多少，绝不写 `0 / 0`。 */
function sizeTextOf(task, progress) {
  const done = formatBytes(task.completed_bytes)
  const total = formatBytes(task.total_bytes)
  if (progress.kind === 'indeterminate') {
    return done === '-' ? '总长度未知' : `${done} 已传输，总长度未知`
  }
  if (total === '-') return done === '-' ? '' : done
  return `${done} / ${total}`
}

export function transferRow(task, { tasks = [], cancelRequested = false } = {}) {
  const progress = progressOf(task)
  const state = taskState(task, { tasks, cancelRequested })
  return {
    task,
    state,
    progress,
    // 进度条只在**分母已知**时画。总长未知时画一条 0% 的条，看起来是「卡住了」，
    // 而实际上它可能正在正常传输；这种情况下唯一诚实的话是「准备中」。
    showBar: progress.kind === 'determinate',
    pendingText: progress.kind === 'indeterminate' ? '准备中' : '',
    sizeText: sizeTextOf(task, progress),
    // 速率与剩余时间只在跑的时候有意义：终态之后它们要么是 0，要么是最后一刻的陈旧值。
    rateText: task.status === 'running' ? formatByteRate(task.bytes_per_second) : '',
    etaText: task.status === 'running' ? formatEta(task.estimated_remaining_seconds) : '',
    errorText: task.status === 'failed' ? taskErrorLabel(task.error_code) || task.error_message || '下载失败' : '',
    attemptsText: Number(task.attempt_count) > 1 ? `第 ${task.attempt_count} 次尝试` : '',
    // 冻结的任务体只有 created_at／updated_at，**没有 finished_at**（那不是契约里的键）。
    // 所以「完成于」写不出来，只写发起时间；更新时间为终点会被读成完成时刻。
    createdText: formatDateTime(task.created_at),
    canCancel: canCancel(task) && !cancelRequested,
    canRetry: canRetry(task),
    canOpen: canOpenFile(task),
  }
}

export function transferRows(tasks, { cancelRequested = [] } = {}) {
  const list = Array.isArray(tasks) ? tasks : []
  return list.map((task) => transferRow(task, { tasks: list, cancelRequested: cancelRequested.includes(task.id) }))
}

/** 面板里还剩几条没跑完，用来决定要不要继续轮询。 */
export function liveCount(tasks) {
  return (Array.isArray(tasks) ? tasks : []).filter((task) => !isTerminal(task)).length
}
