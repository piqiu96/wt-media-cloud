<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { createFileTransferClient } from '../../shared/api/fileTransfer.js'
// 「重新下载」走的是**发起下载那个入口**（`POST /materials/{id}/downloads`），不是
// transfer 自己的路由 —— 所以它挂在素材的 client 上。把它塞进 `fileTransfer.js` 会
// 让那个模块开始持有素材路由，而它存在的理由恰恰是「任务是执行器中立的」。同一棵树
// 里的素材页面就是这么调的（`await client.createDownload(row.id)`），这里不发明第二种。
import { createMaterialsClient } from '../../shared/api/materials.js'
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { isDesktop } from '../../utils.js'
import { openSavedFile, savedFileStates } from './desktopBridge.js'
import { createDownloadFailureMessage } from './downloadErrors.js'
import { hasLiveTask, isTerminal } from './downloadFacts.js'
import { useDownloadCentre } from './downloadCentre.js'
import { splitTransferRows, transferRows } from './transferRows.js'

const client = createFileTransferClient()
const materials = createMaterialsClient()
const { visible, close } = useDownloadCentre()
const tasks = ref([])
const loading = ref(false)
const error = ref('')
// 本机记忆：我按过取消的那几条。冻结的任务体没有 cancel_requested_at，取消一个
// running 任务后它仍然报 running（终态只能由执行器写），所以「正在取消」推不出来，
// 只能记。刷新出终态后这条记录就不再有意义，由 taskState 的终态分支盖住。
const cancelRequested = ref([])
// 「这些文件现在在哪儿」，按文件名索引（见 `desktopBridge.savedFileStates`）。
// 空表 ＝ 没查过：浏览器读不到本机、Desktop 还没扫完 —— 界面此时一个字都不说，
// 不能把「我读不到」写成「文件不在」。
const presence = ref({})

const POLL_INTERVAL_MS = 2000
let timer = null

const rows = computed(() => transferRows(tasks.value, {
  cancelRequested: cancelRequested.value,
  presence: presence.value,
}))

// 两栏：非终态「正在下载」，终态「最近完成」（CHG-20260930-069）。
const groupedRows = computed(() => splitTransferRows(rows.value))
const activeRows = computed(() => groupedRows.value.active)
const recentRows = computed(() => groupedRows.value.recent)
const activeTab = ref('active')
const visibleRows = computed(() => (activeTab.value === 'active' ? activeRows.value : recentRows.value))

/**
 * 要问的名字集合，以及它的**字符串**指纹。
 *
 * 指纹存在的理由是轮询：任务列表每 2 秒换一个数组，`computed` 每次都重算并返回一个新
 * 数组，若直接 `watch` 那个数组，扫描就会跟着 tick 跑 —— 每 2 秒一次跨进程列目录，
 * 而文件几乎从不变。拼成字符串后 `watch` 比的是值：名字集合没变就一次都不问。
 */
const fileNames = computed(() => [...new Set(
  tasks.value.map((task) => task.file_name).filter((name) => typeof name === 'string' && name.trim())
)].sort())
const namesKey = computed(() => fileNames.value.join('\n'))

// 现在这张 presence 表是**为哪个名字集合**量出来的。相同就不再问第二遍 —— 这是
// 「按批、不跟着 tick 走」具体落在哪一行。
let measuredKey = null

function ensurePresence() {
  const key = namesKey.value
  if (key === measuredKey) return
  measuredKey = key
  if (!key) {
    presence.value = {}
    return
  }
  savedFileStates(fileNames.value).then(
    (table) => { presence.value = table },
    () => {
      // 问不到就退回「没查过」—— 它不做任何断言，界面上与本功能之前**完全一样**，
      // 所以这里不该弹提示（一个后台的状态读取失败不值得打断运营）。下一次打开或
      // 名字集合变化时会再问一次。
      presence.value = {}
    },
  )
}

// 轮询只在「面板开着」且「还有非终态任务」时进行。
//
// CHG §4 禁的是**拿前端定时器编进度**（用假百分比充当下载验收证据），不是禁止拉取。
// 这里的每一次 tick 都是一次真实的 GET，屏幕上的每个数字都来自服务端，定时器只决定
// 「多久问一次」；没有非终态任务时它立刻停下 —— 一个永远跑着的 2 秒轮询会让这份证据
// 反过来变成噪声，也浪费掉本机 Agent 与云端之间的带宽。
function stopPolling() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function ensurePolling() {
  if (!visible.value || !hasLiveTask(tasks.value)) {
    stopPolling()
    return
  }
  if (!timer) timer = setInterval(load, POLL_INTERVAL_MS)
}

