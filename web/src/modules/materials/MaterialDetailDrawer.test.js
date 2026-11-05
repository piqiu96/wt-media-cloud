import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialDetailDrawer.vue', import.meta.url), 'utf8')
const template = source.slice(source.indexOf('<template>'))

/** 取出两段定界文本之间的内容；找不到就抛，别让断言在空串上静默通过。 */
function sliceBetween(text, start, end) {
  const from = text.indexOf(start)
  const to = text.indexOf(end, from + start.length)
  if (from === -1 || to === -1) throw new Error(`找不到切片：${start} … ${end}`)
  return text.slice(from, to)
}

describe('material detail drawer', () => {
  it('uses the approved detail sections and common read-only fields', () => {
    expect([...template.matchAll(/<ResourceSection title="([^"]+)"/g)].map(match=>match[1])).toEqual(['基本信息','内容表现','使用情况','视频文件'])
    expect(template).toContain('<ResourceOverlay')
    expect(template).toContain('kind="drawer"')
    expect(template).toContain('<ResourceDescriptions>')
    expect(template).not.toContain('<t-tabs')
    expect(template).not.toContain('detail-card')
    expect(template).toContain('label="素材 ID">{{ material.id }}')
    expect(template).toContain("material.created_by_name || '—'")
  })
  it('keeps the original work link only in basic information', () => {
    expect(template.match(/:href="material.source_url"/g)).toHaveLength(1)
    const hero = sliceBetween(template, 'class="detail-hero"', '<ResourceSection')
    expect(hero).not.toContain('href=')
    expect(hero).toContain('MaterialCover')
    expect(template).toContain('material.author_home_url')
  })
  it('uses metric presence instead of presenting missing values as zero', () => {
    expect(template).toContain('metricLabel(material,key)')
    for (const key of ['view_count','like_count','favorite_count','comment_count','share_count']) expect(template).toContain(key)
  })
  it('fetches video access only at click time and resolves the running host', () => {
    expect(template).toContain('@click.prevent="openCloudVideo"')
    expect(template).toContain("v-if=\"material.video_status === 'ready'\"")
    const handler=sliceBetween(source,'function openCloudVideo()', '\n}')
    expect(handler).toContain('handOverCloudVideo({')
    expect(handler).toContain('isDesktopRuntime()')
    expect(handler).not.toContain('window.open(')
    expect(source).not.toContain('videoUrl')
  })
  it('keeps mutation actions in the context footer only', () => {
    const body=template.slice(0,template.indexOf('<template #footer>'))
    const footer=sliceBetween(template,'<template #footer>', '</template>\n  </ResourceOverlay>')
    expect(body).not.toContain("$emit('download'")
    expect(body).not.toContain("$emit('add'")
    expect(body).not.toContain('@click="cancelDownload"')
    expect(footer).toContain("mode === 'readonly' && showPageLink")
    expect(footer).toContain("mode === 'library' && !mine")
    expect(footer).toContain("mode === 'library' && mine")
    expect(footer).toContain("material.usage_status === 'removed'")
    expect(footer).toContain('needsRedownload(downloadStatus, localFile?.presence)')
    expect(footer).toContain("downloadStatus === 'downloading'")
    expect(footer).toContain('Boolean(downloadReason)')
    expect(footer).toContain("$emit('restore', material)")
    expect(footer).toContain("$emit('give-up', material)")
    expect(footer).toContain('@click="cancelDownload"')
    expect(footer).toContain(':disabled="cancelRequested"')
    expect(footer).toContain("cancelRequested ? '正在取消…' : '取消下载'")
    expect(footer).not.toContain('video_status')
    expect(footer).not.toContain("$emit('compose'")
  })
  it('keeps missing relation data unknown and displays real successful-output facts', () => {
    expect(template).toContain('v-if="material.usage_status"')
    expect(template).toContain('<template v-else>—</template>')
    expect(template).toContain('v-for="fact in usageRows"')
    expect(source).toContain('usageFacts(props.material)')
    expect(template).toContain("material.added_at ? formatDateTime(material.added_at) : '—'")
  })
})

describe('the local copy of a downloaded material', () => {
  it('takes the file name from the download tasks and the directory from this machine', () => {
    expect(source).toContain("import { createFileTransferClient } from '../../shared/api/fileTransfer.js'")
    expect(source).toContain("import { downloadedFileName, isTerminal, needsRedownload } from '../transfer/downloadFacts.js'")
    expect(source).toContain("import { isDesktopRuntime, revealSavedFile, savedFileStates } from '../transfer/desktopBridge.js'")
    expect(source).toContain('downloadedFileName(tasks')
    expect(source).toContain('savedFileStates([name])')
    // 名字从扫描结果那一条上读回来（`savedFileStates` 的表里带着 `name`），不另存一份。
    expect(source).toContain('revealSavedFile(localFile.value.name')
  })

  // 切到这张卡自己的 `</section>` 为止，不是切到下一张卡的标题：后者会把两张卡之间的空档
  // 也算进来，把这块搬出卡片照样通过（实测：搬到空档里 29 条仍全绿）。
  it('puts the download directory in the 文件信息 card, with a way to open it', () => {
    const card = sliceBetween(template, '<ResourceSection title="视频文件">', '</ResourceSection>')
    expect(card).toContain('下载目录')
    expect(card).toContain('localFile.directory')
    expect(card).toContain('打开目录')
  })

  // 下载归「我的素材」（§2）：素材库上下文打开的详情不该出现本机文件的那一行，
  // 那一页连「下载」入口都没有。
  it('asks this machine only in the 我的素材 context', () => {
    // 锚在**这个** watcher 自己的依赖数组上：详情里另有一个同开头的 watcher（云端视频
    // 地址），按前缀切会切到那一个，断言就在别的代码上通过了。
    const watcher = sliceBetween(source, 'watch(() => [props.visible, props.material?.id, props.mode]', '\n})')
    expect(watcher).toContain("mode !== 'mine'")
  })

  /**
   * 读不到本机时整行不出现 —— 浏览器读不到本机文件，Desktop 也要扫过才知道。
   *
   * 与下载中心同一条约定（`desktopBridge.savedFileStates` 在浏览器里返回空表）：
   * 把「我读不到」写成「文件不在」，运营会据此重新下一份几百兆。
   */
  it('says nothing at all when this machine has nothing measured', () => {
    // 一整行连标签一起钉：条件与元素分开断言时，条件写在别处也能通过。
    expect(template).toContain('<div v-if="localFile?.directory" class="detail-local-file">')
    const block = sliceBetween(template, 'class="detail-local-file"', '</ResourceSection>')
    expect(block).not.toContain('未下载')
    expect(block).not.toContain('本地文件')
  })

  // 这一行说的是磁盘上那一份，与云端那份源文件的就绪状态无关：拿 `video_status`
  // 当条件是错的 —— 它会在一块「文件就在这儿」的素材上说「未就绪」，也会在一块
  // 云端就绪但本机从没下过的素材上画一个不存在的目录。
  it('does not read the local copy off the file state', () => {
    const block = sliceBetween(template, 'class="detail-local-file"', '</ResourceSection>')
    expect(block).not.toContain('video_status')
    expect(source).toContain('downloadedFileName(tasks, id)')
    // 判据在**取事实的那一段**里：这条 watch 体里一次都不提文件状态。整份源码不能这样断言
    // ——文件状态在别处有正当用处（云端视频地址那条 watch 就按它分支）。
    const watcher = sliceBetween(source, 'watch(() => [props.visible, props.material?.id, props.mode]', '\n})')
    expect(watcher).not.toContain('video_status')
  })
})

/**
 * 详情抽屉的「文件状态」徽章与我的素材页同语义：下载生命周期优先，素材云侧
 * `video_status` 兜底。下载生命周期在 mine 上下文里从它**已拉的**这张用户任务表就地
 * 派生，不为此加接口。
 */
describe('the drawer file state derives from the user download lifecycle', () => {
  it('derives the download lifecycle from the newest user_download task', () => {
    // 只看本机 + 这个动作（与 downloadedFileName 同一套过滤），三档映射。
    expect(source).toContain("task?.execution_scope !== 'local_agent'")
    expect(source).toContain("task?.purpose !== 'user_download'")
    expect(source).toContain("return 'downloading'")
    expect(source).toContain("return 'downloaded'")
    expect(source).toContain("return 'failed'")
  })

  it('uses the same verified video projection as the list', () => {
    expect(source).toContain('materialVideoState({ ...props.material, download_status: downloadStatus.value }, localFile.value)')
  })

  it('fills the derived status in the mine-context watcher, without reading video_status there', () => {
    const watcher = sliceBetween(source, 'watch(() => [props.visible, props.material?.id, props.mode]', '\n})')
    expect(watcher).toContain('deriveDownloadStatus(tasks, id)')
    expect(watcher).toContain('deriveActiveTask(tasks, id)')
    expect(watcher).not.toContain('video_status')
  })
})

/**
 * 下载进行中（CHG-069 任务 23）：正文「文件信息」卡里渲染下载进度/速度/预计时间，
 * 页脚主操作换成「取消下载」。读数与下载中心同一套 —— 从 transferRow 来，不是前端编的。
 */
describe('the drawer while a download is in progress', () => {
  it('renders the live download facts in the 文件信息 card', () => {
    const card = sliceBetween(template, '<ResourceSection title="视频文件">', '</ResourceSection>')
    expect(card).toContain('下载进度')
    expect(card).toContain('activeRow.progress.percent')
    expect(card).toContain('activeRow.pendingText')
    expect(card).toContain('activeRow.sizeText')
    expect(card).toContain('activeRow.rateText')
    expect(card).toContain('activeRow.etaText')
    // 进度条只在分母已知时画（与下载中心同一约定）。
    expect(card).toContain('activeRow.showBar')
  })

  it('finds the active task through the same user_download filter as the lifecycle', () => {
    expect(source).toContain("task?.execution_scope !== 'local_agent'")
    expect(source).toContain("task?.purpose !== 'user_download'")
    expect(source).toContain("task?.status === 'pending' || task?.status === 'running'")
  })

  it('cancels through the transfer client and remembers the request locally', () => {
    expect(source).toContain('transfer.cancelTask(activeTransferTask.value.id)')
    expect(source).toContain('isTerminal(refreshed)')
    expect(source).toContain('cancelRequested.value = true')
    expect(source).toContain('cancelRequested.value = true')
  })

  // 只有正在下载（有进行中任务）才出这一块；没有就整块不出现。
  it('shows the download block only while a transfer is actually running', () => {
    expect(template).toContain('v-if="activeRow" class="detail-download"')
  })

  /**
   * 取消之后要让宿主页面重读。
   *
   * 这里重读的只是抽屉自己的 `downloadTasks` / `downloadStatus` / `activeTransferTask`，
   * 而「我的素材」列表那一行的文件状态由同一批任务另算一份（素材库列表自 2026-10-27
   * 起不再展示文件状态）—— 不通知宿主，取消完回到列表，那一行还写着「下载中」。
   */
  it('tells the host page to re-read after cancelling', () => {
    expect(source).toMatch(/defineEmits\(\[[^\]]*'cancel'/)
    const handler = sliceBetween(source, 'async function cancelDownload()', '\n}')
    expect(handler).toContain("emit('cancel')")
  })
})
