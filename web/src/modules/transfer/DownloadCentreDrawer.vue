<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { createFileTransferClient } from '../../shared/api/fileTransfer.js'
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { isDesktop } from '../../utils.js'
import { openSavedFile } from './desktopBridge.js'
import { hasLiveTask, isTerminal } from './downloadFacts.js'
import { useDownloadCentre } from './downloadCentre.js'
import { transferRows } from './transferRows.js'

const client = createFileTransferClient()
const { visible, close } = useDownloadCentre()
const tasks = ref([])
const loading = ref(false)
const error = ref('')
// 本机记忆：我按过取消的那几条。冻结的任务体没有 cancel_requested_at，取消一个
// running 任务后它仍然报 running（终态只能由执行器写），所以「正在取消」推不出来，
// 只能记。刷新出终态后这条记录就不再有意义，由 taskState 的终态分支盖住。
const cancelRequested = ref([])

const POLL_INTERVAL_MS = 2000
let timer = null

const rows = computed(() => transferRows(tasks.value, { cancelRequested: cancelRequested.value }))

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
}

watch(visible, (open) => {
  if (open) load()
  else stopPolling()
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
  <t-drawer v-model:visible="visible" class="download-centre-drawer" header="下载中心" size="min(46vw, 640px)" destroy-on-close @close="close">
    <t-loading :loading="loading" :show-overlay="true">
      <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:12px" @close="error=''" />
      <p v-if="!rows.length" class="transfer-empty">还没有下载任务。</p>
      <ul class="transfer-list">
        <li v-for="row in rows" :key="row.task.id" class="transfer-item">
          <div class="transfer-item__head">
            <span class="transfer-item__title" :title="row.task.asset_title">{{ row.task.asset_title || `素材 #${row.task.asset_id}` }}</span>
            <ResourceStatusBadge :tone="row.state.tone" :label="row.state.label" />
          </div>

          <div class="transfer-item__progress">
            <t-progress v-if="row.showBar" theme="line" :percentage="row.progress.percent" :label="false" />
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
          <p class="transfer-item__time">发起于 {{ row.createdText }}</p>

          <div class="transfer-item__actions">
            <t-button v-if="row.canCancel" size="small" class="wt-secondary-button" variant="outline" @click="cancel(row.task)">取消</t-button>
            <t-button v-if="row.canRetry" size="small" theme="primary" @click="retry(row.task)">重试</t-button>
            <!--
              打开文件是桌面端专属：文件落在运营这台机器上（Agent 写的），浏览器打不开它。
              守卫写在按钮上而不是点下去再报错 —— 一个点了必然失败的按钮不该出现在那里。
            -->
            <t-button v-if="row.canOpen && isDesktop()" size="small" class="wt-secondary-button" variant="outline" @click="open(row.task)">打开文件</t-button>
          </div>
        </li>
      </ul>
    </t-loading>
  </t-drawer>
</template>

<style scoped>
.transfer-empty { margin: 0; color: var(--wt-text-tertiary); font-size: 14px; }
.transfer-list { display: flex; flex-direction: column; gap: 12px; margin: 0; padding: 0; list-style: none; }
.transfer-item { padding: 14px 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); background: var(--wt-bg-card); }
.transfer-item__head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.transfer-item__title { color: var(--wt-text-primary); font-size: 14px; font-weight: 600; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.transfer-item__progress { display: flex; align-items: center; min-height: 22px; margin-top: 10px; }
.transfer-item__pending { color: var(--wt-text-tertiary); font-size: 13px; }
/* 各项用 gap 分开而不是「·」串起来：宽度不够时「·」会在任意位置折断，标签和数字被拆散。 */
.transfer-item__meta { display: flex; flex-wrap: wrap; gap: 4px 14px; margin-top: 8px; color: var(--wt-text-tertiary); font-size: 12px; font-variant-numeric: tabular-nums; }
.transfer-item__error { margin: 8px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
.transfer-item__time { margin: 6px 0 0; color: var(--wt-text-tertiary); font-size: 12px; }
.transfer-item__actions { display: flex; gap: 8px; margin-top: 12px; }
</style>
