import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const teamsSource = readFileSync(new URL('./TeamsPage.vue', import.meta.url), 'utf8')
const gamesSource = readFileSync(new URL('./GamesPage.vue', import.meta.url), 'utf8')
const appLayoutSource = readFileSync(new URL('../../../../layout/AppLayout.vue', import.meta.url), 'utf8')

describe('M2-A3 management pages', () => {
  it('splits team and game management into cloud-only menu entries', () => {
    expect(appLayoutSource).toContain('用户管理')
    expect(appLayoutSource).toContain('运营分组')
    expect(appLayoutSource).toContain('游戏管理')
    expect(appLayoutSource).toContain("'/operation-teams'")
    expect(appLayoutSource).toContain("'/games'")
  })

  it('gives operation team page search/list/pagination basics', () => {
    for (const required of ['运营分组', '综合搜索', '分组 ID / 名称', '查询', '重置', '用户数', 't-table', 't-pagination', '该分组下还有']) {
      expect(teamsSource).toContain(required)
    }
    expect(teamsSource).toContain(':disabled="row.user_count > 0"')
  })

  it('gives game page search/list/filter/pagination basics', () => {
    for (const required of ['游戏管理', '综合搜索', '游戏 ID / 名称', '状态', '查询', '重置', '关联情况', '用户授权', '媒体账号', '新建游戏', 't-table', 't-pagination', '查看关联详情']) {
      expect(gamesSource).toContain(required)
    }
    expect(gamesSource).toContain(":disabled=\"row.status === 'enabled' && hasReferences(row)\"")
    expect(gamesSource).toContain(':disabled="hasReferences(row)"')
  })
})
