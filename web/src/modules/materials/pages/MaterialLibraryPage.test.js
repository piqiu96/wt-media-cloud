import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialLibraryPage.vue', import.meta.url), 'utf8')

/** 取出两段定界文本之间的内容；找不到就抛，别让断言在空串上静默通过。 */
function sliceBetween(text, start, end) {
  const from = text.indexOf(start)
  const to = text.indexOf(end, from)
  if (from === -1 || to === -1) throw new Error(`找不到切片：${start} … ${end}`)
  return text.slice(from, to)
}

// 「素材库不提供下载」这条规则管的是**行操作**，不是整页：加入成功的即时反馈里
// 可以给一步「立即下载」（走查四轮，2026-09-30 用户裁定）。所以这条断言量的是
// 行操作那一块，而不是整份源码。
const opBlock = sliceBetween(source, '#op=', '</template>')

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

  it('adds to my materials through the one idempotent command', () => {
    expect(source).toContain('await client.addUsage(row.id)')
    // 200 与 201 的响应体相同，客户端分支不了 —— 也就不该假装能分支。
    expect(source).not.toContain('created ===')
  })

  it('renders the four video states through the shared formatters', () => {
    expect(source).toContain('videoStatusLabel(row.video_status)')
    expect(source).toContain('videoStatusTone(row.video_status)')
  })

  // 走查四轮（2026-09-30 用户带设计图 + 裁定）：封面、标题、来源平台合成一个「素材」
  // 单元格，减少列数；素材 ID 仍独立成列且是第一业务列（规范 §5.1，用户明确裁定保留）。
  it('collapses cover, title and platform into one material cell behind a leading id column', () => {
    const idAt = source.indexOf("{ colKey: 'id', title: '素材 ID'")
    const materialAt = source.indexOf("{ colKey: 'material', title: '素材'")
    expect(idAt).toBeGreaterThan(-1)
    expect(materialAt).toBeGreaterThan(-1)
    expect(idAt, '素材 ID 列仍必须是第一业务列').toBeLessThan(materialAt)
    // 三样识别信息不再各自成列。
    for (const gone of ["title: '封面'", "title: '标题'", "title: '来源平台'"]) {
      expect(source, `不该再有 ${gone} 列`).not.toContain(gone)
    }
    expect(source).toContain('<template #material="{ row }">')
    expect(source).toContain('MaterialCover')
    expect(source).toMatch(/<a[^>]*:href="row\.source_url"/)
    expect(source).toContain('{{ row.platform')
    // 走查反馈：ID 不带 # 前缀——运营要把这串数字复制去别处查，# 只会跟着被复制。
    expect(source).toContain('<template #id="{ row }">{{ row.id }}</template>')
    expect(source).not.toContain('#{{ row.id }}')
    // 副行只带来源平台；游戏有自己的一列，同一格里再说一遍就是重复。
    expect(source).not.toContain('· {{ row.author_name')
    expect(source).not.toContain('formatBytes(row.video_size_bytes)')
  })

  // 列名换成了「文件状态」，因为它描述的是源视频文件的准备进度，不是这条素材的业务状态
  // （业务状态维度本轮不存在，见 change.md §3）。取值没换，仍是 video_status 四态。
  it('names the video status column 文件状态 and keeps the four-state labels', () => {
    expect(source).toContain("{ colKey: 'video_status', title: '文件状态'")
    expect(source).not.toContain("title: '视频状态'")
    expect(source).toContain('videoStatusLabel(row.video_status)')
    expect(source).toContain('videoStatusTone(row.video_status)')
  })

  /**
   * 走查三轮裁定「素材库不提供下载入口」，走查四轮把它收窄到**行与详情抽屉**：
   * 加入成功的即时反馈可以给一步「立即下载」（用户 2026-09-30 裁定）。
   *
   * 所以这条断言量的是行操作那一块 + 抽屉绑定，不是整份源码 —— 整份源码里现在
   * 确实有 `createDownload`，那是反馈里的那一步。
   */
  it('offers no download in the row or the drawer, only on the add-success toast', () => {
    expect(opBlock).not.toContain('下载')
    expect(source).not.toContain('@download')
    expect(source).not.toContain('downloadCentre')
    const toast = sliceBetween(source, 'function notifyAdded(', '\n}')
    expect(toast).toContain('立即下载')
  })

  // 「已加入 → 去我的素材」需要知道每行的加入状态。列表接口不返回它，但
  // `/api/v1/my-materials` 已经能全量取回当前用户的关系，join 一次即可——
  // 不为此新增后端字段（2026-09-30 用户裁定）。
  it('reads my materials so a row knows it is already joined', () => {
    expect(source).toContain('await client.listMyMaterials()')
    expect(source).toContain('usage.material_id')
    expect(source).toContain('const mineIds = ref(')
    expect(source).toMatch(/onMounted\(\(\) => \{[\s\S]*loadMine\(\)/)
  })

  // 走查四轮（用户裁定 + 设计图）：已加入的行主操作是「去我的素材」，
  // 未加入的行仍是「加入我的素材」。两个主操作互斥。
  it('swaps the row primary action to 去我的素材 once the material is joined', () => {
    expect(opBlock).toContain('@click="openDetail(row)">详情</t-button>')
    expect(opBlock).not.toContain('>查看</t-button>')
    expect(opBlock).toContain('@click="addToMine(row)">加入我的素材</t-button>')
    expect(opBlock).toContain('@click="goToMyMaterials()">去我的素材</t-button>')
    expect(source).toMatch(/v-if="!isMine\(row\)"[\s\S]{0,200}?addToMine\(row\)/)
    expect(source).toMatch(/v-else[\s\S]{0,200}?goToMyMaterials\(\)/)
    // 跳转目标就是「我的素材」页，两个端同名（cloud / desktop router 都是 MyMaterial）。
    expect(source).toContain("{ name: 'MyMaterial' }")
  })

  // 走查四轮：加入成功的反馈里给两个下一步。「立即下载」只在文件已就绪时出现——
  // 未就绪时那颗按钮必然换来一个 409，而不是一次下载。
  it('offers 立即下载 and 去我的素材 on the add-success toast', () => {
    const toast = sliceBetween(source, 'function notifyAdded(', '\n}')
    expect(toast).toContain('MessagePlugin.success')
    expect(toast).toContain('已加入我的素材')
    expect(toast).toContain('立即下载')
    expect(toast).toContain('去我的素材')
    expect(toast).toContain("row.video_status === 'ready'")
    expect(toast).toContain('downloadNow(row)')
    // 「立即下载」走的是与「我的素材」同一颗命令，不另开一条下载路径；
    // 409 的两种拒绝只能靠 error.type 区分，翻译件也是同一份。
    const downloadFn = sliceBetween(source, 'async function downloadNow(', '\n}')
    expect(downloadFn).toContain('client.createDownload(row.id)')
    expect(downloadFn).toContain('createDownloadFailureMessage')
    expect(source).toContain("from '../../transfer/downloadErrors.js'")
  })

  // 反馈发出去之后行要立刻翻转，不能让运营再点一次刷新才看见「去我的素材」。
  it('flips the row to 去我的素材 without a reload', () => {
    expect(source).toMatch(/mineIds\.value\.add\(row\.id\)/)
  })

  // 详情抽屉是两页共用的一个组件：各写一份的结局是同一个素材在两页显示得不一样，
  // 且没有任何东西会报错。走查三轮：抽屉按上下文给动作——素材库上下文只有
  // 「加入我的素材」，不提供下载；走查四轮补上「已加入」时的「去我的素材」。
  it('uses the shared material detail drawer in library mode', () => {
    expect(source).toContain("import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'")
    expect(source).toContain('<MaterialDetailDrawer')
    expect(source).toContain('mode="library"')
    expect(source).toContain('@add="addToMine"')
    expect(source).toContain('@go-mine="goToMyMaterials"')
    expect(source).toContain(':mine="isMine(detail)"')
    expect(source).not.toContain('material-detail-drawer')
  })

  // 走查三轮（交互对齐 §2.3/§5.3/§5.5）：行操作收敛为「详情 | 单一主操作」；
  // 「查看」是全系统要消灭的同义词。
  it('narrows each row to 详情 followed by one primary action', () => {
    expect(opBlock).toContain('@click="openDetail(row)">详情</t-button>')
    expect(opBlock).not.toContain('>查看</t-button>')
    // 行里只有这两颗：详情 + 主操作（加入或去我的素材），没有第二个业务动作。
    expect(opBlock.match(/<t-button/g)).toHaveLength(3)
  })
})
