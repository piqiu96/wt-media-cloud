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
    // 已取消行的出路就是这个按钮；它旁边还得有重试与打开文件各自的条件，
    // 三者混用同一个布尔量会让某一行同时出现两个动作。
    expect(source).toContain('v-if="row.canRetry"')
    expect(source).toContain('v-if="row.canOpen && isDesktop()"')
  })

  /**
   * 「重新下载」是**新建一条任务**，所以它必须走发起下载那个入口。
   *
   * 走 `retryTask` 是错的：那个接口改的是原来那条行，且服务端要求它没在等准备
   * （依赖已交付），一条已在等准备的失败行会被直接拒绝 —— 而用户按的是「重新下载」，
   * 得到的会是「重试失败」。
   */
  it('re-downloads through the download entry point, not through retry', () => {
    expect(source).toContain('await materials.createDownload(task.asset_id)')
    expect(source).not.toContain('materials.retryTask')
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

// 三栏（CHG-20260930-069 任务 23）：进行中 / 失败 / 历史。栏由服务端按状态分好、
// 按各自窗口查询 —— 不再在客户端把终态行再劈成两栏。
describe('download centre drawer tabs', () => {
  it('renders the three switchable tabs, counting only the loaded tab', () => {
    expect(source).toContain('<t-tabs')
    expect(source).toContain('value="active"')
    expect(source).toContain('value="failed"')
    expect(source).toContain('value="history"')
    // 计数只挂在当前这一栏：别的栏的数据此刻不在手里，给一个没拉回来的数才是编造。
    // 历史有窗口与上限，只标名不计数。
    expect(source).toContain('activeLabel')
    expect(source).toContain('failedLabel')
    expect(source).not.toContain('recent')
  })

  // 每栏各拉各的查询（`status`/`finished_after`/`limit`）。窗口是展示层截断——
  // DB 从不删行，这里只决定某一栏还显示哪些。
  it('queries each tab through its own status window', () => {
    expect(source).toContain('const QUERIES = {')
    expect(source).toContain("active: { status: 'pending,running' }")
    expect(source).toContain("failed: () => ({ status: 'failed', finished_after: daysAgoISO(90) })")
    expect(source).toContain("history: () => ({ status: 'success,cancelled', finished_after: daysAgoISO(30), limit: 50 })")
    expect(source).toContain('function daysAgoISO(days)')
    expect(source).toContain('client.listTasks(query)')
    // 切 Tab 就拉那个 Tab 的查询。
    expect(source).toContain('watch(activeTab, () => { if (visible.value) load() })')
  })

  // 取消 7 天窗口在客户端再收一次：历史查询用 30 天（覆盖成功），cancelled 只留 7 天内的。
  it('re-widens the cancelled rows to their 7-day window on history', () => {
    expect(source).toContain('const visibleRows = computed')
    expect(source).toContain("row.task.status === 'cancelled'")
    expect(source).toContain('withinDays(row.task, 7)')
  })

  it('shows history as a table with terminal facts', () => {
    expect(source).toContain('<t-table')
    expect(source).toContain('row.finishedText')
    expect(source).toContain('row.sizeText')
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
