import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const proxyPage = readFileSync(new URL('./modules/proxy/pages/ProxyPage.vue', import.meta.url), 'utf8')
const profilesPage = readFileSync(new URL('./modules/profiles/pages/ProfilesPage.vue', import.meta.url), 'utf8')

describe('proxy operation page boundary', () => {
  it('keeps BitBrowser proxy mutations out of proxy management', () => {
    expect(proxyPage).not.toContain('同步至窗口')
    expect(proxyPage).not.toContain('createSyncToProfile')
    expect(proxyPage).not.toContain('scanLocalProxies')
    expect(proxyPage).not.toContain('confirmLocalProxyScan')
  })

  it('puts proxy binding and readback in the browser profiles page', () => {
    expect(profilesPage).toContain("createProxyClient")
    expect(profilesPage).toContain('绑定代理')
    expect(profilesPage).toContain('proxyClient.assign')
    expect(profilesPage).toContain('proxyClient.unbind')
    expect(profilesPage).toContain('proxyClient.previewLocalScan')
    expect(profilesPage).toContain('批量绑定代理')
    expect(profilesPage).toContain('proxyClient.recommend')
    expect(profilesPage).toContain('proxyClient.assignBatch')
  })

  it('keeps batch checks and ledger actions in proxy management', () => {
    expect(proxyPage).toContain('批量检测')
    expect(proxyPage).toContain('@click="openEdit(row)"')
    expect(proxyPage).toContain('>详情</t-button>')
    expect(proxyPage).toContain('>检测</t-button>')
    expect(proxyPage).toContain('>配额</t-button>')
    expect(proxyPage).toContain('>删除</t-button>')
    expect(proxyPage).not.toContain('>状态</t-button>')
  })

  it('makes unavailable proxy choices and pending sync visible in browser windows', () => {
    expect(profilesPage).toContain('暂无可绑定代理')
    expect(profilesPage).toContain('代理配置待同步')
  })
})
