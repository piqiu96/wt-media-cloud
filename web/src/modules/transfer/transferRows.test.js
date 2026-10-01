import { describe, expect, it } from 'vitest'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { splitTransferRows, transferRow, transferRows } from './transferRows.js'

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

describe('transfer row', () => {
  /**
   * 每个动作位都要从行对象上出得来。
   *
   * 这一条存在的理由是**漏传**：`canRedownload` 加进 `downloadFacts` 却忘了写进
   * `transferRow` 的返回体时，函数自己的用例照样全绿，界面上那个按钮永远不出现，
   * 而没有任何东西会报错。模板读的是这一层。
   */
  it('carries every row action through to the row object', () => {
    const row = transferRow(task({ status: 'cancelled' }))
    expect(row.canCancel).toBe(false)
    expect(row.canRetry).toBe(false)
    expect(row.canRedownload).toBe(true)
    expect('canOpen' in row).toBe(true)
  })

  it('puts 重新下载 on the rows that were stuck with nothing to do', () => {
    // 走查里报的那一条：已取消行原先零动作。
    expect(transferRow(task({ status: 'cancelled' })).canRedownload).toBe(true)
    // 在等准备的下载失败后（依赖没交付）：重试会被服务端拒绝，出路是这一条。
    expect(transferRow(task({ status: 'failed', error_code: 'dependency_failed' })).canRetry).toBe(false)
    expect(transferRow(task({ status: 'failed', error_code: 'dependency_failed' })).canRedownload).toBe(true)
  })

  // 一条在跑的行不该同时出现「取消」和「重新下载」：后者会撞上去重键，得到同一条任务，
  // 而屏幕上的按钮看起来像是「再下一份」。
  it('never shows a re-download on an unfinished row', () => {
    expect(transferRow(task({ status: 'running' })).canRedownload).toBe(false)
    expect(transferRow(task({ status: 'pending' })).canRedownload).toBe(false)
    expect(transferRow(task({ status: 'running' })).canCancel).toBe(true)
  })

  // 完成的行只有「打开文件」，不擅自多一个重新下载 —— 文件在不在它没查过。
  it('leaves a successful row to the open-file action', () => {
    const row = transferRow(task({ status: 'success', file_name: '演示素材-42.mp4' }))
    expect(row.canOpen).toBe(true)
    expect(row.canRedownload).toBe(false)
    expect(row.progress).toEqual({ kind: 'complete', percent: 100 })
  })

  // 终态行的完成时间来自契约新暴露的 `finished_at`（任务 23）；非终态没有它。
  // 不能用 `updated_at` 兜底：它会被租约续租移动，拿它当完成时刻是错的。
  it('carries the terminal completion time through finished_at, not updated_at', () => {
    const finished = transferRow(task({ status: 'success', finished_at: '2026-09-30T05:17:00Z', updated_at: '2026-09-30T09:00:00Z' }))
    expect(finished.finishedAt).toBe('2026-09-30T05:17:00Z')
    expect(finished.finishedText).toBe(formatDateTime('2026-09-30T05:17:00Z'))
    const running = transferRow(task({ status: 'running', finished_at: null, updated_at: '2026-09-30T09:00:00Z' }))
    expect(running.finishedAt).toBeNull()
    expect(running.finishedText).toBe('')
  })
})

