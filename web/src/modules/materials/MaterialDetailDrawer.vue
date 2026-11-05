<script setup>
import { feedback } from '../../shared/ui/feedback/index.js'
// 素材详情抽屉，素材库与我的素材共用一份。行内只放识别信息（封面、ID、标题、文件状态），
// 其余在这里；云端视频地址不随素材 body 返回，点击「打开云端视频」时才向
// 详情链接接口取准确对象路径，再交给共用的公开读取地址转换。
import { computed, ref, watch } from 'vue'
import { createSessionClient } from '../../shared/api/session.js'
import { localDownloadReason } from '../transfer/localDownloadEligibility.js'
import { createMaterialsClient } from '../../shared/api/materials.js'
import { createFileTransferClient } from '../../shared/api/fileTransfer.js'
import { downloadedFileName, isTerminal, needsRedownload } from '../transfer/downloadFacts.js'
import { transferRow } from '../transfer/transferRows.js'
import { isDesktopRuntime, revealSavedFile, savedFileStates } from '../transfer/desktopBridge.js'
import { handOverCloudVideo } from './cloudVideo.js'
import ResourceOverlay from '../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../shared/ui/feedback/FeedbackAlert.vue'
import ResourceSection from '../../shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../shared/ui/resource/ResourceDescriptions.vue'
import { materialVideoState } from './materialFilters.js'
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { platformLabel } from '../../shared/utils/platforms.js'
import { formatBytes } from '../../shared/utils/units.js'
import MaterialCover from './components/MaterialCover.vue'
import MaterialPerformance from './components/MaterialPerformance.vue'
import {
  downloadActionLabel,
  categoryName,
  joinStatus,
  myUsageLabel,
  myUsageTone,
  usageFacts,
  usageStatusLabel,
  usageStatusTone,
} from './labels.js'

const props = defineProps({
  visible: { type: Boolean, default: false },
  material: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  categories: { type: Array, default: () => [] },
  // 上下文决定动作：素材库给「加入我的素材」，我的素材这一组按关系与文件状态分支，
  // readonly 只读（关联实体引用的那类打开方式）——正文照旧，页脚一个动作都不给。
  mode: { type: String, default: 'library' },
  showPageLink: { type: Boolean, default: true },
  // 只在素材库上下文有意义：用它把主操作换成「去我的素材」。
  mine: { type: Boolean, default: false },
})
const emit = defineEmits(['update:visible', 'add', 'download', 'redownload', 'go-mine', 'go-page', 'give-up', 'restore', 'cancel', 'retry'])

const downloadUser = ref(null)
const downloadReason = computed(() => localDownloadReason(downloadUser.value))
watch(() => props.visible, visible => { if (visible && props.mode === 'mine' && !downloadUser.value) createSessionClient().me().then(user => { downloadUser.value = user }).catch(() => {}) }, { immediate: true })
const client = createMaterialsClient()
const transfer = createFileTransferClient()

// 素材库上下文里，`usage_status` 是「当前查看者加没加入」，不是「使用中/已放弃」；
// 我的素材继续用关系自己的两档文案。两边共用一份数据，但说给用户的话不同。
const join = computed(() => joinStatus(props.material))
const usageRows = computed(() => usageFacts(props.material))
// 本机那一份：扫描结果里的一条（`{ name, directory, presence, bytes }`），没量到就是
// `null`。名字也在里面 —— 「打开目录」按下去那一刻要把它送出去。
const localFile = ref(null)
// 这个运营在这条素材上的下载生命周期（downloading/downloaded/failed；空串=没下过）。
// 详情从它已拉的这张用户任务表里就地派生，不为此加接口。
const downloadStatus = ref('')
// 该素材**最新一条进行中的 user_download 任务**（pending/running）：下载进度/速度/
// 预计时间/取消下载都挂在它身上，与下载中心同一套读数。null=这条素材没有在下载。
const activeTransferTask = ref(null)
// 「正在取消」是本机记忆而不是服务端字段（与下载中心同一条约定）：取消 running 任务后
// 它仍然报 running，终态由执行器写。按下取消后记在这里，按钮收回、显示「正在取消」。
const cancelRequested = ref(false)
// 已拉的那张用户任务表整份留着：transferRow 的 needsCloudPreparation 要看同素材的云
// 准备兄弟行，不能让进度把「等云端准备」误画成「卡住」。
const downloadTasks = ref([])

/**
 * 从这条用户的下载任务表里挑该素材**最新一次** user_download 的状态。
 *
 * `ListTasks` 是 created_at DESC, id DESC，第一条匹配即最新；与下载中心同一套过滤
 * （只看本机 + 这个动作），并映射成素材页同款三档。
 */
