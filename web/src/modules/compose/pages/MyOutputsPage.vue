<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { createComposeClient } from '../../../shared/api/compose.js'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { createFileTransferClient } from '../../../shared/api/fileTransfer.js'
import { createSessionClient } from '../../../shared/api/session.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { createdRangeParams } from '../createdRange.js'
import { formatBytes, formatDurationMs } from '../../../shared/utils/units.js'
import ComposeOutputDetail from '../components/ComposeOutputDetail.vue'
import ComposeStrategySummary from '../components/ComposeStrategySummary.vue'
import MaterialDetailDrawer from '../../materials/MaterialDetailDrawer.vue'
import { useDownloadCentre } from '../../transfer/downloadCentre.js'
import { createDownloadFailureMessage } from '../../transfer/downloadErrors.js'
import { downloadedFileName } from '../../transfer/downloadFacts.js'
import { isDesktopRuntime, revealSavedFile, savedFileStates } from '../../transfer/desktopBridge.js'
import { canQueueOutputDownload, generationLabel, outputDownloadState, outputTitle } from '../outputManagement.js'
import { publicObjectReadUrl } from '../../../shared/utils/objectReadUrl.js'

const client = createComposeClient()
const materialsClient = createMaterialsClient()
const transferClient = createFileTransferClient()
const sessionClient = createSessionClient()
const route = useRoute()
const downloadCentre = useDownloadCentre()
const downloadingID = ref(null)
let downloadPoller = null

const rows = ref([])
const materials = ref([])
const taskRows = ref([])
const lookupsReady = ref(false)
const attemptedTasks = new Set()
const attemptedMaterials = new Set()
const downloadTasks = ref([])
const localFiles = ref({})
const downloadKnown = ref(false)
const strategies = ref([])
const teams = ref([])
const games = ref([])
const templates = ref([])
const loading = ref(false)
const error = ref('')
const currentUser = ref(null)
const operatorOptions = ref([])
const operatorsLoading = ref(false)
const filters = ref({ search: '', trigger: '', download: '', created_range: [], team_id: '', operator: '' })
const pagination = ref({ current: 1, pageSize: 20 })
const selectedRowKeys = ref([])
const batchDownloading = ref(false)
const batchResult = ref('')

const triggerOptions = [{ label: '人工', value: 'manual' }, { label: '定时', value: 'scheduled' }]
const downloadOptions = [
  { label: '未下载', value: 'not_downloaded' },
  { label: '下载中', value: 'downloading' },
  { label: '已下载', value: 'downloaded' },
  { label: '下载失败', value: 'failed' },
  { label: '本机文件缺失', value: 'missing' },
  { label: '本机未核验', value: 'unverified' },
]
const taskStatusLabels = { pending: '排队中', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
const taskStatusTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }
const isSenior = computed(() => currentUser.value?.role === 'senior_operator')
const isAdmin = computed(() => currentUser.value?.role === 'admin')
const canFilterOperators = computed(() => isSenior.value || isAdmin.value)
const operatorFilterOptions = computed(() => {
  const options = []
  if (isSenior.value) options.push({ value: 'mine', label: '我的' }, { value: 'team', label: '团队全部' })
  return options.concat(operatorOptions.value.map((option) => ({ value: `op:${option.id}`, label: option.display_name })))
})

const columns = computed(() => [
  { colKey: 'row-select', type: 'multiple', width: 48, checkProps: ({ row }) => ({ disabled: !canDownload(row) }) },
  { colKey: 'id', title: '成片 ID', width: 90 },
  { colKey: 'output', title: '成片', width: 260 },
  { colKey: 'material_id', title: '来源素材', width: 190 },
  { colKey: 'strategy', title: '合成策略', width: 180 },
  { colKey: 'operator', title: '负责人', width: 110 },
  { colKey: 'trigger', title: '生成方式', width: 95 },
  { colKey: 'download_status', title: '下载状态', width: 170 },
  { colKey: 'created_at', title: '生成时间', width: 150 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['详情', '重新下载'], { loading: true }), fixed: 'right' },
])

