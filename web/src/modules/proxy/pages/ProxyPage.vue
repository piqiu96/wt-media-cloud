<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { onBeforeRouteLeave } from "vue-router"
import { createProxyClient } from "../../../shared/api/proxy.js"
import { createSessionClient } from "../../../shared/api/session.js"
import { createUsersClient } from "../../../apps/cloud/pages/users/usersApi.js"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import { invoke } from "@tauri-apps/api/core"
import { isDesktop } from "../../../utils.js"
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue"
import { feedback } from "../../../shared/ui/feedback/index.js"
import OperationProgressDialog from "../../../shared/ui/feedback/OperationProgressDialog.vue"
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue"
import ResourceForm from "../../../shared/ui/resource/ResourceForm.vue"
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue"
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue"
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue"
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue"
import ResourceChoiceCards from "../../../shared/ui/resource/ResourceChoiceCards.vue"
import ResourceSelectionToolbar from "../../../shared/ui/resource/ResourceSelectionToolbar.vue"
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue"
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue"
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue"
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue"
import { opColumnWidth } from "../../../shared/ui/resource/opColumnWidth.js"
import { markImportUnknown, prepareFailedCheckRetry, reconcileImportItems } from "./importResult.js"
import { MAX_IMPORT_ROWS, PROXY_IMPORT_TEMPLATE, readProxyImportFile } from "./importFile.js"

const proxyClient = createProxyClient()
const localAgent = createLocalAgentService({ invoke })
const sessionClient = createSessionClient()
const usersClient = createUsersClient()

const proxies = ref([])
const loading = ref(true)
const error = ref("")
const currentUser = ref(null)
const teams = ref([])
const selectedRowKeys = ref([])
const checkingIds = ref([])
const pagination = ref({ current: 1, pageSize: 20, total: 0 })

const advancedFilters = ref(false)
const filterTeam = ref("")
const filterStatus = ref("")
const filterRegion = ref("")
const filterCheck = ref("")
const filterExpiry = ref("")
const filterText = ref("")

const isAdmin = computed(() => currentUser.value?.role === "admin")
const isGovernor = computed(() => ["admin", "senior_operator"].includes(currentUser.value?.role))
const canExtract = computed(() => isDesktop() && ["operator", "senior_operator"].includes(currentUser.value?.role))

const teamOptions = computed(() => teams.value.map((team) => ({ label: team.name, value: String(team.id) })))
const exactFilterTeamID = computed(() => String(teams.value.find(team =>
  String(team.id) === filterTeam.value.trim() || String(team.name || '').trim() === filterTeam.value.trim())?.id || ''))
const ownTeamLabel = computed(() => currentUser.value?.team_name || "本人团队")

const visibleProxies = computed(() => proxies.value.filter((proxy) => {
  if (filterCheck.value === "unchecked" && proxy.last_check_result) return false
  if (filterCheck.value === "ok" && proxy.last_check_result !== "ok") return false
  if (filterCheck.value === "failed" && (!proxy.last_check_result || proxy.last_check_result === "ok")) return false
  if (filterExpiry.value === "expired" && !isExpired(proxy)) return false
  if (filterExpiry.value === "valid" && isExpired(proxy)) return false
  return true
}))

const pagedProxies = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return visibleProxies.value.slice(start, start + pagination.value.pageSize)
})
const hasProxyFilter = computed(() => [filterTeam.value, filterStatus.value, filterRegion.value,
  filterCheck.value, filterExpiry.value, filterText.value]
  .some(value => String(value || '').trim()))

const proxyActionLabels = computed(() => isGovernor.value
  ? ["详情", "编辑", "检测", "设置配额", "删除"]
  : ["详情", "编辑", "检测"])
const columns = computed(() => [
  { colKey: "row-select", type: "multiple", width: 48, disabled: ({ row }) => !row.host || !row.port },
  { colKey: "id", title: "ID", width: 70 },
  { colKey: "name", title: "名称 / 地址", width: 200 },
  { colKey: "region", title: "地区", width: 90 },
  { colKey: "source", title: "来源", width: 80 },
  { colKey: "usage", title: "已用 / 额度", width: 110 },
  { colKey: "enabled", title: "资源启停", width: 100 },
  { colKey: "last_check_result", title: "检测结果", width: 110 },
  { colKey: "expires_at", title: "到期时间", width: 120 },
  { colKey: "remark", title: "备注", width: 170 },
  { colKey: "op", title: "操作", width: opColumnWidth(proxyActionLabels.value, { variant: "text", loading: true, primary: true }), fixed: "right" },
])

const tableScroll = computed(() => ({ x: `${columns.value.reduce((sum, column) => sum + (column.width || 0), 0)}px` }))
watch([() => pagination.value.current, () => pagination.value.pageSize], () => { selectedRowKeys.value = [] })

// 新增 / 编辑抽屉
const formVisible = ref(false)
const formMode = ref("create")
const formError = ref("")
const savingForm = ref(false)
const parsingAddress = ref(false)
const extracting = ref(false)
const extractionPreviewVisible = ref(false)
const extractionPreview = ref(null)
const extractedCredentialsApplied = ref(false)
const form = ref(emptyForm())
const editingProxy = ref(null)

// Detail drawer
const detailVisible = ref(false)
const detailLoading = ref(false)
const detailProxy = ref(null)
const detailBindings = ref([])

// 配额
const quotaVisible = ref(false)
const quotaProxy = ref(null)
const quotaValue = ref(1)
const savingQuota = ref(false)
const quotaError = ref("")

// 导入
const importVisible = ref(false)
const importTeam = ref("")
const importText = ref("")
const importRows = ref([])
const importSelected = ref([])
const importError = ref("")
const importPreviewing = ref(false)
const importing = ref(false)
const importStep = ref("input")
const importMetadata = ref([])
const importFileName = ref("")
const importDefaultSupplier = ref("")
const importDefaultRegion = ref("")
const importDefaultQuota = ref(3)
const editingImportLine = ref(null)
const editingImportRaw = ref("")
const editingImportQuota = ref(3)

// 启停
const statusConfirmVisible = ref(false)
const statusTarget = ref(null)
const statusNext = ref(false)

// 批量检测与导入共用的进度弹窗
const progressVisible = ref(false)
const progressTitle = ref("处理进度")
const progressPhase = ref("running")
const progressItems = ref([])
const progressMode = ref("")
const progressLabels = ref({})
const retryPayload = ref(null)
const pendingImportResult = computed(() => progressMode.value === "import"
  && progressPhase.value === "done"
  && progressItems.value.some(item => item.status === "failed" || item.status === "unconfirmed"))
let pageActive = true

function warnBeforeUnload(event) {
  if (!(progressVisible.value && progressPhase.value === "running")) return
  event.preventDefault()
  event.returnValue = ""
}

onBeforeRouteLeave(() => {
  if (!(progressVisible.value && progressPhase.value === "running")) return
  feedback.toast.info("正在处理，请等待当前操作完成后再离开。")
  return false
})

onBeforeUnmount(() => {
  pageActive = false
  window.removeEventListener("beforeunload", warnBeforeUnload)
})

onMounted(async () => {
  window.addEventListener("beforeunload", warnBeforeUnload)
  try {
    currentUser.value = await sessionClient.me()
  } catch {}
  if (isAdmin.value) {
    try {
      const result = await usersClient.listTeams()
      teams.value = Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e.message
    }
  }
  await loadProxies()
})