function deriveDownloadStatus(tasks, assetId) {
  const wanted = Number(assetId)
  if (!Number.isFinite(wanted)) return ''
  for (const task of Array.isArray(tasks) ? tasks : []) {
    if (task?.asset_type !== 'material') continue
    if (Number(task?.asset_id) !== wanted) continue
    if (task?.execution_scope !== 'local_agent') continue
    if (task?.purpose !== 'user_download') continue
    if (task?.status === 'pending' || task?.status === 'running') return 'downloading'
    if (task?.status === 'success') return 'downloaded'
    if (task?.status === 'failed' || task?.status === 'cancelled') return 'failed'
    return ''
  }
  return ''
}

/**
 * 该素材**最新一条进行中的 user_download 任务**（pending/running），没有就是 `null`。
 *
 * 与 `deriveDownloadStatus` 同一套过滤，只是拿整条任务而不是压缩成三档：进度、速度、
 * 预计时间与「取消下载」都需要任务本身（`completed_bytes`/`bytes_per_second`/id）。
 */
function deriveActiveTask(tasks, assetId) {
  const wanted = Number(assetId)
  if (!Number.isFinite(wanted)) return null
  for (const task of Array.isArray(tasks) ? tasks : []) {
    if (task?.asset_type !== 'material') continue
    if (Number(task?.asset_id) !== wanted) continue
    if (task?.execution_scope !== 'local_agent') continue
    if (task?.purpose !== 'user_download') continue
    if (task?.status === 'pending' || task?.status === 'running') return task
  }
  return null
}

/** 进行中那一条的完整行（进度/大小/速度/预计时间/能否取消）。null=没在下载。 */
const activeRow = computed(() => (
  activeTransferTask.value
    ? transferRow(activeTransferTask.value, { tasks: downloadTasks.value, cancelRequested: cancelRequested.value })
    : null
))

const fileStatus = computed(() => materialVideoState({ ...props.material, download_status: downloadStatus.value }, localFile.value))

/**
 * 云端视频每次点击现取对象路径，不预先拿也不留。
 *
 * 交付方式两个宿主不同（浏览器自己开标签页，打包的桌面端由壳交给系统浏览器），
 * 判据与理由都在 `cloudVideo.js`；这里只回答「现在跑在哪个宿主里」。
 *
 * 未就绪的素材没有地址，按钮也就不渲染 —— 一次必然 409 的请求不发出去。
 */
function openCloudVideo() {
  return handOverCloudVideo({
    client,
    materialId: props.material.id,
    desktop: isDesktopRuntime(),
    reportError: (message) => feedback.toast.error(message),
  })
}

/**
 * 这个素材在本机的那一份在哪个文件夹里。
 *
 * 两步，都不由前端拼：文件名从这张用户的下载任务表里挑（执行器报的那一个），目录由
 * Desktop 在所有已知保存位置里量出来。**「已下载」的判据在本机事实里，不在
 * `video_status` 上** —— 文件状态说的是云端那份源文件，一块本机从没下过的就绪素材
 * 也有它，而一份下好但云端还没就绪的文件照样躺在磁盘上。
 */
watch(() => [props.visible, props.material?.id, props.mode], async ([open, id, mode]) => {
  localFile.value = null
  downloadStatus.value = ''
  activeTransferTask.value = null
  cancelRequested.value = false
  if (!open || !id || mode !== 'mine') return
  try {
    const tasks = await transfer.listTasks()
    downloadTasks.value = tasks
    downloadStatus.value = deriveDownloadStatus(tasks, id)
    activeTransferTask.value = deriveActiveTask(tasks, id)
    const name = downloadedFileName(tasks, id)
    if (!name) return
    localFile.value = (await savedFileStates([name]))[name] ?? null
  } catch {
    // 读不到本机（浏览器、Agent 没起来）只少这一行：界面此时与没有这项功能时一样，
    // 不弹提示，也不把「读不到」写成「文件不在」。
  }
})

/**
 * 取消这条素材正在进行的下载。
 *
 * 与下载中心同一个两阶段语义：`CancelTask` 对 pending 立即终态化，对 running 只写
 * 取消请求、终态由执行器写。响应体是刷新后的任务，仍可能 running —— 那时记下
 * 「正在取消」并把按钮收回，等执行器真正把它写终态。
 */
async function cancelDownload() {
  if (!activeTransferTask.value || cancelRequested.value) return
  try {
    const refreshed = await transfer.cancelTask(activeTransferTask.value.id)
    if (!isTerminal(refreshed)) cancelRequested.value = true
    const tasks = await transfer.listTasks()
    downloadTasks.value = tasks
    const id = props.material?.id
    downloadStatus.value = deriveDownloadStatus(tasks, id)
    activeTransferTask.value = deriveActiveTask(tasks, id)
    // 上面重读的是抽屉自己的几份 ref；宿主列表那一行按同一批任务算出的「文件状态」不跟着
    // 变，所以要让页面知道该重读了。
    emit('cancel')
  } catch (e) {
    feedback.toast.error(e?.message || '取消失败')
  }
}

