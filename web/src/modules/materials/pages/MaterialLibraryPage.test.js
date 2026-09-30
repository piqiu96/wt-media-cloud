import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialLibraryPage.vue', import.meta.url), 'utf8')

describe('material library page', () => {
  /**
   * `?material_id=N` 是一个**真实入口**，不是装饰。
   *
   * 内容池里「已转素材」那一列（`ContentPoolPage.vue`）与挖掘任务的发现结果
   * （`CrawlTasksPage.vue`）都会跳到这个地址。它们的落点要求页面能打开**任何**一条素材，
   * 而不只是恰好在第一页、又没被筛选掉的那一条 —— 所以走 `getMaterial`（单条接口），
   * 不是在已经加载的行里找。
   *
   * 这段契约原先由 `ContentPoolPage.test.js` 的一条用例钉着，钉的是旧的实现
   * （内容池页按 `route.path` 把自己渲染成素材库的只读投影）。投影被真实页面取代后，
   * 那条用例随之删除，契约重建在这里。
   */
  it('opens the material a deep link names, through the single-material endpoint', () => {
    expect(source).toContain('route.query.material_id')
    // 非数字的 id 不请求：`/materials/NaN` 只会换来一个 400，而那个 400 读起来像服务端坏了。
    expect(source).toContain('/^\\d+$/.test(String(raw))')
    expect(source).toContain('await client.get(Number(raw))')
    // 单条接口，而不是在行里找：深链必须能打开分页外或筛选外的那一条。
    expect(source).toContain('async function openFromQuery()')
    expect(source).toMatch(/onMounted\(\(\) => \{[^}]*openFromQuery\(\)/s)
  })

  // 筛选只给服务端真的会读的那一个。视频状态与游戏在这一侧筛（API client 只转发 search），
  // 页面不得凭空出现一个「团队」下拉 —— 那会让运营以为筛过了，而服务端回的是全量。
  it('asks the server for exactly the search it supports and filters the rest locally', () => {
    expect(source).toContain('client.list({ search: search.value })')
    expect(source).toContain('const filteredRows = computed')
    expect(source).toContain('row.video_status === statusFilter.value')
    expect(source).toContain('row.game_id === gameFilter.value')
    // 筛选项只列已加载数据里出现过的游戏：列一个点下去没有任何行的选项是同一种误导。
    expect(source).toContain('const gameOptions = computed')
    expect(source).toContain('new Set(rows.value.map((row) => row.game_id).filter(Boolean))')
  })

  // 409 的两种拒绝（素材未准备好、本机无可用节点）只能靠 error.type 区分，
  // 由 downloadErrors 翻成人话交给用户；直接把 err.message 抛出去会丢掉这个判别位。
  it('translates a refused download through its error type', () => {
    expect(source).toContain('createDownloadFailureMessage(e)')
    expect(source).toContain('await client.createDownload(row.id)')
    expect(source).toContain('downloadCentre.open()')
  })

  // 下载按钮不看 `video_status`：服务端接受未准备与失败，点了会先准备。判据随
  // `canDownload` 一起拿掉了，所以这里钉的是它的**缺席**——一个灰按钮会把「要多走一步」
  // 说成「这条路不通」，而运营据此去别处找绑定，正是走查里报的那一条。
  // 走查反馈（CHG-20260930-069）：按钮只叫「下载」，下面不再挂提示小字——
  // 状态徽章已经说明了点下去会发生什么，再补一句只是噪声。
  it('offers the download action on every row, named just「下载」and without a hint under it', () => {
    expect(source).not.toMatch(/:disabled="!canDownload\(/)
    expect(source).not.toContain('canDownload')
    expect(source).toContain('@click="download(row)">下载</t-button>')
    expect(source).not.toContain('downloadHint')
    expect(source).not.toContain('下载到本机</t-button>')
  })

  it('adds to my materials through the one idempotent command', () => {
    expect(source).toContain('await client.addUsage(row.id)')
    // 200 与 201 的响应体相同，客户端分支不了 —— 也就不该假装能分支。
    expect(source).not.toContain('created ===')
  })

  it('renders the four video states through the shared formatters', () => {
    expect(source).toContain('videoStatusLabel(row.video_status)')
    expect(source).toContain('videoStatusTone(row.video_status)')
  })

  // 走查修正（CHG-20260930-069）：行内以封面与素材 ID 识别内容；作者、平台链接与
  // 体积不再出现在行里，都归详情抽屉。ID 是第一列——走查反馈说运营扫行时先找编号。
  // 标题本身蓝色可点，跳来源平台落地页（source_url），与内容池页同一形状。
  it('puts the id column first, and makes the title the link to the source page', () => {
    const idAt = source.indexOf("{ colKey: 'id', title: '素材 ID'")
    const coverAt = source.indexOf("{ colKey: 'cover', title: '封面'")
    expect(idAt).toBeGreaterThan(-1)
    expect(coverAt).toBeGreaterThan(-1)
    expect(idAt, '素材 ID 列必须在封面列之前').toBeLessThan(coverAt)
    expect(source).toContain('MaterialCover')
    expect(source).toMatch(/<a[^>]*:href="row\.source_url"/)
    expect(source).toContain('class="wt-primary-link')
    expect(source).not.toContain('· {{ row.author_name')
    expect(source).not.toContain('formatBytes(row.video_size_bytes)')
  })

  // 详情抽屉是两页共用的一个组件：各写一份的结局是同一个素材在两页显示得不一样，
  // 且没有任何东西会报错。
  it('uses the shared material detail drawer rather than its own copy', () => {
    expect(source).toContain("import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'")
    expect(source).toContain('<MaterialDetailDrawer')
    expect(source).not.toContain('material-detail-drawer')
  })
})