async function loadProxies() {
  error.value = ""
  loading.value = true
  try {
    const params = {}
    if (isAdmin.value && filterTeam.value.trim()) params.team_search = filterTeam.value.trim()
    if (filterStatus.value) params.enabled = filterStatus.value === "active"
    if (filterRegion.value.trim()) params.region = filterRegion.value.trim()
    if (filterText.value.trim()) params.search = filterText.value.trim()
    const result = await proxyClient.list(params)
    proxies.value = Array.isArray(result) ? result : (Array.isArray(result?.list) ? result.list : [])
    pagination.value.current = 1
    pagination.value.total = visibleProxies.value.length
    selectedRowKeys.value = selectedRowKeys.value.filter((id) => proxies.value.some((proxy) => proxy.id === id))
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  pagination.value.current = 1
  pagination.value.total = visibleProxies.value.length
  selectedRowKeys.value = []
}

function applyClientFilters() {
  applyFilters()
}

async function submitSearch() {
  applyFilters()
  await loadProxies()
}

function resetFilters() {
  filterTeam.value = ""
  filterStatus.value = ""
  filterRegion.value = ""
  filterCheck.value = ""
  filterExpiry.value = ""
  filterText.value = ""
  loadProxies()
}

function emptyForm() {
  return {
    name: "", team_id: "", region: "", supplier: "", source: "static",
    protocol: "http", proxy_address: "", extract_url: "", host: "", port: undefined,
    username: "", password: "", tags: [], expires_at: null, max_profiles: 3, remark: "",
  }
}

function openCreate() {
  formMode.value = "create"
  form.value = emptyForm()
  form.value.team_id = isAdmin.value ? exactFilterTeamID.value : String(currentUser.value?.team_id || "")
  formError.value = ""
  extractionPreviewVisible.value = false
  extractionPreview.value = null
  extractedCredentialsApplied.value = false
  formVisible.value = true
}

function openEdit(proxy) {
  formMode.value = "edit"
  editingProxy.value = proxy
  form.value = {
    name: proxy.name || "",
    team_id: String(proxy.team_id || ""),
    region: proxy.region || "",
    supplier: proxy.supplier || "",
    source: proxy.source || "static",
    protocol: proxy.protocol || "http",
    proxy_address: "",
    extract_url: "",
    host: proxy.host || "",
    port: Number(proxy.port) || 0,
    username: proxy.username || "",
    password: "",
    tags: Array.isArray(proxy.tags) ? [...proxy.tags] : [],
    expires_at: proxy.expires_at ? new Date(proxy.expires_at).getTime() : null,
    max_profiles: Number(proxy.max_profiles) || 3,
    remark: proxy.remark || "",
  }
  formError.value = ""
  extractionPreviewVisible.value = false
  extractionPreview.value = null
  extractedCredentialsApplied.value = false
  formVisible.value = true
}

const connectionChanged = computed(() => {
  if (formMode.value !== "edit" || !editingProxy.value) return false
  const proxy = editingProxy.value
  return form.value.protocol !== proxy.protocol
    || form.value.host.trim() !== (proxy.host || "")
    || Number(form.value.port) !== Number(proxy.port)
    || (form.value.username.trim() !== "" && form.value.username.trim() !== (proxy.username || ""))
    || form.value.password !== ""
    || form.value.source !== proxy.source
})

const editAffectedWindows = computed(() => Number(editingProxy.value?.assigned_profile_count || 0))

function applyParsedAddress(parsed) {
  if (!parsed) return
  form.value.protocol = parsed.protocol || form.value.protocol
  form.value.host = parsed.host || ""
  form.value.port = Number(parsed.port) || 0
  form.value.username = parsed.username || ""
  form.value.password = parsed.password || ""
}

async function parseAddress() {
  if (!form.value.proxy_address.trim()) return
  parsingAddress.value = true
  formError.value = ""
  try {
    applyParsedAddress(await proxyClient.parseAddress(form.value.proxy_address.trim()))
  } catch (e) {
    formError.value = e.message || "代理地址无法解析"
  } finally {
    parsingAddress.value = false
  }
}

async function extractAddress() {
  if (!canExtract.value) {
    formError.value = "动态提取需要本人桌面端在线；网页版和管理员只能维护提取来源或手动录入地址。"
    return
  }
  extracting.value = true
  formError.value = ""
  try {
    const extractUrl = form.value.extract_url.trim()
      || (formMode.value === "edit" && editingProxy.value?.id
        ? (await proxyClient.extractionConfig(editingProxy.value.id)).extract_url
        : "")
    if (!extractUrl) {
      formError.value = "请填写提取地址。"
      return
    }
    const extracted = await localAgent.proxyExtract({
      extractUrl,
      proxyProtocol: form.value.protocol,
    })
    extractionPreview.value = extracted
    extractionPreviewVisible.value = true
  } catch (e) {
    formError.value = e.message || "动态代理提取失败"
  } finally {
    extracting.value = false
  }
}

function useExtractedAddress() {
  if (!extractionPreview.value) return
  applyParsedAddress({ ...extractionPreview.value, protocol: extractionPreview.value.proxy_protocol })
  extractedCredentialsApplied.value = true
  extractionPreviewVisible.value = false
  extractionPreview.value = null
  feedback.toast.info("已回填提取结果；保存后请点击检测。")
}

async function refreshDynamic() {
  if (!editingProxy.value || !canExtract.value || !isGovernor.value) return
  const proxyId = editingProxy.value.id
  const confirmed = await feedback.confirmWarning({
    title: "提取新地址并检测",
    message: `将按已保存的提取地址重新获取并检测，检测结果会替换当前记录。已有 ${editAffectedWindows.value} 个关联窗口不会自动改写，需在窗口页重新同步。`,
    confirmText: "提取并检测",
  })
  if (!confirmed) return
  savingForm.value = true
  formError.value = ""
  let submitted = false
  try {
    const config = await proxyClient.extractionConfig(proxyId)
    const extracted = await localAgent.proxyExtract({
      extractUrl: config.extract_url,
      proxyProtocol: config.proxy_protocol,
    })
    submitted = true
    const updated = await proxyClient.refreshDynamic(proxyId, {
      extract_url: config.extract_url,
      resource_updated_at: config.resource_updated_at,
      ...extracted,
    })
    feedback.toast.info(updated.assignment_blocker
      ? `已提取新地址，但检测未通过：${updated.assignment_blocker}`
      : "已提取新地址并检测通过。")
    formVisible.value = false
    await loadProxies()
  } catch (e) {
    formError.value = submitted && e.errcode === 23004
      ? "代理信息在提取期间已修改，本次结果没有保存。请重新打开代理后再提取。"
      : submitted
      ? `保存或检测结果未确认，请先查看代理详情再决定是否重新提取。${e.message || ""}`
      : (e.message || "动态代理提取失败")
  } finally {
    savingForm.value = false
  }
}

async function saveForm() {
  formError.value = ""
  const target = form.value
  if (!target.name.trim()) {
    formError.value = "请填写代理名称。"
    return
  }
  if (isAdmin.value && formMode.value === "create" && !target.team_id) {
    formError.value = "请选择目标团队。"
    return
  }
  if (target.source === "static" && (!target.host.trim() || !Number(target.port))) {
    formError.value = "静态代理需要地址和端口。"
    return
  }
  if (target.source === "api") {
    const hasStoredExtractURL = formMode.value === "edit" && Boolean(editingProxy.value?.extract_url_configured)
    if (!target.extract_url.trim() && !hasStoredExtractURL && !target.host.trim()) {
      formError.value = "动态代理需要提取地址，或先手动填写地址。"
      return
    }
  }
  if (target.host.trim() && (!Number(target.port) || Number(target.port) < 1 || Number(target.port) > 65535)) {
    formError.value = "端口必须是 1-65535。"
    return
  }
  savingForm.value = true
  try {
    const payload = {
      name: target.name.trim(),
      source: target.source,
      protocol: target.protocol,
      host: target.host.trim(),
      port: Number(target.port) || 0,
      username: target.username.trim(),
      password: target.password,
      replace_credentials: extractedCredentialsApplied.value && target.source === "api",
      region: target.region.trim(),
      supplier: target.supplier.trim(),
      tags: target.tags,
      extract_url: target.extract_url.trim(),
      remark: target.remark.trim(),
    }
    if (target.expires_at) payload.expires_at = new Date(target.expires_at).toISOString()
    if (formMode.value === "create") {
      payload.team_id = isAdmin.value ? Number(target.team_id) : undefined
      payload.max_profiles = Number(target.max_profiles) || 3
      await proxyClient.create(payload)
      feedback.toast.success(`代理「${payload.name}」已创建；检测通过后才能分配窗口。`)
    } else {
      await proxyClient.update(editingProxy.value.id, payload)
      if (connectionChanged.value) {
        feedback.toast.info(editAffectedWindows.value
          ? `代理已更新；${editAffectedWindows.value} 个关联窗口的代理配置待同步，请到浏览器窗口页重新同步。`
          : "代理已更新，旧检测结果已失效，请重新检测后再分配窗口。")
      } else {
        feedback.toast.success("代理资料已更新。")
      }
    }
    formVisible.value = false
    await loadProxies()
  } catch (e) {
    formError.value = e.message || "保存代理失败"
  } finally {
    savingForm.value = false
  }
}

async function openDetail(proxy) {
  detailVisible.value = true
  detailLoading.value = true
  detailProxy.value = proxy
  detailBindings.value = []
  try {
    const [detail, bindings] = await Promise.all([
      proxyClient.get(proxy.id),
      proxyClient.listBindings(proxy.id),
    ])
    detailProxy.value = detail
    detailBindings.value = Array.isArray(bindings) ? bindings : []
  } catch (e) {
    error.value = e.message || "代理详情读取失败"
  } finally {
    detailLoading.value = false
  }
}

async function copyText(text, label) {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    feedback.toast.success(`${label}已复制。`)
  } catch {
    feedback.toast.error(`${label}复制失败，请手动选择复制。`)
  }
}

