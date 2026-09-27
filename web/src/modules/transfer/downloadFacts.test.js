import { describe, expect, it } from 'vitest'
import { canCancel, canOpenFile, canRedownload, canRetry, hasLiveTask, isTerminal, needsCloudPreparation, progressOf, taskState } from './downloadFacts.js'

const task = (over = {}) => ({
  id: 't-1',
  asset_type: 'material',
  asset_id: 42,
  asset_title: '演示素材',
  execution_scope: 'local_agent',
  status: 'running',
  total_bytes: 1000,
  completed_bytes: 250,
  attempt_count: 1,
  max_attempts: 3,
  ...over,
})

describe('transfer terminal states', () => {
  it('knows exactly which statuses end a task', () => {
    expect(['success', 'failed', 'cancelled'].every((status) => isTerminal(task({ status })))).toBe(true)
    expect(['pending', 'running'].some((status) => isTerminal(task({ status })))).toBe(false)
    expect(isTerminal(null)).toBe(false)
  })

  // 轮询闸门：全终态时不该再拉。这条被钉住，是为了让「只在需要时轮询」不是一句注释。
  it('only asks for polling while something is unfinished', () => {
    expect(hasLiveTask([task({ status: 'success' }), task({ status: 'running' })])).toBe(true)
    expect(hasLiveTask([task({ status: 'success' }), task({ status: 'cancelled' })])).toBe(false)
    expect(hasLiveTask([])).toBe(false)
  })
})

describe('transfer progress', () => {
  // 红线所在：**非终态不得出现 100%**。字节数追上总数、任务还没报 success 的那一小段
  // 时间里，100% 是唯一会让人关掉页面走开的假读数。
  it('never reports a hundred percent before the task is a success', () => {
    expect(progressOf(task({ completed_bytes: 1000, total_bytes: 1000 }))).toEqual({ kind: 'determinate', percent: 99 })
    expect(progressOf(task({ completed_bytes: 1200, total_bytes: 1000 }))).toEqual({ kind: 'determinate', percent: 99 })
    expect(progressOf(task({ status: 'success', completed_bytes: 1000, total_bytes: 1000 }))).toEqual({ kind: 'complete', percent: 100 })
  })

  // 分母还没定下来时画比例条就是编数字。云端还没算出总长，此时只能说「准备中」。
  it('refuses to compute a percentage with no denominator', () => {
    expect(progressOf(task({ status: 'pending', total_bytes: 0, completed_bytes: 0 }))).toEqual({ kind: 'indeterminate', percent: 0 })
    expect(progressOf(task({ total_bytes: 0, completed_bytes: 900 }))).toEqual({ kind: 'indeterminate', percent: 0 })
  })

  it('floors the percentage instead of rounding it up', () => {
    expect(progressOf(task({ completed_bytes: 999, total_bytes: 1000 })).percent).toBe(99)
    expect(progressOf(task({ completed_bytes: 1, total_bytes: 3 })).percent).toBe(33)
    expect(progressOf(task({ completed_bytes: 0, total_bytes: 1000 })).percent).toBe(0)
  })
})

describe('one click, two tasks', () => {
  // 「等待云端准备」的依据是**同素材的一条非终态 cloud 任务**，不是时间也不是猜测。
  it('reads waiting-on-cloud from a sibling cloud task rather than from the clock', () => {
    const local = task({ status: 'pending', total_bytes: 0 })
    const cloud = task({ id: 't-0', execution_scope: 'cloud', status: 'running', total_bytes: 0 })
    expect(needsCloudPreparation(local, [local, cloud])).toBe(true)

    expect(needsCloudPreparation(local, [local, { ...cloud, status: 'success' }])).toBe(false)
    expect(needsCloudPreparation(local, [local, { ...cloud, asset_id: 99 }])).toBe(false)
    expect(needsCloudPreparation(local, [local])).toBe(false)
    // 云任务自己是 cloud 范围，不该被自己判成「在等云端」。
    expect(needsCloudPreparation(cloud, [local, cloud])).toBe(false)
    // 同素材上另一条未终结的**本机**任务不是「云端在准备」：那是同一个素材被点了两次
    // 下载（两条都在等本机执行器），云端那边根本没有任务。少了范围判定这一格会读成
    // 「等待云端准备」，而它在等的东西不存在 —— 于是永远等下去。
    expect(needsCloudPreparation(local, [local, task({ id: 't-2', status: 'pending' })])).toBe(false)
  })

  it('stops saying so once the local task is over', () => {
    const local = task({ status: 'cancelled' })
    const cloud = task({ id: 't-0', execution_scope: 'cloud', status: 'running' })
    expect(needsCloudPreparation(local, [local, cloud])).toBe(false)
  })
})

describe('task row state', () => {
  it('labels each terminal status as itself', () => {
    expect(taskState(task({ status: 'success' }))).toEqual({ key: 'success', label: '已完成', tone: 'success' })
    expect(taskState(task({ status: 'failed' })).tone).toBe('danger')
    expect(taskState(task({ status: 'cancelled' })).label).toBe('已取消')
  })

  it('shows a pending local task as waiting for cloud preparation', () => {
    const local = task({ status: 'pending', total_bytes: 0 })
    const cloud = task({ id: 't-0', execution_scope: 'cloud', status: 'pending' })
    expect(taskState(local, { tasks: [local, cloud] })).toEqual({ key: 'waiting_cloud', label: '等待云端准备', tone: 'warning' })
    expect(taskState(local, { tasks: [local] })).toEqual({ key: 'pending', label: '排队中', tone: 'neutral' })
  })

  // 「已请求取消」是本机记忆，不是服务端字段：冻结的任务体没有 cancel_requested_at，
  // 取消一个 running 任务后它仍然报 running。所以这一格由「我按过取消」推出，
  // 且必须是 info 而不是 success —— 请求发出去了不等于已经停下。
  it('marks a cancel request without claiming the task stopped', () => {
    expect(taskState(task({ status: 'running' }), { cancelRequested: true })).toEqual({ key: 'cancelling', label: '正在取消', tone: 'info' })
    expect(taskState(task({ status: 'pending' }), { cancelRequested: true }).key).toBe('cancelling')
    // 已经到了终态就以终态为准，本机记忆不再覆盖服务端的事实。
    expect(taskState(task({ status: 'cancelled' }), { cancelRequested: true }).key).toBe('cancelled')
    expect(taskState(task({ status: 'success' }), { cancelRequested: true }).key).toBe('success')
  })
})

