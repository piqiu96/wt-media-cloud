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
import MaterialDetailDrawer from '../materials/MaterialDetailDrawer.vue'
import { isDesktop } from '../../utils.js'
import { invoke } from '@tauri-apps/api/core'
import { isDesktopRuntime, openSavedFile, savedFileStates } from './desktopBridge.js'
import { createDownloadFailureMessage } from './downloadErrors.js'
import { hasLiveTask, isFailed, isHistory, isTerminal, withinDays } from './downloadFacts.js'
import { useDownloadCentre } from './downloadCentre.js'
import { transferRows } from './transferRows.js'

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
// 本机的持久化设备 id。悬置行（pending 且 assigned_device_id 是这台）要靠它认出来；
// 浏览器读不到本机，读不到就留着 null —— `canRetake` 那一支恒 false，不认就不重取。
const localDeviceId = ref(null)

const POLL_INTERVAL_MS = 2000
let timer = null

// 一次拉回全量（CHG-20260930-069 任务 23）。**不带** `status`/`finished_after`：终态与
// 非终态都要，而 `finished_after` 会把 `finished_at` 为 null 的非终态行滤掉 —— 「进行中」
// 正需要它们。分栏与窗口都在客户端做（见 `buckets`），窗口是**展示层截断**，DB 从不删行。
const LIST_LIMIT = 200

const activeTab = ref('active')

