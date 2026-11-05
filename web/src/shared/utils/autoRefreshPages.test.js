import { publishedRangeParams, metricRangeParams } from '../../modules/materials/materialFilters.js'
import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'

// 内容挖掘 / 内容生产 / 系统设置的 11 个列表页：用户 2026-10-27 裁定不设刷新按钮，
// 改为 10 秒自动刷新。这里逐页钉住三件会静默失效的事：定时器是否接上、quiet 读取
// 是否真的不重置页码、按钮是否真的没了。
// 第四项标明分页方式：服务端分页实际运行 load 检查请求和页码，其余页面保留
// 既有 quiet / resetPage 开关约束（定时那一遍不要求回第一页）。
const PAGES = [
  ['内容池', 'src/modules/contentpool/pages/ContentPoolPage.vue', 'load'],
  ['挖掘策略', 'src/modules/contentpool/pages/DiscoveryStrategiesPage.vue', 'load'],
  ['挖掘任务', 'src/modules/contentpool/pages/CrawlTasksPage.vue', 'load'],
  ['素材库', 'src/modules/materials/pages/MaterialLibraryPage.vue', 'load', 'server'],
  ['我的素材', 'src/modules/materials/pages/MyMaterialsPage.vue', 'load', 'server'],
  ['合成策略', 'src/modules/compose/pages/ComposeStrategyPage.vue', 'load', 'server'],
  ['合成任务', 'src/modules/compose/pages/ComposeTasksPage.vue', 'load', 'server'],
  ['成片管理', 'src/modules/compose/pages/MyOutputsPage.vue', 'load', 'if (resetPage) pagination.value.current = 1'],
  ['用户管理', 'src/apps/cloud/pages/users/UsersPage.vue', 'loadAll'],
  ['运营团队', 'src/apps/cloud/pages/users/TeamsPage.vue', 'loadAll'],
  ['类目管理', 'src/apps/cloud/pages/users/CategoriesPage.vue', 'loadCategories'],
]

function read(path) {
  return readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8')
}

// 运行页面自身的读取函数，避免把“保留页码”绑定到某一种 if 写法。
function serverLoader(path, client, overrides = {}) {
  const source = read(path)
  const start = source.indexOf('async function load(')
  const load = source.slice(start, source.indexOf('\n}', start) + 2)
  const ref = (value) => ({ value })
  const state = {
    appliedQuery: ref(null), appliedAdvancedCount: ref(0), appliedFilters: ref(null),
    client, sessionClient: { me: async () => ({role:'admin'}) }, currentUser: ref({role:'admin'}), templates: ref([]), categories: ref([]), teams: ref([]), pagination: ref({ current: 3, pageSize: 25 }),
    rows: ref([{ id: 'previous' }]), total: ref(72), loading: ref(false), error: ref('previous notice'),
    filters: ref({ search: ' query ', created_range: [] }), activeView: ref('batches'),
    isAdmin: ref(false), isSenior: ref(false), knownTaskRows: ref({}),
    search: ref('query'), teamFilter: ref(''), categoryFilters: ref([]), joinedFilter: ref(''),
    myUsageFilter: ref(''), teamUsageFilter: ref(''), statusFilter: ref(''),
    sortBy: ref('published_at'), sortOrder: ref('desc'), selectedRowKeys: ref([]),
    operatorIdFromOption: () => null, createdRangeParams: () => ({}),
    publishedRangeParams, metricRangeParams, publishedRange: ref([]), metricFilters: ref({}), usageStatusFilter: ref('active'),
    toRows: (items) => items, loadPresence: async () => {}, ...overrides,
  }
  const run = new Function(...Object.keys(state), 'options', `let loadSequence = 0; ${load}; return load(options)`)
  return { state, run: (options={quiet:true}) => run(...Object.values(state), options) }
}

const SERVER_PAGES = PAGES.filter(([, , , pagination]) => pagination === 'server')

