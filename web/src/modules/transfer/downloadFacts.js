// 下载中心的事实推导。纯函数，不碰网络也不碰计时器 —— 所以它可以被单测钉住，
// 而不是只能靠肉眼看进度条。
//
// 这个模块存在的唯一理由是那条红线：**任何状态都不得渲染出看起来完成、而实际没有
// 终态 `success` 的进度条**。把它写成函数而不是写在模板里，是为了让「100%」这个
// 数只有一个来源，且那个来源明确拒绝在非 success 时给出 100。

export const TERMINAL_STATUSES = ['success', 'failed', 'cancelled']

export function isTerminal(task) {
  return TERMINAL_STATUSES.includes(task?.status)
}

/** 失败 Tab 的依据：只有 `failed` 进失败，成功与取消都是历史。 */
export function isFailed(task) {
  return task?.status === 'failed'
}

/** 历史 Tab 的依据：成功与已取消都是事实记录，只有失败单列。 */
export function isHistory(task) {
  return task?.status === 'success' || task?.status === 'cancelled'
}

/**
 * 行是否落在「从现在往前 days 天」的窗口里。
 *
 * 保留窗口是**展示层截断**（DB 从不删行，见 CHG-069 任务 23 裁定），所以这条只决定一行
 * 还显不显示，不碰任何事实。取 `finished_at`（终态行必有，任务 23 起进入契约）；它缺了
 * 才退回 `updated_at`，而 `updated_at` 会被租约续租移动，只是兜底。
 */
export function withinDays(task, days, now = Date.now()) {
  if (!Number.isFinite(days) || days <= 0) return false
  const source = task?.finished_at || task?.updated_at
  const stamp = source ? Date.parse(source) : Number.NaN
  if (!Number.isFinite(stamp)) return false
  return now - stamp <= days * 24 * 60 * 60 * 1000
}

/** 轮询闸门：没有非终态任务时不该继续拉。 */
export function hasLiveTask(tasks) {
  return tasks.some((task) => !isTerminal(task))
}

/**
 * 进度。
 *
 * - `total_bytes === 0` → `indeterminate`：分母还没定下来（云端还没算出总长），
 *   此时画一条按比例的进度条就是编数字，百分比只能是「准备中」。
 * - 非终态一律封顶 99：字节数已经追上总数但任务还没报 success 的那一小段时间里，
 *   100% 是个假终态，而它恰好是唯一会让人关掉页面走开的读数。
 */
export function progressOf(task) {
  if (task?.status === 'success') return { kind: 'complete', percent: 100 }
  const total = Number(task?.total_bytes || 0)
  const done = Number(task?.completed_bytes || 0)
  if (total <= 0) return { kind: 'indeterminate', percent: 0 }
  const percent = Math.floor((done / total) * 100)
  return { kind: 'determinate', percent: Math.min(Math.max(percent, 0), 99) }
}

/**
 * 这个本机任务是不是卡在等云端准备。
 *
 * 依据是**同素材的一条非终态 cloud 任务**，不是时间也不是猜测：一键下载在素材还没
 * 准备好时会先建云端准备任务、再把本机任务挂上去，两条任务在同一个列表里（都以当前
 * 用户为 requested_by）。云任务结束了这条自然不再成立，界面也就不用自己编一个
 * 「预计还要多久」。
 */
export function needsCloudPreparation(task, tasks = []) {
  if (task?.execution_scope !== 'local_agent' || isTerminal(task)) return false
  return tasks.some((other) => (
    other.id !== task.id
    && other.execution_scope === 'cloud'
    && other.asset_id === task.asset_id
    && !isTerminal(other)
  ))
}

