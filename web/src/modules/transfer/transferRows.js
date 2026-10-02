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
import { canCancel, canOpenFile, canRedownload, canRetake, fileFact, filePresence, isFailed, isHistory, isTerminal, progressOf, taskState } from './downloadFacts.js'
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

export function transferRow(task, { tasks = [], cancelRequested = false, presence = {}, localDeviceId = null } = {}) {
  const progress = progressOf(task)
  const state = taskState(task, { tasks, cancelRequested })
  // 量出来的那一条（可能是「查过、不在」），和由它推出的三种呈现。`presence` 缺省是
  // 空表 —— 没人查过时每一行都是 `unknown`，界面不作任何断言。
  const found = fileFact(task, presence)
  const presenceOfFile = filePresence(task, presence)
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
    // 发起时间一直是写的；任务 23 起契约暴露了 `finished_at`（终态前为 null），终态行
    // 因此能写出完成时间。`updated_at` 会被租约续租移动，不能拿它当完成时刻。
    createdText: formatDateTime(task.created_at),
    finishedAt: task.finished_at || null,
    finishedText: task.finished_at ? formatDateTime(task.finished_at) : '',
    canCancel: canCancel(task) && !cancelRequested,
    canOpen: canOpenFile(task, presenceOfFile),
    canRedownload: canRedownload(task),
    // 悬置/可重取由「重取」按钮接（见 DownloadCentreDrawer）；localDeviceId 是浏览器读
    // 不到的（缺省 null），所以 web 上只有「解绑可重取」这一支成立。
    canRetake: canRetake(task, localDeviceId),
    // `presence` 是给界面读的：文件在旧目录里时要说出来在哪儿，不然「打开文件」能用而
    // 「重新下载」没出现的组合看起来像少了点什么。`fileFact` 是那一条记录（含目录与
    // 体积），`null` 表示没查过或不适用。
    presence: presenceOfFile,
    fileFact: found,
  }
}

export function transferRows(tasks, { cancelRequested = [], presence = {}, localDeviceId = null } = {}) {
  const list = Array.isArray(tasks) ? tasks : []
  return list.map((task) => transferRow(task, {
    tasks: list,
    cancelRequested: cancelRequested.includes(task.id),
    presence,
    localDeviceId,
  }))
}

/** 面板里还剩几条没跑完，用来决定要不要继续轮询。 */
export function liveCount(tasks) {
  return (Array.isArray(tasks) ? tasks : []).filter((task) => !isTerminal(task)).length
}

// 下载中心的三栏（CHG-20260930-069 任务 23）：非终态「进行中」，失败单列「失败」，
// 其余终态「历史」。依据任务事实（isFailed／isHistory／isTerminal），一行恰好属于一栏，
// 三栏计数之和等于行数。
export function splitTransferRows(rows) {
  const list = Array.isArray(rows) ? rows : []
  return {
    active: list.filter((row) => !isTerminal(row.task)),
    failed: list.filter((row) => isFailed(row.task)),
    history: list.filter((row) => isHistory(row.task)),
  }
}
