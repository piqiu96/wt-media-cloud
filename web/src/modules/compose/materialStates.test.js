import { describe, expect, it } from 'vitest'
import {
  CANDIDATE_SORTS,
  addAction,
  candidateRows,
  candidateTag,
  cappedRows,
  hasTaskRecord,
  matchesKeyword,
  materialStatesById,
  stateOf,
  usageCounts,
} from './materialStates.js'

// 发布时间与点赞数的次序刻意都与素材 ID 的次序相反：排序用例如果在「按 id 排」的实现下
// 也能通过，那它证明不了任何事。
const CANDIDATES = [
  { id: 12, title: '开镜集锦', category_id: 'delta', published_at: '2026-10-10T08:00:00Z', like_count: 30 },
  { id: 15, title: '跑图实况', category_id: 'delta', published_at: '2026-10-14T08:00:00Z', like_count: 90 },
  { id: 21, title: '夜景混剪', category_id: 'delta', published_at: '2026-10-16T08:00:00Z', like_count: 10 },
  { id: 33, title: '露娜高光', category_id: 'echo', published_at: null, like_count: 500 },
]

// 策略下的三种履历：成功过、正在跑、只取消过。素材 33 没有任何状态 —— 它是唯一未使用过的。
const STATES = [
  { material_id: 12, success_count: 2, active_count: 0, failed_count: 1, cancelled_count: 0, last_task_at: '2026-10-15T08:00:00Z' },
  { material_id: 15, success_count: 0, active_count: 1, failed_count: 0, cancelled_count: 0, last_task_at: '2026-10-16T09:00:00Z' },
  { material_id: 21, success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 2, last_task_at: '2026-10-14T07:00:00Z' },
]

describe('material states lookup', () => {
  // 服务端承诺每个请求的 ID 都有回话，但「没返回」被当成「没用过」时不会报错，只会静默
  // 把已使用过的素材放到未使用过里，所以缺席补零这一步单独钉住。
  it('fills a missing id with zeros instead of leaving it undefined', () => {
    const byId = materialStatesById(STATES)
    expect(stateOf(byId, 33)).toEqual({ success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 0, last_task_at: null })
    expect(stateOf(byId, 999).success_count).toBe(0)
  })

  it('survives a null response', () => {
    expect(materialStatesById(null)).toEqual({})
    expect(stateOf(null, 12).success_count).toBe(0)
  })
})