/**
 * 在文件管理器里打开那个文件夹。
 *
 * 发给 Rust 的是**名字**：目录由它在已知保存位置里找出来，前端从头到尾不知道路径。
 */
async function openDirectory() {
  try {
    await revealSavedFile(localFile.value.name)
  } catch (e) {
    // Tauri 的 invoke 拒绝时给的是字符串而不是 Error，两条路都要接住，
    // 否则界面上会出现一个空的错误提示。
    feedback.toast.error(e?.message || String(e))
  }
}
</script>

<template>
  <ResourceOverlay :visible="visible" title="素材详情" kind="drawer" :width="800"
    :footer="(mode === 'readonly' && showPageLink) || (mode === 'mine' && downloadStatus === 'downloading' && !!activeTransferTask)" @update:visible="$emit('update:visible', $event)">
    <FeedbackAlert v-if="error" theme="error" :message="error" />
    <t-button v-if="error" variant="text" @click="$emit('retry')">重试</t-button>
    <t-loading :loading="loading" :show-overlay="true">
      <div v-if="material" class="detail-workspace">
        <section class="detail-hero">
          <MaterialCover class="detail-cover" :url="material.cover_url" />
          <h3>{{ material.title || '未命名素材' }}</h3>
        </section>
        <ResourceSection title="基本信息"><ResourceDescriptions>
          <t-descriptions-item label="素材 ID">{{ material.id }}</t-descriptions-item>
          <t-descriptions-item label="类目">{{ categoryName(categories, material.category_id) }}</t-descriptions-item>
          <t-descriptions-item label="来源平台">{{ platformLabel(material.platform) }}</t-descriptions-item>
          <t-descriptions-item label="作者"><a v-if="material.author_home_url" class="wt-primary-link" :href="material.author_home_url" target="_blank" rel="noopener noreferrer">{{ material.author_name || '作者主页' }}</a><template v-else>{{ material.author_name || '—' }}</template></t-descriptions-item>
          <t-descriptions-item label="发布时间">{{ formatDateTime(material.published_at) }}</t-descriptions-item>
          <t-descriptions-item label="入库时间">{{ formatDateTime(material.created_at) }}</t-descriptions-item>
          <t-descriptions-item label="操作人">{{ material.created_by_name || '—' }}</t-descriptions-item>
          <t-descriptions-item label="原作品"><a v-if="material.source_url" class="wt-primary-link" :href="material.source_url" target="_blank" rel="noopener noreferrer">打开原作品</a><template v-else>—</template></t-descriptions-item>
        </ResourceDescriptions></ResourceSection>
        <ResourceSection title="内容表现"><MaterialPerformance :material="material" detail /></ResourceSection>
        <ResourceSection title="使用情况"><ResourceDescriptions>
          <t-descriptions-item :label="mode === 'mine' ? '使用状态' : '加入状态'"><template v-if="mode === 'mine'"><ResourceStatusBadge v-if="material.usage_status" :tone="usageStatusTone(material.usage_status)" :label="usageStatusLabel(material.usage_status)" /><template v-else>—</template></template><ResourceStatusBadge v-else :tone="join.tone" :label="join.label" /></t-descriptions-item>
          <t-descriptions-item label="我的使用"><ResourceStatusBadge :tone="myUsageTone(material.my_usage)" :label="myUsageLabel(material.my_usage)" /></t-descriptions-item>
          <t-descriptions-item v-if="mode === 'mine'" label="加入时间">{{ material.added_at ? formatDateTime(material.added_at) : '—' }}</t-descriptions-item>
          <t-descriptions-item v-for="fact in usageRows" :key="fact.key" :label="fact.label">{{ fact.value }}</t-descriptions-item>
        </ResourceDescriptions></ResourceSection>
        <ResourceSection title="视频文件"><ResourceDescriptions>
          <t-descriptions-item label="视频状态"><ResourceStatusBadge :tone="fileStatus.tone" :label="fileStatus.label" /></t-descriptions-item>
          <t-descriptions-item label="文件大小">{{ formatBytes(material.video_size_bytes) }}</t-descriptions-item>
          <t-descriptions-item label="准备完成时间">{{ formatDateTime(material.video_prepared_at) }}</t-descriptions-item>
          <t-descriptions-item label="视频文件"><a v-if="material.video_status === 'ready'" class="wt-primary-link" href="#" @click.prevent="openCloudVideo">打开视频</a><template v-else>—</template></t-descriptions-item>
        </ResourceDescriptions>
          <!-- 下载进行中：进度/速度/预计时间都是服务端任务表里的读数（与下载中心同一
               套），不是前端编的；取消也是真实请求。它与「下载目录」那一行不同屏 ——
               下载中还没有「已下载」的文件可量。 -->
          <div v-if="activeRow" class="detail-download">
            <div class="detail-download__progress">
              <div class="detail-download__head">
                <span class="detail-download__label">下载进度</span>
                <span v-if="activeRow.progress.kind === 'determinate'" class="detail-download__percent">{{ activeRow.progress.percent }}%</span>
                <span v-else class="detail-download__pending">{{ activeRow.pendingText }}</span>
              </div>
              <t-progress v-if="activeRow.showBar" theme="line" :percentage="activeRow.progress.percent" :label="false" />
              <p class="detail-download__meta">
                <span>{{ activeRow.sizeText }}</span>
                <span v-if="activeRow.rateText">· {{ activeRow.rateText }}</span>
                <span v-if="activeRow.etaText">· 预计 {{ activeRow.etaText }}</span>
              </p>
            </div>

          </div>
          <!-- 本机那一份在哪个文件夹里。这一行说的是磁盘上的文件，与上面那条云端地址
               是两份东西：路径可能很长，所以不塞进四列网格，整行铺开、换行不截断。
               量不到（浏览器读不了本机、Desktop 还没扫到）时整行不出现 —— 与下载中心
               同一条约定：读不到本机不等于文件不在。 -->
          <div v-if="localFile?.directory" class="detail-local-file">
            <div class="detail-local-file__where">
              <span class="detail-local-file__label">下载目录</span>
              <span class="detail-local-file__path">{{ localFile.directory }}</span>
            </div>
            <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDirectory">打开目录</t-button>
          </div>
          <p v-if="material.last_error" class="detail-error">{{ material.last_error }}</p>
        </ResourceSection>
      </div>
    </t-loading>

    <template #footer>
      <t-button v-if="mode === 'readonly' && showPageLink" class="wt-secondary-button" variant="outline" @click="$emit('go-page', material)">前往素材页面</t-button>
      <t-button v-if="mode === 'mine' && downloadStatus === 'downloading' && activeTransferTask" class="wt-secondary-button" variant="outline" :disabled="cancelRequested" @click="cancelDownload">{{ cancelRequested ? '正在取消…' : '取消下载' }}</t-button>
    </template>
  </ResourceOverlay>
