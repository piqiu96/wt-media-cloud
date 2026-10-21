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
    expect(source).toMatch(/await client\.removeUsage\(row\.usage_id\)\s*\n\s*feedback\.toast\.success/)
    expect(source).not.toMatch(/const \w+ = await client\.removeUsage/)
  })

  it('defaults to published_at descending and lets ID/time columns toggle the order', () => {
    expect(source).toContain("const sortBy = ref('published_at')")
    expect(source).toContain("const sortOrder = ref('desc')")
    expect(source).toContain("{ colKey: 'id', title: '素材 ID', width: 90, sorter: true }")
    expect(source).toContain("{ colKey: 'published_at', title: '发布时间', width: 150, sorter: true }")
    expect(source).toContain("{ colKey: 'added_at', title: '加入时间', width: 105, sorter: true }")
    expect(source).toContain("sortBy.value = sort?.sortBy || 'published_at'")
    expect(source).toContain("sortOrder.value = sort?.descending === false ? 'asc' : 'desc'")
    expect(source).toContain('@sort-change="changeSort"')
    expect(source).toContain('sort_by: sortBy.value, sort_order: sortOrder.value')
  })

  // 这条守卫是给一个几乎发生过的 bug 的：筛选栏里有一个「文件状态」下拉，而过滤函数
  // 只看了搜索词 —— 那个下拉会安静地什么都不做。状态筛选由 /my-materials 的
  // video_status 参数完成，页面不再对返回行二次过滤，但控件必须与请求参数接线。
  it('sends the file status filter it renders to the server', () => {
    expect(source).toContain('video_status: statusFilter.value')
    expect(source).toContain('v-model="statusFilter"')
    expect(source).not.toContain('row.video_status !== statusFilter.value')
  })

  /**
   * 走查七轮（2026-09-30）推翻了「本页看不到已放弃的行」这条旧事实：列表接口不再
   * 只取 `status = 'active'`，已放弃的关系照样返回，所以本页第一次有了「恢复使用」。
   *
   * 恢复走**关系自己的端点**（`POST /material-usages/{id}/restore`），不是回素材库
   * 再点一次「加入我的素材」：那条命令按素材 id 走，对一个已经放弃过、又被别的入口
   * 恢复过的关系说不清它恢复的是哪一行。`uq_material_usages_user_material` 保证
   * 一个(用户,素材)只有一行，两条路径最终落到同一行。
   */
  it('restores a given-up relation through the relation endpoint', () => {
    expect(source).toContain('await client.restoreUsage(row.usage_id)')
    expect(source).not.toContain('restoreUsage(row.id)')
    expect(source).toContain('>恢复使用</t-button>')
    expect(source).not.toContain('addUsage')
  })

  it('opens the material a deep link names', () => {
    expect(source).toContain('route.query.material_id')
    expect(source).toContain('/^\\d+$/.test(String(raw))')
    expect(source).toContain('await client.get(row.id)')
  })

  it('shares the detail drawer and translates a refused download through its error type', () => {
    expect(source).toContain("import MaterialDetailDrawer from '../MaterialDetailDrawer.vue'")
    expect(source).toContain('<MaterialDetailDrawer')
    // 409 的两种拒绝（素材未准备好、本机无可用节点）只能靠 error.type 区分，
    // 由 downloadErrors 翻成人话交给用户；直接把 err.message 抛出去会丢掉这个判别位。
    expect(source).toContain('createDownloadFailureMessage(e)')
    expect(source).not.toContain('material-detail-drawer')
  })

  /**
   * 走查七轮：行操作按「使用状态 × 下载状态」矩阵分支（CHG-069 任务 23，任务状态与业务
   * 状态严格隔离），每行只给一个主操作。
   *
   * 规范 §5.6 与用户同轮裁定（「按钮按照超过 5 个才有更多按钮出现」）：本项目按钮数
   * 上限是 3 个（详情 + 一个主操作 + 放弃使用），永远到不了 5，所以**没有「更多」**——
   * 用户提示词里那套「3 个动作也收进更多」的写法按 §5.6 不采用。
   *
   * CHG-20261015-098 起已下载那一格再加一维：**本机实测还有没有那份文件**。有 → 不给
   * 主操作（只剩 详情｜放弃使用）；实测没有 → 给「重新下载」。两个分支都由
   * `needsRedownload` 判，与详情抽屉共用同一个判据（见下一条用例）。
   */
  it('branches each row on the usage × download matrix, flat and never behind 更多', () => {
    const op = source.slice(source.indexOf('#op="{ row }"'), source.indexOf('</t-space>'))
    expect(op).toContain('@click="openDetail(row)">详情</t-button>')
    expect(op).not.toContain('>查看</t-button>')
    // 已放弃：只有「恢复使用」，不再给必然空转的下载。
    expect(op).toContain("row.usage_status === 'removed'")
    expect(op).toContain('@click="restore(row)">恢复使用</t-button>')
    // 使用中：已下载但本机那份实测不在 → 重新下载；否则下载中不给主操作（文件已经在
    // 准备了，再点还是同一条命令），未下载/失败给「下载/重新下载」。
    expect(op).toContain('needsRedownload(row.download_status, localPresence(row))')
    expect(op).toContain('@click="download(row)">重新下载</t-button>')
    expect(op).toContain("row.download_status !== 'downloading' && row.download_status !== 'downloaded'")
    expect(op).toContain('downloadActionLabel(row.download_status)')
    expect(op).toContain('@click="giveUp(row)">放弃使用</t-button>')
    // 创建合成任务的入口整条搬去了合成任务页：行内不该再有任何合成动作。
    expect(op).not.toContain('合成')
    // 矩阵判据从 video_status 切走：行内不再碰云端源文件态。
    expect(op).not.toContain('video_status')
    expect(source).not.toContain('<t-dropdown')
    expect(source).not.toContain('>更多<')
  })

  /**
   * 「本机那份文件还在不在」是这一页唯一读本机的能力，它必须落在 Desktop 桥上的**批量**
   * 查询里：`savedFileStates` 一次 invoke 问完所有已下载行。
   *
   * 只对 `downloaded` 的行取名字 —— 未下载的行没有文件名可问，问出来的是空。名字集合没变
   * 就不重复扫（`measuredKey`），否则每次 `load()` 都白走一趟 IPC。
   */
  it('asks the desktop bridge once per changed set of downloaded file names', () => {
    expect(source).toContain('downloadedFileName(tasks.value, row.id)')
    expect(source).toContain('savedFileStates(')
    expect(source).toContain('measuredKey')
    // 浏览器里返回空表，presence 落到 unknown —— 不能把「读不到本机」画成「文件不在」。
    expect(source).toContain('filePresence(')
    expect(source).not.toContain('FILE_PRESENCE.absent')
  })

  // 「移出」是收藏夹的用词。关系是使用关系：放弃的是「使用」，不是把素材从清单里删掉。
  it('says 放弃使用 instead of 移出', () => {
    expect(source).not.toContain('>移出</t-button>')
    expect(source).not.toContain('已移出我的素材')
    expect(source).toContain('已放弃使用')
  })

  // 走查七轮用户提示词第一节：这一页不是收藏夹，副标题不该说「收藏」。
  // CHG-20261015-098：这一页不再「进入后续生产」——合成的入口搬去了合成任务页，
  // 副标题因此只说这里实际能做的事，诺言不多一句。
  it('describes the page by what it actually offers, not a favourites list', () => {
    expect(source).toContain('已加入的素材，在这里下载、查看使用状态与详情')
    expect(source).not.toContain('你收藏的素材')
    expect(source).not.toContain('进入后续生产')
  })

  /**
   * 规范 §7.2：使用状态与文件状态并排、不合并——同一个素材可以同时是「使用中 + 准备失败」，
   * 压成一个混合状态最先丢掉的正是运营要做判断的那种组合。
   *
   * 游戏这一列同时被收进「素材」格副行（与作者同格）：走查七轮把它从列里拿掉，
   * 是因为列里的「游戏」和素材识别信息说的是同一件事，而列本身占了 100px。
   */
  it('carries 使用状态 next to 文件状态 and folds the game into the 素材 cell', () => {
    expect(source).toContain("{ colKey: 'usage_status', title: '使用状态'")
    expect(source.indexOf("{ colKey: 'usage_status'")).toBeGreaterThan(source.indexOf("{ colKey: 'video_status'"))
    expect(source.indexOf("{ colKey: 'usage_status'")).toBeLessThan(source.indexOf("{ colKey: 'added_at'"))
    expect(source).not.toContain("{ colKey: 'game'")
    expect(source).toContain('usageStatusLabel(row.usage_status)')
    expect(source).toContain('usageStatusTone(row.usage_status)')
    // 副行是「游戏 · 作者」。
    expect(source).toContain('gameName(games.value, row.game_id)')
    expect(source).toContain('row.author_name')
  })

  // 关系自己的两个字段必须随行带出来：使用状态列读 `usage_status`，详情卡片读同一个值。
  it('carries the relation state and its own timestamp out of the usage row', () => {
    expect(source).toContain('usage_status: usage.status')
    expect(source).toContain('added_at: usage.created_at')
  })

  // 详情抽屉在我的素材上下文里要按关系状态给动作，所以打开时必须把关系那一维带上——
  // 单条素材接口返回的是素材，它不知道「我」和这条素材是什么关系。
  it('hands the relation state to the drawer it opens', () => {
    expect(source).toMatch(/client\.get\(row\.id\)[\s\S]{0,200}?usage_status: row\.usage_status/)
  })

  // 抽屉按上下文给动作：我的素材上下文提供下载/重新下载，不再提供「加入我的素材」
  // （它在这里就是自己），也不再提供合成（CHG-20261015-098 撤掉 @compose）。
  it('opens the drawer in mine mode with the actions of a relation, and no add', () => {
    expect(source).toContain('mode="mine"')
    expect(source).toContain('@download="download"')
    expect(source).toContain('@give-up="giveUp"')
    expect(source).toContain('@restore="restore"')
    expect(source).not.toContain('@add=')
    expect(source).not.toContain('@compose')
  })

  // 「加入合成」的后端端点在 CHG-082 落地了，但入口在 CHG-20261015-098 又整条搬去了
  // 合成任务页。这里钉住的是**没有任何残留**：既不能有一个说「已加入合成」的 toast
  // （那是伪造的提交），也不能有一条指向合成页 `{ name: 'Compose' }` 的跳转。
  it('leaves no compose affordance behind, faked or routed', () => {
    expect(source).not.toContain("{ name: 'Compose' }")
    expect(source).not.toContain('已加入合成')
    // `useRouter` 也随之退了：这一页现在没有任何跳转，`route` 只用来读深链查询参数。
    expect(source).not.toContain('useRouter')
    expect(source).toContain("import { useRoute } from 'vue-router'")
  })

  // 走查修正（CHG-20260930-069）：与素材库同一条行形状 —— 素材 ID 第一列、封面随后，
  // 标题蓝色可点跳来源平台落地页，作者、链接与体积在详情里；下载按钮只叫「下载」，
  // 下面不挂提示小字。
  it('shares the library row shape: id first, clickable title, no download hint', () => {
    const idAt = source.indexOf("{ colKey: 'id', title: '素材 ID'")
    const materialAt = source.indexOf("{ colKey: 'material', title: '素材'")
    expect(idAt).toBeGreaterThan(-1)
    expect(materialAt).toBeGreaterThan(-1)
    expect(idAt, '素材 ID 列必须是第一业务列，且排在「素材」格之前').toBeLessThan(materialAt)
    // 走查反馈：ID 不带 # 前缀——运营要把这串数字复制去别处查，# 只会跟着被复制。
    expect(source).toContain('<template #id="{ row }">{{ row.id }}</template>')
    expect(source).not.toContain('#{{ row.id }}')
    expect(source).toContain('MaterialCover')
    expect(source).toMatch(/<a[^>]*:href="row\.source_url"/)
    expect(source).toContain('class="wt-primary-link')
    expect(source).toContain('theme="primary" @click="download(row)">{{ downloadActionLabel(row.download_status) }}</t-button>')
    expect(source).not.toContain('downloadHint')
    expect(source).not.toContain('下载到本机</t-button>')
    expect(source).not.toContain('· {{ row.author_name')
    expect(source).not.toContain('formatBytes(row.video_size_bytes)')
  })

  // 走查四轮：两页共用同一条行形状与同一个列名。「视频状态」只描述源视频文件的准备
  // 进度，叫「文件状态」才不会和并不存在的素材业务状态混起来。
  it('uses the same 素材 cell and 文件状态 column name as the library', () => {
    expect(source).toContain('<template #material="{ row }">')
    expect(source).toContain("{ colKey: 'video_status', title: '文件状态'")
    expect(source).not.toContain("title: '视频状态'")
    for (const gone of ["title: '封面'", "title: '标题'", "title: '来源平台'"]) {
      expect(source, `不该再有 ${gone} 列`).not.toContain(gone)
    }
  })

  /**
   * 「文件状态」列 = 纯 `download_status`（CHG-069 任务 23 裁定：不再回落 video_status）。
   *
   * 下载状态是**这条关系**的（后端按「最新一条 user_download 任务」派生），不是素材的：
   * 同一行素材对两个下载过不同次的人要显示不同状态，所以它随 usage 走、落在行对象上，
   * 与 `usage_status` 同列同格。云端源文件态（可下载/准备失败）只留在详情抽屉的
   * 「文件信息」区 —— 把两个维度混在一起，就把「还没下」画成「没准备」。
   */
  it('derives the 文件状态 cell purely from the relation download status', () => {
    expect(source).toContain('download_status: usage.download_status || \'\'')
    expect(source).toContain('downloadStatusLabel(row.download_status)')
    expect(source).toContain('downloadStatusTone(row.download_status)')
    const cell = source.slice(source.indexOf('function fileStatus(row)'), source.indexOf('function ', source.indexOf('function fileStatus(row)') + 1))
    // 列只读 download_status；video_status 兜底已经拿掉。
    expect(cell).not.toContain('video_status')
    expect(cell).toContain('downloadStatusLabel(row.download_status)')
    expect(cell).toContain('downloadStatusTone(row.download_status)')
    // 下载状态与素材云侧状态不合并：video_status 那四档仍是服务端筛选参数，不参与本格。
    expect(source).toContain('video_status: statusFilter.value')
    expect(source).not.toContain('row.video_status')
  })

  /**
   * 「已开始下载」只是开始 —— 这一页也要重读。
   *
   * 「文件状态」格与行上的按钮都由使用记录派生的 `download_status` 算，`createDownload`
   * 只改服务端；不重读的话这一行停在上一帧（还写着「下载」），用户看到的就是「点了没反应，
   * 得手动刷新」。
   */
  it('re-reads the list after starting a download', () => {
    expect(source).toMatch(/await client\.createDownload\(row\.id\)(.|\n)*?await load\(\)/)
  })

  // 详情抽屉里的取消只重读了它自己那几份 ref；宿主列表那一行不跟着变，所以它要发信号。
  it('re-reads the list when the detail drawer cancels a download', () => {
    expect(source).toContain('@cancel="load"')
  })
})

  // CHG-20261015-098（用户裁定）：创建合成任务的入口**只在合成任务页**。我的素材这一页
  // 曾经有工具条批量和行内「加入合成」两个口子（CHG-20261013-082），现在整条撤掉 ——
  // 素材改为在合成任务页的创建抽屉里从「我的素材」中选。这里钉住的是净删除：任何一处
  // 残留都会让同一个能力出现两个入口，而它们的行为迟早会不一致。
  describe('compose task creation moved out (CHG-20261015-098)', () => {
    it('keeps no compose creation entry or dialog host', () => {
      // 逐个词查的是**子串**：旧变量名 `createMaterials` 不能进这张表——它是这一页
      // 一直在用的 `createMaterialsClient()` 的前缀，词表写宽了会把正当代码报成残留。
      for (const gone of [
        '加入合成',
        '创建合成任务',
        'ComposeTaskCreateDialog',
        'openCreate',
        'createSelected',
      ]) {
        expect(source, `我的素材不该再有 ${gone}`).not.toContain(gone)
      }
    })

    it('uses relation keys for the new batch download selection', () => {
      expect(source).toContain("row-key=\"usage_id\"")
      expect(source).toContain('v-model:selected-row-keys="selectedRowKeys"')
      expect(source).toContain("checkProps: ({ row }) => ({ disabled: !canSelectForDownload(row) || batchLoading.value })")
      expect(source).toContain('class="wt-selection-toolbar"')
      expect(source).toContain('>批量下载</t-button>')
    })
  })