// 横向滚动宽度由列宽推导，不写死。
const tableScroll = computed(() => ({
  x: `${columns.value.reduce((sum, col) => sum + (col.width || 0), 0)}px`,
}))

// 综合搜索与生成方式由列表接口完成。下载状态来自 Desktop 本机事实（传输任务 +
// 本地文件核验），列表接口不返回该字段，只能保留为本地叠加。
const filteredRows = computed(() => rows.value.filter((row) => !filters.value.download || downloadFor(row).key === filters.value.download))

const pagedRows = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return filteredRows.value.slice(start, start + pagination.value.pageSize)
})

function strategyName(row) {
  const found = strategies.value.find((item) => item.id === row.compose_strategy_id)
  return found ? found.name : `策略 ${row.compose_strategy_id}`
}

// 负责人候选目录与任务页同一套：高级运营固定本团队，管理员按所选团队取。
async function loadOperators() {
  if (!canFilterOperators.value) return
  if (isAdmin.value && !filters.value.team_id) {
    operatorOptions.value = []
    filters.value.operator = ''
    return
  }
  operatorsLoading.value = true
  try {
    const data = await client.listComposeOperators({ team_id: isAdmin.value ? Number(filters.value.team_id) : undefined })
    operatorOptions.value = Array.isArray(data) ? data : []
    if (filters.value.operator.startsWith('op:') && !operatorOptions.value.some((option) => filters.value.operator === `op:${option.id}`)) filters.value.operator = ''
  } catch {
    operatorOptions.value = []
  } finally {
    operatorsLoading.value = false
  }
}

function onTeamFilterChange() {
  filters.value.operator = ''
  loadOperators()
}

function materialFor(row) { return materials.value.find((item) => item.id === row.material_id) }
function taskFor(row) { return taskRows.value.find((item) => item.id === row.compose_task_id) }
function titleFor(row) { return outputTitle(row, materialFor(row)?.title, strategies.value.find((item) => item.id === row.compose_strategy_id)?.name) }
function triggerFor(row) { return taskFor(row)?.trigger_type || '' }
function downloadFor(row) {
  if (!downloadKnown.value) return { key: 'unknown', label: '状态暂不可用', tone: 'neutral' }
  const own = downloadTasks.value.filter((task) => task.asset_type === 'composite_output')
  const name = downloadedFileName(own, row.id)
  return outputDownloadState(own, row.id, name ? localFiles.value[name] : null, isDesktopRuntime())
}
function canDownload(row) { return canQueueOutputDownload(downloadFor(row)) }

async function refreshDownloadFacts() {
  try {
    const data = await transferClient.listTasks()
    const tasks = Array.isArray(data) ? data : []
    const own = tasks.filter((task) => task.asset_type === 'composite_output')
    const outputIds = [...rows.value.map((row) => row.id), detail.value?.output?.id].filter(Boolean)
    const names = outputIds.map((id) => downloadedFileName(own, id)).filter(Boolean)
    const files = await savedFileStates(names)
    downloadTasks.value = own
    localFiles.value = files
    downloadKnown.value = true
    selectedRowKeys.value = selectedRowKeys.value.filter((id) => {
      const row = rows.value.find((item) => item.id === id)
      return row && canDownload(row)
    })
  } catch {
    downloadKnown.value = false
  }
}

async function loadLookups() {
  const [materialRows, strategyRows, composeTasks, teamRows, gameRows] = await Promise.all([
    materialsClient.list().catch(() => []),
    client.listStrategies().catch(() => []),
    client.listComposeTasks({}).catch(() => []),
    client.listTeams().catch(() => []),
    client.listGames().catch(() => []),
  ])
  materials.value = [...new Map([...materials.value, ...(Array.isArray(materialRows) ? materialRows : [])].map((item) => [item.id, item])).values()]
  strategies.value = Array.isArray(strategyRows) ? strategyRows : []
  taskRows.value = Array.isArray(composeTasks) ? composeTasks : []
  teams.value = Array.isArray(teamRows) ? teamRows : []
  games.value = Array.isArray(gameRows) ? gameRows : []
  lookupsReady.value = true
}

