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

  // 走查三轮（交互对齐）：下载归「我的素材」。素材库行与它打开的详情抽屉都不再
  // 提供下载——留着入口就是把「先领取再下载」说成两条并行的路。
  it('offers no download anywhere on the library page', () => {
    expect(source).not.toContain('@click="download(row)"')
    expect(source).not.toContain('createDownload')
    expect(source).not.toContain('createDownloadFailureMessage')
    expect(source).not.toContain('downloadCentre')
    expect(source).not.toContain('@download')
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
    // 走查反馈：ID 不带 # 前缀——运营要把这串数字复制去别处查，# 只会跟着被复制。
    expect(source).toContain('<template #id="{ row }">{{ row.id }}</template>')
    expect(source).not.toContain('#{{ row.id }}')
    expect(source).toContain('MaterialCover')
    expect(source).toMatch(/<a[^>]*:href="row\.source_url"/)
    expect(source).toContain('class="wt-primary-link')
    expect(source).not.toContain('· {{ row.author_name')
    expect(source).not.toContain('formatBytes(row.video_size_bytes)')
  })

  // 详情抽屉是两页共用的一个组件：各写一份的结局是同一个素材在两页显示得不一样，
  // 且没有任何东西会报错。走查三轮：抽屉按上下文给动作——素材库上下文只有
  // 「加入我的素材」，不提供下载。
  it('uses the shared material detail drawer in library mode', () => {
    expect(source).toContain("import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'")
    expect(source).toContain('<MaterialDetailDrawer')
    expect(source).toContain('mode="library"')
    expect(source).toContain('@add="addToMine"')
    expect(source).not.toContain('material-detail-drawer')
  })

  // 走查三轮（交互对齐 §2.3/§5.3/§5.5）：行操作收敛为「详情 | 加入我的素材」，
  // 每行单一主操作；「查看」是全系统要消灭的同义词。
  it('narrows each row to详情 followed by one primary action', () => {
    expect(source).toContain('@click="openDetail(row)">详情</t-button>')
    expect(source).not.toContain('>查看</t-button>')
    expect(source).toContain('theme="primary" @click="addToMine(row)">加入我的素材</t-button>')
  })
})