describe('used / unused split', () => {
  // 一条 (策略, 素材) 只跑一次：有任何任务记录（成功 / 排队中 / 失败 / 已取消）都是已使用过，
  // 只有四个计数全为零的那条还能新建。
  it('counts any task record as used and only an all-zero pair as unused', () => {
    const byId = materialStatesById(STATES)
    expect(usageCounts(CANDIDATES, byId)).toEqual({ unused: 1, used: 3 })
    expect(hasTaskRecord({ success_count: 1, active_count: 0, failed_count: 0, cancelled_count: 0 })).toBe(true)
    expect(hasTaskRecord({ success_count: 0, active_count: 1, failed_count: 0, cancelled_count: 0 })).toBe(true)
    expect(hasTaskRecord({ success_count: 0, active_count: 0, failed_count: 2, cancelled_count: 0 })).toBe(true)
    expect(hasTaskRecord({ success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 1 })).toBe(true)
    expect(hasTaskRecord({ success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 0 })).toBe(false)
  })

  it('keeps only the never-run pair in 未使用过, with no tag and an 加入 entry', () => {
    const byId = materialStatesById(STATES)
    const rows = candidateRows(CANDIDATES, byId, 'unused', '')
    expect(rows.map((row) => row.id)).toEqual([33])
    expect(rows[0].tag).toBeNull()
    expect(addAction(rows[0].state, false)).toEqual({ label: '加入', disabled: false })
  })

  // 排队中的行在已使用过里、带标签、没有入口：这一刻它确实已经占着这条 (策略, 素材)。
  it('keeps an in-flight material in 已使用过, tagged 排队中 and without an entry', () => {
    const byId = materialStatesById(STATES)
    const row = candidateRows(CANDIDATES, byId, 'used', '').find((item) => item.id === 15)
    expect(row.tag).toEqual({ label: '排队中', tone: 'warning' })
    expect(addAction(row.state, false)).toBeNull()
  })

  it('labels a failure-only row 上次失败 and gives it no entry either', () => {
    const byId = materialStatesById([{ material_id: 21, success_count: 0, active_count: 0, failed_count: 2, cancelled_count: 0, last_task_at: '2026-10-14T07:00:00Z' }])
    const row = candidateRows(CANDIDATES, byId, 'used', '').find((item) => item.id === 21)
    expect(row.tag).toEqual({ label: '上次失败', tone: 'danger' })
    expect(addAction(row.state, false)).toBeNull()
  })

  // 已取消是最后一个进来的计数：漏掉它，取消过的素材会重新变成可选的，提交时才被服务端拒。
  it('labels a cancelled-only row 已取消 and gives it no entry either', () => {
    const byId = materialStatesById(STATES)
    const row = candidateRows(CANDIDATES, byId, 'used', '').find((item) => item.id === 21)
    expect(row.state.failed_count).toBe(0)
    expect(row.tag).toEqual({ label: '已取消', tone: 'muted' })
    expect(addAction(row.state, false)).toBeNull()
  })

  // 一次只显示一个标签，按对运营下一步动作的影响排序。
  it('shows one tag at a time, most actionable first', () => {
    expect(candidateTag({ success_count: 1, active_count: 1, failed_count: 3, cancelled_count: 1 })).toEqual({ label: '排队中', tone: 'warning' })
    expect(candidateTag({ success_count: 0, active_count: 0, failed_count: 3, cancelled_count: 1 })).toEqual({ label: '上次失败', tone: 'danger' })
    expect(candidateTag({ success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 2 })).toEqual({ label: '已取消', tone: 'muted' })
    expect(candidateTag({ success_count: 1, active_count: 0, failed_count: 0, cancelled_count: 0 })).toBeNull()
  })

  it('carries the state onto the row so the view does not look it up twice', () => {
    const byId = materialStatesById(STATES)
    const rows = candidateRows(CANDIDATES, byId, 'used', '')
    expect(rows.map((row) => row.id)).toEqual([21, 15, 12])
    expect(rows.find((row) => row.id === 12).state.success_count).toBe(2)
    expect(rows.find((row) => row.id === 12).title).toBe('开镜集锦')
  })
})

describe('candidate ordering', () => {
  const byId = materialStatesById(STATES)

  it('defaults to newest published first', () => {
    expect(CANDIDATE_SORTS[0].value).toBe('published_desc')
    expect(candidateRows(CANDIDATES, byId, 'used', '').map((row) => row.id)).toEqual([21, 15, 12])
  })

  // 两组都排：只有一组跟着变，另一半看起来就是坏的。
  it('sorts both groups, and leaves a row without published_at at the bottom', () => {
    const unused = [
      { id: 41, title: 'A', published_at: '2026-10-02T00:00:00Z', like_count: 5 },
      { id: 40, title: 'B', published_at: '2026-10-09T00:00:00Z', like_count: 50 },
      { id: 44, title: 'C', published_at: null, like_count: 7 },
      { id: 43, title: 'D', published_at: '2026-10-09T00:00:00Z', like_count: 9 },
    ]
    const noneUsed = materialStatesById([])
    // 同一天的两条按 id 降序兜底（43 在 40 前），没有发布时间的排在最后 —— 它不属于时间轴。
    expect(candidateRows(unused, noneUsed, 'unused', '').map((row) => row.id)).toEqual([43, 40, 41, 44])
    expect(candidateRows(unused, noneUsed, 'unused', '', 'likes_desc').map((row) => row.id)).toEqual([40, 43, 44, 41])
  })

  it('orders by likes when asked to', () => {
    expect(candidateRows(CANDIDATES, byId, 'used', '', 'likes_desc').map((row) => row.id)).toEqual([15, 12, 21])
  })

  // 排序是全序：渲染上限按行数截断，同键的行定不了序，截断边界就会随服务端返回顺序漂移。
  it('breaks a tie with the id so the same input always yields the same rows', () => {
    const sameDay = [
      { id: 7, title: 'A', published_at: '2026-10-09T00:00:00Z', like_count: 1 },
      { id: 9, title: 'B', published_at: '2026-10-09T00:00:00Z', like_count: 1 },
    ]
    expect(candidateRows(sameDay, materialStatesById([]), 'unused', '').map((row) => row.id)).toEqual([9, 7])
    expect(candidateRows(sameDay, materialStatesById([]), 'unused', '', 'likes_desc').map((row) => row.id)).toEqual([9, 7])
  })
})

