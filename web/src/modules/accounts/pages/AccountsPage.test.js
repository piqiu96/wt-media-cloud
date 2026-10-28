import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./AccountsPage.vue', import.meta.url), 'utf8')

describe('media account resource page', () => {
  it('uses shared resource layout and preserves account operations', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('wt-resource-table')
    expect(source).toContain('resourceAccountStats')
    expect(source).toContain('@click="openCreate"')
    expect(source).toContain('@click="checkFromRow(row)"')
    expect(source).toContain('@click="toggleEnabled(row)"')
  })

  it('shows the routine account operations directly without a separate cookie column', () => {
    expect(source).not.toContain('<t-dropdown')
    expect(source).not.toContain('{ colKey: "cookie", title: "Cookie"')
    expect(source).toContain('@click="openCookieDialog(row)"')
    expect(source).toContain('@click="openEdit(row)"')
  })

  it('uses the browser-window resource table structure', () => {
    expect(source).toContain('class="filter-bar"')
    expect(source).toContain('class="table-scroll-wrap"')
    expect(source).toContain(":scroll=\"{ x: 'max-content' }\"")
    expect(source).toContain('class="pagination-bar"')
  })

  it('keeps account filter labels and a six-action-width operation column', () => {
    expect(source).toContain('>综合搜索</span>')
    expect(source).toContain('>账号状态</span>')
    expect(source).toContain('{ colKey: "op", title: "操作", width: 400, fixed: "right" }')
  })

  // 2026-10-27：列表要能看到账号归谁用，高级运营/管理员按使用人收窄，普通运营不显示该筛选。
  it('shows the account owner and narrows the list by owner for seniors and admins', () => {
    expect(source).toContain('{ colKey: "user_name", title: "使用人", width: 100 }')
    expect(source).toContain("row.user_name || '--'")
    expect(source).toContain('const canFilterByOwner = computed(')
    expect(source).toContain('function defaultOwnerFilter()')
    expect(source).toContain('>使用人</span>')
    expect(source).toContain(':total="ownerFilteredAccounts.length"')
    expect(source).toContain('ownerFilteredAccounts.value.slice(start, start + pagination.value.pageSize)')
  })

  it('keeps media-account category selections within one code-owned type', () => {
    expect(source).toContain('function categoryOptionDisabled(category, categoryIds)')
    expect(source).toContain('同一个社媒账号的多个类目必须属于同一一级类型')
    expect(source).toContain(':disabled="categoryOptionDisabled(category, createForm.categoryIds)"')
    expect(source).toContain(':disabled="categoryOptionDisabled(category, editCategoryIds)"')
  })

  // 2026-10-27：本地操作按账号归属：高级运营只能动本人账号，管理员一律只读。
  it('gates local account work by ownership on desktop', () => {
    expect(source).toContain('function canOperateLocalAccount(account)')
    expect(source).toContain('return isOwnAccount(account)')
    expect(source).toContain('管理员不执行本地操作')
    expect(source).toContain('仅账号本人可执行本地操作')
    expect(source).toContain(':disabled="isDesktop && !canOperateLocalAccount(row)"')
    expect(source).toContain(':disabled="!canOperateLocalAccount(editAccount)"')
    expect(source).toContain('已跳过 ${nonOwnedCount} 条非本人账号')
  })
})