// 任务列表只保留最近 200 条。当前页遇到更早的成片时按真实关联 ID 补取，
// 避免把「查不到任务」误当成一种生成方式；素材名称同理。
watch(() => `${lookupsReady.value}|${pagedRows.value.map((row) => `${row.material_id}:${row.compose_task_id}`).join(',')}`, async () => {
  if (!lookupsReady.value) return
  const visible = pagedRows.value
  const missingTasks = visible.map((row) => row.compose_task_id).filter((id) => id && !taskFor({ compose_task_id: id }) && !attemptedTasks.has(id))
  const missingMaterials = visible.map((row) => row.material_id).filter((id) => id && !materialFor({ material_id: id }) && !attemptedMaterials.has(id))
  if (!missingTasks.length && !missingMaterials.length) return
  missingTasks.forEach((id) => attemptedTasks.add(id))
  missingMaterials.forEach((id) => attemptedMaterials.add(id))
  const [tasks, materialRows] = await Promise.all([
    Promise.all(missingTasks.map((id) => client.getComposeTask(id).catch(() => null))),
    Promise.all(missingMaterials.map((id) => materialsClient.get(id).catch(() => null))),
  ])
  if (tasks.some(Boolean)) taskRows.value = [...taskRows.value, ...tasks.filter(Boolean)]
  if (materialRows.some(Boolean)) materials.value = [...materials.value, ...materialRows.filter(Boolean)]
})

// media 是执行时量到的文件事实（时长等）；解析不动就当没有，不替服务端编数。
function durationText(row) { return formatDurationMs(row.media?.duration_ms) }

async function load({ resetPage = false } = {}) {
  loading.value = true
  error.value = ''
  try {
    const params = {}
    Object.assign(params, createdRangeParams(filters.value.created_range))
    // 「成片 ID/标题、素材 ID/名称、策略名称」的搜索由服务端一次完成，
    // 不再二次拉素材列表再在本地匹配。
    params.search = filters.value.search.trim()
    params.trigger_type = filters.value.trigger
    // 负责人筛选与服务端同一套 scope，和任务页一致：普通运营不发；高级运营默认我的，
    // 可切团队全部或指定运营；管理员默认全部，按团队收窄。
    const namedOperator = filters.value.operator.startsWith('op:') ? Number(filters.value.operator.slice(3)) : 0
    if (isAdmin.value) {
      if (filters.value.team_id) params.team_id = Number(filters.value.team_id)
      if (namedOperator) params.operator_id = namedOperator
      else if (filters.value.team_id) params.operator_scope = 'team'
    } else if (isSenior.value) {
      if (namedOperator) params.operator_id = namedOperator
      else if (filters.value.operator === 'team') params.operator_scope = 'team'
    }
    const data = await client.listComposeOutputs(params)
    rows.value = Array.isArray(data) ? data : []
    if (resetPage) pagination.value.current = 1
    selectedRowKeys.value = selectedRowKeys.value.filter((id) => rows.value.some((row) => row.id === id))
    await refreshDownloadFacts()
  } catch (e) {
    error.value = e?.message || '读取成片列表失败'
  } finally {
    loading.value = false
  }
}

function reset() {
  filters.value = { search: '', trigger: '', download: '', created_range: [], team_id: '', operator: '' }
  load({ resetPage: true })
}

async function loadTemplates() {
  try {
    const data = await client.listTemplates()
    templates.value = Array.isArray(data) ? data : []
  } catch {
    templates.value = []
  }
}

function clearSelection() { selectedRowKeys.value = [] }