const STATUS_LABELS = {
  pending: '排队中',
  running: '下载中',
  success: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

const STATUS_TONES = {
  pending: 'neutral',
  running: 'info',
  success: 'success',
  failed: 'danger',
  cancelled: 'neutral',
}

/**
 * 一行任务该怎么显示。
 *
 * `cancelRequested` 是**本机记忆**而不是服务端字段：冻结的 `FileTransferTask`
 * 没有 `cancel_requested_at`，取消一个 running 任务后它也仍然报 `running`（终态
 * 只能由执行器写）。所以「正在取消」只能由「我按过取消」这件事推出，随下一次刷新
 * 任务变成 cancelled 而消失。把它标成 `info` 而不是 `success` 也是这个原因 ——
 * 请求发出去了，不等于已经停下。
 */
export function taskState(task, { tasks = [], cancelRequested = false } = {}) {
  if (task?.status === 'success') return { key: 'success', label: STATUS_LABELS.success, tone: 'success' }
  if (task?.status === 'failed') return { key: 'failed', label: STATUS_LABELS.failed, tone: 'danger' }
  if (task?.status === 'cancelled') return { key: 'cancelled', label: STATUS_LABELS.cancelled, tone: 'neutral' }
  if (cancelRequested) return { key: 'cancelling', label: '正在取消', tone: 'info' }
  if (needsCloudPreparation(task, tasks)) return { key: 'waiting_cloud', label: '等待云端准备', tone: 'warning' }
  return { key: task?.status || 'pending', label: STATUS_LABELS[task?.status] || task?.status || '-', tone: STATUS_TONES[task?.status] || 'neutral' }
}

/**
 * 一个已下载的文件现在在哪儿。
 *
 * 这是**量出来的**，不是推出来的：名单和目录都由 Desktop 的 Rust 侧在已知的保存位置里
 * 逐一列目录得到（`local_saved_file_states`）。`unknown` 是「没人查过」——浏览器根本
 * 查不了本机文件，Desktop 也要扫过才知道——它和「查过、不在」是两件事，混起来就会把
 * 「没查」渲染成「文件没了」。
 */
export const FILE_PRESENCE = Object.freeze({
  /** 在当前选定的保存位置里。 */
  current: 'present_current',
  /** 不在当前保存位置，但在一个已知的历史保存位置里（改过目录的文件就是这个）。 */
  elsewhere: 'present_elsewhere',
  /** 查过了，所有已知位置都没有它。 */
  absent: 'absent',
  /** 没查过。 */
  unknown: 'unknown',
})

/**
 * 这个素材在本机的那一份叫什么名字，没有就是 `null`。
 *
 * 详情抽屉要问「这个文件的目录在哪」之前，先得有名字送去问 —— 而名字只能从任务表里来，
 * 磁盘上没有「哪些文件是这个 App 下的」这件事（`migrationCandidates.js` 同一段理由）。
 *
 * 只有**这台机器、这个动作、成功过、报了名字**的那一条算：`execution_scope` 是本机
 * 而不是云端、`purpose` 是运营自己点的「下载」（合成准备的落点是另一个功能的产物）、
 * `status` 是 `success`（跑着的和失败的行上没有文件）。顺序用服务端给的顺序（`ListTasks`
 * 是 `created_at DESC, id DESC`），第一条匹配的就是最近那一次 —— 一条更晚的失败不能让
 * 更早的那次成功消失：磁盘上的文件不因为重下失败而没了。
 */
export function downloadedFileName(tasks, assetId) {
  const wanted = Number(assetId)
  if (!Number.isFinite(wanted)) return null
  for (const task of Array.isArray(tasks) ? tasks : []) {
    if (task?.asset_type !== 'material') continue
    if (Number(task?.asset_id) !== wanted) continue
    if (task?.execution_scope !== 'local_agent') continue
    if (task?.purpose !== 'user_download') continue
    if (task?.status !== 'success') continue
    const name = typeof task?.file_name === 'string' ? task.file_name : ''
    if (!name.trim()) continue
    return name
  }
  return null
}

/**
 * 这个任务的**文件名**在扫描结果里的那一条，没有就是 `null`。
 *
 * 按名字查而不是按任务查：一个名字可能对应多条任务（重试、重新下载），而磁盘上只有一个
 * 文件 —— 名字是执行器写下去的那一个，也是唯一能定位它的东西。
 */
export function fileFact(task, presence = {}) {
  const name = task?.file_name
  if (!name) return null
  // 缺键与 `null` 都是「没查过」；「查过、不在」在扫描结果里是一条实实在在的记录
  // （`presence: absent`），不是缺键。这个区别是这一整块的意义所在。
  return presence?.[name] ?? null
}

/** 上面那一条的 `presence`，没查过就是 `unknown`。 */
export function filePresence(task, presence = {}) {
  return fileFact(task, presence)?.presence ?? FILE_PRESENCE.unknown
}

/**
 * 打开文件只在 Desktop 有意义：文件落在运营这台机器上，浏览器打不开它。
 *
 * `presence` 默认 `unknown`：没查过就不作断言，按钮照旧给出来（单参调用与 Cloud Web
 * 的行保持今天的行为）。只有**实测不在**时才收回这个按钮——那时它点下去必然是一句
 * 「可能已被移动或删除」，而一个必然失败的按钮不该出现在那里。文件在旧目录里仍然给：
 * `local_open_saved_file` 会在所有已知的保存位置里找它。
 */
export function canOpenFile(task, presence = FILE_PRESENCE.unknown) {
  if (task?.status !== 'success' || !task?.file_name) return false
  return presence !== FILE_PRESENCE.absent
}

/**
 * 这一行的失败能不能断言「它在等的那条准备没有交付」。
 *
 * 只有两个码可以这么断言：
 * - `dependency_failed` 只由 `failDependentsOfTerminalTasks` 写，那条 SQL 的 WHERE
 *   要求被改的行自己有 `dependency_task_id`；
 * - `cancelled_by_user` 落在一条 **`failed`** 行上时只可能来自 `FailDependents` ——
 *   用户直接取消一条下载，终态写的是 `cancelled`，而这一格的前提是 `failed`。
 *
 * 其余码客户端分不出来：一条本机下载丢了租约也是 `lease_lost`，而那种行的重试是成立
 * 的（它的 `dependency_task_id` 为空，服务端会放行）。分不出来时**留下按钮**，让服务端
 * 用它自己的理由拒绝 —— 藏掉一个成立的动作，比多显示一个会被拒绝的动作更糟。
 *
 * 也就是说这里排除的只是「服务端必然拒绝」的那一类，不是「我猜它不行」的那一类。
 */
const WAITER_RELEASE_CODES = ['dependency_failed', 'cancelled_by_user']

export function canRetry(task) {
  return task?.status === 'failed'
    && Number(task.attempt_count || 0) < Number(task.max_attempts || 0)
    && !WAITER_RELEASE_CODES.includes(task?.error_code)
}

/**
 * 「重新下载」：终态行重新发起一次下载，走的是和第一次点击**完全相同**的那个入口。
 *
 * 失败与已取消的行无条件给：这正是走查里报的第三条 ——「已取消的无法再次点击下载」。
 * 这两态的行今天一个动作都没有（`canRetry` 只认 failed，`canOpenFile` 只认 success，
 * `canCancel` 只认非终态），运营看着一条卡住的行无从下手。
 *
 * 成功的行另说：文件在不在**不是这里能断言的**。`presence` 默认 `unknown` —— 浏览器
 * 根本查不了本机文件，Desktop 也要先扫过才知道 —— 而 `unknown` 不等于「不在」。
 * 只有实测不在（`absent`：文件被搬走或被删）时才推荐重新下载；文件还在时该出现的
 * 动作是「打开文件」。
 */
export function canRedownload(task, presence = FILE_PRESENCE.unknown) {
  if (task?.asset_type !== 'material' || !task?.asset_id) return false
  if (task?.status === 'failed' || task?.status === 'cancelled') return true
  return task?.status === 'success' && presence === FILE_PRESENCE.absent
}

export function canCancel(task) {
  return task?.status === 'pending' || task?.status === 'running'
}