</template>

<style scoped>
.detail-download-reason { color: var(--wt-text-secondary); font-size: 12px; }
.detail-workspace { display: flex; flex-direction: column; gap: 28px; }
.detail-workspace :deep(.resource-section) { margin-top: 0; }
.detail-hero { display: flex; gap: 16px; align-items: center; }
.detail-cover { width: 42px; height: 52px; flex-shrink: 0; }
.detail-hero h3 { margin: 0; color: var(--wt-text-primary); font-size: 16px; font-weight: 600; line-height: 24px; overflow-wrap: anywhere; }
.detail-error { margin: 12px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* 整行铺开而不是塞进四列网格：目录可能很长，截断掉的恰好是「在哪个盘的哪个文件夹」。
   与上面那行网格用一条虚线分开，同 `.detail-usage` 的做法。 */
.detail-local-file { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 14px; padding-top: 14px; border-top: 1px dashed var(--wt-border); }
.detail-local-file__where { display: flex; align-items: baseline; gap: 10px; min-width: 0; }
.detail-local-file__label { flex: 0 0 auto; color: var(--wt-text-tertiary); font-size: 12px; }
.detail-local-file__path { color: var(--wt-text-primary); font-size: 13px; line-height: 1.5; word-break: break-all; }
/* 下载进行中那一块：与「下载目录」行同一种上下分隔（虚线）。下载中还没有文件可量，
   所以它与那一行不同屏。 */
.detail-download { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-top: 14px; padding-top: 14px; border-top: 1px dashed var(--wt-border); }
.detail-download__progress { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.detail-download__head { display: flex; align-items: baseline; gap: 8px; }
.detail-download__label { flex: 0 0 auto; color: var(--wt-text-tertiary); font-size: 12px; }
.detail-download__percent { color: var(--wt-primary); font-size: 14px; font-weight: 600; }
.detail-download__pending { color: var(--wt-text-tertiary); font-size: 13px; }
.detail-download__meta { display: flex; align-items: center; gap: 4px; margin: 0; color: var(--wt-text-tertiary); font-size: 12px; }
/* 页脚只放业务动作、靠右收：关闭走抽屉右上角的 ×（规范 §6.3），不再占页脚一格。 */
.material-detail__actions { display: flex; justify-content: flex-end; align-items: center; gap: 8px; }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
</style>
