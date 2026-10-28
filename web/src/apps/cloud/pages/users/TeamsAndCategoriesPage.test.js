import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const teamsSource = readFileSync(new URL('./TeamsPage.vue', import.meta.url), 'utf8')
const categoriesSource = readFileSync(new URL('./CategoriesPage.vue', import.meta.url), 'utf8')
const appLayoutSource = readFileSync(new URL('../../../../layout/AppLayout.vue', import.meta.url), 'utf8')

describe('M2-A3 management pages', () => {
  it('lists team and category management in the shared menu, gated by role', () => {
    expect(appLayoutSource).toContain('用户管理')
    expect(appLayoutSource).toContain('运营团队')
    expect(appLayoutSource).toContain('类目管理')
    expect(appLayoutSource).toContain("'/operation-teams'")
    expect(appLayoutSource).toContain("'/categories'")
    // 2026-10-27：这三项从「仅 Cloud 的 adminOnly」改为按角色的公共路径表，
    // Desktop 与管理员的可见性都由它决定。
    expect(appLayoutSource).toContain('SYSTEM_MANAGEMENT_PATHS')
  })

  it('gives operation team page search/list/pagination basics', () => {
    for (const required of ['运营团队', '综合搜索', '团队 ID / 名称', '查询', '重置', '用户数', 't-table', 't-pagination', '该团队下还有']) {
      expect(teamsSource).toContain(required)
    }
    expect(teamsSource).toContain(':disabled="row.user_count > 0"')
  })

  it('sends the team keyword to the backend and paginates the returned rows', () => {
    expect(teamsSource).toContain('client.listTeams({ keyword: keyword.value.trim() })')
    expect(teamsSource).not.toContain('filteredTeams')
    expect(teamsSource).toContain(':total="teams.length"')
  })

  // 用户数由服务端随团队返回：读用户列表算人数对高级运营是 403，整列都会变成 0。
  it('reads the membership count from the team list instead of counting users', () => {
    expect(teamsSource).toContain('team.user_count ?? 0')
    expect(teamsSource).not.toContain('listUsers')
  })

  it('keeps team writes with the admin and shows seniors the same list', () => {
    expect(teamsSource).toContain('const canManage = computed(() => isAdminRole(currentUser.value))')
    expect(teamsSource).toContain('v-if="canManage" theme="primary" @click="openCreate"')
    expect(teamsSource).toContain("if (canManage.value) base.push({ colKey: 'operations'")
  })

  it('gives category page search/list/filter/pagination basics', () => {
    for (const required of ['类目管理', '综合搜索', '类目 ID / 名称', '状态', '查询', '重置', '关联情况', '用户授权', '媒体账号', '新增类目', 't-table', 't-pagination', '查看关联详情']) {
      expect(categoriesSource).toContain(required)
    }
    expect(categoriesSource).toContain(":disabled=\"row.enabled && hasReferences(row)\"")
    expect(categoriesSource).toContain(':disabled="hasReferences(row)"')
  })

  // 高级运营读的是 management 视图：全部状态、无关联摘要、无写入口。
  it('gives seniors the read-only management view and admins the full one', () => {
    expect(categoriesSource).toContain("const scope = canManage.value ? 'all' : 'management'")
    expect(categoriesSource).toContain('client.listCategories({ ...filters.value, scope })')
    expect(categoriesSource).toContain('v-if="canManage" theme="primary" @click="openCreate"')
    expect(categoriesSource).toContain('if (!canManage.value) return')
    expect(categoriesSource).toContain("if (canManage.value) base.push({ colKey: 'references'")
    expect(categoriesSource).toContain("if (canManage.value) base.push({ colKey: 'operations'")
  })
})