function requestStatusChange(proxy, next) {
  statusTarget.value = proxy
  statusNext.value = next
  statusConfirmVisible.value = true
}

async function confirmStatusChange() {
  const proxy = statusTarget.value
  if (!proxy) return
  statusConfirmVisible.value = false
  try {
    await proxyClient.update(proxy.id, { enabled: statusNext.value })
    feedback.toast.success(statusNext.value
      ? "代理已启用；仍需有效期内且检测正常才可分配。"
      : "代理已停用；现有窗口配置和已占名额保持不变，仅阻止新分配。")
    await loadProxies()
  } catch (e) {
    error.value = e.message
    await loadProxies()
  }
}

async function deleteProxy(proxy) {
  const assigned = Number(proxy.assigned_profile_count || 0)
  if (assigned > 0) {
    error.value = `该代理仍关联 ${assigned} 个浏览器窗口，请先由窗口本人在浏览器窗口页解绑或更换代理后再删除。`
    return
  }
  if (!await feedback.confirmWarning({
    title: "删除代理",
    message: `确定删除代理「${proxy.name || `${proxy.host}:${proxy.port}`}」？删除后不可恢复。`,
    confirmText: "删除",
    danger: true,
  })) return
  try {
    await proxyClient.delete(proxy.id)
    feedback.toast.success("代理已删除。")
    await loadProxies()
  } catch (e) {
    error.value = e.message
  }
}

function openQuota(proxy) {
  quotaProxy.value = proxy
  quotaValue.value = Number(proxy.max_profiles) || 3
  quotaError.value = ""
  quotaVisible.value = true
}

async function saveQuota() {
  quotaError.value = ""
  if (!Number.isInteger(Number(quotaValue.value)) || Number(quotaValue.value) < 1) {
    quotaError.value = "最大窗口数必须是正整数。"
    return
  }
  const assigned = Number(quotaProxy.value?.assigned_profile_count || 0)
  if (Number(quotaValue.value) < assigned) {
    quotaError.value = `最大窗口数不能低于当前已占用数（${assigned}）。`
    return
  }
  savingQuota.value = true
  try {
    await proxyClient.updateMaxProfiles(quotaProxy.value.id, Number(quotaValue.value))
    feedback.toast.success("窗口额度已更新。")
    quotaVisible.value = false
    await loadProxies()
  } catch (e) {
    quotaError.value = e.message
  } finally {
    savingQuota.value = false
  }
}

async function checkOne(proxy) {
  if (!proxy.host || !proxy.port) {
    error.value = "该资源还没有地址，无法检测；请先编辑补充地址或提取。"
    return
  }
  checkingIds.value = [...checkingIds.value, proxy.id]
  try {
    const result = await proxyClient.check(proxy.id)
    if (result.last_check_result === "ok") {
      feedback.toast.success("系统检测通过，线路正常。")
    } else {
      feedback.toast.error(`系统检测未通过：${checkResultLabel(result.last_check_result)}${result.assignment_blocker ? `（${result.assignment_blocker}）` : ""}`)
    }
    await loadProxies()
  } catch (e) {
    error.value = e.message || "系统检测失败"
  } finally {
    checkingIds.value = checkingIds.value.filter((id) => id !== proxy.id)
  }
}

async function runBatchCheck(targets, { retry = false } = {}) {
  progressMode.value = "check"
  progressTitle.value = "批量检测代理"
  progressLabels.value = { success: "正常", failed: "检测失败" }
  if (retry) {
    const targetIds = new Set(targets.map(proxy => proxy.id))
    progressItems.value = prepareFailedCheckRetry(progressItems.value, targetIds)
  } else {
    progressItems.value = targets.map((proxy) => ({
      key: proxy.id,
      label: `${proxy.name || `${proxy.host}:${proxy.port}`}（${proxy.host}:${proxy.port}）`,
      status: "pending",
    }))
  }
  progressPhase.value = "running"
  progressVisible.value = true
  for (const item of progressItems.value.filter(candidate => candidate.status === "pending")) {
    if (!pageActive) break
    item.status = "processing"
    const proxy = targets.find((candidate) => candidate.id === item.key)
    try {
      const result = await proxyClient.check(proxy.id)
      item.status = result.last_check_result === "ok" ? "success" : "failed"
      item.reason = result.last_check_result === "ok" ? "" : (result.assignment_blocker || checkResultLabel(result.last_check_result))
    } catch (e) {
      item.status = "failed"
      item.reason = e.message || "检测执行失败"
    }
  }
  progressPhase.value = "done"
  retryPayload.value = { kind: "check", ids: progressItems.value.filter((item) => item.status === "failed").map((item) => item.key) }
  if (pageActive) await loadProxies()
}

