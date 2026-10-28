import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import MaterialLibraryPage from './modules/materials/pages/MaterialLibraryPage.vue'
import MyMaterialsPage from './modules/materials/pages/MyMaterialsPage.vue'
import DiscoveryStrategiesPage from './modules/contentpool/pages/DiscoveryStrategiesPage.vue'
import CrawlTasksPage from './modules/contentpool/pages/CrawlTasksPage.vue'
import ContentPoolPage from './modules/contentpool/pages/ContentPoolPage.vue'
import AccountsPage from './modules/accounts/pages/AccountsPage.vue'
import ProfilesPage from './modules/profiles/pages/ProfilesPage.vue'
import ProxyPage from './modules/proxy/pages/ProxyPage.vue'
import UsersPage from './apps/cloud/pages/users/UsersPage.vue'
import TeamsPage from './apps/cloud/pages/users/TeamsPage.vue'
import CategoriesPage from './apps/cloud/pages/users/CategoriesPage.vue'

const pages = [
  ['素材库', MaterialLibraryPage, './modules/materials/pages/MaterialLibraryPage.vue'],
  ['我的素材', MyMaterialsPage, './modules/materials/pages/MyMaterialsPage.vue'],
  ['挖掘策略', DiscoveryStrategiesPage, './modules/contentpool/pages/DiscoveryStrategiesPage.vue'],
  ['挖掘任务', CrawlTasksPage, './modules/contentpool/pages/CrawlTasksPage.vue'],
  ['内容池', ContentPoolPage, './modules/contentpool/pages/ContentPoolPage.vue'],
  ['社媒账号', AccountsPage, './modules/accounts/pages/AccountsPage.vue'],
  ['浏览器窗口', ProfilesPage, './modules/profiles/pages/ProfilesPage.vue'],
  ['代理管理', ProxyPage, './modules/proxy/pages/ProxyPage.vue'],
  ['用户管理', UsersPage, './apps/cloud/pages/users/UsersPage.vue'],
  ['运营团队', TeamsPage, './apps/cloud/pages/users/TeamsPage.vue'],
  ['类目管理', CategoriesPage, './apps/cloud/pages/users/CategoriesPage.vue'],
]

describe('resource search layout', () => {
  for (const [name, component, path] of pages) {
    it(`${name} compiles with filters in the existing resource card`, () => {
      const source = readFileSync(new URL(path, import.meta.url), 'utf8')
      expect(component).toBeTruthy()
      expect(source).toContain('class="filter-bar"')
      expect(source).toContain('class="wt-filter-actions"')
      expect(source).toContain('class="table-scroll-wrap"')
      expect(source.indexOf('class="filter-bar"')).toBeLessThan(source.indexOf('class="table-scroll-wrap"'))
      expect(source).not.toContain('class="filter-row"')
    })
  }
})