// 一次点击在库里是两行（云端准备 `compose_input_prepare` + 本机下载 `user_download`，
// 同一毫秒同建），下载中心只显示**本机那一条**，用状态区分阶段。
//
// 状态推导仍喂**全量**列表：`taskState → needsCloudPreparation` 要看到兄弟云任务才知道
// 「等待云端准备」，所以在 `transferRows` 之后才滤 —— 在它之前滤会把那条徽标退化成
// 「排队中」。DB 两行不动，这里只是展示层合并。
//
// **每个素材一行**（走查修正）：同一素材的失败/成功/取消各是一次执行，记录都还在库里；
// 这张表是传输观察面，不是执行流水账，所以按素材收敛到**最新那一条**任务。服务端按
// `created_at DESC, id DESC` 返回，首见即最新 —— 与「我的素材」的 `download_status` 从
// 最新一条 user_download 派生是同一个口径。
const rows = computed(() => {
  const seen = new Set()
  return transferRows(tasks.value, {
    cancelRequested: cancelRequested.value,
    presence: presence.value,
    localDeviceId: localDeviceId.value,
  }).filter((row) => row.task.purpose === 'user_download')
    .filter((row) => {
      const key = row.task.asset_id || `task:${row.task.id}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
})

// 轮询闸门也只认 user_download：云准备行是本机行的影子，随本机行一起终态。
const liveDownloads = computed(() => tasks.value.filter((task) => task.purpose === 'user_download'))

// 三栏：收敛后每行按**最新那条任务**的状态归栏 —— 非终态进进行中、失败单列、其余终态进
// 历史。窗口跟着最新那条走（失败 90 天；历史里成功 30 天、取消 7 天）：同一素材更早的
// 失败不该把它的行从「历史」拉回「失败」。
const buckets = computed(() => {
  const list = rows.value
  return {
    active: list.filter((row) => !isTerminal(row.task)),
    failed: list.filter((row) => isFailed(row.task) && withinDays(row.task, 90)),
    history: list.filter((row) => isHistory(row.task)
      && (row.task.status === 'success' ? withinDays(row.task, 30) : withinDays(row.task, 7))),
  }
})

const visibleRows = computed(() => buckets.value[activeTab.value] || [])

// 计数只挂在当前这一栏上：历史有窗口，别的栏此刻没在屏幕上，给一个没算过的数才是编造。
const activeLabel = computed(() => (activeTab.value === 'active' ? `进行中 (${buckets.value.active.length})` : '进行中'))
const failedLabel = computed(() => (activeTab.value === 'failed' ? `失败 (${buckets.value.failed.length})` : '失败'))

// 历史 Tab 的表格式。`visibleRows` 已经做过取消 7 天窗口，表格直接吃这一份。
const historyColumns = [
  { colKey: 'title', title: '素材', width: 220 },
  { colKey: 'size', title: '大小', width: 90 },
  { colKey: 'finished_at', title: '完成时间', width: 140 },
  { colKey: 'status', title: '状态', width: 90 },
  { colKey: 'op', title: '操作', width: 180, fixed: 'right' },
]

// 失败行的「详情」：打开这条素材的详情抽屉。传输动作（重新下载）在下载中心里，详情只
// 回答「这是什么」——所以用不带页脚动作的 transfer 上下文，不冒充 mine/library 的关系。
const detailVisible = ref(false)
const detail = ref(null)
const detailLoading = ref(false)

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

// 轮询只在「面板开着」且「停在进行中」且「还有非终态任务」时进行。失败/历史都是终态
// 事实，拉一次就够；进行中 Tab 每一次 tick 都是一次真实的 GET，屏幕上的每个数字都来自
// 服务端，定时器只决定「多久问一次」。没有非终态任务时它立刻停下 —— 一个永远跑着的
// 2 秒轮询会浪费本机 Agent 与云端之间的带宽。
// 本机的持久化设备 id，用来认出悬置在这台设备上的任务（见 `canRetake`）。Desktop 运行时
// 才问（浏览器里 invoke 会炸，见 desktopBridge 的守卫说明），问不到就留着 null —— 认不
// 出就不重取，比「猜一个设备去重取」诚实。
async function loadLocalDevice() {
  if (!isDesktopRuntime()) return
  try {
    const identity = await invoke('local_device_identity')
    localDeviceId.value = identity?.device_id || null
  } catch {
    localDeviceId.value = null
  }
}

function stopPolling() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function ensurePolling() {
  if (!visible.value || activeTab.value !== 'active' || !hasLiveTask(liveDownloads.value)) {
    stopPolling()
    return
  }
  if (!timer) timer = setInterval(load, POLL_INTERVAL_MS)
}

async function load() {
  try {
    const data = await client.listTasks({ limit: LIST_LIMIT })
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

// 切 Tab 不再重新拉取（一次拉回全量、分栏在客户端做），只重估轮询闸门：停在进行中才轮询。
watch(activeTab, () => { if (visible.value) ensurePolling() })

watch(visible, async (open) => {
  if (!open) {
    stopPolling()
    return
  }
  // 重新打开要重扫一次：面板关着的这段时间里文件可能被搬走、被删，也可能换了保存位置。
  // 清掉指纹就够了 —— `load()` 结尾的 `ensurePresence()` 会补上这一次。
  measuredKey = null
  // 先问本机 device_id 再拉列表：悬置行的「重取」要拿它认出「发给这台设备」，首帧就得有，
  // 免得画面上先出现一帧没有重取按钮的样子。
  await loadLocalDevice()
  if (activeTab.value !== 'active') {
    // 停在别的 Tab 时切回去本身会触发上面的 activeTab watcher。
    activeTab.value = 'active'
    return
  }
  // 没有在跑的任务但有历史时直接落在历史，免得先看一屏空列表（沿用旧偏好）。
  load().then(() => {
    if (!hasLiveTask(liveDownloads.value)) activeTab.value = 'history'
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

/**
 * 失败 / 已取消行重新发起下载。
 *
 * 这是**新建一条任务**：服务端去重键的 generation 把终态算作已结束，所以重新点击会得到
 * 一条新行。走查裁定后 UI 上只有这一个动作 —— 原先并排的「重试」改的是原来那条行（且
 * 要求它没在等准备），两个按钮都在说「再来一次」，收成一个。
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

/**
 * 重取：把一条悬置（发给本机、没人领的 pending）或可重取（解绑遗留）的任务重新驱动起来。
 *
 * 悬置行先取消 —— pending 没有执行器，取消是立即终态（契约「cancelled when it had no
 * executor」）；再重新发起下载，去重键的 generation 把刚取消的那条算作已结束，新任务于是
 * 落在**当前**可信节点上，而不是仍指着已被替换的旧节点。可重取行已是终态，直接发起。
 * 两条都走与第一次点击**完全相同**的入口（`materials.createDownload`），设备维度的去重
 * 保证不会重复造任务。
 */
async function retake(task) {
  try {
    if (task.status === 'pending') {
      // 取消失败也照发：挡不住「重新发起」这一步；若确实没取消成，去重键会把新请求折叠回
      // 原行，不会造出重复任务。
      try { await client.cancelTask(task.id) } catch { /* 继续重取 */ }
    }
    await materials.createDownload(task.asset_id)
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

async function openDetail(row) {
  detailLoading.value = true
  try {
    detail.value = await materials.get(row.task.asset_id)
    detailVisible.value = true
  } catch (e) {
    MessagePlugin.error(e?.message || '打开素材失败')
  } finally {
    detailLoading.value = false
  }
}
</script>

<template>
  <t-drawer :close-btn="true" v-model:visible="visible" class="download-centre-drawer" header="下载中心" size="min(62vw, 880px)" :footer="false" destroy-on-close @close="close">
    <t-loading :loading="loading" :show-overlay="true">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:12px" @close="error=''" />
      <t-tabs v-model="activeTab" class="transfer-tabs">
        <t-tab-panel value="active" :label="activeLabel" />
        <t-tab-panel value="failed" :label="failedLabel" />
        <t-tab-panel value="history" label="历史" />
      </t-tabs>
      <p v-if="!visibleRows.length" class="transfer-empty">{{ activeTab === 'active' ? '暂无正在下载的任务' : activeTab === 'failed' ? '暂无失败的任务' : '暂无历史记录' }}</p>
      <template v-else>
        <!-- 进行中：紧凑列表，保留封面/标题/进度条/大小·速度·ETA/取消。 -->
        <ul v-if="activeTab === 'active'" class="transfer-list">
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
            <!-- 查过、所有已知位置都没有。这一句只在**成功**的行上说：别的行本来就没落盘。
                 只陈述事实 —— 已成功的素材不重下（走查裁定），所以这里不再承诺「可以重新下载」。 -->
            <p v-else-if="row.presence === 'absent' && row.task.status === 'success'" class="transfer-item__where">
              文件已不在本机已知的保存位置
            </p>

            <div class="transfer-item__footer">
              <p class="transfer-item__time">发起于 {{ row.createdText }}</p>
              <div class="transfer-item__actions">
                <!-- 进行中的行：取消；悬置在这台设备上、没人领的 pending 行多一个「重取」。
                     终态的「重新下载」「打开文件」在这一栏恒不成立（它们要求终态）。 -->
                <t-button v-if="row.canRetake" size="small" theme="primary" @click="retake(row.task)">重取</t-button>
                <t-button v-if="row.canCancel" size="small" class="wt-secondary-button" variant="outline" @click="cancel(row.task)">取消</t-button>
              </div>
            </div>
          </li>
        </ul>

        <!-- 失败：紧凑异常行，回答「哪条、为什么、什么时候」；传输动作就地给。 -->
        <ul v-else-if="activeTab === 'failed'" class="transfer-list">
          <li v-for="row in visibleRows" :key="row.task.id" class="transfer-item">
            <div class="transfer-item__head">
              <span class="transfer-item__title" :title="row.task.asset_title">{{ row.task.asset_title || `素材 #${row.task.asset_id}` }}</span>
              <ResourceStatusBadge :tone="row.state.tone" :label="row.state.label" />
            </div>
            <p class="transfer-item__error">{{ row.errorText }}</p>
            <div class="transfer-item__footer">
              <p class="transfer-item__time">失败于 {{ row.finishedText || row.createdText }}</p>
              <div class="transfer-item__actions">
                <t-button v-if="row.canRedownload" size="small" theme="primary" @click="redownload(row.task)">重新下载</t-button>
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
              </div>
            </div>
          </li>
        </ul>

        <!-- 历史：表格式。完成时间用终态行的 finished_at；成功可打开文件。已取消的行里，
             解绑遗留（device_unbound）的给「重取」—— 换机重投递的入口，为当前设备建新任务；
             其余已取消/失败给「重新下载」。已成功的素材不重下（走查裁定）。 -->
        <t-table v-else class="transfer-table" :data="visibleRows" :columns="historyColumns" :row-key="(row) => row.task.id" size="small" :scroll="{ x: 640 }">
          <template #title="{ row }">
            <span class="transfer-table__title" :title="row.task.asset_title">{{ row.task.asset_title || `素材 #${row.task.asset_id}` }}</span>
          </template>
          <template #size="{ row }">{{ row.sizeText }}</template>
          <template #finished_at="{ row }">{{ row.finishedText || row.createdText }}</template>
          <template #status="{ row }"><ResourceStatusBadge :tone="row.state.tone" :label="row.state.label" /></template>
          <template #op="{ row }">
            <t-space class="wt-resource-actions">
              <t-button v-if="row.canOpen && isDesktop()" size="small" class="wt-secondary-button" variant="outline" @click="open(row.task)">打开文件</t-button>
              <!-- 解绑遗留（device_unbound）的行叫「重取」而不是「重新下载」：它是换机重投递
                   的入口，重取会为当前设备建一条新任务。其余已取消/失败的行照旧「重新下载」。 -->
              <t-button v-if="row.canRetake" size="small" theme="primary" @click="retake(row.task)">重取</t-button>
              <t-button v-else-if="row.canRedownload" size="small" theme="primary" @click="redownload(row.task)">重新下载</t-button>
            </t-space>
          </template>
        </t-table>
      </template>
    </t-loading>

    <MaterialDetailDrawer v-model:visible="detailVisible" :material="detail" :loading="detailLoading" mode="transfer" />
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
/* `display: block` 是这一条的**尺子**：省略号三件套在行内元素上不生效，标题会直接冲出
   单元格（走查报的「每行最后标题冲出边界」就是这个）。有了它，宽度才由单元格定下来。 */
.transfer-table__title { display: block; max-width: 100%; color: var(--wt-text-primary); font-weight: 600; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.download-centre-drawer :deep(.t-drawer__body) { padding: 16px; }
@media (max-width: 620px) {
  .transfer-item { padding: 12px; }
  .transfer-item__footer { align-items: flex-start; flex-direction: column; }
  .transfer-item__actions { justify-content: flex-start; }
  .download-centre-drawer :deep(.t-drawer__body) { padding: 14px; }
}
</style>