const materialDrawer = ref({ visible: false, loading: false, row: null })
const strategyDrawer = ref({ visible: false, loading: false, row: null })
const taskDrawer = ref({ visible: false, loading: false, row: null })

async function openMaterial(id) {
  if (!id) return
  materialDrawer.value = { visible: true, loading: true, row: null }
  try { materialDrawer.value.row = await materialsClient.get(id) }
  catch (error) { feedback.toast.error(error?.message || '打开素材失败'); materialDrawer.value.visible = false }
  finally { materialDrawer.value.loading = false }
}

async function openStrategy(id) {
  if (!id) return
  strategyDrawer.value = { visible: true, loading: true, row: null }
  try { strategyDrawer.value.row = await client.getStrategy(id) }
  catch (error) { feedback.toast.error(error?.message || '打开策略失败'); strategyDrawer.value.visible = false }
  finally { strategyDrawer.value.loading = false }
}

async function openTask(id) {
  if (!id) return
  taskDrawer.value = { visible: true, loading: true, row: null }
  try { taskDrawer.value.row = await client.getComposeTask(id) }
  catch (error) { feedback.toast.error(error?.message || '打开任务失败'); taskDrawer.value.visible = false }
  finally { taskDrawer.value.loading = false }
}

function teamName(id) { return teams.value.find((item) => item.id === id)?.name || String(id || '—') }
function gameName(id) { return games.value.find((item) => item.id === id)?.name || String(id || '—') }

const detailVisible = ref(false)
const detail = ref(null)
const detailTab = ref('basic')
const detailTask = ref(null)
const previewUrl = ref('')
const previewLoading = ref(false)

async function openDetail(row) {
  detailVisible.value = true
  detailTab.value = 'basic'
  detail.value = null
  detailTask.value = null
  previewUrl.value = ''
  try {
    // 详情按 id 重取：行是列表加载那一刻的副本，追踪信息要库里现在的那份。
    // 预览每次打开都由现有接口取准确对象地址，不复用上一次的链接。
    // 地址取失败不挡详情：预览区给一句错误，来源追踪照常看。
    const [data, link] = await Promise.all([
      client.getComposeOutput(row.id),
      client.getComposeOutputVideoUrl(row.id).catch(() => null),
    ])
    detail.value = data
    await refreshDownloadFacts()
    if (data?.output?.compose_task_id) {
      detailTask.value = await client.getComposeTask(data.output.compose_task_id).catch(() => null)
    }
    previewUrl.value = publicObjectReadUrl(link?.url)
    if (!previewUrl.value) feedback.toast.error('预览地址获取失败，可在预览区重试')
  } catch (e) {
    feedback.toast.error(e?.message || '打开成片失败')
    detailVisible.value = false
  }
}

async function retryPreview() {
  if (!detail.value || previewLoading.value) return
  previewLoading.value = true
  try {
    const link = await client.getComposeOutputVideoUrl(detail.value.output.id)
    previewUrl.value = publicObjectReadUrl(link?.url)
    if (!previewUrl.value) feedback.toast.error('预览地址获取失败，请稍后重试')
  } catch (e) {
    feedback.toast.error(e?.message || '预览地址获取失败，请稍后重试')
  } finally {
    previewLoading.value = false
  }
}

async function download(row) {
  if (downloadingID.value !== null) return
  if (!canDownload(row)) return
  downloadingID.value = row.id
  try {
    await client.createOutputDownload(row.id)
    feedback.toast.info('下载任务已提交，可在下载中心查看进度')
    downloadCentre.open()
    await refreshDownloadFacts()
  } catch (e) {
    feedback.toast.error(createDownloadFailureMessage(e))
  } finally {
    downloadingID.value = null
  }
}

