import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// 内容挖掘 / 内容生产 / 系统设置的 11 个列表页：用户 2026-10-27 裁定不设刷新按钮，
// 改为 10 秒自动刷新。这里逐页钉住三件会静默失效的事：定时器是否接上、quiet 读取
// 是否真的不重置页码、按钮是否真的没了。
// 第四项是该页「不重置页码」的那一行：多数页面按 quiet 区分，成片管理沿用
// 既有的 resetPage 开关（定时那一遍不带它）。
const PAGES = [
  ['内容池', 'src/modules/contentpool/pages/ContentPoolPage.vue', 'load'],
  ['挖掘策略', 'src/modules/contentpool/pages/DiscoveryStrategiesPage.vue', 'load'],
  ['挖掘任务', 'src/modules/contentpool/pages/CrawlTasksPage.vue', 'load'],
  ['素材库', 'src/modules/materials/pages/MaterialLibraryPage.vue', 'load'],
  ['我的素材', 'src/modules/materials/pages/MyMaterialsPage.vue', 'load'],
  ['合成策略', 'src/modules/compose/pages/ComposeStrategyPage.vue', 'load'],
  ['合成任务', 'src/modules/compose/pages/ComposeTasksPage.vue', 'load'],
  ['成片管理', 'src/modules/compose/pages/MyOutputsPage.vue', 'load', 'if (resetPage) pagination.value.current = 1'],
  ['用户管理', 'src/apps/cloud/pages/users/UsersPage.vue', 'loadAll'],
  ['运营团队', 'src/apps/cloud/pages/users/TeamsPage.vue', 'loadAll'],
  ['类目管理', 'src/apps/cloud/pages/users/CategoriesPage.vue', 'loadCategories'],
]

function read(path) {
  return readFileSync(new URL(`../../../${path}`, import.meta.url), 'utf8')
}

describe('list pages auto refresh', () => {
  it.each(PAGES)('%s refreshes on a timer instead of a button', (_name, path, loader, pageGuard = 'if (!quiet) pagination.value.current = 1') => {
    const source = read(path)
    expect(source).toContain("import { useAutoRefresh } from '")
    expect(source).toContain(`useAutoRefresh(() => ${loader}({ quiet: true })`)
    // 让开用户正在做的事：每个页面都必须给出跳过判据。
    expect(source).toContain('shouldSkip: () =>')
    // quiet 一遍不重置页码、不闪整页 loading。
    expect(source).toContain('if (!quiet) loading.value = true')
    expect(source).toContain(pageGuard)
    // 刷新按钮已移除（查询/重置按钮不受影响）。
    expect(source).not.toContain('>刷新<')
  })

  // 账号属于本机执行面：移除独立刷新入口，仍由查询和操作完成后读取最新结果，
  // 不把内容列表的定时刷新约束强加给本机执行页。
  it('accounts has no standalone refresh button', () => {
    expect(read('src/modules/accounts/pages/AccountsPage.vue')).not.toContain('>刷新<')
  })
})