function startBatchCheck() {
  const targets = pagedProxies.value.filter((proxy) => selectedRowKeys.value.includes(proxy.id) && proxy.host && proxy.port)
  if (!targets.length) return
  runBatchCheck(targets)
}

async function retryFailed() {
  if (progressMode.value === "check") {
    const targets = proxies.value.filter((proxy) => retryPayload.value?.ids?.includes(proxy.id))
    if (!targets.length) {
      progressVisible.value = false
      return
    }
    await runBatchCheck(targets, { retry: true })
    return
  }
  if (progressMode.value === "import") {
    const failedLines = new Set(retryPayload.value?.rows?.map((row) => row.line) || [])
    const rows = importRows.value
      .filter((row) => failedLines.has(row.line))
      .map((row) => importPayloadRow(row, true))
    if (!rows.length) {
      progressVisible.value = false
      return
    }
    await submitImport(rows, { retry: true })
  }
}

async function verifyImportResult() {
  if (progressMode.value !== "import" || progressPhase.value === "running") return
  const uncertain = progressItems.value.filter(item => item.status === "unconfirmed")
  if (!uncertain.length) return
  progressPhase.value = "running"
  try {
    const rows = importRows.value.filter(row => uncertain.some(item => item.key === String(row.line)))
    const preview = await proxyClient.previewImport(isAdmin.value ? Number(importTeam.value) : null, rows.map(row => row.raw))
    const current = Array.isArray(preview?.rows) ? preview.rows : []
    for (const [index, row] of rows.entries()) {
      const item = uncertain.find(candidate => candidate.key === String(row.line))
      const observed = current[index]
      if (!item || !observed) continue
      if (observed.duplicate) {
        item.reason = `${observed.duplicate_reason || "同连接代理已存在"}；请在代理列表核对，暂不重复导入。`
      } else {
        item.status = "failed"
        item.reason = observed.error || "系统中未找到同连接代理，可重试此行。"
      }
    }
    retryPayload.value = { kind: "import", rows: rows.filter(row => progressItems.value.find(item => item.key === String(row.line))?.status === "failed") }
    await loadProxies()
  } catch (error) {
    feedback.toast.error(error.message || "核实导入结果失败，请稍后再试。")
  } finally {
    progressPhase.value = "done"
  }
}

function importPreviewLabel(row) {
  return row?.parsed?.host ? `${row.parsed.protocol || "代理"}://${row.parsed.host}:${row.parsed.port}` : `第 ${row?.sourceLine || row?.line || "?"} 行（请在输入框核对原文）`
}

function closeProgress() {
  progressVisible.value = false
  if (pendingImportResult.value) return
  progressMode.value = ""
  retryPayload.value = null
  if (importStep.value === "done") resetImport()
}

function sourceLabel(source) {
  return source === "api" ? "动态" : "静态"
}

function checkResultLabel(result) {
  return {
    ok: "正常",
    conn_failed: "连接失败",
    auth_failed: "认证失败",
    timeout: "超时",
    invalid_params: "参数无效",
  }[result] || (result || "未检测")
}

function checkTone(result) {
  if (result === "ok") return "success"
  if (!result) return "neutral"
  return "danger"
}

function isExpired(proxy) {
  return Boolean(proxy.expired) || (proxy.expires_at && new Date(proxy.expires_at).getTime() <= Date.now())
}

