import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./AppLayout.vue', import.meta.url), 'utf8')

describe('application navigation', () => {
  it('hides account opening while retaining the resource navigation entries', () => {
    expect(source).not.toContain("title: '账号开户'")
    expect(source).toContain("title: '运营资源'")
    expect(source).toContain("path: '/browser-windows'")
    expect(source).toContain("path: '/proxies'")
    expect(source).toContain("path: '/accounts'")
  })

  it('renders functional menu groups as collapsible submenus', () => {
    expect(source).toContain('<t-submenu')
    expect(source).toContain('v-model:expanded="expandedGroups"')
  })

  it('keeps expanded groups across reloads while locating the active route', () => {
    expect(source).toContain("const SIDEBAR_EXPANDED_GROUPS_KEY = 'wt-media:sidebar-expanded-groups'")
    expect(source).toContain('localStorage.setItem(SIDEBAR_EXPANDED_GROUPS_KEY')
    expect(source).toContain('!expandedGroups.value.includes(activeGroup.value)')
  })

  it('renders resource pages with their navigation hierarchy instead of internal route names', () => {
    expect(source).toContain('const breadcrumbItems = computed')
    expect(source).toContain('group.title, child.title')
    expect(source).toContain('v-for="item in breadcrumbItems"')
  })

  // 下载中心只有一个入口，而它必须真的接上：顶栏那颗按钮、页面调用的那个 store、
  // 以及挂载的面板组件，三样缺一样就是「点了没反应」，而那种失败不会报错。
  it('wires the download centre into the topbar', () => {
    expect(source).toContain("from '../modules/transfer/DownloadCentreDrawer.vue'")
    expect(source).toContain("from '../modules/transfer/downloadCentre.js'")
    expect(source).toContain('const downloadCentre = useDownloadCentre()')
    expect(source).toContain('@click="downloadCentre.open"')
    expect(source).toContain('<DownloadCentreDrawer />')
    // 图标名必须在精灵里存在，否则两端都渲染成空白（见上面 resource-management 那条注释）。
    // `t-icon-download` 已核对于 web/public/tdesign-icons/0.4.3/index.js。
    expect(source).toContain('<t-icon name="download" />')
  })

  it('uses the modern oriental sidebar visual language', () => {
    expect(source).toContain('background: #FAFAF8')
    expect(source).toContain('rgba(30, 64, 175, 0.06)')
    expect(source).toContain('border-left: 3px solid #2563EB')
    expect(source).toContain('font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans SC", sans-serif')
  })
})
