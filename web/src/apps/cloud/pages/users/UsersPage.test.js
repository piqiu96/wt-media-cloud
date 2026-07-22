import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./UsersPage.vue', import.meta.url), 'utf8')

describe('UsersPage product surface', () => {
  it('shows numeric UID, team administration, filters, edit operations and one-time password result', () => {
    for (const required of ['UID', '运营分组', '筛选', '编辑用户', '重置密码', '一次性密码', '停用', '删除']) {
      expect(source).toContain(required)
    }
  })

  it('uses admin and no longer exposes the retired technician role', () => {
    expect(source).toContain("value=\"admin\"")
    expect(source).not.toContain('technician')
  })
})