describe('keyword', () => {
  it('matches the title and the material id', () => {
    expect(matchesKeyword({ id: 21, title: '夜景混剪' }, '夜景')).toBe(true)
    expect(matchesKeyword({ id: 21, title: '夜景混剪' }, '21')).toBe(true)
    expect(matchesKeyword({ id: 21, title: '夜景混剪' }, '跑图')).toBe(false)
    expect(matchesKeyword({ id: 21, title: '夜景混剪' }, '  ')).toBe(true)
  })

  it('narrows the rows without touching the counts', () => {
    const byId = materialStatesById(STATES)
    expect(candidateRows(CANDIDATES, byId, 'used', '夜景').map((row) => row.id)).toEqual([21])
    expect(usageCounts(CANDIDATES, byId)).toEqual({ unused: 1, used: 3 })
  })
})

describe('render cap', () => {
  it('counts what it omitted so the page can say so', () => {
    const rows = CANDIDATES.map((item) => ({ ...item }))
    expect(cappedRows(rows, [], 2)).toEqual({ rows: rows.slice(0, 2), omitted: 2 })
  })

  // 已选中的行不受上限影响：被截掉的行删不掉。
  it('never hides a selected row', () => {
    const rows = CANDIDATES.map((item) => ({ ...item }))
    const capped = cappedRows(rows, [33], 2)
    expect(capped.rows.map((row) => row.id)).toEqual([12, 15, 33])
    expect(capped.omitted).toBe(1)
  })

  it('keeps everything when the list is under the cap', () => {
    expect(cappedRows(CANDIDATES, [], 50).omitted).toBe(0)
    expect(cappedRows([], [], 50).rows).toEqual([])
  })
})

describe('row action', () => {
  // 已经跑过的 (策略, 素材) 不给按钮：抽屉里再开一个入口就成了第二条通路、第二个口径，
  // 重跑要对任务页那条任务点「重试」。
  it('offers no action at all for a pair that already has a task record', () => {
    expect(addAction({ success_count: 2, active_count: 0, failed_count: 1, cancelled_count: 0 }, false)).toBeNull()
    expect(addAction({ success_count: 0, active_count: 1, failed_count: 0, cancelled_count: 0 }, false)).toBeNull()
    expect(addAction({ success_count: 0, active_count: 0, failed_count: 2, cancelled_count: 0 }, false)).toBeNull()
    expect(addAction({ success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 1 }, false)).toBeNull()
  })

  it('says 加入 for a pair that never had a task', () => {
    expect(addAction({ success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 0 }, false)).toEqual({ label: '加入', disabled: false })
  })

  it('says 已加入 once the row is in the selection, so it cannot be added twice', () => {
    expect(addAction({ success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 0 }, true)).toEqual({ label: '已加入', disabled: true })
  })
})