function formatTime(value) {
  if (!value) return "-"
  return new Date(value).toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

function formatDate(value) {
  if (!value) return "-"
  return new Date(value).toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" })
}

// --- 导入 ---

function openImport() {
  if (pendingImportResult.value) {
    progressVisible.value = true
    return
  }
  importTeam.value = isAdmin.value ? exactFilterTeamID.value : String(currentUser.value?.team_id || "")
  resetImport()
  importVisible.value = true
}

function resetImport() {
  importText.value = ""
  importRows.value = []
  importSelected.value = []
  importError.value = ""
  importStep.value = "input"
  importVisible.value = false
  importMetadata.value = []
  importFileName.value = ""
  importDefaultSupplier.value = ""
  importDefaultRegion.value = ""
  importDefaultQuota.value = 3
  editingImportLine.value = null
  editingImportRaw.value = ""
  editingImportQuota.value = 3
}

async function onFileSelected(event) {
  const file = event.target?.files?.[0]
  if (!file) return
  importError.value = ""
  importPreviewing.value = true
  try {
    const parsed = await readProxyImportFile(file)
    importText.value = parsed.lines.join("\n")
    importMetadata.value = parsed.metadata
    importFileName.value = parsed.name
    importRows.value = []
    importSelected.value = []
    importStep.value = "input"
  } catch (error) {
    importError.value = error.message || "文件读取失败，请检查格式。"
  } finally {
    event.target.value = ""
    importPreviewing.value = false
  }
}

function onImportTextInput() {
  importMetadata.value = []
  importFileName.value = ""
  importRows.value = []
  importSelected.value = []
  importStep.value = "input"
}

function downloadImportTemplate() {
  const blob = new Blob(["\uFEFF", PROXY_IMPORT_TEMPLATE], { type: "text/csv;charset=utf-8" })
  const url = URL.createObjectURL(blob)
  const link = document.createElement("a")
  link.href = url
  link.download = "代理导入模板.csv"
  link.click()
  URL.revokeObjectURL(url)
}

function importLines() {
  return importText.value.split("\n").map((line) => line.trim()).filter(Boolean)
}

function importSourceLines() {
  return importText.value.split("\n").map((line, index) => ({ line: line.trim(), sourceLine: index + 1 }))
    .filter(row => row.line).map(row => row.sourceLine)
}

async function previewImport() {
  importError.value = ""
  const lines = importLines()
  if (!lines.length) {
    importError.value = "请粘贴或选择要导入的代理内容。"
    return
  }
  if (lines.length > MAX_IMPORT_ROWS) {
    importError.value = `每次最多导入 ${MAX_IMPORT_ROWS} 行。`
    return
  }
  if (isAdmin.value && !importTeam.value) {
    importError.value = "请选择目标团队。"
    return
  }
  if (!Number.isInteger(Number(importDefaultQuota.value)) || Number(importDefaultQuota.value) < 1 || Number(importDefaultQuota.value) > 1000) {
    importError.value = "默认窗口数须为 1–1000 的整数。"
    return
  }
  importPreviewing.value = true
  try {
    const sourceLines = importSourceLines()
    const result = await proxyClient.previewImport(isAdmin.value ? Number(importTeam.value) : null, lines)
    importRows.value = (Array.isArray(result?.rows) ? result.rows : []).map((row, index) => {
      const metadata = importMetadata.value[index] || {}
      return {
        ...row,
        metadata: {
          name: metadata.name || "",
          region: metadata.region || importDefaultRegion.value.trim(),
          supplier: metadata.supplier || importDefaultSupplier.value.trim(),
          max_profiles: metadata.max_profiles || Number(importDefaultQuota.value) || 3,
          remark: metadata.remark || "",
        },
        sourceLine: metadata.sourceLine || sourceLines[index] || row.line,
        error: metadata.error || row.error,
      }
    })
    importSelected.value = importRows.value
      .filter((row) => isImportable(row))
      .map((row) => row.line)
    importStep.value = "preview"
  } catch (e) {
    importError.value = e.message || "导入预览失败"
  } finally {
    importPreviewing.value = false
  }
}

function isImportable(row) {
  return Boolean(row.parsed) && !row.error && !row.duplicate
}

function toggleImportRow(line, checked) {
  const row = importRows.value.find((item) => item.line === line)
  if (!row || !isImportable(row)) return
  importSelected.value = checked
    ? [...new Set([...importSelected.value, line])]
    : importSelected.value.filter((value) => value !== line)
}

function beginImportCorrection(row) {
  editingImportLine.value = row.line
  editingImportRaw.value = row.raw
  editingImportQuota.value = row.metadata?.max_profiles || 3
}

async function applyImportCorrection() {
  const line = editingImportLine.value
  if (!line || !editingImportRaw.value.trim()) return
  const quota = Number(editingImportQuota.value)
  if (!Number.isInteger(quota) || quota < 1 || quota > 1000) {
    importError.value = "窗口数须为 1–1000 的整数。"
    return
  }
  const lines = importLines()
  lines[line - 1] = editingImportRaw.value.trim()
  importText.value = lines.join("\n")
  importMetadata.value[line - 1] = {
    ...(importMetadata.value[line - 1] || {}),
    sourceLine: importRows.value.find(row => row.line === line)?.sourceLine || line,
    max_profiles: quota, error: "",
  }
  const previous = new Set(importSelected.value)
  editingImportLine.value = null
  editingImportRaw.value = ""
  editingImportQuota.value = 3
  await previewImport()
  if (importStep.value === "preview") {
    importSelected.value = importRows.value.filter(row => isImportable(row) && (previous.has(row.line) || row.line === line)).map(row => row.line)
  }
}

function importPayloadRow(row, selected) {
  return { line: row.line, raw: row.raw, selected, ...(row.metadata || {}) }
}

const importSelectedCount = computed(() => importSelected.value.length)

async function submitImport(rows, { retry = false } = {}) {
  importStep.value = "submitting"
  importing.value = true
  importError.value = ""
  const payload = (rows || importRows.value.map((row) => importPayloadRow(row, importSelected.value.includes(row.line))))
  progressMode.value = "import"
  progressTitle.value = "批量导入代理"
  progressPhase.value = "running"
  if (retry) {
    for (const row of payload) {
      const item = progressItems.value.find(candidate => candidate.key === String(row.line))
      if (item && item.status === "failed") {
        item.status = "pending"
        item.reason = ""
      }
    }
  } else {
    progressItems.value = payload.map((row) => ({
      key: String(row.line),
      label: `第 ${importRows.value.find(item => item.line === row.line)?.sourceLine || row.line} 行：${importPreviewLabel(importRows.value.find(item => item.line === row.line))}`,
      status: row.selected ? "pending" : "skipped",
    }))
  }
  progressVisible.value = true
  importVisible.value = false
  try {
    const result = await proxyClient.bulkImport(isAdmin.value ? Number(importTeam.value) : null, payload)
    progressItems.value = reconcileImportItems(progressItems.value, result?.rows || [])
    retryPayload.value = { kind: "import", rows: importRows.value.filter(row => progressItems.value.find(item => item.key === String(row.line))?.status === "failed") }
    importStep.value = "done"
    feedback.toast.success(`导入处理完成：成功 ${result?.imported ?? 0} 条，跳过 ${result?.skipped ?? 0} 条，失败 ${result?.failed ?? 0} 条。`)
    await loadProxies()
  } catch (e) {
    progressItems.value = markImportUnknown(progressItems.value)
    importStep.value = "done"
    feedback.toast.error(e.message || "导入结果未返回，请先核实。")
  } finally {
    importing.value = false
    progressPhase.value = "done"
  }
}

function confirmImport() {
  if (!importSelectedCount.value) return
  const rows = importRows.value.map((row) => importPayloadRow(row, importSelected.value.includes(row.line)))
  submitImport(rows)
}

function closeImport() {
  if (importing.value) return
  resetImport()
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <div class="wt-resource-page resource-visual-approved">
      <ResourcePageHeader title="代理管理" description="团队代理资源、连接检测与窗口额度">
        <template #actions>
          <t-button theme="primary" @click="openCreate">新增代理</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="openImport">批量导入</t-button>
          <t-button v-if="pendingImportResult" variant="text" @click="progressVisible = true">查看上次导入结果</t-button>
        </template>
      </ResourcePageHeader>

      <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />

      <ResourceCard class="proxy-resource-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">搜索</span><t-input v-model="filterText" clearable placeholder="名称 / ID / 地址 / 备注" @enter="submitSearch" /></label>
          <label v-if="isAdmin" class="wt-filter-field">
            <span class="wt-filter-field__label">团队</span>
            <t-input v-model="filterTeam" clearable placeholder="输入团队名称或 ID" @enter="submitSearch" />
          </label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">资源启停</span>
            <t-select v-model="filterStatus" clearable placeholder="全部" @change="submitSearch">
              <t-option value="active" label="启用" />
              <t-option value="paused" label="停用" />
            </t-select>
          </label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">检测结果</span>
            <t-select v-model="filterCheck" clearable placeholder="全部" @change="applyClientFilters">
              <t-option value="ok" label="正常" />
              <t-option value="failed" label="检测失败" />
              <t-option value="unchecked" label="未检测" />
            </t-select>
          </label>
          <label class="wt-filter-field"><span class="wt-filter-field__label">到期</span>
            <t-select v-model="filterExpiry" clearable placeholder="全部" @change="applyClientFilters">
              <t-option value="valid" label="有效" />
              <t-option value="expired" label="已到期" />
            </t-select>
          </label>
          <label v-show="advancedFilters" class="wt-filter-field"><span class="wt-filter-field__label">地区</span><t-input v-model="filterRegion" clearable placeholder="地区" @enter="submitSearch" /></label>
          <div class="wt-filter-actions">
            <t-button theme="default" variant="text" :aria-expanded="advancedFilters" @click="advancedFilters = !advancedFilters">{{ advancedFilters ? "收起筛选" : "更多筛选" }}{{ !advancedFilters && filterRegion ? " · 已设置" : "" }}</t-button>
            <t-button theme="primary" @click="submitSearch">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
          </div>
        </div>

        <ResourceSelectionToolbar :count="selectedRowKeys.length" unit="条代理" @clear="selectedRowKeys = []">
          <t-button theme="default" variant="outline" @click="startBatchCheck">批量检测（{{ selectedRowKeys.length }}）</t-button>
        </ResourceSelectionToolbar>

        <div class="table-scroll-wrap">
          <t-table
            class="wt-resource-table"
            :data="pagedProxies"
            :columns="columns"
            row-key="id"
            size="small"
            hover
            v-model:selected-row-keys="selectedRowKeys"
            :scroll="tableScroll"
            empty="暂无符合条件的数据"
          >
            <template #empty>
              <ResourceEmptyState
                :title="error ? '代理列表加载失败' : hasProxyFilter ? '没有找到符合条件的代理' : '还没有代理资源'"
                :description="error ? '保留了当前筛选条件，可以重新加载。' : hasProxyFilter ? '调整筛选条件后再查询。' : '添加代理或批量导入，检测通过后即可在窗口中分配。'"
              >
                <t-button v-if="error" theme="primary" @click="loadProxies">重新加载</t-button>
                <t-button v-else-if="hasProxyFilter" variant="outline" @click="resetFilters">清除筛选</t-button>
                <template v-else>
                  <t-button theme="primary" @click="openCreate">新增代理</t-button>
                  <t-button variant="outline" @click="openImport">批量导入</t-button>
                </template>
              </ResourceEmptyState>
            </template>
            <template #id="{ row }"><span class="proxy-id">{{ row.id }}</span></template>
            <template #name="{ row }">
              <div class="proxy-name" :title="row.name || ''">{{ row.name || "-" }}</div>
              <div class="proxy-addr" :title="row.host ? `${row.host}:${row.port}` : '尚未提取地址'">{{ row.host ? `${row.host}:${row.port}` : "尚未提取地址" }}</div>
            </template>
            <template #region="{ row }">{{ row.region || "-" }}</template>
            <template #source="{ row }"><span>{{ sourceLabel(row.source) }}</span></template>
            <template #usage="{ row }">
              <span>{{ row.assigned_profile_count || 0 }} / {{ row.max_profiles || 3 }}</span>
              <div v-if="Number(row.reserved_profile_count || 0) > 0" class="proxy-sub">待确认占用 {{ row.reserved_profile_count }}</div>
              <div class="proxy-sub">剩余 {{ row.remaining_profile_count ?? ((row.max_profiles || 3) - (row.assigned_profile_count || 0)) }}</div>
            </template>
            <template #enabled="{ row }">
              <t-switch
                v-if="isGovernor"
                :value="row.enabled === true"
                size="small"
                @change="(value) => requestStatusChange(row, value)"
              />
              <ResourceStatusBadge v-else :tone="row.enabled === true ? 'success' : 'neutral'" :label="row.enabled === true ? '启用' : '停用'" />
            </template>
            <template #last_check_result="{ row }">
              <ResourceStatusBadge :tone="checkTone(row.last_check_result)" :label="checkResultLabel(row.last_check_result)" />
            </template>
            <template #expires_at="{ row }">
              <span :class="{ 'proxy-expired': isExpired(row) }">{{ formatDate(row.expires_at) }}</span>
              <div v-if="isExpired(row)" class="proxy-sub proxy-expired">已到期</div>
            </template>
            <template #remark="{ row }"><span class="proxy-remark" :title="row.remark || ''">{{ row.remark || "-" }}</span></template>
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button class="is-primary" size="small" theme="primary" variant="text" :loading="checkingIds.includes(row.id)" :disabled="!row.host || !row.port" @click="checkOne(row)">检测</t-button>
                <t-button size="small" theme="primary" variant="text" @click="openDetail(row)">详情</t-button>
                <t-button size="small" theme="primary" variant="text" @click="openEdit(row)">编辑</t-button>
                <t-button v-if="isGovernor" size="small" theme="primary" variant="text" @click="openQuota(row)">设置配额</t-button>
                <t-button v-if="isGovernor" size="small" theme="default" class="is-danger" variant="text" @click="deleteProxy(row)">删除</t-button>
              </ResourceRowActions>
            </template>
          </t-table>
        </div>
        <div v-if="visibleProxies.length" class="pagination-bar">
          <t-pagination
            v-model:current="pagination.current"
            v-model:pageSize="pagination.pageSize"
            :total="visibleProxies.length"
            :page-size-options="[10, 20, 50, 100]"
            :show-jumper="false"
          />
        </div>
      </ResourceCard>

      <ResourceOverlay
        v-model:visible="formVisible"
        :title="formMode === 'create' ? '新增代理' : '编辑代理'"
        :footer="true"
        :kind="formMode === 'edit' && !isGovernor ? 'dialog' : 'drawer'" :width="formMode === 'edit' && !isGovernor ? 640 : 800" :busy="savingForm">
        <ResourceNotice v-if="formMode === 'edit' && connectionChanged && editAffectedWindows > 0" tone="warning"
          details="保存不会解除现有关联；请重新检测后同步窗口配置。">原检测结果将失效；{{ editAffectedWindows }} 个关联窗口需同步，本次保存不修改窗口配置。</ResourceNotice>
        <FeedbackAlert v-if="formError" :message="formError" theme="error" style="margin-bottom:12px" />

        <ResourceSection title="基础信息" variant="form">
          <ResourceForm class="resource-form-grid">
            <t-form-item label="代理名称" required-mark class="resource-form-full"><t-input v-model="form.name" placeholder="便于识别的资源名称" /></t-form-item>
            <t-form-item v-if="isAdmin && formMode === 'create'" label="运营团队" required-mark>
              <t-select v-model="form.team_id" :options="teamOptions" placeholder="请选择目标团队" filterable />
            </t-form-item>
            <t-form-item v-else label="运营团队"><span class="resource-form-value">{{ formMode === 'create' ? ownTeamLabel : (editingProxy?.team_name || ownTeamLabel) }}</span></t-form-item>
            <t-form-item v-if="formMode === 'create' || isGovernor" label="资源来源" class="resource-form-full">
              <ResourceChoiceCards v-model="form.source" :options="[
                { value: 'static', label: '静态地址', description: '使用固定的连接地址' },
                { value: 'api', label: '动态代理', description: '从供应商链接提取地址' },
              ]" />
            </t-form-item>
          </ResourceForm>
        </ResourceSection>
        <ResourceSection v-if="formMode === 'create' || isGovernor" title="连接配置" variant="form">
          <ResourceForm class="resource-form-grid">
            <t-form-item v-if="form.source === 'static'" label="粘贴代理地址" class="resource-form-full">
              <t-input v-model="form.proxy_address" placeholder="host:port 或 socks5://账号:密码@host:port" @blur="parseAddress" />
            </t-form-item>
            <template v-else>
              <t-form-item label="提取链接" class="resource-form-full"><t-input v-model="form.extract_url" placeholder="供应商提供的代理提取链接" /></t-form-item>
              <t-form-item label="" class="resource-form-full">
                <t-space><t-button theme="default" variant="outline" :loading="extracting" :disabled="!canExtract" @click="extractAddress">测试提取</t-button>
                  <t-button v-if="formMode === 'edit' && isGovernor && canExtract" theme="default" variant="outline" :loading="savingForm" @click="refreshDynamic">提取新地址并检测</t-button></t-space>
                <span v-if="!canExtract" class="form-hint">请在本人桌面端提取，或手动填写已有地址。</span>
              </t-form-item>
            </template>
            <t-form-item label="协议"><t-select v-model="form.protocol"><t-option value="http" label="HTTP" /><t-option value="https" label="HTTPS" /><t-option value="socks5" label="SOCKS5" /></t-select></t-form-item>
            <t-form-item label="端口" required-mark><t-input-number v-model="form.port" :min="1" :max="65535" /></t-form-item>
            <t-form-item label="主机地址" required-mark class="resource-form-full"><t-input v-model="form.host" placeholder="主机名或 IP 地址" /></t-form-item>
            <t-form-item label="认证用户名"><t-input v-model="form.username" :placeholder="formMode === 'edit' ? '留空保留原认证信息' : '无认证可留空'" /></t-form-item>
            <t-form-item label="认证密码"><t-input v-model="form.password" type="password" :placeholder="formMode === 'edit' ? '留空保留原密码' : '无认证可留空'" /></t-form-item>
          </ResourceForm>
        </ResourceSection>
        <ResourceSection title="资源资料" variant="form">
          <ResourceForm class="resource-form-grid">
            <t-form-item label="地区"><t-input v-model="form.region" placeholder="例如：上海" /></t-form-item>
            <t-form-item label="供应商"><t-input v-model="form.supplier" placeholder="供应商名称" /></t-form-item>
            <t-form-item v-if="isGovernor && formMode === 'create'" label="初始窗口额度"><t-input-number v-model="form.max_profiles" :min="1" :max="1000" /></t-form-item>
            <t-form-item v-if="isGovernor" label="到期时间"><t-date-picker v-model="form.expires_at" value-type="time-stamp" clearable :placeholder="formMode === 'create' ? '不填表示长期有效' : '留空保留原日期'" /></t-form-item>
            <t-form-item label="业务标签" class="resource-form-full"><t-tag-input v-model="form.tags" clearable placeholder="回车添加标签" /></t-form-item>
            <t-form-item label="备注" class="resource-form-full"><t-textarea v-model="form.remark" :autosize="{ minRows: 3, maxRows: 6 }" placeholder="补充线路用途或使用注意事项" /></t-form-item>
          </ResourceForm>
        </ResourceSection>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" :disabled="savingForm" @click="formVisible = false">取消</t-button>
            <t-button theme="primary" :loading="savingForm" @click="saveForm">保存</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="extractionPreviewVisible" title="动态代理提取预览" kind="dialog" :width="560">
        <template v-if="extractionPreview">
          <ResourceDescriptions size="small">
            <t-descriptions-item label="提取到的地址">{{ extractionPreview.proxy_protocol?.toUpperCase() }}://{{ extractionPreview.host }}:{{ extractionPreview.port }}</t-descriptions-item>
            <t-descriptions-item label="认证信息">{{ extractionPreview.username || extractionPreview.password ? "已返回，敏感内容不显示" : "未返回" }}</t-descriptions-item>
          </ResourceDescriptions>
          <ResourceNotice tone="info" style="margin-top:16px">本次只提取地址。使用结果后还需保存并检测；现有关联窗口不会自动改写。</ResourceNotice>
        </template>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" @click="extractionPreviewVisible = false">取消</t-button>
            <t-button theme="primary" @click="useExtractedAddress">使用此结果</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="detailVisible" :title="'代理详情'" :footer="false" kind="drawer" :width="800">
        <t-loading :loading="detailLoading">
          <template v-if="detailProxy">
            <ResourceSection title="资源信息"><ResourceDescriptions size="small">
              <t-descriptions-item label="代理ID">{{ detailProxy.id }}</t-descriptions-item>
              <t-descriptions-item label="名称">{{ detailProxy.name || "-" }}</t-descriptions-item>
              <t-descriptions-item label="所属团队">{{ detailProxy.team_name || "-" }}</t-descriptions-item>
              <t-descriptions-item label="来源">{{ sourceLabel(detailProxy.source) }}<span v-if="detailProxy.extract_url_configured" class="proxy-sub">（已配置提取地址）</span></t-descriptions-item>
              <t-descriptions-item label="协议 / 地址">{{ detailProxy.protocol }}://{{ detailProxy.host || "-" }}:{{ detailProxy.port || "-" }}</t-descriptions-item>
              </ResourceDescriptions></ResourceSection>
            <ResourceSection title="连接信息"><ResourceDescriptions size="small">
              <t-descriptions-item label="认证用户名">
                <span>{{ detailProxy.username || "-" }}</span>
                <t-button v-if="detailProxy.username" size="small" variant="text" @click="copyText(detailProxy.username, '认证用户名')">复制</t-button>
              </t-descriptions-item>
              <t-descriptions-item label="认证密码">
                <span>{{ detailProxy.password || "-" }}</span>
                <t-button v-if="detailProxy.password" size="small" variant="text" @click="copyText(detailProxy.password, '认证密码')">复制</t-button>
              </t-descriptions-item>
              </ResourceDescriptions></ResourceSection>
            <ResourceSection title="使用与检测"><ResourceDescriptions size="small">
              <t-descriptions-item label="地区">{{ detailProxy.region || "-" }}</t-descriptions-item>
              <t-descriptions-item label="供应商">{{ detailProxy.supplier || "-" }}</t-descriptions-item>
              <t-descriptions-item label="业务标签">{{ (detailProxy.tags || []).join("、") || "-" }}</t-descriptions-item>
              <t-descriptions-item label="资源启停">{{ detailProxy.enabled ? "启用" : "停用" }}</t-descriptions-item>
              <t-descriptions-item label="到期时间">
                <span :class="{ 'proxy-expired': isExpired(detailProxy) }">{{ formatDate(detailProxy.expires_at) }}</span>
                <span v-if="isExpired(detailProxy)" class="proxy-expired">（已到期）</span>
              </t-descriptions-item>
              <t-descriptions-item label="检测结果">{{ checkResultLabel(detailProxy.last_check_result) }}（{{ formatTime(detailProxy.last_check_at) }}）</t-descriptions-item>
              <t-descriptions-item label="检测原因">{{ detailProxy.last_check_message || "-" }}</t-descriptions-item>
              <t-descriptions-item label="窗口额度">
                {{ detailProxy.assigned_profile_count || 0 }} / {{ detailProxy.max_profiles || 3 }} 已用
                <span v-if="Number(detailProxy.reserved_profile_count || 0) > 0"> · {{ detailProxy.reserved_profile_count }} 个待确认占用</span>
                · 剩余 {{ detailProxy.remaining_profile_count ?? 0 }}
              </t-descriptions-item>
              <t-descriptions-item label="备注"><span class="proxy-detail-remark">{{ detailProxy.remark || "-" }}</span></t-descriptions-item>
            </ResourceDescriptions></ResourceSection>
            <ResourceSection :title="`关联窗口 · ${detailBindings.length} 个`">
            <t-table
              :data="detailBindings"
              row-key="id"
              size="small"
              :columns="[
                { colKey: 'id', title: 'ID', width: 80 },
                { colKey: 'bit_name', title: '窗口名称', width: 180 },
                { colKey: 'bit_profile_id', title: '比特窗口ID', width: 150 },
                { colKey: 'user_id', title: '运营UID', width: 90 },
                { colKey: 'local_state', title: '状态', width: 90 },
              ]"
              empty="暂无关联窗口"
            >
              <template #local_state="{ row }">{{ row.local_state === 'active' ? '比特中可见' : row.local_state === 'local_missing' ? '最近同步未找到' : '尚未确认' }}</template>
            </t-table>
            </ResourceSection>
          </template>
        </t-loading>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="quotaVisible" title="设置窗口额度" :width="560" :busy="savingQuota">
        <FeedbackAlert v-if="quotaError" :message="quotaError" theme="error" style="margin-bottom:12px" />
        <ResourceNotice>{{ quotaProxy?.name || "-" }} · 已使用 {{ quotaProxy?.assigned_profile_count || 0 }} 个窗口名额</ResourceNotice>
        <ResourceForm><t-form-item label="最大窗口数"><t-input-number v-model="quotaValue" :min="1" :max="1000" /></t-form-item></ResourceForm>
        <ResourceNotice>仅调整窗口名额，已有绑定保持不变。</ResourceNotice>
        <template #footer><t-button theme="default" variant="outline" :disabled="savingQuota" @click="quotaVisible = false">取消</t-button><t-button theme="primary" :loading="savingQuota" @click="saveQuota">保存</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="statusConfirmVisible" :title="statusNext ? '启用代理' : '停用代理'" :width="560">
        <p>资源：{{ statusTarget?.name || `${statusTarget?.host}:${statusTarget?.port}` }}（团队：{{ statusTarget?.team_name || ownTeamLabel }}）</p>
        <p>受影响窗口：{{ statusTarget?.assigned_profile_count || 0 }} 个</p>
        <p class="form-hint">
          {{ statusNext
            ? "启用后仍需在有效期内且检测正常才可分配窗口。"
            : "停用只阻止新分配，不会清除现有窗口配置，也不会释放已占额度。" }}
        </p>
              <template #footer><t-button theme="default" variant="outline" @click="statusConfirmVisible = false">取消</t-button><t-button theme="primary" @click="confirmStatusChange">{{ statusNext ? '启用' : '停用' }}</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay
        v-model:visible="importVisible"
        title="批量导入代理"
        :footer="importStep === 'input' || importStep === 'preview'"
        @close="closeImport"
        kind="dialog" :width="importStep === 'input' ? 720 : 960" :busy="importing">
        <template v-if="importStep === 'input' || importStep === 'preview'">
          <FeedbackAlert v-if="importError" :message="importError" theme="error" style="margin-bottom:12px" />
          <div class="import-team">
            <span>目标团队：</span>
            <t-select v-if="isAdmin" v-model="importTeam" :options="teamOptions" placeholder="请选择目标团队" style="width:220px" />
            <strong v-else>{{ ownTeamLabel }}</strong>
          </div>
          <ResourceNotice details="支持 host:port、host:port:user:pass、protocol://user:pass@host:port。Excel/CSV 请使用下载模板。">每行一个代理，仅新增，不覆盖已有资源。</ResourceNotice>
          <div class="import-file">
            <t-button size="small" variant="outline" @click="$refs.importFile.click()">选择文件</t-button>
            <t-button size="small" variant="text" @click="downloadImportTemplate">下载模板</t-button>
            <span class="form-hint">{{ importFileName || '支持 .xlsx / .csv / .txt' }}</span>
            <input ref="importFile" type="file" accept=".xlsx,.csv,.txt" style="display:none" @change="onFileSelected" />
          </div>
          <t-textarea v-model="importText" :rows="5" placeholder="粘贴代理文本，每行一个…" @input="onImportTextInput" />
          <div class="import-defaults">
            <label class="wt-filter-field"><span class="wt-filter-field__label">默认供应商</span><t-input v-model="importDefaultSupplier" placeholder="可留空" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">默认地区</span><t-input v-model="importDefaultRegion" placeholder="可留空" /></label>
            <label class="wt-filter-field"><span class="wt-filter-field__label">默认窗口数</span><t-input-number v-model="importDefaultQuota" :min="1" :max="1000" /></label>
          </div>
          <div class="import-actions">
            <t-button theme="primary" :loading="importPreviewing" @click="previewImport">解析预览</t-button>
          </div>

          <template v-if="importRows.length && importStep === 'preview'">
            <t-divider>解析结果（已选 {{ importSelectedCount }} 条）</t-divider>
            <t-table :data="importRows" row-key="line" size="small" :columns="[
              { colKey: 'selected', title: '选择', width: 70 },
              { colKey: 'sourceLine', title: '行号', width: 70 },
              { colKey: 'raw', title: '连接地址', width: 220 },
              { colKey: 'metadata', title: '资源资料', width: 220 },
              { colKey: 'status', title: '结果', width: 130 },
              { colKey: 'action', title: '处理', width: 90 },
            ]">
              <template #selected="{ row }">
                <t-checkbox
                  :checked="importSelected.includes(row.line)"
                  :disabled="!isImportable(row)"
                  @change="(checked) => toggleImportRow(row.line, checked)"
                />
              </template>
              <template #raw="{ row }">
                <t-input v-if="editingImportLine === row.line" v-model="editingImportRaw" size="small" @enter="applyImportCorrection" />
                <span v-else>{{ importPreviewLabel(row) }}</span>
              </template>
              <template #metadata="{ row }">
                <t-input-number v-if="editingImportLine === row.line" v-model="editingImportQuota" :min="1" :max="1000" size="small" />
                <span v-else>{{ row.metadata?.name || '按地址命名' }} · {{ row.metadata?.region || '地区未填' }} · {{ row.metadata?.supplier || '供应商未填' }} · {{ row.metadata?.max_profiles || 3 }} 窗口</span>
              </template>
              <template #status="{ row }">
                <ResourceStatusBadge v-if="isImportable(row)" tone="success" label="可导入" />
                <ResourceStatusBadge v-else-if="row.duplicate" tone="warning" :label="row.duplicate_reason || '重复'" />
                <ResourceStatusBadge v-else tone="danger" :label="row.error || '无效'" />
              </template>
              <template #action="{ row }">
                <t-button v-if="editingImportLine === row.line" size="small" variant="text" @click="applyImportCorrection">保存</t-button>
                <t-button v-else-if="row.error" size="small" variant="text" @click="beginImportCorrection(row)">修正</t-button>
                <span v-else>—</span>
              </template>
            </t-table>
          </template>
        </template>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" :disabled="importing" @click="closeImport">取消</t-button>
            <t-button
              v-if="importStep === 'preview'"
              theme="primary"
              :loading="importing"
              :disabled="!importSelectedCount"
              @click="confirmImport"
            >确认导入已选 {{ importSelectedCount }} 条</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <OperationProgressDialog
        :visible="progressVisible"
        :title="progressTitle"
        :phase="progressPhase"
        :items="progressItems"
        :running-label="progressMode === 'import' ? '正在导入' : '正在检测'"
        :status-labels="progressLabels"
        retry-label="重试失败项"
        @retry="retryFailed"
        @verify="verifyImportResult"
        @done="closeProgress"
      />
    </div>
  </t-loading>
</template>

<style scoped>
.proxy-resource-card { padding: 18px 20px; }
.proxy-fixed-team { display: inline-flex; align-items: center; height: 32px; color: var(--td-text-color-primary); }
.proxy-id { color: var(--td-text-color-secondary); font-variant-numeric: tabular-nums; }
.proxy-name { color: var(--wt-text-primary); font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.proxy-detail-remark { white-space: pre-wrap; overflow-wrap: anywhere; }
.proxy-addr { color: var(--td-text-color-secondary); font-size: 12px; margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.proxy-sub { color: var(--td-text-color-placeholder); font-size: 12px; margin-top: 2px; }
.proxy-remark { display: block; max-width: 170px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.proxy-expired { color: var(--td-error-color); }
.form-hint { color: var(--td-text-color-placeholder); font-size: 12px; line-height: 1.5; margin-top: 4px; }
.import-team { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.import-team .form-hint { margin-top: 0; }
.import-file { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.import-defaults { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 12px; }
.import-defaults :deep(.t-input-number) { width: 100%; }
.import-actions { margin-top: 8px; }
@media (max-width: 640px) { .import-defaults { grid-template-columns: 1fr; } }
</style>
