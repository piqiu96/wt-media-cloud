import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./DownloadCentreDrawer.vue', import.meta.url), 'utf8')

/** 取出两段定界文本之间的内容；找不到就抛，别让断言在空串上静默通过。 */
function sliceBetween(text, start, end) {
  const from = text.indexOf(start)
  const to = text.indexOf(end, from + start.length)
  if (from === -1 || to === -1) throw new Error(`找不到切片：${start} … ${end}`)
  return text.slice(from, to)
}

/**
 * 这个面板的按钮是由 `transferRow` 算出来的布尔量控制的，所以「算得对」和「接得上」是
 * 两件事。前者由 `downloadFacts.test.js`／`transferRows.test.js` 钉住；这里钉后者 ——
 * 一个算对了却没人在模板里读的字段，在界面上和不存在的区别是零。
 */
describe('download centre drawer', () => {
  it('shows the re-download button under the fact the row computed', () => {
    expect(source).toContain('v-if="row.canRedownload"')
    expect(source).toContain('@click="redownload(row.task)"')
    // 终态行的出路就是这个按钮；它旁边还得有「打开文件」各自的条件，
    // 两者混用同一个布尔量会让某一行同时出现两个动作。
    expect(source).toContain('v-if="row.canOpen && isDesktop()"')
  })

  /**
   * 「重取」（CHG-20261002-074 阶段 3）：悬置/可重取的行走**重取**，而不是「重新下载」。
   *
   * 两个按钮都在说「再来一次」，但重取要先处理「发给这台设备却没人领」的悬置行（取消再
   * 发起）；可重取的行（解绑遗留）则是换机重投递的入口，为当前设备建新任务。它们互斥：
   * 可重取的行给「重取」，其余终态行才给「重新下载」。
   */
  it('shows the re-take button under the fact the row computed', () => {
    expect(source).toContain('v-if="row.canRetake"')
    expect(source).toContain('@click="retake(row.task)"')
    expect(source).toContain('v-else-if="row.canRedownload"')
  })

  // 重取悬置行先取消（pending 没有执行器，取消是立即终态）再重新发起；可重取行已是终态，
  // 直接发起。两条都走与第一次点击完全相同的下载入口。
  it('re-takes through the download entry point, cancelling a suspended task first', () => {
    expect(source).toContain("if (task.status === 'pending') {")
    expect(source).toContain('await client.cancelTask(task.id)')
    expect(source).toContain('await materials.createDownload(task.asset_id)')
    expect(source).toContain('MessagePlugin.error(createDownloadFailureMessage(e))')
  })

  // 新任务要出现在这张列表里，重取之后同样要刷新（与「重新下载」同一条理由）。
  it('refreshes the list after re-taking', () => {
    expect(source).toMatch(/async function retake\(task\) \{(.|\n)*?await load\(\)/)
  })

  /**
   * 从「历史」再打开面板也要取数。
   *
   * 面板每次收尾会自己落到「历史」（没有在跑的任务时），所以下一次打开时 `activeTab`
   * 往往不是「进行中」。原先是把它切回去就 `return`，理由是「切 Tab 会触发上面那个
   * watcher」—— 但那个 watcher 只重估轮询闸门、不取数，闸门又依赖已缓存的列表。于是
   * 关闭期间（或上一次操作后）新发起的下载在面板里根本不存在，得手动刷新才看得到。
   */
  it('fetches on every open, including one that starts on the history tab', () => {
    const watcher = sliceBetween(source, 'watch(visible, async (open) => {', '\n})')
    expect(watcher).toContain("activeTab.value = 'active'")
    expect(watcher).not.toMatch(/activeTab\.value = 'active'\s*\n\s*return/)
    expect(watcher).toMatch(/load\(\)\.then\(/)
  })

  // 悬置行的「发给这台设备」由本机 device_id 认出：面板打开时先问 runtime，再穿给每一行。
  it('asks the runtime for the local device id and feeds it into the rows', () => {
    expect(source).toContain("await invoke('local_device_identity')")
    expect(source).toContain('identity?.device_id')
    expect(source).toContain('localDeviceId: localDeviceId.value,')
  })

  /**
   * 「重试」收进「重新下载」（走查裁定）：模板里不再有它，也不再调那个接口。
   *
   * 两个按钮都在说「再来一次」，而 `retryTask` 改的是原来那条行、还要求它没在等准备
   * （依赖已交付）—— 一条在等准备的失败行会被服务端直接拒绝。留着它，用户按下去得到的
   * 会是「重试失败」。
   */
  it('has no retry button or retry call left', () => {
    expect(source).not.toContain('canRetry')
    expect(source).not.toContain('retryTask')
    expect(source).not.toContain('>重试<')
  })

  /**
   * 「重新下载」是**新建一条任务**，所以它必须走发起下载那个入口。
   */
  it('re-downloads through the download entry point', () => {
    expect(source).toContain('await materials.createDownload(task.asset_id)')
    // 失败的话术取自发起下载那份词表（`error.type` 是冻结的名字），不是任务行的 error_code。
    expect(source).toContain('MessagePlugin.error(createDownloadFailureMessage(e))')
  })

  // 新任务要出现在这张列表里。不刷新的话画面停在旧行上，同一个素材看起来像没被重新下载过。
  it('refreshes the list after creating the task', () => {
    expect(source).toMatch(/async function redownload\(task\) \{(.|\n)*?await load\(\)/)
  })

  /**
   * 「文件现在在哪儿」必须真的接进来 —— 算对了不接线，界面上和不存在的区别是零。
   *
   * `transferRows` 的 `presence` 缺省是空表，漏传不会报错，只会让每一行都悄悄退回
   * 「没查过」，于是一整块功能静默失效。
   */
  it('feeds the file scan into the rows and reads it back in the template', () => {
    expect(source).toContain('presence: presence.value,')
    expect(source).toContain("row.presence === 'present_elsewhere'")
    // 在旧位置里的文件要说清在哪儿 —— 只说「不在当前目录」等于把人推去重新下一份。
    expect(source).toContain('row.fileFact.directory')
    expect(source).toContain("row.presence === 'absent'")
  })

  /**
   * 扫描**按批**，且只按名字集合的指纹触发，不跟着 2 秒 tick 走。
   *
   * 这个面板每 2 秒拉一次任务。若扫描挂在任务列表上，就变成每 2 秒一次跨进程列目录，
   * 而文件几乎从不变 —— 那是白烧本机 IO，还会让「读本机」变成常态噪声。
   */
  it('scans once per name set instead of once per poll', () => {
    expect(source).toContain('if (key === measuredKey) return')
    expect(source).toContain('const namesKey = computed(()')
    // 拼成字符串的指纹是这条的关键：数组每次都变，字符串不变。
    expect(source).toContain('fileNames.value.join(')
    // 重新打开要重扫（面板关着时文件可能被搬走／删掉／换了目录）。
    expect(source).toMatch(/if \(!open\) \{(.|\n)*?measuredKey = null/)
  })

  // 扫描失败退回「没查过」，不是「不在」：后台读本机失败不弹提示，也绝不断言文件没了。
  it('falls back to asserting nothing when the scan cannot answer', () => {
    expect(source).toMatch(/\(\) => \{(.|\n)*?presence\.value = \{\}/)
  })
})

/**
 * 一次点击在库里是两行（云端准备 compose_input_prepare + 本机下载 user_download），
 * 下载中心只显示本机那一条 —— 展示层合并，DB 不动。
 *
 * 关键顺序：`transferRows` 之前不能滤 —— `taskState → needsCloudPreparation` 要看到
 * 兄弟云任务才知道「等待云端准备」，所以过滤必须在 `transferRows` **之后**。
 */
describe('download centre shows one row per click', () => {
  it('filters the rows to user_download, after the shared row derivation', () => {
    expect(source).toContain("}).filter((row) => row.task.purpose === 'user_download')")
    // 状态推导仍喂全量列表：transferRows 的实参是 tasks.value 本身，过滤是链在后面的
    // 一步 —— 在它之前滤掉云行，needsCloudPreparation 就看不见兄弟云任务了。
    const block = sliceBetween(source, 'const rows = computed', 'const liveDownloads')
    expect(block).toContain('transferRows(tasks.value, {')
    expect(block.indexOf('transferRows(tasks.value')).toBeLessThan(block.indexOf(".filter((row)"))
  })

  it('gates the polling on user_download rows alone', () => {
    expect(source).toContain("const liveDownloads = computed(() => tasks.value.filter((task) => task.purpose === 'user_download'))")
    expect(source).toContain('hasLiveTask(liveDownloads.value)')
  })
})

// 三栏（CHG-20260930-069 任务 23）：进行中 / 失败 / 历史。**一次拉取**，按素材收敛成
// 一行，再按该素材最新那条任务的状态归栏；窗口仍在客户端套（展示层截断，DB 不删行）。
describe('download centre drawer tabs', () => {
  it('renders the three switchable tabs, counting only the current one', () => {
    expect(source).toContain('<t-tabs')
    expect(source).toContain('value="active"')
    expect(source).toContain('value="failed"')
    expect(source).toContain('value="history"')
    expect(source).toContain('activeLabel')
    expect(source).toContain('failedLabel')
    expect(source).not.toContain('recent')
  })

  // 不再三栏各查各的：一次拉回全部任务，分栏与窗口都在客户端做（每个素材一行之后）。
  // 不带 `status`/`finished_after` —— `finished_after` 会把 `finished_at` 为 null 的
  // 非终态行滤掉，而「进行中」正需要它们。
  it('loads the task list once instead of querying per tab', () => {
    expect(source).not.toContain('const QUERIES = {')
    expect(source).not.toContain('daysAgoISO')
    expect(source).toContain('client.listTasks({ limit: LIST_LIMIT })')
    // 切 Tab 不再重新拉取（数据一次在手），只重估轮询闸门。
    expect(source).not.toContain('watch(activeTab, () => { if (visible.value) load() })')
  })

  // 每个素材一行 = 该素材最新那条任务：服务端用 `created_at DESC, id DESC` 返回，首见即最新。
  it('keeps one row per material, taking the newest task the server returned', () => {
    const block = sliceBetween(source, 'const rows = computed', 'const liveDownloads')
    expect(block).toContain('transferRows(tasks.value, {')
    expect(block).toContain(".filter((row) => row.task.purpose === 'user_download')")
    expect(block).toContain('seen.has(key)')
    expect(block).toContain('seen.add(key)')
    // 收敛发生在 `transferRows` 之后：状态推导要看到兄弟云任务，先滤会让徽标退化。
    expect(block.indexOf('transferRows(tasks.value')).toBeLessThan(block.indexOf('.filter((row)'))
  })

  // 归栏按**最新任务**的状态：非终态→进行中，failed→失败，success/cancelled→历史；
  // 窗口在客户端套（失败 90 天，历史成功 30 天、取消 7 天）。
  it('buckets each material by its newest task and applies the windows locally', () => {
    expect(source).toContain('const buckets = computed')
    expect(source).toContain('!isTerminal(row.task)')
    expect(source).toContain('isFailed(row.task) && withinDays(row.task, 90)')
    expect(source).toContain("row.task.status === 'success' ? withinDays(row.task, 30) : withinDays(row.task, 7)")
    expect(source).toContain('const visibleRows = computed')
  })

  it('shows history as a table with terminal facts', () => {
    expect(source).toContain('<t-table')
    expect(source).toContain('row.finishedText')
    expect(source).toContain('row.sizeText')
  })

  // 走查修正（任务 23 收尾）：历史表格的标题要真的被截断。省略号三件套在**行内**元素上
  // 不生效 —— 得先给它一把可量的尺子（`display: block`），否则长标题直接冲出单元格。
  it('clips the history title inside its cell', () => {
    const block = sliceBetween(source, '.transfer-table__title {', '}')
    expect(block).toContain('display: block')
    expect(block).toContain('overflow: hidden')
    expect(block).toContain('white-space: nowrap')
    expect(block).toContain('text-overflow: ellipsis')
  })

  // 走查修正（任务 23 收尾）：下载中心抽屉与详情抽屉同宽 min(62vw, 880px)。
  // 旧宽度 min(46vw, 640px) 下历史表格五列（素材/大小/完成时间/状态/操作 ≈720px）
  // 必然横向滚动——参考详情的弹窗尺寸、做更大，让整张表一眼看全。
  it('sizes the drawer like the detail drawer so the history table fits', () => {
    expect(source).toContain('size="min(62vw, 880px)"')
  })

  // 每一栏各自的空态：进行中为空不等于没有任务，失败/历史为空也不等于都在跑。
  it('gives each tab its own empty text', () => {
    expect(source).toContain('暂无正在下载的任务')
    expect(source).toContain('暂无失败的任务')
    expect(source).toContain('暂无历史记录')
  })

  // 轮询只在「停在进行中」且有非终态任务时进行：失败/历史是终态事实，拉一次就够。
  it('polls only while on the active tab and something is unfinished', () => {
    expect(source).toContain("activeTab.value !== 'active'")
    expect(source).toContain('hasLiveTask(liveDownloads.value)')
  })

  // 失败行的「详情」打开这条素材的详情，用不带页脚动作的 transfer 上下文 —— 传输动作
  //（重新下载）就在下载中心这一栏里，详情只回答「这是什么」。
  it('opens the failed-row detail in a footer-less transfer context', () => {
    expect(source).toContain('@click="openDetail(row)"')
    expect(source).toContain('materials.get(row.task.asset_id)')
    expect(source).toContain('mode="transfer"')
  })
})