describe('transfer rows', () => {
  // 行对象是按任务逐条算的，`cancelRequested` 是**本机记忆**（冻结的任务体没有
  // cancel_requested_at），所以要按 id 指派，不能整列表一起盖。
  it('applies the local cancel memory to the task it names', () => {
    const rows = transferRows([task({ id: 'a' }), task({ id: 'b' })], { cancelRequested: ['b'] })
    expect(rows.map((row) => row.canCancel)).toEqual([true, false])
    expect(rows.map((row) => row.state.key)).toEqual(['running', 'cancelling'])
  })

  // `tasks` 要原样穿进去：一条本机任务是不是「在等云端准备」，靠的是同列表里那条
  // 同素材的 cloud 任务。
  it('passes the whole list down so a local row can see its cloud sibling', () => {
    const local = task({ id: 'a', status: 'pending', total_bytes: 0 })
    const cloud = task({ id: 'b', execution_scope: 'cloud', status: 'running', total_bytes: 0 })
    expect(transferRows([local, cloud])[0].state.key).toBe('waiting_cloud')
  })

  it('returns an empty list for anything that is not a list', () => {
    expect(transferRows(null)).toEqual([])
    expect(transferRows(undefined)).toEqual([])
  })

  /**
   * 扫描结果按**名字**索引，穿到每一行上。
   *
   * 这是走查第二条的落点：已完成的行要能说出「文件在旧目录里」而不是只会说「可能已被
   * 移动或删除」。名字索引而不是任务索引，因为磁盘上只有一个文件，而名字可能对应多条
   * 任务（重试、重新下载各一条）。
   */
  it('gives each row the file fact its own name was scanned as', () => {
    const presence = {
      'a.mp4': { name: 'a.mp4', presence: 'present_elsewhere', directory: '旧位置', bytes: 12 },
      'b.mp4': { name: 'b.mp4', presence: 'absent', directory: '', bytes: null },
    }
    const rows = transferRows([
      task({ id: 'x', status: 'success', file_name: 'a.mp4' }),
      task({ id: 'y', status: 'success', file_name: 'b.mp4' }),
      task({ id: 'z', status: 'success', file_name: 'c.mp4' }),
    ], { presence })

    expect(rows[0].presence).toBe('present_elsewhere')
    expect(rows[0].fileFact.directory).toBe('旧位置')
    expect(rows[0].canOpen).toBe(true)
    expect(rows[0].canRedownload).toBe(false)

    expect(rows[1].presence).toBe('absent')
    // 实测不在：打开文件没了，重新下载来了 —— 这一格就是第二条报障的修法。
    expect(rows[1].canOpen).toBe(false)
    expect(rows[1].canRedownload).toBe(true)

    // 名单里没有的名字是「没查过」，不是「不在」。
    expect(rows[2].presence).toBe('unknown')
    expect(rows[2].fileFact).toBeNull()
    expect(rows[2].canOpen).toBe(true)
    expect(rows[2].canRedownload).toBe(false)
  })

  // 不传扫描结果时每一行都是「没查过」：界面上不该凭空出现「文件已不在」。
  it('asserts nothing about files when nobody scanned', () => {
    const row = transferRow(task({ status: 'success', file_name: 'a.mp4' }))
    expect(row.presence).toBe('unknown')
    expect(row.fileFact).toBeNull()
    expect(row.canRedownload).toBe(false)
  })
})

// 下载中心的三栏（CHG-20260930-069 任务 23）：非终态进「进行中」，失败单列「失败」，
// 其余终态进「历史」。分组依据任务事实（isFailed／isHistory／isTerminal），不依据行的
// 显示状态；一行恰好属于一栏，三栏计数之和等于行数。
describe('splitTransferRows', () => {
  it('puts pending and running under active, failed alone, and the rest under history', () => {
    const rows = transferRows([
      task({ id: 'p', status: 'pending' }),
      task({ id: 'r', status: 'running' }),
      task({ id: 's', status: 'success', file_name: 'a.mp4' }),
      task({ id: 'f', status: 'failed', error_code: 'lease_lost' }),
      task({ id: 'c', status: 'cancelled' }),
    ])
    const { active, failed, history } = splitTransferRows(rows)
    expect(active.map((row) => row.task.id)).toEqual(['p', 'r'])
    expect(failed.map((row) => row.task.id)).toEqual(['f'])
    expect(history.map((row) => row.task.id)).toEqual(['s', 'c'])
    expect(active.length + failed.length + history.length).toBe(rows.length)
  })

  it('keeps the original order inside each group', () => {
    const rows = transferRows([
      task({ id: 'r1', status: 'running' }),
      task({ id: 's1', status: 'success' }),
      task({ id: 'f1', status: 'failed' }),
      task({ id: 'p1', status: 'pending' }),
      task({ id: 'c1', status: 'cancelled' }),
    ])
    const { active, failed, history } = splitTransferRows(rows)
    expect(active.map((row) => row.task.id)).toEqual(['r1', 'p1'])
    expect(failed.map((row) => row.task.id)).toEqual(['f1'])
    expect(history.map((row) => row.task.id)).toEqual(['s1', 'c1'])
  })
})
