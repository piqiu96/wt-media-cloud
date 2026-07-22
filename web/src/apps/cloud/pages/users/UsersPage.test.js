import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./UsersPage.vue', import.meta.url), 'utf8')

describe('UsersPage product surface', () => {
  it('shows numeric UID, filters, pagination, edit operations and one-time password result', () => {
    for (const required of ['用户管理', 'UID', '运营分组', '筛选', '编辑用户', '重置密码', '解除比特绑定', '一次性密码', '停用', '删除', 't-pagination']) {
      expect(source).toContain(required)
    }
  })

  it('declares BitBrowser unbind as non-destructive administrator repair', () => {
    expect(source).toContain('解除后，该用户需要在 Desktop 重新绑定正确的比特浏览器账号')
    expect(source).toContain('系统不会删除已有浏览器窗口、媒体账号和历史记录')
    expect(source).toContain('clearBitBrowserBinding')
  })

  it('uses configured game options instead of free text game IDs', () => {
    expect(source).toContain('请选择已启用游戏')
    expect(source).toContain('enabledGames')
    expect(source).toContain('Array.isArray(gameIds) ? gameIds : []')
    expect(source).not.toContain('输入游戏 ID 后回车')
  })

  it('uses admin and no longer exposes the retired technician role', () => {
    expect(source).toContain("value=\"admin\"")
    expect(source).not.toContain('technician')
  })

  it('clears every plaintext password field when the one-time result closes', () => {
    const clearFunction = source.match(/function clearOneTimePassword\(\) \{([\s\S]*?)\n\}/)?.[1] || ''
    expect(clearFunction).toContain("oneTimePassword.value = ''")
    expect(clearFunction).toContain("userForm.value.password = ''")
    expect(clearFunction).toContain("newPassword.value = ''")
  })

  it('declares one-time passwords as readable response fields', () => {
    const businessSchema = readFileSync(new URL('../../../../../../contracts/business-schemas/v1/identity.yaml', import.meta.url), 'utf8')
    const openapiSchema = readFileSync(new URL('../../../../../../contracts/cloud-api/v1/identity.openapi.yaml', import.meta.url), 'utf8')
    expect(businessSchema.match(/one_time_password: \{[^}]*\}/)?.[0]).not.toContain('writeOnly')
    for (const declaration of openapiSchema.match(/one_time_password: \{[^}]*\}/g) || []) {
      expect(declaration).not.toContain('writeOnly')
    }
  })
})