async function load() {
  try {
    const data = await client.listTasks()
    tasks.value = Array.isArray(data) ? data : []
    error.value = ''
  } catch (e) {
    error.value = e?.message || '读取下载任务失败'
  } finally {
    ensurePolling()
  }
  // 名字集合变了才去问本机（见 `ensurePresence`）。放在 `finally` 之后：读任务失败时
  // 上一轮的名单还在，那正是已经查过的那些名字，不必重问。
  ensurePresence()
}

watch(visible, (open) => {
  if (!open) {
    stopPolling()
    return
  }
  // 重新打开要重扫一次：面板关着的这段时间里文件可能被搬走、被删，也可能换了保存位置。
  // 清掉指纹就够了 —— `load()` 结尾的 `ensurePresence()` 会补上这一次。
  measuredKey = null
  // 没有在跑的任务但有历史时直接落在「最近完成」，免得先看一屏空列表。
  activeTab.value = 'active'
  load().then(() => {
    if (!hasLiveTask(tasks.value) && recentRows.value.length) activeTab.value = 'recent'
  })
})

// 面板开着的时候新出现一条已下载的任务（刚点完下载），它的文件要立刻被扫到。
watch(namesKey, () => {
  if (visible.value) ensurePresence()
})

onBeforeUnmount(stopPolling)

async function cancel(task) {
  try {
    const refreshed = await client.cancelTask(task.id)
    // 只有服务端也说它还没终结时才记下「我请求过」。终态由执行器写，这里不替它宣布。
    if (!isTerminal(refreshed) && !cancelRequested.value.includes(task.id)) {
      cancelRequested.value = [...cancelRequested.value, task.id]
    }
    await load()
  } catch (e) {
    MessagePlugin.error(e?.message || '取消失败')
  }
}

async function retry(task) {
  try {
    await client.retryTask(task.id)
    cancelRequested.value = cancelRequested.value.filter((id) => id !== task.id)
    await load()
  } catch (e) {
    MessagePlugin.error(e?.message || '重试失败')
  }
}

/**
 * 终态行重新发起下载。
 *
 * 这是**新建一条任务**，不是重试：服务端去重键的 generation 把终态算作已结束，所以
 * 重新点击会得到一条新行，而 `retryTask` 改的是原来那条（且要求它没在等准备）。
 * 两者不是一个动作，所以 UI 上也是两个按钮。
 */
async function redownload(task) {
  try {
    await materials.createDownload(task.asset_id)
    // 新任务要出现在这张列表里 —— 不刷新的话画面停在旧行上，看起来像没反应。
    await load()
  } catch (e) {
    MessagePlugin.error(createDownloadFailureMessage(e))
  }
}

async function open(task) {
  try {
    await openSavedFile(task.file_name)
  } catch (e) {
    // Tauri 的 invoke 拒绝时给的是字符串而不是 Error，两条路都要接住，
    // 否则界面上会出现一个空的错误提示。
    MessagePlugin.error(e?.message || String(e))
  }
}
</script>