async function batchDownload() {
  if (batchDownloading.value) return
  batchDownloading.value = true
  const targets = rows.value.filter((row) => selectedRowKeys.value.includes(row.id))
  const failures = []
  let submitted = 0
  let skipped = 0
  try {
    for (const row of targets) {
      if (!canDownload(row)) {
        skipped += 1
        continue
      }
      try {
        await client.createOutputDownload(row.id)
        submitted += 1
      } catch (error) {
        failures.push(`成片 ${row.id}：${createDownloadFailureMessage(error)}`)
      }
    }
    batchResult.value = `共 ${targets.length} 条：已提交 ${submitted} 个下载任务，失败 ${failures.length} 条，跳过 ${skipped} 条。${failures.join('；')}`
    if (failures.length) feedback.toast.error(`有 ${failures.length} 条下载任务提交失败，请查看页面提示`)
    else feedback.toast.info(`已提交 ${submitted} 个下载任务`)
    if (submitted) downloadCentre.open()
    clearSelection()
    await refreshDownloadFacts()
  } finally {
    batchDownloading.value = false
  }
}

async function openLocalFolder(row) {
  const state = downloadFor(row)
  if (state.key !== 'downloaded' || !state.fileName) return
  try {
    await revealSavedFile(state.fileName)
  } catch (error) {
    feedback.toast.error(error?.message || '打开本机文件失败')
    await refreshDownloadFacts()
  }
}

