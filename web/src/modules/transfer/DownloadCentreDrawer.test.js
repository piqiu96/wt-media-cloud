import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./DownloadCentreDrawer.vue', import.meta.url), 'utf8')

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
})