describe('row actions', () => {
  it('offers retry only while attempts remain', () => {
    expect(canRetry(task({ status: 'failed', attempt_count: 1, max_attempts: 3 }))).toBe(true)
    expect(canRetry(task({ status: 'failed', attempt_count: 3, max_attempts: 3 }))).toBe(false)
    expect(canRetry(task({ status: 'running', attempt_count: 1, max_attempts: 3 }))).toBe(false)
  })

  it('offers cancel on anything unfinished and open-file only on a reported success', () => {
    expect(canCancel(task({ status: 'pending' }))).toBe(true)
    expect(canCancel(task({ status: 'running' }))).toBe(true)
    expect(canCancel(task({ status: 'success' }))).toBe(false)

    // 成功但执行器没报文件名时不能「打开」：名字是执行器给的，前端不推导它。
    expect(canOpenFile(task({ status: 'success', file_name: '演示素材-42.mp4' }))).toBe(true)
    expect(canOpenFile(task({ status: 'success', file_name: null }))).toBe(false)
    expect(canOpenFile(task({ status: 'failed', file_name: 'x.mp4' }))).toBe(false)
  })

  /**
   * 重试的排除项只放**服务端必然拒绝**的那一类。
   *
   * `retryTask` 的 WHERE 要求 `dependency_task_id IS NULL`：一条在等准备的下载失败后，
   * 指针还在，重试改不动它，租约里也没有对象键／大小／hash 可发 —— 那条行的 `attempt_count`
   * 会一直是 0，用户看到的是「点了没反应」。
   */
  it('hides retry only for failures that a retry could never move', () => {
    expect(canRetry(task({ status: 'failed', attempt_count: 1, max_attempts: 3, error_code: 'download_integrity_failed' }))).toBe(true)
    expect(canRetry(task({ status: 'failed', error_code: 'dependency_failed' }))).toBe(false)
    expect(canRetry(task({ status: 'failed', error_code: 'cancelled_by_user' }))).toBe(false)
    // `lease_lost` 分不出来：一条本机下载自己丢了租约也是这个码，而那种行重试是成立的
    // （依赖为空）。留着按钮，让服务端用自己的理由拒绝 —— 藏掉一个成立的动作更糟。
    expect(canRetry(task({ status: 'failed', attempt_count: 1, max_attempts: 3, error_code: 'lease_lost' }))).toBe(true)
    expect(canRetry(task({ status: 'cancelled', error_code: 'dependency_failed' }))).toBe(false)
  })

  /**
   * 「重新下载」是走查里报的第三条：「已取消的无法再次点击下载」。
   *
   * 已取消行今天零动作（`canRetry` 只认 failed、`canOpenFile` 只认 success、
   * `canCancel` 只认非终态）—— 一条卡住的行看上去无路可走。
   */
  it('gives every terminal material row a way back to a download', () => {
    expect(canRedownload(task({ status: 'cancelled' }))).toBe(true)
    expect(canRedownload(task({ status: 'failed' }))).toBe(true)
    // 未终结的行不该有它：此时该出现的是「取消」，而重新点一次只会撞上去重键。
    expect(canRedownload(task({ status: 'running' }))).toBe(false)
    expect(canRedownload(task({ status: 'pending' }))).toBe(false)
  })

  /**
   * 成功行是唯一需要**实测**才能推荐重新下载的一态。
   *
   * `presence` 默认 `unknown`：浏览器查不了本机文件，Desktop 也要先扫过才知道。
   * 未知不等于不在 —— 文件好好地在那儿，却因为「没查过」而给人一个重新下载的按钮，
   * 那是拿猜当事实。文件实测不在（被搬走／被删）时它才是那个该出现的动作。
   */
  it('recommends a re-download of a success only once the file is measured gone', () => {
    expect(canRedownload(task({ status: 'success' }))).toBe(false)
    expect(canRedownload(task({ status: 'success' }), 'unknown')).toBe(false)
    expect(canRedownload(task({ status: 'success' }), 'present_current')).toBe(false)
    expect(canRedownload(task({ status: 'success' }), 'present_elsewhere')).toBe(false)
    expect(canRedownload(task({ status: 'success' }), 'absent')).toBe(true)
    // 成功行的原动作是「打开文件」，不是因为多了这个按钮就少一个。
    expect(canOpenFile(task({ status: 'success', file_name: 'x.mp4' }))).toBe(true)
  })

  // 不是素材的任务（`compose_input_prepare` 这类中间产物）没有「重新下载」这个动作：
  // 素材页面那个入口只吃素材 id，拿一个别的 id 去调是错的。
  it('offers a re-download only for material tasks', () => {
    expect(canRedownload(task({ status: 'cancelled', asset_type: 'compose_input' }))).toBe(false)
    expect(canRedownload(task({ status: 'cancelled', asset_id: 0 }))).toBe(false)
    expect(canRedownload(null)).toBe(false)
  })
})

// 文案用例全在 `downloadErrors.test.js`：这个文件钉的是「事实」，措辞跟着词表走，
// 两处各写一份必然分叉。