<template>
  <t-drawer v-model:visible="visible" class="download-centre-drawer" header="下载中心" size="min(46vw, 640px)" :footer="false" destroy-on-close @close="close">
    <t-loading :loading="loading" :show-overlay="true">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:12px" @close="error=''" />
      <t-tabs v-model="activeTab" class="transfer-tabs">
        <t-tab-panel value="active" :label="`正在下载 (${activeRows.length})`" />
        <t-tab-panel value="recent" :label="`最近完成 (${recentRows.length})`" />
      </t-tabs>
      <p v-if="!visibleRows.length" class="transfer-empty">{{ activeTab === 'active' ? '暂无正在下载的任务' : '最近没有完成的任务' }}</p>
      <ul v-else class="transfer-list">
        <li v-for="row in visibleRows" :key="row.task.id" class="transfer-item">
          <div class="transfer-item__head">
            <span class="transfer-item__title" :title="row.task.asset_title">{{ row.task.asset_title || `素材 #${row.task.asset_id}` }}</span>
            <ResourceStatusBadge :tone="row.state.tone" :label="row.state.label" />
          </div>

          <div v-if="row.showBar || row.pendingText" class="transfer-item__progress">
            <t-progress v-if="row.showBar" class="transfer-item__progress-bar" theme="line" :percentage="row.progress.percent" :label="false" />
            <span v-if="row.showBar" class="transfer-item__percent">{{ row.progress.percent }}%</span>
            <!--
              分母未知时不画进度条：一条 0% 的条看起来是「卡住了」，而它可能正在正常传输。
              「准备中」是这一刻唯一诚实的话。
            -->
            <span v-else-if="row.pendingText" class="transfer-item__pending">{{ row.pendingText }}</span>
          </div>

          <div class="transfer-item__meta">
            <span>{{ row.sizeText }}</span>
            <span v-if="row.rateText">{{ row.rateText }}</span>
            <span v-if="row.etaText">{{ row.etaText }}</span>
            <span v-if="row.attemptsText">{{ row.attemptsText }}</span>
          </div>

          <p v-if="row.errorText" class="transfer-item__error">{{ row.errorText }}</p>

          <!--
            文件在**别的**保存位置里 —— 这正是「改了保存路径后就找不到文件了」那一类：
            文件没丢，只是不在新目录里。「打开文件」照旧可用（Rust 侧会在所有已知位置里
            找），所以这句话要说出它在哪儿，而不是让人以为得重新下一份。
          -->
          <p v-if="row.presence === 'present_elsewhere'" class="transfer-item__where">
            文件还在原来的保存位置：{{ row.fileFact.directory }}
          </p>
          <!-- 查过、所有已知位置都没有。这一句只在**成功**的行上说：别的行本来就没落盘。 -->
          <p v-else-if="row.presence === 'absent' && row.task.status === 'success'" class="transfer-item__where">
            文件已不在本机已知的保存位置，可以重新下载
          </p>

          <div class="transfer-item__footer">
            <p class="transfer-item__time">发起于 {{ row.createdText }}</p>
            <div class="transfer-item__actions">
              <t-button v-if="row.canCancel" size="small" class="wt-secondary-button" variant="outline" @click="cancel(row.task)">取消</t-button>
              <t-button v-if="row.canRetry" size="small" theme="primary" @click="retry(row.task)">重试</t-button>
              <!-- 终态行的出路。已取消行原先一个动作都没有，「已取消的无法再次点击下载」
                   就是这么来的；失败行里那些在等准备的（重试必然被服务端拒绝）也只有这一条能走。 -->
              <t-button v-if="row.canRedownload" size="small" theme="primary" @click="redownload(row.task)">重新下载</t-button>
              <!--
                打开文件是桌面端专属：文件落在运营这台机器上（Agent 写的），浏览器打不开它。
                守卫写在按钮上而不是点下去再报错 —— 一个点了必然失败的按钮不该出现在那里。
              -->
              <t-button v-if="row.canOpen && isDesktop()" size="small" class="wt-secondary-button" variant="outline" @click="open(row.task)">打开文件</t-button>
            </div>
          </div>
        </li>
      </ul>
    </t-loading>
  </t-drawer>
</template>

<style scoped>
.transfer-empty { margin: 0; padding: 28px 8px; color: var(--wt-text-tertiary); font-size: 14px; text-align: center; }
.transfer-tabs { margin-bottom: 8px; }
.transfer-list { display: flex; flex-direction: column; gap: 10px; margin: 0; padding: 0; list-style: none; }
.transfer-item { padding: 12px 14px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-card); box-shadow: var(--wt-shadow-card); }
.transfer-item__head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.transfer-item__title { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; line-height: 1.45; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.transfer-item__progress { display: flex; align-items: center; gap: 10px; min-height: 18px; margin-top: 8px; }
.transfer-item__progress-bar { flex: 1 1 auto; min-width: 0; }
.transfer-item__percent { flex: 0 0 36px; color: var(--wt-text-tertiary); font-size: 12px; font-variant-numeric: tabular-nums; text-align: right; }
.transfer-item__pending { color: var(--wt-text-tertiary); font-size: 13px; }
/* 各项用 gap 分开而不是「·」串起来：宽度不够时「·」会在任意位置折断，标签和数字被拆散。 */
.transfer-item__meta { display: flex; flex-wrap: wrap; gap: 4px 14px; margin-top: 6px; color: var(--wt-text-tertiary); font-size: 12px; font-variant-numeric: tabular-nums; }
.transfer-item__error { margin: 6px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* 目录可能很长，换行而不是截断：截掉的恰好是「在哪个盘的哪个文件夹」这件事。 */
.transfer-item__where { margin: 6px 0 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 1.5; word-break: break-all; }
.transfer-item__footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 10px; padding-top: 9px; border-top: 1px solid var(--wt-border); }
.transfer-item__time { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; white-space: nowrap; }
.transfer-item__actions { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 8px; }
.download-centre-drawer :deep(.t-drawer__body) { padding: 16px; }
@media (max-width: 620px) {
  .transfer-item { padding: 12px; }
  .transfer-item__footer { align-items: flex-start; flex-direction: column; }
  .transfer-item__actions { justify-content: flex-start; }
  .download-centre-drawer :deep(.t-drawer__body) { padding: 14px; }
}
</style>