onMounted(() => {
  load()
  loadLookups()
  loadTemplates()
  // 只为负责人筛选：读失败不拦页面，服务端仍按角色默认范围返回。
  sessionClient.me().then((me) => {
    currentUser.value = me
    if (isSenior.value && !filters.value.operator) filters.value.operator = 'mine'
    loadOperators()
  }).catch(() => {})
  downloadPoller = setInterval(() => {
    if (downloadTasks.value.some((task) => task.status === 'pending' || task.status === 'running')) refreshDownloadFacts()
  }, 5000)
  const id = String(route.query.output_id || '')
  if (/^\d+$/.test(id)) openDetail({ id: Number(id) })
})
onUnmounted(() => { if (downloadPoller) clearInterval(downloadPoller) })
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true">
    <div class="wt-resource-page my-outputs-page">
      <FeedbackAlert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
      <ResourcePageHeader title="成片管理" description="管理已生成的视频成片，查看来源、预览并下载文件">
        <template #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button>
        </template>
      </ResourcePageHeader>
      <ResourceCard class="my-outputs-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="filters.search" :maxlength="100" clearable placeholder="成片 ID/标题、素材 ID/名称" /></label>
          <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">团队</span><t-select v-model="filters.team_id" clearable placeholder="全部" @change="onTeamFilterChange">
            <t-option v-for="team in teams" :key="team.id" :value="team.id" :label="team.name" />
          </t-select></label>
          <label v-if="canFilterOperators" class="wt-filter-field"><span class="wt-filter-field__label">负责人</span><t-select v-model="filters.operator" :options="operatorFilterOptions" :clearable="!isSenior" :loading="operatorsLoading" :disabled="isAdmin && !filters.team_id" placeholder="全部" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">生成方式</span><t-select v-model="filters.trigger" :options="triggerOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">下载状态</span><t-select v-model="filters.download" :options="downloadOptions" clearable placeholder="全部" /></label>
          <label class="wt-filter-field compose-date-filter"><span class="wt-filter-field__label">生成时间</span><t-date-range-picker v-model="filters.created_range" value-type="YYYY-MM-DD" clearable /></label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="load({ resetPage: true })">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="reset">重置</t-button>
          </div>
        </div>
        <FeedbackAlert v-if="batchResult" theme="info" :message="batchResult" closable class="output-batch-result" @close="batchResult=''" />
        <div v-if="selectedRowKeys.length" class="output-selection-toolbar" aria-live="polite">
          <span>已选 {{ selectedRowKeys.length }} 项</span>
          <t-button theme="primary" :loading="batchDownloading" @click="batchDownload">批量下载</t-button>
        </div>
        <div class="table-scroll-wrap">
          <t-table v-model:selected-row-keys="selectedRowKeys" class="wt-resource-table" :data="pagedRows" :columns="columns" row-key="id" hover size="small" :scroll="tableScroll" empty="还没有成片">
            <template #id="{ row }"><span class="output-id">{{ row.id }}</span></template>
            <template #output="{ row }">
              <div class="output-asset-cell">
                <button type="button" class="output-asset-cell__preview" aria-label="查看成片详情" @click="openDetail(row)">▶</button>
                <div class="output-related-cell">
                  <strong class="output-related-cell__main" :title="titleFor(row)">{{ titleFor(row) }}</strong>
                  <span class="output-related-cell__sub">{{ durationText(row) }} · {{ formatBytes(row.file_size_bytes) }}</span>
                </div>
              </div>
            </template>
            <template #material_id="{ row }"><t-link theme="primary" @click="openMaterial(row.material_id)">素材 {{ row.material_id }}</t-link><span class="output-related-cell__sub output-source-title">{{ materialFor(row)?.title || '—' }}</span></template>
            <template #strategy="{ row }">
              <div class="output-related-cell">
                <t-link theme="primary" :title="strategyName(row)" @click="openStrategy(row.compose_strategy_id)">{{ strategyName(row) }}</t-link>
              </div>
            </template>
            <template #operator="{ row }">{{ row.operator_name || '未记录' }}</template>
            <template #trigger="{ row }">{{ generationLabel(triggerFor(row)) }}</template>
            <template #download_status="{ row }"><ResourceStatusBadge :tone="downloadFor(row).tone" :label="downloadFor(row).label" /></template>
            <template #created_at="{ row }">{{ formatDateTime(row.created_at) }}</template>
            <template #op="{ row }">
              <t-space class="wt-resource-actions">
                <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
                <t-button v-if="canDownload(row)" size="small" class="wt-secondary-button" variant="outline" :loading="downloadingID === row.id" @click="download(row)">{{ downloadFor(row).key === 'not_downloaded' ? '下载' : '重新下载' }}</t-button>
              </t-space>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="filteredRows.length" :page-size-options="[10, 20, 50]" /></div>
      </ResourceCard>

      <t-drawer :close-btn="true" v-model:visible="detailVisible" header="成片详情" size="min(92vw, 800px)" :footer="Boolean(detail && canDownload(detail.output))">
        <template #footer>
          <t-space v-if="detail" class="wt-resource-actions">
            <t-button v-if="canDownload(detail.output)" theme="primary" :loading="downloadingID === detail.output.id" @click="download(detail.output)">{{ downloadFor(detail.output).key === 'not_downloaded' ? '下载' : '重新下载' }}</t-button>
          </t-space>
        </template>
        <t-loading :loading="!detail" :show-overlay="true">
          <ComposeOutputDetail
            v-if="detail"
            v-model:tab="detailTab"
            :detail="detail"
            :templates="templates"
            :preview-url="previewUrl"
            :preview-loading="previewLoading"
            management
            :title="outputTitle(detail.output, detail.trace?.material?.title || materialFor(detail.output)?.title, detail.trace?.strategy?.name || strategyName(detail.output))"
            :trigger-type="detailTask?.trigger_type || triggerFor(detail.output)"
            :task-name="detailTask?.display_name || taskFor(detail.output)?.display_name || ''"
            :download-state="downloadFor(detail.output)"
            linkable
            @retry-preview="retryPreview"
            @open-material="openMaterial"
            @open-strategy="openStrategy"
            @open-task="openTask"
            @open-folder="openLocalFolder(detail.output)"
          />
        </t-loading>
      </t-drawer>
      <MaterialDetailDrawer :visible="materialDrawer.visible" :material="materialDrawer.row" :loading="materialDrawer.loading" :games="games" mode="readonly" :show-page-link="false" @update:visible="materialDrawer.visible = $event" />
      <t-drawer :visible="strategyDrawer.visible" header="策略详情" size="min(62vw, 880px)" :close-btn="true" :footer="false" @update:visible="strategyDrawer.visible = $event">
        <t-loading :loading="strategyDrawer.loading" :show-overlay="true"><ComposeStrategySummary v-if="strategyDrawer.row" :strategy="strategyDrawer.row" :team="teamName(strategyDrawer.row.team_id)" :game="gameName(strategyDrawer.row.game_id)" :templates="templates" /></t-loading>
      </t-drawer>
      <t-drawer :visible="taskDrawer.visible" header="任务详情" size="min(62vw, 880px)" :close-btn="true" :footer="false" @update:visible="taskDrawer.visible = $event">
        <t-loading :loading="taskDrawer.loading" :show-overlay="true">
          <section v-if="taskDrawer.row" class="output-task-summary">
            <h4>任务信息</h4>
            <div class="output-task-summary__grid">
              <div><span>任务 ID</span><strong>{{ taskDrawer.row.id }}</strong></div>
              <div><span>任务名称</span><strong>{{ taskDrawer.row.display_name || `合成任务 ${taskDrawer.row.id}` }}</strong></div>
              <div><span>生成方式</span><strong>{{ generationLabel(taskDrawer.row.trigger_type) }}</strong></div>
              <div><span>任务状态</span><ResourceStatusBadge :tone="taskStatusTones[taskDrawer.row.status] || 'neutral'" :label="taskStatusLabels[taskDrawer.row.status] || '—'" /></div>
              <div><span>来源素材</span><strong>素材 {{ taskDrawer.row.material_id }}</strong></div>
              <div><span>合成策略</span><strong>{{ strategyName(taskDrawer.row) }}</strong></div>
              <div><span>创建时间</span><strong>{{ formatDateTime(taskDrawer.row.created_at) }}</strong></div>
              <div><span>开始时间</span><strong>{{ formatDateTime(taskDrawer.row.started_at) }}</strong></div>
              <div><span>完成时间</span><strong>{{ formatDateTime(taskDrawer.row.finished_at) }}</strong></div>
              <div><span>执行次数</span><strong>{{ taskDrawer.row.attempt_count }}</strong></div>
            </div>
          </section>
        </t-loading>
      </t-drawer>
    </div>
  </t-loading>
