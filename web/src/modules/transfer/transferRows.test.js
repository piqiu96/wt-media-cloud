import { describe, expect, it } from 'vitest'
import { transferRow, transferRows } from './transferRows.js'

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
})
