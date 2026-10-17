// 「这条素材在这个策略下用过没有」—— 创建抽屉按它把候选分成未使用过 / 已使用过两组。
//
// 口径（与后端 ComposeMaterialState 同一个判据）：**已使用过 = 这条策略在这条素材上有过任务记录**，
// 排队中 / 成功 / 失败 / 已取消 四者任一都算，四个计数全为零才是未使用过。一条 (策略, 素材)
// 只跑一次：已使用过的行只读、连按钮都不给，重跑一律走任务页对应任务的「重试」。
//
// 全部是纯函数：分组、标签、计数、关键词、排序、渲染上限都按同一份状态算，所以列表里看到的和
// 下拉里的数字不会各说各话。

export const CANDIDATE_SORTS = [
  { value: 'published_desc', label: '发布时间倒序' },
  { value: 'likes_desc', label: '点赞倒序' },
]

const EMPTY_STATE = { success_count: 0, active_count: 0, failed_count: 0, cancelled_count: 0, last_task_at: null }

// 服务端保证每个请求到的 ID 都有一条，缺席补零：把「没返回」当成「没用过」是本功能唯一
// 无法自察的错误答案，所以补零这一步显式做，且调用方拿到的永远是完整映射。
export function materialStatesById(states) {
  const byId = {}
  for (const state of states || []) {
    if (!state || state.material_id === undefined || state.material_id === null) continue
    byId[state.material_id] = state
  }
  return byId
}

export function stateOf(byId, materialId) {
  return (byId && byId[materialId]) || EMPTY_STATE
}

// 已使用过 = 有过任务记录。服务端创建守卫按同一个判据拒收，所以这里画成只读的行正是
// 提交时会被拒的那些 —— 两边不会给出不同的答案。
export function isUsed(state) {
  return (
    Number(state?.success_count || 0) > 0 ||
    Number(state?.active_count || 0) > 0 ||
    Number(state?.failed_count || 0) > 0 ||
    Number(state?.cancelled_count || 0) > 0
  )
}

// 名字说明判据本身：有过任务记录。isUsed 是分组用的说法，两者是同一个判据。
export const hasTaskRecord = isUsed

// 这条 (策略, 素材) 还有排队中或执行中的任务，用来决定行的标签。
export function isInFlight(state) {
  return Number(state?.active_count || 0) > 0
}

// 一次只显示一个标签，按对运营下一步动作的影响排序：排队中（在跑，等它）> 上次失败（可重试）
// > 已取消（可重试）。tone 用的是已加载样式表里真有的三个色。
export function candidateTag(state) {
  if (isInFlight(state)) return { label: '排队中', tone: 'warning' }
  if (Number(state?.failed_count || 0) > 0) return { label: '上次失败', tone: 'danger' }
  if (Number(state?.cancelled_count || 0) > 0) return { label: '已取消', tone: 'muted' }
  return null
}

// 行上的那一个动作，null 表示这一行不给按钮。已使用过的行没有「加入」以外的说法可用：
// 重跑要在任务页对着那条任务重试，抽屉里再开一个入口就成了第二条通路、第二个口径。
export function addAction(state, alreadySelected) {
  if (isUsed(state)) return null
  if (alreadySelected) return { label: '已加入', disabled: true }
  return { label: '加入', disabled: false }
}

export function matchesKeyword(candidate, keyword) {
  const needle = String(keyword || '').trim().toLowerCase()
  if (!needle) return true
  return `${candidate.title || ''} ${candidate.id}`.toLowerCase().includes(needle)
}

// 两组各自的条数，供下拉里的「未使用过（5）」。分母是传进来的这批候选（游戏筛选之后、
// 关键词之前）：跟着输入框抖动的数字不算信息。
export function usageCounts(candidates, byId) {
  const counts = { unused: 0, used: 0 }
  for (const candidate of candidates || []) {
    if (isUsed(stateOf(byId, candidate.id))) counts.used += 1
    else counts.unused += 1
  }
  return counts
}

// 两种排序都是全序：渲染上限按行数截断，同键的行如果不能定序，截断的边界就会随
// 服务端返回顺序漂移。相同键一律用 id 降序兜底。
function compareCandidates(sort) {
  if (sort === 'likes_desc') {
    return (left, right) => Number(right.like_count || 0) - Number(left.like_count || 0) || right.id - left.id
  }
  return (left, right) => {
    const leftAt = left.published_at ? String(left.published_at) : ''
    const rightAt = right.published_at ? String(right.published_at) : ''
    // 发布时间是 ISO 8601 固定宽度，字典序即时间序；没带时间的那几条排在最后，不混进时间轴里。
    if (leftAt !== rightAt) {
      if (!leftAt) return 1
      if (!rightAt) return -1
      return rightAt.localeCompare(leftAt)
    }
    return right.id - left.id
  }
}

// 一个分组里的行，两组都按同一个排序字段排 —— 运营切排序时想看的是整份候选的次序，
// 只有一组跟着变会让人以为另一半是坏的。
export function candidateRows(candidates, byId, usage, keyword, sort = 'published_desc') {
  const rows = []
  for (const candidate of candidates || []) {
    const state = stateOf(byId, candidate.id)
    if ((usage === 'used') !== isUsed(state)) continue
    if (!matchesKeyword(candidate, keyword)) continue
    rows.push({ ...candidate, state, tag: candidateTag(state) })
  }
  rows.sort(compareCandidates(sort))
  return rows
}

// 渲染上限只作用于未选中的行：已选中的行被截掉就删不掉了。返回被省略的条数，让页面能
// 说清「还有 N 条，用搜索缩小范围」，而不是安静地少显示几行。
export function cappedRows(rows, selectedIds, limit) {
  const selected = new Set(selectedIds || [])
  const kept = []
  let omitted = 0
  for (const row of rows) {
    if (selected.has(row.id) || kept.length < limit) {
      kept.push(row)
      continue
    }
    omitted += 1
  }
  return { rows: kept, omitted }
}