</template>

<style scoped>
.my-outputs-card { padding: 14px 20px 18px; }
.filter-bar > .compose-date-filter { flex-basis: 300px; }
.compose-date-filter :deep(.t-date-range-picker) { min-width: 0; width: 100%; }
.output-batch-result { margin: 0 0 12px; }
.output-selection-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; padding: 10px 12px; margin-bottom: 12px; background: var(--wt-bg-page); border-radius: var(--wt-radius-sm); font-size: 13px; color: var(--wt-text-primary); }
.output-id { color: var(--wt-text-primary); font-weight: 600; font-variant-numeric: tabular-nums; }
.output-asset-cell { display: flex; align-items: center; gap: 10px; min-width: 0; }
.output-asset-cell__preview { flex: 0 0 56px; width: 56px; height: 56px; border: 0; border-radius: var(--wt-radius-sm); background: var(--wt-bg-page); color: var(--wt-primary); cursor: pointer; }
.output-related-cell { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.output-related-cell__main { overflow: hidden; color: var(--wt-text-primary); font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.output-related-cell__sub { color: var(--wt-text-tertiary); font-size: 12px; }
.output-source-title { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.output-task-summary { padding: 16px; border: 1px solid var(--wt-border); border-radius: var(--wt-radius-md); }
.output-task-summary h4 { margin: 0 0 16px; font-size: 15px; }
.output-task-summary__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 24px; }
.output-task-summary__grid > div { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
.output-task-summary__grid span { color: var(--wt-text-tertiary); font-size: 12px; }
.output-task-summary__grid strong { color: var(--wt-text-primary); font-size: 14px; font-weight: 500; overflow-wrap: anywhere; }
@media (max-width: 700px) { .output-task-summary__grid { grid-template-columns: 1fr; } }
</style>
