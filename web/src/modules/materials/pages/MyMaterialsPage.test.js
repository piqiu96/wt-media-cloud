import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MyMaterialsPage.vue', import.meta.url), 'utf8')

describe('my materials page', () => {
  /**
   * 一行是**一条关系**，而它展示的是嵌入的素材。
   *
   * 服务端把 `material` 嵌在每条 usage 上（冻结的 MaterialUsage 就是这么定义的），
   * 所以这一页不需要第二个请求 —— 但两个 id 必须分开：`row.id` 是素材，`row.usage_id`
   * 是关系。移出操作删的是关系，拿素材 id 去删会删错东西（或者 404），而两种错法
   * 在界面上都只表现为「移出失败」。
   */
  it('keeps the usage id and the material id apart', () => {
    expect(source).toContain('usage_id: usage.id')
    expect(source).toContain('await client.removeUsage(row.usage_id)')
    expect(source).not.toContain('removeUsage(row.id)')
    // 行键必须是关系 id：用素材 id 的话同一个素材的两次关系会共用一行。
    expect(source).toContain('row-key="usage_id"')
  })

  // 移出是真 204（api.NoContentEmpty），http.js 对 204 返回 null。解引用它就会崩，
  // 而在浏览器里那是一个「移出失败」的提示，看不出真正的原因。
  it('treats the remove response as empty and reloads instead of reading it', () => {
    expect(source).toMatch(/await client\.removeUsage\(row\.usage_id\)\s*\n\s*MessagePlugin\.success/)
    expect(source).not.toMatch(/const \w+ = await client\.removeUsage/)
  })

  // 这条守卫是给一个几乎发生过的 bug 的：筛选栏里有一个「视频状态」下拉，而过滤函数
  // 只看了搜索词 —— 那个下拉会安静地什么都不做。列表接口只返回 active 的关系，
  // 所以状态筛选是唯一一个能把「已选」和「没选」区分开的条件。
  it('actually applies the video status filter it renders', () => {
    expect(source).toContain('if (statusFilter.value && row.video_status !== statusFilter.value) return false')
    expect(source).toContain('const filteredRows = computed')
    expect(source).toContain('v-model="statusFilter"')
  })

  // 列表只返回 active 的关系，所以看不到已移出的行，也就没有「撤销移出」。
  // 恢复 = 在素材库里再点一次「加入我的素材」（冻结合同里没有恢复端点）。
  it('offers no undo, because the list cannot contain a removed row', () => {
    // 只看模板：页面上不该出现「恢复」这类控件。脚本里的注释提到它不算数，
    // 而且那条注释正是要说明为什么这里没有它。
    const template = source.slice(source.indexOf('<template>'))
    expect(template).not.toContain('恢复')
    expect(template).not.toContain('撤销')
    // 详情抽屉里那一颗叫「加入我的素材」，做的就是恢复 —— 同一个命令。
    expect(source).toContain('@add="addBack"')
    expect(source).toContain('await client.addUsage(row.id)')
  })

  it('opens the material a deep link names', () => {
    expect(source).toContain('route.query.material_id')
    expect(source).toContain('/^\\d+$/.test(String(raw))')
    expect(source).toContain('await client.get(row.id)')
  })

  it('shares the detail drawer and the download failure wording with the library', () => {
    expect(source).toContain("import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'")
    expect(source).toContain('<MaterialDetailDrawer')
    expect(source).toContain('createDownloadFailureMessage(e)')
    expect(source).not.toContain('material-detail-drawer')
  })

  // 走查修正（CHG-20260930-069）：与素材库同一条行形状 —— 素材 ID 第一列、封面随后，
  // 标题蓝色可点跳来源平台落地页，作者、链接与体积在详情里；下载按钮只叫「下载」，
  // 下面不挂提示小字。
  it('shares the library row shape: id first, clickable title, no download hint', () => {
    const idAt = source.indexOf("{ colKey: 'id', title: '素材 ID'")
    const coverAt = source.indexOf("{ colKey: 'cover', title: '封面'")
    expect(idAt).toBeGreaterThan(-1)
    expect(coverAt).toBeGreaterThan(-1)
    expect(idAt, '素材 ID 列必须在封面列之前').toBeLessThan(coverAt)
    expect(source).toContain('MaterialCover')
    expect(source).toMatch(/<a[^>]*:href="row\.source_url"/)
    expect(source).toContain('class="wt-primary-link')
    expect(source).toContain('@click="download(row)">下载</t-button>')
    expect(source).not.toContain('downloadHint')
    expect(source).not.toContain('下载到本机</t-button>')
    expect(source).not.toContain('· {{ row.author_name')
    expect(source).not.toContain('formatBytes(row.video_size_bytes)')
  })
})
