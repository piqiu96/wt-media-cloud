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