describe('list pages auto refresh', () => {
  it.each(PAGES)('%s refreshes on a timer instead of a button', (_name, path, loader, pageGuard = 'if (!quiet) pagination.value.current = 1') => {
    const source = read(path)
    expect(source).toContain("import { useAutoRefresh } from '")
    expect(source).toContain(`useAutoRefresh(() => ${loader}({ quiet: true })`)
    // 让开用户正在做的事：每个页面都必须给出跳过判据。
    expect(source).toContain('shouldSkip: () =>')
    // quiet 一遍不重置页码、不闪整页 loading。
    expect(source).toContain('if (!quiet) loading.value = true')
    if (pageGuard !== 'server') expect(source).toContain(pageGuard)
    // 刷新按钮已移除（查询/重置按钮不受影响）。
    expect(source).not.toContain('>刷新<')
  })

  it.each(SERVER_PAGES)('%s quietly rereads the current server page and preserves filters', async (_name, path) => {
    const request = vi.fn().mockResolvedValue({ items: [{ id: 'updated' }], total: 73 })
    const { state, run } = serverLoader(path, { list: request, listMyMaterials: request, listComposeTaskBatches: request, listComposeTasks: request, listStrategies: request, listTemplates: async () => [], listCategories: async () => [], listTeams: async () => [] })
    await run()
    expect(request).toHaveBeenCalledOnce()
    expect(request.mock.calls[0][0]).toMatchObject({ page: 3, page_size: 25, search: 'query' })
    expect(state.pagination.value).toEqual({ current: 3, pageSize: 25 })
    expect(state.filters.value.search).toBe(' query ')
    expect(state.rows.value).toEqual([{ id: 'updated' }])
    expect(state.total.value).toBe(73)
    expect(state.error.value).toBe('previous notice')
    expect(state.loading.value).toBe(false)
  })

  it.each(SERVER_PAGES)('%s keeps the last server page when a quiet read fails', async (_name, path) => {
    const request = vi.fn().mockRejectedValue(new Error('offline'))
    const { state, run } = serverLoader(path, { list: request, listMyMaterials: request, listComposeTaskBatches: request, listComposeTasks: request, listStrategies: request, listTemplates: async () => [], listCategories: async () => [], listTeams: async () => [] })
    await run()
    expect(state.pagination.value).toEqual({ current: 3, pageSize: 25 })
    expect(state.rows.value).toEqual([{ id: 'previous' }])
    expect(state.total.value).toBe(72)
    expect(state.error.value).toBe('previous notice')
    expect(state.loading.value).toBe(false)
  })

  it.each(SERVER_PAGES)('%s does not apply an unsubmitted draft during polling', async (_name,path) => {
    const request=vi.fn().mockResolvedValue({items:[],total:0})
    const {state,run}=serverLoader(path,{list:request,listMyMaterials:request,listComposeTaskBatches:request,listComposeTasks:request,listStrategies:request,listTemplates:async()=>[],listCategories:async()=>[],listTeams:async()=>[]})
    await run()
    state.search.value='not submitted'
    state.filters.value.search='not submitted'
    await run()
    expect(request).toHaveBeenLastCalledWith(expect.objectContaining({search:'query',page:3}))
  })
  it('clears task selection when applying a new query',async()=>{
    const source=read('src/modules/compose/pages/ComposeTasksPage.vue')
    const query=source.match(/function query\(\) \{[^\n]+\}/)[0]
    const selectedTaskKeys={value:[12]},pagination={value:{current:3}},load=vi.fn()
    new Function('selectedTaskKeys','pagination','load',`${query};query()`)(selectedTaskKeys,pagination,load)
    expect(selectedTaskKeys.value).toEqual([])
    expect(pagination.value.current).toBe(1)
    expect(load).toHaveBeenCalledWith({applyFilters:true})
  })

  // 账号属于本机执行面：移除独立刷新入口，仍由查询和操作完成后读取最新结果，
  // 不把内容列表的定时刷新约束强加给本机执行页。
  it('accounts has no standalone refresh button', () => {
    expect(read('src/modules/accounts/pages/AccountsPage.vue')).not.toContain('>刷新<')
  })
})
