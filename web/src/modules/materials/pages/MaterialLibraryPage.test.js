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

  // 只有 ready 的素材有对象键可下，其余点下去必然是 409 —— 按钮就该是灰的，
  // 而不是让运营点一次、收一个错误、再点一次。
  it('disables the download action until the video is ready', () => {
    // 本页只有行内那一处；详情抽屉里那颗在共用的 MaterialDetailDrawer.vue 上，同一个守卫。
    const guarded = source.match(/:disabled="!canDownload\(/g) || []
    expect(guarded).toHaveLength(1)
  })

  it('adds to my materials through the one idempotent command', () => {
    expect(source).toContain('await client.addUsage(row.id)')
    // 200 与 201 的响应体相同，客户端分支不了 —— 也就不该假装能分支。
    expect(source).not.toContain('created ===')
  })

  it('renders the four video states and the size through the shared formatters', () => {
    expect(source).toContain('videoStatusLabel(row.video_status)')
    expect(source).toContain('videoStatusTone(row.video_status)')
    expect(source).toContain('formatBytes(row.video_size_bytes)')
    expect(source).toContain("from '../../../shared/utils/units.js'")
  })

  // 详情抽屉是两页共用的一个组件：各写一份的结局是同一个素材在两页显示得不一样，
  // 且没有任何东西会报错。
  it('uses the shared material detail drawer rather than its own copy', () => {
    expect(source).toContain("import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'")
    expect(source).toContain('<MaterialDetailDrawer')
    expect(source).not.toContain('material-detail-drawer')
  })
})
