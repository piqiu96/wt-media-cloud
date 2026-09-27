import { describe, expect, it } from 'vitest'
import { canCancel, canOpenFile, canRetry, hasLiveTask, isTerminal, needsCloudPreparation, progressOf, taskState } from './downloadFacts.js'
import { createDownloadFailureMessage, taskErrorLabel } from './downloadErrors.js'

const task = (over = {}) => ({
  id: 't-1',
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
})

describe('failure wording', () => {
  it('explains the two refusals a click can produce', () => {
    expect(createDownloadFailureMessage({ type: 'material_unavailable' })).toContain('尚未准备好')
    expect(createDownloadFailureMessage({ type: 'local_transfer_node_unavailable' })).toContain('下载节点')
  })

  // 认不出来就把服务端的话原样交出去。换成「操作失败」等于把唯一的诊断信息扔掉。
  it('passes an unrecognised refusal through instead of flattening it', () => {
    expect(createDownloadFailureMessage({ type: 'something_new', message: '服务端说得很具体' })).toBe('服务端说得很具体')
    expect(createDownloadFailureMessage({})).toBe('发起下载失败')
  })

  it('explains a task error code, and shows an unknown one rather than blank', () => {
    expect(taskErrorLabel('download_integrity_failed')).toContain('完整性')
    expect(taskErrorLabel('download_disk_insufficient')).toContain('磁盘')
    expect(taskErrorLabel('cancelled_by_user')).toContain('取消')
    // 没见过的码是最需要被看到的：空白会让人以为「没出错」。
    expect(taskErrorLabel('brand_new_code')).toBe('brand_new_code')
    expect(taskErrorLabel('')).toBe('')
  })
})
