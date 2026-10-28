import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const proxyPage = readFileSync(new URL('./modules/proxy/pages/ProxyPage.vue', import.meta.url), 'utf8')
const profilesPage = readFileSync(new URL('./modules/profiles/pages/ProfilesPage.vue', import.meta.url), 'utf8')

describe('proxy operation page boundary', () => {
  it('keeps BitBrowser window mutations out of proxy management', () => {
    expect(proxyPage).not.toContain('同步至窗口')
    expect(proxyPage).not.toContain('profileCreate')
    expect(proxyPage).not.toContain('profileUpdate')
    expect(proxyPage).not.toContain('profileRestore')
    expect(proxyPage).not.toContain('profileScan')
    expect(proxyPage).not.toContain('confirmLocalScan')
  })

  it('puts proxy binding and readback in the browser profiles page', () => {
    expect(profilesPage).toContain('createProxyClient')
    expect(profilesPage).toContain('"管理代理"')
    expect(profilesPage).toContain('proxyClient.assign(')
    expect(profilesPage).toContain('proxyClient.unbind(')
    expect(profilesPage).toContain('proxyClient.release(')
    expect(profilesPage).toContain('proxyClient.clearProfileProxy(')
    expect(profilesPage).toContain('proxyClient.previewLocalScan(')
    expect(profilesPage).toContain('proxyClient.confirmLocalScan(')
    expect(profilesPage).toContain('>批量设置代理</t-button>')
  })

  it('keeps proxy health checks and ledger actions in proxy management', () => {
    expect(proxyPage).toContain('批量检测（')
    expect(proxyPage).toContain('@click="openEdit(row)"')
    expect(proxyPage).toContain('>详情</t-button>')
    expect(proxyPage).toContain('>检测</t-button>')
    expect(proxyPage).toContain('>设置配额</t-button>')
    expect(proxyPage).toContain('>删除</t-button>')
    expect(proxyPage).not.toContain('>状态</t-button>')
  })

  it('makes unavailable proxy choices and unsynced proxy edits visible', () => {
    expect(profilesPage).toContain('当前团队暂无代理')
    expect(profilesPage).toContain('当前代理均不可分配')
    expect(proxyPage).toContain('个关联窗口需同步')
  })
})
