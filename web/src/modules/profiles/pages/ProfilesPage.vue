<script setup>
import { computed, onMounted, ref, watch } from "vue"
import { useRouter } from "vue-router"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import { createUsersClient } from "../../../apps/cloud/pages/users/usersApi.js"
import { createBitAccountClient } from "../../../shared/api/bitAccount.js"
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import { createProxyClient } from "../../../shared/api/proxy.js"
import { createSessionClient } from "../../../shared/api/session.js"
import { executionErrorMessage } from "../../../shared/utils/executionErrors.js"
import { loginStatusText } from "../../../shared/utils/mediaAccountLabels.js"
import { platformLabel } from "../../../shared/utils/platforms.js"
import { isDesktop } from "../../../utils.js"
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue"
import { feedback } from "../../../shared/ui/feedback/index.js"
import OperationProgressDialog from "../../../shared/ui/feedback/OperationProgressDialog.vue"
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue"
import ResourceForm from "../../../shared/ui/resource/ResourceForm.vue"
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue"
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue"
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue"
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue"
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue"
import ResourceChoiceCards from "../../../shared/ui/resource/ResourceChoiceCards.vue"
import ResourceSelectionToolbar from "../../../shared/ui/resource/ResourceSelectionToolbar.vue"
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue"
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue"
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue"
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue"
import { opColumnWidth } from "../../../shared/ui/resource/opColumnWidth.js"
import { classifyObservedAssignment, isUncertainAssignmentError } from "./proxyAssignmentOutcome.js"
// 静态导入：打包后的 Tauri WebView 无法解析动态 import() 的裸模块名。invoke 只在
// __TAURI_INTERNALS__ 保护的代码里调用，Cloud Web 导入它是惰性的。
import { invoke } from "@tauri-apps/api/core"

const bindingClient = createProfileBindingClient()
const proxyClient = createProxyClient()
const sessionClient = createSessionClient()
const usersClient = createUsersClient()
const accountClient = createMediaAccountClient()
const bitAccountClient = createBitAccountClient()
const localAgent = createLocalAgentService({ invoke })
const router = useRouter()

const currentUser = ref(null)
const profiles = ref([])
const proxies = ref([])
const mediaAccounts = ref([])
const roster = ref([])
const boundIdentity = ref(null)
const loading = ref(true)
const error = ref("")
const bindingMissing = ref(false)
const selectedRowKeys = ref([])
const operatingIds = ref([])
const page = ref(1)
const pageSize = ref(20)

const memberFilter = ref("")
const draftTeam = ref("")
const draftMember = ref("")
const appliedTeam = ref("")
const appliedMember = ref("")
const statusFilter = ref("")
const filterDraft = ref(emptyFilters())
const appliedFilters = ref(emptyFilters())

const runningStates = ref({})

const detailVisible = ref(false)
const detailProfile = ref(null)

const noteVisible = ref(false)
const noteProfile = ref(null)
const noteText = ref("")
const noteSaving = ref(false)
const noteError = ref("")

const createVisible = ref(false)
const createForm = ref(emptyCreateForm())
const createGroups = ref([])
const createGroupLoading = ref(false)
const createError = ref("")
const createCloudRetry = ref(false)
const createUncertain = ref(false)
const createdDialogVisible = ref(false)
const createdProfile = ref(null)
const createdMode = ref("self")

const editVisible = ref(false)
const editProfile = ref(null)
const editForm = ref(emptyCreateForm())
const editGroups = ref([])
const editGroupLoading = ref(false)
const editError = ref("")
const editPending = ref(null)
const editResultVisible = ref(false)
const editResultKind = ref("")
const editResultRows = ref([])
const editResultMessage = ref("")

const manageVisible = ref(false)
const manageProfile = ref(null)
const manageNote = ref("")

const missingVisible = ref(false)
const missingProfile = ref(null)

const mismatchVisible = ref(false)
const mismatchProfile = ref(null)

const releaseConfirmVisible = ref(false)
const releaseProfile = ref(null)

const unbindConfirmVisible = ref(false)
const unbindProfile = ref(null)

const resyncConfirmVisible = ref(false)
const resyncProfile = ref(null)
const resyncProxy = ref(null)

const candidateVisible = ref(false)
const candidateProfile = ref(null)
const candidateSearch = ref("")
const candidateRegion = ref("")
const candidateSelectedId = ref("")
const candidateError = ref("")

const syncVisible = ref(false)
const syncScan = ref(null)
const syncSnapshot = ref(null)
const syncPreview = ref(null)
const syncSelectedAdded = ref([])
const syncSelectedUnknown = ref([])
const syncError = ref("")
const syncDone = ref(false)
const syncResult = ref(null)

const identityVisible = ref(false)
const identityDetectedMask = ref("")
const syncIdentityMismatch = ref(false)

const batchProxyVisible = ref(false)
const batchProxyRows = ref([])

const progressVisible = ref(false)
const progressTitle = ref("处理进度")
const progressPhase = ref("running")
const progressItems = ref([])
const progressMode = ref("")
const progressLabels = ref({})
const progressRunningLabel = ref("正在处理")
const progressRetryLabel = ref("重试失败项")
const progressVerifyLabel = ref("核实待确认项")
const retryPayload = ref(null)

const isAdmin = computed(() => currentUser.value?.role === "admin")
const isSenior = computed(() => currentUser.value?.role === "senior_operator")
const isOperator = computed(() => currentUser.value?.role === "operator")
const isDesktopClient = computed(() => isDesktop())
const selfId = computed(() => String(currentUser.value?.id ?? ""))
const ownTeamName = computed(() => currentUser.value?.team_name || "本人团队")
const viewingMember = computed(() => isSenior.value && Boolean(memberFilter.value) && memberFilter.value !== selfId.value)
const isOwnDesktop = computed(() => isDesktopClient.value && (isOperator.value || (isSenior.value && !viewingMember.value)))
const canMaintainNote = computed(() => isOperator.value || isSenior.value || isAdmin.value)

const memberOptions = computed(() => {
  const options = roster.value
    .filter((user) => String(user.id) !== selfId.value)
    .map((user) => ({ label: user.nickname || user.username || `用户 ${user.id}`, value: String(user.id) }))
  if (currentUser.value) {
    options.unshift({ label: `${currentUser.value.nickname || currentUser.value.username || "本人"}（自己）`, value: selfId.value })
  }
  return options
})

const teamOptions = computed(() => {
  const seen = new Map()
  for (const user of roster.value) {
    if (user.team_id === null || user.team_id === undefined) continue
    const key = String(user.team_id)
    if (!seen.has(key)) seen.set(key, user.team_name || `团队 ${key}`)
  }
  return [...seen].map(([value, label]) => ({ label, value }))
})

const adminMemberOptions = computed(() => roster.value
  .filter((user) => user.role !== "admin")
  .filter((user) => !draftTeam.value || String(user.team_id ?? "") === draftTeam.value)
  .map((user) => ({
    label: `${user.nickname || user.username || `用户 ${user.id}`}${user.status === "disabled" ? "（已停用）" : ""}`,
    value: String(user.id),
  })))

const proxyById = computed(() => {
  const map = {}
  for (const proxy of proxies.value) map[proxy.id] = proxy
  return map
})

const accountsByProfile = computed(() => {
  const map = new Map()
  for (const account of mediaAccounts.value) {
    const key = String(account.browser_profile_id || "")
    if (!key) continue
    if (!map.has(key)) map.set(key, [])
    map.get(key).push(account)
  }
  return map
})

const scopedProfiles = computed(() => {
  if (!isAdmin.value) return profiles.value
  return profiles.value.filter((profile) => {
    if (appliedTeam.value && String(profile.team_id ?? "") !== appliedTeam.value) return false
    if (appliedMember.value && String(profile.user_id ?? "") !== appliedMember.value) return false
    return true
  })
})

const baseProfiles = computed(() => {
  const filters = appliedFilters.value
  const search = filters.search.trim().toLowerCase()
  return scopedProfiles.value.filter((profile) => {
    if (filters.mode === "managed" && !profile.proxy_id) return false
    if (filters.mode === "self" && profile.proxy_id) return false
    if (filters.business && profile.business_status !== filters.business) return false
    if (filters.running) {
      const state = runningState(profile)
      if (filters.running === "open" && state !== true) return false
      if (filters.running === "closed" && state !== false) return false
      if (filters.running === "unknown" && state !== undefined) return false
    }
    if (search) {
      const haystack = [profile.name, profile.id, profile.bit_profile_id, profile.remark, profile.cloud_remark]
        .filter(Boolean)
        .join("\n")
        .toLowerCase()
      if (!haystack.includes(search)) return false
    }
    return true
  })
})

const filteredProfiles = computed(() => {
  if (statusFilter.value === "normal") return baseProfiles.value.filter((profile) => !hasPendingIssue(profile))
  if (statusFilter.value === "pending") return baseProfiles.value.filter((profile) => hasPendingIssue(profile))
  return baseProfiles.value
})

const pagedProfiles = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredProfiles.value.slice(start, start + pageSize.value)
})

const pendingCount = computed(() => baseProfiles.value.filter((profile) => hasPendingIssue(profile)).length)
const allPendingProfiles = computed(() => profiles.value.filter((profile) => hasPendingIssue(profile)))
const statItems = computed(() => [
  { key: "normal", label: "正常", value: baseProfiles.value.length - pendingCount.value, tone: "success" },
  { key: "pending", label: "待处理", value: pendingCount.value, tone: "warning" },
])

const selectedProfiles = computed(() => profiles.value.filter((profile) => selectedRowKeys.value.includes(profile.id)))
const batchOpenReady = computed(() => selectedProfiles.value.length > 0 && selectedProfiles.value.every((profile) => rowOperable(profile) && runningState(profile) === false))
const batchCloseReady = computed(() => selectedProfiles.value.length > 0 && selectedProfiles.value.every((profile) => rowOperable(profile) && runningState(profile) === true))
const batchHint = computed(() => {
  if (!selectedProfiles.value.length || batchOpenReady.value || batchCloseReady.value) return ""
  return "所选窗口的运行状态不一致或未知，批量打开/关闭需全选同一运行状态；可先按运行状态筛选后重选。"
})

const rowOwnerTitle = computed(() => (!isOwnDesktop.value && (isAdmin.value || isSenior.value)) ? "授权运营" : "比特分组")
const rowActionLabels = computed(() => isOwnDesktop.value
  ? ["详情", "打开", "编辑", "处理异常", "运营备注"]
  : ["详情", "运营备注"])

const columns = computed(() => {
  const list = []
  if (isOwnDesktop.value) list.push({ colKey: "row-select", type: "multiple", width: 48 })
  list.push({ colKey: "id", title: "ID", width: 70 })
  list.push({ colKey: "name", title: "窗口名称", width: 190 })
  list.push({ colKey: "group_owner", title: rowOwnerTitle.value, width: 128 })
  if (isAdmin.value) list.push({ colKey: "team", title: "运营团队", width: 120 })
  list.push({ colKey: "proxy_mode", title: "代理方式", width: 96 })
  list.push({ colKey: "proxy_info", title: "代理信息", width: 180 })
  list.push({ colKey: "accounts", title: "关联账号", width: 170 })
  list.push({ colKey: "cloud_remark", title: "运营备注", width: 170 })
  list.push({ colKey: "business_status", title: "业务启停", width: 96 })
  if (isOwnDesktop.value) list.push({ colKey: "running", title: "运行状态", width: 96 })
  list.push({ colKey: "last_sync", title: "最近同步", width: 118 })
  list.push({ colKey: "op", title: "操作", width: opColumnWidth(rowActionLabels.value, { variant: "text", loading: true, primary: isOwnDesktop.value }), fixed: "right" })
  return list
})

const tableScroll = computed(() => ({ x: `${columns.value.reduce((sum, column) => sum + (column.width || 0), 0)}px` }))

const pageDescription = computed(() => {
  if (isOwnDesktop.value) return "同步比特窗口、管理代理方式与关联账号"
  if (isAdmin.value) return "按团队与运营查看所有窗口"
  if (isSenior.value && viewingMember.value) return `${ownTeamName.value} · 查看 ${ownerLabel(Number(memberFilter.value))} 的窗口`
  if (isSenior.value) return `${ownTeamName.value} · 默认查看自己`
  return "查看本人窗口；本机操作请在桌面端进行"
})

const candidateRows = computed(() => {
  const search = candidateSearch.value.trim().toLowerCase()
  return proxies.value.filter((proxy) => {
    if (candidateRegion.value && proxy.region !== candidateRegion.value) return false
    if (!search) return true
    return [proxy.name, proxy.host, proxy.supplier].filter(Boolean).join("\n").toLowerCase().includes(search)
  })
})
const candidateRegionOptions = computed(() => {
  const set = new Set()
  for (const proxy of proxies.value) if (proxy.region) set.add(proxy.region)
  return [...set].map((region) => ({ label: region, value: region }))
})
const candidateSelected = computed(() => proxies.value.find((proxy) => proxy.id === candidateSelectedId.value) || null)
const candidateUsable = computed(() => Boolean(candidateSelected.value) && proxySelectable(candidateSelected.value, candidateProfile.value))

const batchProxyOptions = computed(() => proxies.value
  .filter((proxy) => proxySelectable(proxy, null))
  .map((proxy) => ({
    label: `${proxy.name || `${proxy.host}:${proxy.port}`}（剩余 ${proxyRemaining(proxy)}）`,
    value: proxy.id,
  })))
const batchProxyIssue = computed(() => {
  if (!batchProxyRows.value.length) return ""
  if (!batchProxyOptions.value.length) return "当前团队没有同时满足启用、未到期、检测正常且有余量的托管代理。"
  const assignments = batchProxyRows.value
  for (const row of assignments) {
    if (!row.targetId) return `请为「${row.profile.name || row.profile.id}」选择目标代理。`
    const target = proxyById.value[row.targetId]
    if (!target) return "所选目标代理已不存在，请重新选择。"
    if (row.profile.proxy_id !== target.id && !proxySelectable(target, null)) {
      return `「${target.name || target.id}」${proxyAvailabilityReason(target, null)}，不能用于更换。`
    }
  }
  const demand = {}
  for (const row of assignments) {
    if (row.profile.local_status !== "active") continue
    if (row.profile.proxy_id === row.targetId) continue
    demand[row.targetId] = (demand[row.targetId] || 0) + 1
  }
  for (const [targetId, count] of Object.entries(demand)) {
    const target = proxyById.value[targetId]
    if (!target) continue
    const remaining = proxyRemaining(target)
    if (count > remaining) {
      return `「${target.name || target.id}」需要 ${count} 个名额，当前仅剩 ${remaining} 个。`
    }
  }
  return ""
})
const batchProxyAccountCount = computed(() => batchProxyRows.value
  .reduce((sum, row) => sum + (accountsByProfile.value.get(String(row.profile.id)) || []).length, 0))

const syncAddedProfiles = computed(() => {
  const scan = syncScan.value
  if (!scan) return []
  const added = new Set((scan.diff || []).filter((item) => item.kind === "added").map((item) => item.bit_profile_id))
  return (scan.profiles || []).filter((profile) => added.has(profile.bit_profile_id))
})
const syncChangedProfiles = computed(() => {
  const scan = syncScan.value
  if (!scan) return []
  const changed = new Set((scan.diff || []).filter((item) => item.kind === "changed").map((item) => item.bit_profile_id))
  return (scan.profiles || []).filter((profile) => changed.has(profile.bit_profile_id))
})
const syncMissingProfiles = computed(() => {
  const scan = syncScan.value
  if (!scan) return []
  const missing = new Set((scan.diff || []).filter((item) => item.kind === "missing").map((item) => item.bit_profile_id))
  return (scan.profiles || []).filter((profile) => missing.has(profile.bit_profile_id))
})
const syncProxyChanges = computed(() => syncPreview.value?.changes || [])
const syncActionableProxyChanges = computed(() => syncProxyChanges.value.filter((change) => ["changed", "unbound", "unknown"].includes(change.kind)))
const syncSelectableAdded = computed(() => syncAddedProfiles.value.filter((profile) => !profile.ownership_conflict))
const syncHasChanges = computed(() => !syncDone.value && (
  syncSelectableAdded.value.length > 0
  || syncChangedProfiles.value.length > 0
  || syncMissingProfiles.value.length > 0
  || syncActionableProxyChanges.value.length > 0
))
const syncSummaryItems = computed(() => {
  const result = syncResult.value
  if (syncDone.value && result) {
    return [
      { key: "total", label: "窗口总数", value: result.total, tone: "info" },
      { key: "added", label: "已建立关联", value: result.added, tone: "success" },
      { key: "updated", label: "已更新", value: result.updated, tone: "success" },
      { key: "missing", label: "标记缺失", value: result.missing, tone: "warning" },
      { key: "proxy", label: "代理已核对", value: result.proxy, tone: "success" },
      { key: "skipped", label: "跳过", value: result.skipped, tone: "neutral" },
    ]
  }
  return [
    { key: "total", label: "窗口总数", value: syncSnapshot.value?.profiles?.length || 0, tone: "info" },
    { key: "added", label: "新增窗口", value: syncAddedProfiles.value.length, tone: "warning" },
    { key: "updated", label: "字段变更", value: syncChangedProfiles.value.length, tone: "warning" },
    { key: "missing", label: "窗口缺失", value: syncMissingProfiles.value.length, tone: "warning" },
    { key: "proxy", label: "代理变化", value: syncProxyChanges.value.length, tone: "warning" },
  ]
})

const identityMasked = computed(() => maskMainUserId(identityDetectedMask.value))

watch(isOwnDesktop, (value) => {
  if (value) refreshRunningStates()
})

onMounted(async () => {
  try {
    currentUser.value = await sessionClient.me()
    memberFilter.value = isSenior.value ? selfId.value : ""
    await loadAuxiliary()
    await loadProfiles()
  } catch (e) {
    error.value = e.message || "加载浏览器窗口失败"
  } finally {
    loading.value = false
  }
})

function emptyFilters() {
  return { search: "", mode: "", business: "", running: "" }
}

function emptyCreateForm() {
  return { name: "", group_id: "", group_name: "", remark: "", proxy_intent: "self" }
}

async function loadAuxiliary() {
  const tasks = [
    accountClient.list().catch(() => []),
    proxyClient.list().catch(() => []),
    bitAccountClient.get().catch(() => null),
  ]
  if (isSenior.value || isAdmin.value) tasks.push(usersClient.listUsers().catch(() => []))
  const [accounts, proxyList, identity, users] = await Promise.all(tasks)
  mediaAccounts.value = Array.isArray(accounts) ? accounts : []
  proxies.value = Array.isArray(proxyList) ? proxyList : []
  boundIdentity.value = identity || null
  roster.value = Array.isArray(users) ? users : []
}

async function loadProfiles() {
  error.value = ""
  bindingMissing.value = false
  loading.value = true
  try {
    const filters = {}
    if (isSenior.value) filters.user_id = memberFilter.value || selfId.value
    const result = await bindingClient.listProfiles(filters)
    profiles.value = Array.isArray(result) ? result : []
  } catch (e) {
    if (isNotBoundError(e)) {
      profiles.value = []
      bindingMissing.value = !(isSenior.value && viewingMember.value)
    } else {
      error.value = e.message || "加载浏览器窗口失败"
    }
  } finally {
    loading.value = false
    page.value = 1
    pruneSelection()
  }
}

async function reloadAll() {
  await Promise.all([loadProfiles(), loadAuxiliary()])
  if (isOwnDesktop.value) await refreshRunningStates()
}

function pruneSelection() {
  const visible = new Set(filteredProfiles.value.map((profile) => profile.id))
  selectedRowKeys.value = selectedRowKeys.value.filter((id) => visible.has(id))
}

function onSeniorMemberChange() {
  selectedRowKeys.value = []
  resetAppliedFilters()
  loadProfiles()
  if (!viewingMember.value) refreshRunningStates()
}

function onAdminTeamChange() {
  draftMember.value = ""
  selectedRowKeys.value = []
}

function onAdminMemberChange() {
  selectedRowKeys.value = []
}

function submitSearch() {
  appliedFilters.value = { ...filterDraft.value }
  if (isAdmin.value) {
    appliedTeam.value = draftTeam.value
    appliedMember.value = draftMember.value
  }
  page.value = 1
  pruneSelection()
}

function resetAppliedFilters() {
  filterDraft.value = emptyFilters()
  appliedFilters.value = emptyFilters()
  appliedTeam.value = ""
  appliedMember.value = ""
  draftTeam.value = ""
  draftMember.value = ""
  statusFilter.value = ""
  page.value = 1
}

function resetFilters() {
  selectedRowKeys.value = []
  resetAppliedFilters()
  if (isSenior.value) {
    memberFilter.value = selfId.value
    loadProfiles()
  } else {
    pruneSelection()
  }
}

function toggleStatFilter(key) {
  statusFilter.value = statusFilter.value === key ? "" : key
  page.value = 1
  pruneSelection()
}

function isNotBoundError(e) {
  return e?.code === "BIT_NOT_BOUND" || e?.errcode === 23008
}

function isIdentityMismatch(e) {
  return e?.code === "BIT_MAIN_USER_MISMATCH" || e?.errcode === 23002
}

function isUncertainProxyError(e) {
  return isUncertainAssignmentError(e)
}

function isTimeoutError(e) {
  const message = String(e?.message || e || "")
  return e?.code === "RESULT_UNKNOWN" || message.includes("timed out") || message.includes("timeout") || message.includes("超时")
}

function proxyExpired(proxy) {
  return Boolean(proxy?.expired) || Boolean(proxy?.expires_at && new Date(proxy.expires_at).getTime() <= Date.now())
}

function proxyRemaining(proxy) {
  if (!proxy) return 0
  if (proxy.remaining_profile_count !== undefined && proxy.remaining_profile_count !== null) return Number(proxy.remaining_profile_count)
  return Math.max(Number(proxy.max_profile_count || 0) - Number(proxy.assigned_profile_count || 0), 0)
}

function proxySelectable(proxy, profile) {
  if (!proxy) return false
  const current = profile && profile.proxy_id === proxy.id
  if (proxy.assignment_blocker && !(current && proxy.assignment_blocker === "代理窗口配额已满")) return false
  if (proxy.business_status !== "active") return false
  if (proxyExpired(proxy)) return false
  if (proxy.last_check_result !== "ok") return false
  return current || proxyRemaining(proxy) > 0
}

function proxyAvailabilityReason(proxy, profile) {
  if (!proxy) return "代理不存在"
  const current = profile && profile.proxy_id === proxy.id
  if (proxy.assignment_blocker && !(current && proxy.assignment_blocker === "代理窗口配额已满")) return proxy.assignment_blocker.replace(/^代理/, "")
  if (proxy.business_status !== "active") return "已停用"
  if (proxyExpired(proxy)) return "已到期"
  if (proxy.last_check_result !== "ok") return "检测不可用"
  if (current) return "当前托管代理"
  if (proxyRemaining(proxy) <= 0) return "额度已满"
  return "可用"
}

// 重新同步不是新分配，但仍要求资源本身可用；同代理的幂等豁免不能盖住过期或停用。
function proxyResyncable(proxy) {
  if (!proxy || (proxy.assignment_blocker && proxy.assignment_blocker !== "代理窗口配额已满")) return false
  return proxy.business_status === "active" && !proxyExpired(proxy) && proxy.last_check_result === "ok"
}

function proxyConfigMismatch(profile, proxy) {
  if (!profile?.proxy_id || !proxy) return false
  return String(profile.proxy_type || "") !== String(proxy.proxy_protocol || "")
    || String(profile.proxy_host || "") !== String(proxy.host || "")
    || Number(profile.proxy_port || 0) !== Number(proxy.port || 0)
}

function anomalyReasons(profile) {
  const reasons = []
  if (profile.local_status === "local_missing") reasons.push("窗口缺失")
  if (!profile.proxy_id) return reasons
  const proxy = proxyById.value[profile.proxy_id]
  if (!proxy) {
    reasons.push("代理资源不可用")
    return reasons
  }
  if (proxyConfigMismatch(profile, proxy)) reasons.push("配置不一致")
  if (proxyExpired(proxy)) reasons.push("代理已到期")
  if (!proxy.last_check_result) reasons.push("代理未检测")
  else if (proxy.last_check_result !== "ok") reasons.push("代理检测异常")
  return reasons
}

function hasPendingIssue(profile) {
  return anomalyReasons(profile).length > 0
}

function hasRowIssue(profile) {
  return hasPendingIssue(profile)
}

function rowOperable(profile) {
  return profile.local_status === "active" && profile.business_status === "enabled"
}

function isManaged(profile) {
  return Boolean(profile.proxy_id)
}

function actualProxyText(profile) {
  if (!profile.proxy_host || !profile.proxy_port) return ""
  return `${profile.proxy_type ? `${profile.proxy_type}://` : ""}${profile.proxy_host}:${profile.proxy_port}`
}

function proxySubText(profile) {
  if (isManaged(profile)) {
    const proxy = proxyById.value[profile.proxy_id]
    return proxy ? (proxy.name || `${proxy.host}:${proxy.port}`) : "托管资源不可用"
  }
  return profile.proxy_host ? "比特当前代理" : "直连"
}

function managedProxy(profile) {
  return profile?.proxy_id ? proxyById.value[profile.proxy_id] || null : null
}

function proxyModeLabel(profile) {
  return isManaged(profile) ? "系统托管" : "比特自管"
}

function accountsFor(profile) {
  return accountsByProfile.value.get(String(profile.id)) || []
}

function accountTitle(profile) {
  const accounts = accountsFor(profile)
  if (!accounts.length) return ""
  return accounts.map((account) => `${platformLabel(account.platform)} ${account.name || account.platform_account_id || "未识别"}`).join("、")
}

function accountLines(profile) {
  return accountsFor(profile).slice(0, 2).map((account) => `${platformLabel(account.platform)} ${account.name || account.platform_account_id || "未识别"}`)
}

function ownerLabel(userId) {
  const user = roster.value.find((item) => String(item.id) === String(userId))
  if (user) return user.nickname || user.username || `用户 ${user.id}`
  if (String(userId) === selfId.value) return currentUser.value?.nickname || currentUser.value?.username || "本人"
  return `用户 ${userId}`
}

function teamName(profile) {
  const user = roster.value.find((item) => String(item.id) === String(profile.user_id))
  if (user?.team_name) return user.team_name
  if (profile.team_id !== null && profile.team_id !== undefined) return `团队 ${profile.team_id}`
  return "-"
}

function groupOwnerText(profile) {
  if (isOwnDesktop.value || (!isAdmin.value && !isSenior.value)) return profile.group_name || profile.group_id || "-"
  return ownerLabel(profile.user_id)
}

function runningState(profile) {
  const key = profile.bit_profile_id
  return Object.prototype.hasOwnProperty.call(runningStates.value, key) ? runningStates.value[key] : undefined
}

function runningLabel(profile) {
  const state = runningState(profile)
  if (state === true) return "已打开"
  if (state === false) return "已关闭"
  return "未知"
}

function runningTone(profile) {
  const state = runningState(profile)
  if (state === true) return "success"
  return "neutral"
}

let runningPromise = null

async function refreshRunningStates({ silent = true } = {}) {
  if (!isOwnDesktop.value) return null
  if (runningPromise) return runningPromise
  runningPromise = (async () => {
    try {
      const snapshot = await localAgent.profileScan()
      applyRunningSnapshot(snapshot)
      return snapshot
    } catch (e) {
      if (!silent) feedback.toast.error(executionErrorMessage(e, "读取本机窗口运行状态失败"))
      return null
    } finally {
      runningPromise = null
    }
  })()
  return runningPromise
}

function applyRunningSnapshot(snapshot) {
  const next = {}
  for (const profile of snapshot?.profiles || []) {
    const status = String(profile.status ?? "").trim()
    if (status === "1") next[profile.bit_profile_id] = true
    else if (status === "0") next[profile.bit_profile_id] = false
  }
  runningStates.value = next
}

function maskMainUserId(value) {
  const raw = String(value || "")
  if (!raw) return "-"
  if (raw.length <= 8) return "****"
  return `${raw.slice(0, 4)}****${raw.slice(-4)}`
}

function validDate(value) {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getFullYear() < 2000) return null
  return date
}

function formatTime(value) {
  const date = validDate(value)
  return date ? date.toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" }) : "-"
}

function formatDate(value) {
  const date = validDate(value)
  return date ? date.toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }) : "-"
}

function openDetail(profile) {
  detailProfile.value = profile
  detailVisible.value = true
}

function openNote(profile) {
  noteProfile.value = profile
  noteText.value = profile.cloud_remark || ""
  noteError.value = ""
  noteVisible.value = true
}

async function saveNote() {
  if (!noteProfile.value) return
  noteSaving.value = true
  noteError.value = ""
  try {
    await bindingClient.updateProfile(noteProfile.value.id, { cloud_remark: noteText.value })
    noteVisible.value = false
    feedback.toast.success("运营备注已保存")
    await reloadAll()
  } catch (e) {
    noteError.value = e.message || "运营备注保存失败"
  } finally {
    noteSaving.value = false
  }
}

async function toggleRunning(profile) {
  if (!isOwnDesktop.value) return
  if (!rowOperable(profile)) {
    feedback.toast.info(profile.local_status === "local_missing" ? "窗口已缺失，不能操作。" : "窗口已停用，请先启用。")
    return
  }
  const state = runningState(profile)
  if (state === undefined) {
    await refreshRunningStates({ silent: false })
    return
  }
  operatingIds.value = [...operatingIds.value, profile.id]
  try {
    if (state) await localAgent.profileClose(profile.bit_profile_id)
    else await localAgent.profileOpen(profile.bit_profile_id)
    runningStates.value = { ...runningStates.value, [profile.bit_profile_id]: !state }
    feedback.toast.success(state ? "窗口已关闭" : "窗口已打开")
  } catch (e) {
    feedback.toast.error(executionErrorMessage(e, state ? "关闭窗口失败" : "打开窗口失败"))
  } finally {
    operatingIds.value = operatingIds.value.filter((id) => id !== profile.id)
  }
}

const businessTarget = ref(null)
const businessNextEnabled = ref(false)
const businessConfirmVisible = ref(false)
const businessSaving = ref(false)

async function requestBusinessChange(profile, value) {
  if (!isOwnDesktop.value) return
  const nextEnabled = Boolean(value)
  if (profile.local_status !== "active") {
    feedback.toast.info("窗口在本机缺失，不能修改业务启停。")
    return
  }
  if (!nextEnabled) {
    const snapshot = await refreshRunningStates({ silent: false })
    if (!snapshot) {
      feedback.notice({
        title: "无法确认运行状态",
        message: "停用前必须确认窗口已关闭。请先恢复比特浏览器连接并重新读取窗口状态。",
      })
      return
    }
    const state = runningState(profile)
    if (state === true) {
      feedback.notice({
        title: "请先关闭窗口",
        message: "此窗口仍在运行，请先关闭再停用；不会删除窗口、账号或历史记录。",
      })
      return
    }
    if (state === undefined) {
      feedback.notice({
        title: "无法确认运行状态",
        message: "本次读取没有返回该窗口的运行状态。请先同步窗口核对后再停用。",
      })
      return
    }
  }
  businessTarget.value = profile
  businessNextEnabled.value = nextEnabled
  businessConfirmVisible.value = true
}

async function confirmBusinessChange() {
  if (!businessTarget.value) return
  businessSaving.value = true
  try {
    await bindingClient.updateProfile(businessTarget.value.id, {
      business_status: businessNextEnabled.value ? "enabled" : "disabled",
    })
    businessConfirmVisible.value = false
    feedback.toast.success(businessNextEnabled.value ? "窗口已启用" : "窗口已停用")
    await reloadAll()
  } catch (e) {
    feedback.toast.error(e.message || "业务启停修改失败")
  } finally {
    businessSaving.value = false
  }
}

async function fetchProfileGroups() {
  try {
    const result = await localAgent.profileGroups()
    const groups = result?.data?.groups || result?.groups || []
    return Array.isArray(groups) ? groups : []
  } catch {
    return []
  }
}

function groupOptions(groups, current) {
  const options = groups.map((group) => ({ label: group.name || group.id, value: group.id }))
  if (current?.group_id && !options.some((option) => option.value === current.group_id)) {
    options.unshift({ label: current.group_name || current.group_id, value: current.group_id })
  }
  return options
}

function applyGroup(form, groups, groupId) {
  const group = groups.find((item) => item.id === groupId)
  form.group_id = groupId
  form.group_name = group?.name || ""
}

function openCreate() {
  if (!ensureLocalReady()) return
  createForm.value = emptyCreateForm()
  createGroups.value = []
  createError.value = ""
  createCloudRetry.value = false
  createUncertain.value = false
  createdProfile.value = null
  createVisible.value = true
  createGroupLoading.value = true
  fetchProfileGroups().then((groups) => {
    createGroups.value = groups
    createGroupLoading.value = false
  })
}

function ensureLocalReady() {
  if (!isOwnDesktop.value) {
    feedback.toast.info("本机窗口操作只能在桌面端由窗口本人执行。")
    return false
  }
  if (boundIdentity.value && boundIdentity.value.bound === false) {
    feedback.notice({
      title: "尚未绑定比特主账号",
      message: "当前账号还没有绑定比特浏览器主账号，请先在「个人信息」完成绑定后再操作窗口。",
    })
    return false
  }
  return true
}

async function submitCreate() {
  if (!ensureLocalReady()) return
  const form = createForm.value
  if (!form.name.trim()) {
    createError.value = "请填写窗口名称。"
    return
  }
  if (!form.group_id) {
    createError.value = "请选择比特浏览器中的真实分组。"
    return
  }
  createError.value = ""
  createCloudRetry.value = false
  createUncertain.value = false
  progressMode.value = "create"
  progressTitle.value = "新增窗口"
  progressRunningLabel.value = "正在创建"
  progressLabels.value = {}
  progressRetryLabel.value = "重试未完成项"
  progressVerifyLabel.value = "核实并同步"
  progressItems.value = [
    { key: "create", label: "创建比特窗口", status: "pending" },
    { key: "cloud", label: "同步 Cloud 记录", status: "pending" },
  ]
  retryPayload.value = { mode: "create", target: { name: form.name.trim(), group_id: form.group_id, group_name: form.group_name, remark: form.remark || "" } }
  progressPhase.value = "running"
  progressVisible.value = true
  const createItem = progressItems.value[0]
  const cloudItem = progressItems.value[1]
  createItem.status = "processing"
  let snapshot = null
  let bitProfileId = ""
  try {
    const created = await localAgent.profileCreate({
      name: form.name.trim(),
      group_id: form.group_id,
      group_name: form.group_name,
      remark: form.remark || "",
    })
    bitProfileId = String(created?.bit_profile_id || "")
    snapshot = created?.snapshot || null
    if (!bitProfileId) throw new Error("比特没有返回窗口 ID")
    retryPayload.value.bitProfileId = bitProfileId
    if (!snapshot) {
      createItem.status = "unconfirmed"
      createItem.reason = "窗口可能已创建，但没有拿到已核实的快照；请先同步窗口确认，不要重复创建。"
      progressPhase.value = "done"
      return
    }
    createItem.status = "success"
  } catch (e) {
    if (e?.code === "cloud_request_failed" || e?.code === "profile_readback_unverified" || isTimeoutError(e)) {
      createItem.status = "unconfirmed"
      createItem.reason = "创建结果未知，请先核实比特中是否已出现该窗口，再同步窗口；不要重复创建。"
    } else {
      createItem.status = "failed"
      createItem.reason = executionErrorMessage(e, "创建比特窗口失败")
    }
    progressPhase.value = "done"
    return
  }
  cloudItem.status = "processing"
  try {
    const row = await syncCreatedProfile(snapshot, bitProfileId)
    cloudItem.status = "success"
    createdProfile.value = row
    createdMode.value = form.proxy_intent
    await loadProfiles()
  } catch (e) {
    cloudItem.status = isIdentityMismatch(e) || isNotBoundError(e) ? "failed" : "unconfirmed"
    cloudItem.reason = isIdentityMismatch(e)
      ? "本机比特身份与系统绑定不一致，Cloud 记录未同步。"
      : isNotBoundError(e)
        ? "系统账号尚未绑定比特主账号，Cloud 记录未同步。"
        : e.message || "Cloud 记录同步失败"
    retryPayload.value.snapshot = snapshot
    retryPayload.value.bitProfileId = bitProfileId
    createCloudRetry.value = true
  } finally {
    progressPhase.value = "done"
  }
}

async function syncCreatedProfile(snapshot, bitProfileId) {
  const scan = await bindingClient.submit(snapshot)
  const candidate = (scan?.profiles || []).find((profile) => profile.bit_profile_id === bitProfileId)
  if (!candidate) throw new Error("Cloud 未返回创建窗口的比对结果")
  await bindingClient.confirm(scan.id, [bitProfileId])
  const rows = await bindingClient.listProfiles({ user_id: selfId.value, bit_profile_id: bitProfileId })
  const stored = (Array.isArray(rows) ? rows : []).find((profile) => profile.bit_profile_id === bitProfileId)
  // candidate.id 是扫描生成的临时标识，不是数据库 ID；读不到正式记录时宁可留空。
  return stored || { ...candidate, id: "" }
}

async function retryCreateCloud() {
  const payload = retryPayload.value
  if (!payload?.bitProfileId) return
  progressMode.value = "create"
  progressTitle.value = "同步新增窗口"
  progressPhase.value = "running"
  progressRunningLabel.value = "正在同步"
  progressItems.value = [
    { key: "create", label: "创建比特窗口", status: "skipped", reason: "窗口已在比特创建并核实" },
    { key: "cloud", label: "同步 Cloud 记录", status: "processing" },
  ]
  progressVisible.value = true
  try {
    const snapshot = await localAgent.profileScan()
    const row = await syncCreatedProfile(snapshot, payload.bitProfileId)
    progressItems.value[1].status = "success"
    createdProfile.value = row
    createdMode.value = createForm.value.proxy_intent
    createCloudRetry.value = false
    await loadProfiles()
  } catch (e) {
    progressItems.value[1].status = "failed"
    progressItems.value[1].reason = e.message || "Cloud 记录同步失败"
  } finally {
    progressPhase.value = "done"
  }
}

function openCreatedProxy() {
  if (!createdProfile.value) return
  createdDialogVisible.value = false
  if (createdMode.value === "managed") openCandidates(createdProfile.value)
  else openManage(createdProfile.value)
}

async function openEdit(profile) {
  if (!ensureLocalReady()) return
  if (!rowOperable(profile)) {
    feedback.toast.info(profile.local_status === "local_missing" ? "窗口已缺失，不能编辑。" : "窗口已停用，请先启用再编辑。")
    return
  }
  editProfile.value = profile
  editForm.value = {
    name: profile.name || "",
    group_id: profile.group_id || "",
    group_name: profile.group_name || "",
    remark: profile.remark || "",
    proxy_intent: "self",
  }
  editGroups.value = []
  editError.value = ""
  editPending.value = null
  editVisible.value = true
  editGroupLoading.value = true
  const groups = await fetchProfileGroups()
  if (profile.group_id && !groups.some((group) => group.id === profile.group_id)) {
    groups.unshift({ id: profile.group_id, name: profile.group_name || profile.group_id })
  }
  editGroups.value = groups
  editGroupLoading.value = false
}

async function submitEdit() {
  if (!editProfile.value) return
  const form = editForm.value
  if (!form.name.trim()) {
    editError.value = "请填写窗口名称。"
    return
  }
  if (!form.group_id) {
    editError.value = "请选择比特浏览器中的真实分组。"
    return
  }
  editError.value = ""
  const profile = editProfile.value
  const targets = {
    name: form.name.trim(),
    group_id: form.group_id,
    group_name: form.group_name,
    remark: form.remark || "",
  }
  editPending.value = {
    profile,
    targets,
    original: { name: profile.name || "", group_id: profile.group_id || "", group_name: profile.group_name || "", remark: profile.remark || "" },
    snapshot: null,
    fields: [],
  }
  await runEditWrite(false)
}

async function runEditWrite(verifiedOnly) {
  const pending = editPending.value
  if (!pending) return
  const profile = pending.profile
  const targets = pending.targets
  progressMode.value = "edit"
  progressTitle.value = "编辑窗口"
  progressRunningLabel.value = "正在写入"
  progressLabels.value = { success: "已核实", unconfirmed: "待核实", failed: "失败" }
  progressRetryLabel.value = "重试未完成项"
  progressVerifyLabel.value = "核实未完成项"
  progressItems.value = [
    { key: "write", label: "写入比特窗口并读回核实", status: "pending" },
    { key: "cloud", label: "同步 Cloud 记录", status: "pending" },
  ]
  progressPhase.value = "running"
  progressVisible.value = true
  const writeItem = progressItems.value[0]
  const cloudItem = progressItems.value[1]
  writeItem.status = "processing"

  const sendTargets = { ...targets }
  if (verifiedOnly && pending.verified) {
    for (const field of pending.verified) sendTargets[field] = pending.observed?.[field] ?? sendTargets[field]
  }

  let snapshot = null
  let fields = []
  try {
    const result = await localAgent.profileUpdate({
      bit_profile_id: profile.bit_profile_id,
      name: sendTargets.name,
      group_id: sendTargets.group_id,
      group_name: sendTargets.group_name,
      remark: sendTargets.remark,
    })
    snapshot = result?.snapshot || null
    fields = Array.isArray(result?.fields) ? result.fields : []
    applyEditObservation(pending, snapshot, fields)
    const unconfirmed = fields.filter((field) => field.status !== "verified")
    if (!snapshot) {
      writeItem.status = "unconfirmed"
      writeItem.reason = "没有拿到写入后的窗口快照，无法核实实际结果。"
    } else if (unconfirmed.length) {
      writeItem.status = "unconfirmed"
      writeItem.reason = `以下字段未能核实：${unconfirmed.map((field) => fieldLabel(field.field)).join("、")}`
    } else {
      writeItem.status = "success"
    }
  } catch (e) {
    if (e?.code === "cloud_request_failed" || e?.code === "profile_readback_unverified" || isTimeoutError(e)) {
      writeItem.status = "unconfirmed"
      writeItem.reason = "写入结果无法核实，请先读取比特中的实际值。"
    } else {
      writeItem.status = "failed"
      writeItem.reason = executionErrorMessage(e, "写入比特窗口失败")
    }
    progressPhase.value = "done"
    return
  }

  cloudItem.status = "processing"
  try {
    if (!snapshot) throw new Error("没有可用于同步的实际窗口快照")
    await syncObservedSnapshot(snapshot)
    cloudItem.status = "success"
    pending.snapshot = snapshot
    pending.fields = fields
  } catch (e) {
    cloudItem.status = "failed"
    cloudItem.reason = e.message || "Cloud 记录同步失败"
    pending.snapshot = snapshot
    pending.fields = fields
  } finally {
    progressPhase.value = "done"
  }
}

async function syncObservedSnapshot(snapshot) {
  const scan = await bindingClient.submit(snapshot)
  await bindingClient.confirm(scan.id, [])
  return scan
}

async function verifyAndRetryEdit() {
  const pending = editPending.value
  if (!pending) return
  editResultVisible.value = false
  let snapshot
  try {
    snapshot = await localAgent.profileScan()
  } catch (e) {
    feedback.toast.error(executionErrorMessage(e, "读取比特实际值失败"))
    return
  }
  const observed = (snapshot?.profiles || []).find((profile) => profile.bit_profile_id === pending.profile.bit_profile_id)
  if (!observed) {
    feedback.notice({ title: "未读取到窗口", message: "本次扫描没有返回该窗口，无法核实修改结果。请确认窗口仍然存在并重新同步。" })
    return
  }
  const verified = []
  const remaining = []
  for (const field of ["name", "group_id", "group_name", "remark"]) {
    if (String(observed[field] ?? "") === String(pending.targets[field] ?? "")) verified.push(field)
    else remaining.push(field)
  }
  pending.verified = verified
  pending.observed = {
    name: observed.name ?? "",
    group_id: observed.group_id ?? "",
    group_name: observed.group_name ?? "",
    remark: observed.remark ?? "",
  }
  if (!remaining.length) {
    try {
      await syncObservedSnapshot(snapshot)
      editResultKind.value = "resolved"
      editResultRows.value = editFieldRows(pending)
      editResultMessage.value = "已重新读取，所有目标字段均已生效，Cloud 记录已同步。"
      editResultVisible.value = true
      await reloadAll()
    } catch (e) {
      feedback.toast.error(e.message || "Cloud 记录同步失败")
    }
    return
  }
  await runEditWrite(true)
}

function editFieldRows(pending) {
  return ["name", "group_id", "remark"].map((field) => {
    const verifiedNow = field === "group_id"
      ? pending.verified?.includes("group_id") && pending.verified?.includes("group_name")
      : pending.verified?.includes(field)
    const observedValue = field === "group_id"
      ? (pending.observed?.group_name || pending.observed?.group_id || "")
      : (pending.observed?.[field] || "")
    return {
      field: fieldLabel(field),
      target: field === "group_id" ? (pending.targets.group_name || pending.targets.group_id) : pending.targets[field],
      actual: observedValue || "未读到",
      status: verifiedNow ? "已核实一致" : "待核实",
      tone: verifiedNow ? "success" : "warning",
    }
  })
}

function fieldLabel(field) {
  return { name: "窗口名称", group_id: "比特分组", group_name: "比特分组", remark: "比特备注" }[field] || field
}

function closeEditResult() {
  editResultVisible.value = false
  if (["uncertain", "resolved", "cloud_failed"].includes(editResultKind.value)) {
    editVisible.value = false
  }
}

function openManage(profile) {
  if (!ensureLocalReady()) return
  manageProfile.value = profile
  manageNote.value = manageWarning(profile)
  manageVisible.value = true
}

function manageWarning(profile) {
  const proxy = managedProxy(profile)
  if (isManaged(profile) && !proxy) return "托管代理已不存在或不在当前团队范围内，请更换代理或改为比特自管。"
  if (isManaged(profile) && proxy) {
    if (proxyExpired(proxy)) return "该托管代理已到期，不能重新同步或分配给其他窗口；可更换代理或改为比特自管。"
    if (proxy.business_status !== "active") return "该托管代理已停用，不能重新同步或分配给其他窗口；可更换代理或改为比特自管。"
    if (!proxy.last_check_result) return "该托管代理尚未检测，不能重新同步或分配给其他窗口；可先到代理管理检测。"
    if (proxy.last_check_result !== "ok") return "该托管代理最近检测失败，不能重新同步或分配给其他窗口；可先到代理管理检测。"
    if (proxyConfigMismatch(profile, proxy)) return "比特中的代理与托管配置不一致；可重新同步托管配置，或保留当前代理改为比特自管。"
  }
  return ""
}

function openRowProxyAction(profile) {
  if (profile.local_status === "local_missing") {
    openMissing(profile)
    return
  }
  const proxy = managedProxy(profile)
  if (isManaged(profile) && proxy && proxyConfigMismatch(profile, proxy)) {
    openMismatch(profile)
    return
  }
  openManage(profile)
}

function openMissing(profile) {
  missingProfile.value = profile
  missingVisible.value = true
}

function openMismatch(profile) {
  mismatchProfile.value = profile
  mismatchVisible.value = true
}

function openReleaseConfirm(profile) {
  if (!ensureLocalReady()) return
  manageVisible.value = false
  mismatchVisible.value = false
  releaseProfile.value = profile
  releaseConfirmVisible.value = true
}

async function confirmRelease() {
  const profile = releaseProfile.value
  if (!profile) return
  releaseConfirmVisible.value = false
  await runProxyAction("release", profile, managedProxy(profile))
}

function openUnbindConfirm(profile) {
  if (!ensureLocalReady()) return
  manageVisible.value = false
  mismatchVisible.value = false
  unbindProfile.value = profile
  unbindConfirmVisible.value = true
}

async function confirmUnbind() {
  const profile = unbindProfile.value
  if (!profile) return
  unbindConfirmVisible.value = false
  await runProxyAction("unbind", profile, managedProxy(profile))
}

function openResyncConfirm(profile) {
  const proxy = managedProxy(profile)
  if (!proxy) return
  if (!proxyResyncable(proxy)) {
    feedback.toast.info(`该托管代理${proxyAvailabilityReason(proxy, profile)}，不能重新同步。`)
    return
  }
  mismatchVisible.value = false
  manageVisible.value = false
  resyncProfile.value = profile
  resyncProxy.value = proxy
  resyncConfirmVisible.value = true
}

async function confirmResync() {
  const profile = resyncProfile.value
  const proxy = resyncProxy.value
  if (!profile || !proxy) return
  resyncConfirmVisible.value = false
  await runProxyAction("resync", profile, proxy)
}

function openCandidates(profile) {
  if (!ensureLocalReady()) return
  manageVisible.value = false
  candidateProfile.value = profile
  candidateSearch.value = ""
  candidateRegion.value = ""
  candidateError.value = ""
  candidateSelectedId.value = ""
  candidateVisible.value = true
}

function selectCandidate(proxy) {
  if (!proxySelectable(proxy, candidateProfile.value)) return
  candidateSelectedId.value = proxy.id
}

async function confirmCandidate() {
  const profile = candidateProfile.value
  const target = candidateSelected.value
  if (!profile || !target) return
  if (!proxySelectable(target, profile)) {
    candidateError.value = `该代理${proxyAvailabilityReason(target, profile)}，不能应用。`
    return
  }
  candidateVisible.value = false
  createdDialogVisible.value = false
  await runProxyAction("assign", profile, target)
}

function closeManageToCandidates() {
  const profile = manageProfile.value
  if (!profile) return
  if (!ensureLocalReady()) return
  openCandidates(profile)
}

async function runProxyAction(kind, profile, targetProxy) {
  const actionLabel = {
    assign: "应用托管代理",
    resync: "重新同步托管代理",
    release: "改为比特自管",
    unbind: "解绑并改为直连",
  }[kind] || "处理代理"
  progressMode.value = "proxy-action"
  progressTitle.value = actionLabel
  progressPhase.value = "running"
  progressRunningLabel.value = "正在处理"
  progressLabels.value = { success: "已完成", failed: "失败" }
  progressRetryLabel.value = "重试失败项"
  progressVerifyLabel.value = "读取并核对"
  progressItems.value = [{ key: "action", label: actionLabel, status: "processing" }]
  retryPayload.value = {
    mode: "proxy-action",
    kind,
    profileId: profile.id,
    bitProfileId: profile.bit_profile_id,
    profileName: profile.name,
    targetProxyId: targetProxy?.id || profile.proxy_id || "",
  }
  progressVisible.value = true
  try {
    await callProxyAction(kind, profile, targetProxy)
    progressItems.value[0].status = "success"
  } catch (e) {
    if (isUncertainProxyError(e) && kind !== "release") {
      progressItems.value[0].status = "unconfirmed"
      progressItems.value[0].reason = "写入已发出但未能核实，请先读取比特中的实际配置再确认。"
    } else {
      progressItems.value[0].status = "failed"
      progressItems.value[0].reason = isUncertainProxyError(e) ? "请求可能已生效，请重新读取窗口后确认。" : (e.message || `${actionLabel}失败`)
    }
  } finally {
    progressPhase.value = "done"
  }
  if (progressItems.value[0].status === "success") {
    try { await afterProxyChange() } catch { feedback.toast.error("代理已应用，但列表刷新失败，请手动刷新。") }
  }
}

async function callProxyAction(kind, profile, targetProxy) {
  if (kind === "assign" || kind === "resync") {
    if (!targetProxy) throw new Error("目标代理不存在")
    await proxyClient.assign(targetProxy.id, profile.id)
    return
  }
  if (kind === "release") {
    if (!profile.proxy_id) throw new Error("该窗口没有系统托管关系")
    await proxyClient.release(profile.proxy_id, profile.id)
    return
  }
  if (kind === "unbind") {
    if (profile.proxy_id) await proxyClient.unbind(profile.proxy_id, profile.id)
    else await proxyClient.clearProfileProxy(profile.id)
    return
  }
  throw new Error("未知的代理操作")
}

async function afterProxyChange() {
  await Promise.all([loadProfiles(), loadAuxiliary()])
  if (isOwnDesktop.value) await refreshRunningStates()
}

async function verifyProxyAction() {
  const payload = retryPayload.value
  if (!payload || payload.mode !== "proxy-action") return
  let snapshot
  try {
    snapshot = await localAgent.profileScan()
  } catch (e) {
    feedback.toast.error(executionErrorMessage(e, "读取比特实际配置失败"))
    return
  }
  const observed = (snapshot?.profiles || []).find((item) => item.bit_profile_id === payload.bitProfileId)
  if (!observed) {
    progressItems.value[0].status = "unconfirmed"
    progressItems.value[0].reason = "本次扫描没有返回该窗口，无法核对；请重新同步窗口。"
    return
  }
  if (payload.kind === "assign" || payload.kind === "resync") {
    const target = proxyById.value[payload.targetProxyId]
    if (!target) {
      progressItems.value[0].status = "failed"
      progressItems.value[0].reason = "目标代理已不存在，不能继续核对。"
      return
    }
    if (classifyObservedAssignment(observed, target) === "different") {
      progressItems.value[0].status = "failed"
      progressItems.value[0].reason = "比特中的实际配置与目标不一致，可重试应用。"
      return
    }
    progressItems.value[0].status = "unconfirmed"
    progressItems.value[0].reason = "协议与地址已匹配，认证信息无法核对；请同步窗口并检查代理后再处理。"
    return
  }
  const cleared = !observed.proxy_host && Number(observed.proxy_port || 0) === 0
  if (!cleared) {
    progressItems.value[0].status = "failed"
    progressItems.value[0].reason = "比特中的代理尚未清除，可重试解绑。"
    return
  }
  progressItems.value[0].status = "unconfirmed"
  progressItems.value[0].reason = "已读取到直连状态，请同步窗口确认系统关系。"
}

async function runSync() {
  if (!ensureLocalReady()) return
  syncVisible.value = false
  syncDone.value = false
  syncResult.value = null
  syncError.value = ""
  syncIdentityMismatch.value = false
  progressMode.value = "sync"
  progressTitle.value = "同步窗口"
  progressRunningLabel.value = "正在同步"
  progressLabels.value = {}
  progressRetryLabel.value = "重新同步"
  progressItems.value = [
    { key: "scan", label: "读取本机窗口", status: "pending" },
    { key: "submit", label: "提交并比对", status: "pending" },
    { key: "proxy", label: "读取代理差异", status: "pending" },
  ]
  retryPayload.value = { mode: "sync" }
  progressPhase.value = "running"
  progressVisible.value = true
  const [scanItem, submitItem, proxyItem] = progressItems.value

  scanItem.status = "processing"
  let snapshot
  try {
    snapshot = await localAgent.profileScan()
    applyRunningSnapshot(snapshot)
    syncSnapshot.value = snapshot
    scanItem.status = "success"
  } catch (e) {
    scanItem.status = "failed"
    scanItem.reason = executionErrorMessage(e, "读取本机窗口失败")
    progressPhase.value = "done"
    return
  }

  submitItem.status = "processing"
  let scan
  try {
    scan = await bindingClient.submit(snapshot)
    syncScan.value = scan
    submitItem.status = "success"
  } catch (e) {
    submitItem.status = "failed"
    if (isIdentityMismatch(e)) {
      submitItem.reason = "当前比特主账号与系统绑定不一致，本次同步已停止。"
      progressPhase.value = "done"
      progressItems.value[2].status = "skipped"
      progressItems.value[2].reason = "同步已停止"
      identityDetectedMask.value = snapshot?.main_user_id || ""
      syncIdentityMismatch.value = true
      return
    }
    submitItem.reason = isNotBoundError(e) ? "当前账号尚未绑定比特主账号。" : (e.message || "提交本机窗口失败")
    progressPhase.value = "done"
    return
  }

  proxyItem.status = "processing"
  try {
    const preview = await proxyClient.previewLocalScan(scan.id)
    syncPreview.value = preview
    proxyItem.status = "success"
    syncSelectedAdded.value = []
    syncSelectedUnknown.value = []
  } catch (e) {
    proxyItem.status = "failed"
    proxyItem.reason = e.message || "读取代理差异失败"
  } finally {
    progressPhase.value = "done"
  }
}

function toggleSyncAdded(profile, checked) {
  if (profile.ownership_conflict) return
  syncSelectedAdded.value = checked
    ? [...new Set([...syncSelectedAdded.value, profile.bit_profile_id])]
    : syncSelectedAdded.value.filter((id) => id !== profile.bit_profile_id)
}

function toggleSyncUnknown(change, checked) {
  syncSelectedUnknown.value = checked
    ? [...new Set([...syncSelectedUnknown.value, change.profile_id])]
    : syncSelectedUnknown.value.filter((id) => id !== change.profile_id)
}

async function acceptSync() {
  if (!syncScan.value) return
  const added = syncSelectedAdded.value.length
  const proxyCount = syncActionableProxyChanges.value.length
  progressMode.value = "sync-accept"
  progressTitle.value = "接受本机变化"
  progressRunningLabel.value = "正在处理"
  progressLabels.value = {}
  progressRetryLabel.value = "重试未完成项"
  progressItems.value = [
    { key: "windows", label: `接受窗口变化（${added} 个新增窗口）`, status: "pending" },
    { key: "proxy", label: `核对代理关系（${proxyCount} 项）`, status: proxyCount ? "pending" : "skipped", reason: proxyCount ? "" : "没有需要处理的代理差异" },
  ]
  retryPayload.value = {
    mode: "sync-accept",
    appliedAdded: syncSelectedAdded.value.length,
    proxyCount,
  }
  progressPhase.value = "running"
  progressVisible.value = true
  const [windowItem, proxyItem] = progressItems.value
  syncError.value = ""
  try {
    windowItem.status = "processing"
    await bindingClient.confirm(syncScan.value.id, syncSelectedAdded.value)
    windowItem.status = "success"
  } catch (e) {
    windowItem.status = "failed"
    windowItem.reason = e.message || "接受窗口变化失败"
    progressPhase.value = "done"
    return
  }
  if (proxyCount) {
    proxyItem.status = "processing"
    const autoProxy = autoProxyChangeCount()
    try {
      const result = await proxyClient.confirmLocalScan(syncScan.value.id, syncSelectedUnknown.value)
      syncPreview.value = result
      proxyItem.status = "success"
      finalizeSyncResult(syncSelectedAdded.value.length, syncSelectedUnknown.value.length + autoProxy)
    } catch (e) {
      proxyItem.status = "failed"
      proxyItem.reason = e.message || "核对代理关系失败"
      syncError.value = "窗口变化已接受，但代理关系核对未完成。重新同步会以本机实际状态继续核对。"
      finalizeSyncResult(syncSelectedAdded.value.length, 0)
    }
  } else {
    finalizeSyncResult(syncSelectedAdded.value.length, 0)
  }
  progressPhase.value = "done"
  await reloadAll()
  reconcileSyncAdded()
}

function reconcileSyncAdded() {
  if (!syncResult.value) return
  syncResult.value.added = syncSelectedAdded.value
    .filter((bitProfileId) => profiles.value.some((profile) => profile.bit_profile_id === bitProfileId)).length
}

function autoProxyChangeCount() {
  return syncProxyChanges.value.filter((change) => ["changed", "unbound"].includes(change.kind)).length
}

function finalizeSyncResult(appliedAdded, appliedProxy) {
  const ownershipSkipped = syncProxyChanges.value
    .filter((change) => ["conflict", "ownership_conflict"].includes(change.kind)).length
  const unselectedAdded = Math.max(syncSelectableAdded.value.length - appliedAdded, 0)
  const unselectedUnknown = syncProxyChanges.value
    .filter((change) => ["unknown", "unregistered"].includes(change.kind) && !syncSelectedUnknown.value.includes(change.profile_id)).length
  syncDone.value = true
  syncResult.value = {
    total: syncSnapshot.value?.profiles?.length || 0,
    added: appliedAdded,
    updated: syncChangedProfiles.value.length,
    missing: syncMissingProfiles.value.length,
    proxy: appliedProxy,
    skipped: ownershipSkipped + unselectedAdded + unselectedUnknown,
  }
}

function closeSyncDrawer() {
  syncVisible.value = false
  syncScan.value = null
  syncPreview.value = null
  syncDone.value = false
  syncResult.value = null
  syncError.value = ""
  syncSelectedAdded.value = []
  syncSelectedUnknown.value = []
}

async function startBatchWindow(kind, targets) {
  if (!isOwnDesktop.value) return
  if (!targets.length) return
  const targetState = kind === "open"
  progressMode.value = "batch-window"
  progressTitle.value = kind === "open" ? "批量打开窗口" : "批量关闭窗口"
  progressRunningLabel.value = kind === "open" ? "正在打开" : "正在关闭"
  progressLabels.value = { success: kind === "open" ? "已打开" : "已关闭", failed: kind === "open" ? "打开失败" : "关闭失败", skipped: "跳过", unconfirmed: "待确认" }
  progressRetryLabel.value = "重试失败项"
  progressVerifyLabel.value = "读取并核对"
  progressItems.value = targets.map((profile) => ({
    key: profile.id,
    label: `${profile.name || profile.id}（${profile.bit_profile_id}）`,
    status: "pending",
  }))
  retryPayload.value = { mode: "batch-window", kind, profileIds: targets.map((profile) => profile.id) }
  progressPhase.value = "running"
  progressVisible.value = true

  const snapshot = await refreshRunningStates({ silent: false })
  if (!snapshot) {
    for (const item of progressItems.value) {
      item.status = "skipped"
      item.reason = "无法读取本机运行状态"
    }
    progressPhase.value = "done"
    return
  }

  const failures = []
  for (const item of progressItems.value) {
    const profile = targets.find((candidate) => candidate.id === item.key)
    const state = runningState(profile)
    if (profile.local_status !== "active") {
      item.status = "skipped"
      item.reason = "窗口缺失"
      continue
    }
    if (profile.business_status !== "enabled") {
      item.status = "skipped"
      item.reason = "窗口已停用"
      continue
    }
    if (state === undefined) {
      item.status = "skipped"
      item.reason = "运行状态未知"
      continue
    }
    if (state === targetState) {
      item.status = "skipped"
      item.reason = targetState ? "已经打开" : "已经关闭"
      continue
    }
    item.status = "processing"
    try {
      if (targetState) await localAgent.profileOpen(profile.bit_profile_id)
      else await localAgent.profileClose(profile.bit_profile_id)
      item.status = "success"
      runningStates.value = { ...runningStates.value, [profile.bit_profile_id]: targetState }
    } catch (e) {
      if (isTimeoutError(e)) {
        item.status = "unconfirmed"
        item.reason = "操作可能已生效，请读取本机运行状态核对。"
      } else {
        item.status = "failed"
        item.reason = executionErrorMessage(e, targetState ? "打开窗口失败" : "关闭窗口失败")
        failures.push(profile.id)
      }
    }
  }
  retryPayload.value.profileIds = failures
  progressPhase.value = "done"
}

function startBatchWindowFromSelection(kind) {
  const targets = selectedProfiles.value.filter((profile) => rowOperable(profile))
  if (!targets.length) {
    feedback.toast.info("所选窗口都不可执行，请重新选择。")
    return
  }
  startBatchWindow(kind, targets)
}

async function verifyBatchWindow() {
  const payload = retryPayload.value
  if (!payload || payload.mode !== "batch-window") return
  const snapshot = await refreshRunningStates({ silent: false })
  if (!snapshot) return
  const targetState = payload.kind === "open"
  for (const item of progressItems.value) {
    if (item.status !== "unconfirmed") continue
    const profile = profiles.value.find((candidate) => candidate.id === item.key)
    const state = profile ? runningState(profile) : undefined
    if (state === targetState) {
      item.status = "success"
      item.reason = "实际状态已与目标一致。"
    } else {
      item.status = "failed"
      item.reason = "实际状态仍与目标不一致，可重试。"
    }
  }
  payload.profileIds = progressItems.value.filter((item) => item.status === "failed").map((item) => item.key)
}

function openBatchProxy() {
  if (!isOwnDesktop.value) return
  if (!selectedProfiles.value.length) return
  const firstUsable = proxies.value.find((proxy) => proxySelectable(proxy, null))
  batchProxyRows.value = selectedProfiles.value.map((profile) => ({
    profile,
    targetId: firstUsable?.id || "",
  }))
  batchProxyVisible.value = true
}

async function startBatchProxy() {
  if (!batchProxyRows.value.length || batchProxyIssue.value) return
  const assignments = batchProxyRows.value.map((row) => ({ profile: row.profile, targetId: row.targetId }))
  batchProxyVisible.value = false
  progressMode.value = "batch-proxy"
  progressTitle.value = "批量设置代理"
  progressRunningLabel.value = "正在应用"
  progressLabels.value = { success: "已应用", failed: "失败", skipped: "跳过", unconfirmed: "待确认" }
  progressRetryLabel.value = "重试失败项"
  progressVerifyLabel.value = "读取并核对"
  progressItems.value = assignments.map((row) => ({
    key: row.profile.id,
    label: `${row.profile.name || row.profile.id} → ${proxyById.value[row.targetId]?.name || row.targetId}`,
    status: "pending",
  }))
  retryPayload.value = {
    mode: "batch-proxy",
    assignments: assignments.map((row) => ({ profileId: row.profile.id, targetId: row.targetId })),
  }
  progressPhase.value = "running"
  progressVisible.value = true

  const failures = []
  for (const item of progressItems.value) {
    const assignment = assignments.find((row) => row.profile.id === item.key)
    const profile = assignment.profile
    const target = proxyById.value[assignment.targetId]
    if (profile.local_status !== "active") {
      item.status = "skipped"
      item.reason = "窗口缺失"
      continue
    }
    if (!target) {
      item.status = "failed"
      item.reason = "目标代理不存在"
      failures.push(profile.id)
      continue
    }
    if (profile.proxy_id !== target.id && !proxySelectable(target, null)) {
      item.status = "skipped"
      item.reason = proxyAvailabilityReason(target, null)
      continue
    }
    item.status = "processing"
    try {
      await proxyClient.assign(target.id, profile.id)
      item.status = "success"
    } catch (e) {
      if (isUncertainProxyError(e)) {
        item.status = "unconfirmed"
        item.reason = "写入已发出但未能核实，请读取实际配置核对。"
      } else {
        item.status = "failed"
        item.reason = e.message || "应用代理失败"
        failures.push(profile.id)
      }
    }
  }
  retryPayload.value.failedIds = failures
  progressPhase.value = "done"
  await reloadAll()
}

async function verifyBatchProxy() {
  const payload = retryPayload.value
  if (!payload || payload.mode !== "batch-proxy") return
  let snapshot
  try {
    snapshot = await localAgent.profileScan()
  } catch (e) {
    feedback.toast.error(executionErrorMessage(e, "读取比特实际配置失败"))
    return
  }
  for (const item of progressItems.value) {
    if (item.status !== "unconfirmed") continue
    const assignment = payload.assignments.find((row) => row.profileId === item.key)
    const target = assignment ? proxyById.value[assignment.targetId] : null
    const profile = profiles.value.find((candidate) => candidate.id === item.key)
    if (!assignment || !target || !profile) continue
    const observed = (snapshot?.profiles || []).find((candidate) => candidate.bit_profile_id === profile.bit_profile_id)
    if (classifyObservedAssignment(observed, target) === "different") {
      item.status = "failed"
      item.reason = "实际配置与目标不一致，可重试。"
      continue
    }
    item.reason = observed ? "协议与地址已匹配，认证信息无法核对；请同步窗口并检查代理后再处理。" : "未读取到窗口，请同步窗口后核对。"
  }
  payload.failedIds = progressItems.value.filter((item) => item.status === "failed").map((item) => item.key)
  await reloadAll()
}

function retryFailed() {
  const payload = retryPayload.value
  if (!payload) return
  if (payload.mode === "create") {
    if (progressItems.value[1]?.status === "failed" && payload.bitProfileId) {
      retryCreateCloud()
      return
    }
    progressVisible.value = false
    submitCreate()
    return
  }
  if (payload.mode === "edit") {
    if (progressItems.value[1]?.status === "failed") {
      retryEditCloud()
      return
    }
    runEditWrite(false)
    return
  }
  if (payload.mode === "sync") {
    runSync()
    return
  }
  if (payload.mode === "sync-accept") {
    if (progressItems.value[0]?.status === "success") retryProxyStage()
    else runSync()
    return
  }
  if (payload.mode === "batch-window") {
    const targets = profiles.value.filter((profile) => payload.profileIds.includes(profile.id))
    if (!targets.length) {
      progressVisible.value = false
      return
    }
    startBatchWindow(payload.kind, targets)
    return
  }
  if (payload.mode === "batch-proxy") {
    const failedIds = payload.failedIds || []
    const assignments = payload.assignments
      .filter((row) => failedIds.includes(row.profileId))
      .map((row) => {
        const profile = profiles.value.find((candidate) => candidate.id === row.profileId)
        return profile ? { profile, targetId: row.targetId } : null
      })
      .filter(Boolean)
    if (!assignments.length) {
      progressVisible.value = false
      return
    }
    batchProxyRows.value = assignments
    startBatchProxy()
    return
  }
  if (payload.mode === "proxy-action") {
    const profile = profiles.value.find((item) => item.id === payload.profileId)
    const target = proxyById.value[payload.targetProxyId]
    if (!profile) {
      progressVisible.value = false
      return
    }
    runProxyAction(payload.kind, profile, target)
  }
}

async function retryProxyStage() {
  const payload = retryPayload.value
  if (!payload || !syncScan.value) return
  const planned = syncSelectedUnknown.value.length + autoProxyChangeCount()
  progressMode.value = "sync-accept"
  progressTitle.value = "核对代理关系"
  progressRunningLabel.value = "正在核对"
  progressLabels.value = {}
  progressRetryLabel.value = "重试未完成项"
  progressPhase.value = "running"
  progressItems.value = [
    { key: "windows", label: "接受窗口变化", status: "skipped", reason: "窗口变化已接受" },
    { key: "proxy", label: `核对代理关系（${planned} 项）`, status: "processing" },
  ]
  progressVisible.value = true
  try {
    const result = await proxyClient.confirmLocalScan(syncScan.value.id, syncSelectedUnknown.value)
    syncPreview.value = result
    progressItems.value[1].status = "success"
    syncError.value = ""
    finalizeSyncResult(payload.appliedAdded, planned)
  } catch (e) {
    progressItems.value[1].status = "failed"
    progressItems.value[1].reason = e.message || "核对代理关系失败"
    syncError.value = "窗口变化已接受，但代理关系核对未完成。重新同步会以本机实际状态继续核对。"
    finalizeSyncResult(payload.appliedAdded, 0)
  } finally {
    progressPhase.value = "done"
    await reloadAll()
    reconcileSyncAdded()
  }
}

async function retryEditCloud() {
  const pending = editPending.value
  if (!pending) return
  editResultVisible.value = false
  progressMode.value = "edit"
  progressTitle.value = "同步编辑结果"
  progressPhase.value = "running"
  progressRunningLabel.value = "正在同步"
  progressItems.value = [
    { key: "write", label: "写入比特窗口并读回核实", status: "skipped", reason: "写入已完成" },
    { key: "cloud", label: "同步 Cloud 记录", status: "processing" },
  ]
  progressVisible.value = true
  try {
    const snapshot = pending.snapshot || await localAgent.profileScan()
    await syncObservedSnapshot(snapshot)
    progressItems.value[1].status = "success"
    await loadProfiles()
  } catch (e) {
    progressItems.value[1].status = "failed"
    progressItems.value[1].reason = e.message || "Cloud 记录同步失败"
  } finally {
    progressPhase.value = "done"
  }
}

function applyEditObservation(pending, snapshot, fields) {
  const observed = (snapshot?.profiles || []).find((profile) => profile.bit_profile_id === pending.profile.bit_profile_id)
  if (observed) {
    pending.observed = {
      name: observed.name ?? "",
      group_id: observed.group_id ?? "",
      group_name: observed.group_name ?? "",
      remark: observed.remark ?? "",
    }
  }
  pending.fields = Array.isArray(fields) ? fields : []
  pending.verified = pending.fields.filter((field) => field.status === "verified").map((field) => field.field)
}

function verifyPending() {
  const payload = retryPayload.value
  if (!payload) return
  if (payload.mode === "create") {
    progressVisible.value = false
    createVisible.value = false
    feedback.notice({
      title: "核实创建结果",
      message: "接下来会读取本机窗口并比对。若窗口已创建，它会在「新增窗口」中列出，勾选后才建立 Cloud 关联；不要重复创建。",
    }).then(() => runSync())
    return
  }
  if (payload.mode === "edit") {
    verifyAndRetryEdit()
    return
  }
  if (payload.mode === "proxy-action") {
    verifyProxyAction()
    return
  }
  if (payload.mode === "batch-window") {
    verifyBatchWindow()
    return
  }
  if (payload.mode === "batch-proxy") {
    verifyBatchProxy()
  }
}

async function closeProgress() {
  progressVisible.value = false
  const mode = progressMode.value
  const items = progressItems.value
  const success = items.length > 0 && items.every((item) => ["success", "skipped"].includes(item.status))
  if (mode === "create") {
    if (success && createdProfile.value) {
      createVisible.value = false
      createdDialogVisible.value = true
    } else if (items[0]?.status === "failed") {
      createError.value = items[0].reason || "创建比特窗口失败"
    } else if (items[0]?.status === "unconfirmed") {
      createUncertain.value = true
      createError.value = items[0].reason || "创建结果待核实，请先同步窗口确认。"
    } else if (items[1]?.status === "failed" || items[1]?.status === "unconfirmed") {
      createCloudRetry.value = true
      createError.value = items[1].reason || "窗口已在比特创建并核实，但 Cloud 记录未同步。可重试同步。"
    }
  }
  if (mode === "edit") {
    const partial = items[0]?.status === "unconfirmed"
    const cloudFailed = items[1]?.status === "failed"
    if (cloudFailed) {
      editResultKind.value = "cloud_failed"
      editResultMessage.value = items[1].reason || "比特字段已处理，但 Cloud 记录同步失败。"
      editResultRows.value = editFieldRows(editPending.value)
      editResultVisible.value = true
    } else if (partial) {
      editResultKind.value = "uncertain"
      editResultMessage.value = items[0]?.reason || "部分字段未能核实。请先读取实际值，再确认未生效字段。"
      editResultRows.value = editFieldRows(editPending.value)
      editResultVisible.value = true
    } else if (success) {
      editVisible.value = false
      feedback.toast.success("窗口信息已更新并同步")
      reloadAll()
    }
  }
  if (mode === "sync") {
    if (items.every((item) => item.status === "success")) {
      syncVisible.value = true
      syncDone.value = false
      syncResult.value = null
    } else if (syncIdentityMismatch.value) {
      identityVisible.value = true
    }
  }
  if (mode === "sync-accept") {
    if (items[0]?.status === "success") syncVisible.value = true
  }
  if (mode === "proxy-action" && items.some((item) => item.status === "success")) {
    reloadAll()
  }
  progressMode.value = ""
  retryPayload.value = null
}

function changedFieldsText(bitProfileId) {
  const diff = (syncScan.value?.diff || []).find((item) => item.kind === "changed" && item.bit_profile_id === bitProfileId)
  if (!diff || !(diff.fields || []).length) return "-"
  const labels = {
    main_user_id: "比特身份",
    profile_user_id: "环境 ID",
    name: "窗口名称",
    seq: "排序",
    group_id: "分组",
    group_name: "分组",
    remark: "比特备注",
    proxy_type: "代理协议",
    proxy_host: "代理地址",
    proxy_port: "代理端口",
  }
  return [...new Set((diff.fields || []).map((field) => labels[field] || field))].join("、") || "-"
}

function proxyChangeLabel(kind) {
  return {
    changed: "匹配到唯一代理",
    unbound: "已无代理",
    unknown: "新发现代理",
    unregistered: "未登记",
    conflict: "匹配到多个代理",
    ownership_conflict: "归属冲突",
  }[kind] || kind
}

function proxyChangeTone(kind) {
  if (kind === "changed") return "success"
  if (kind === "unbound" || kind === "unknown") return "warning"
  return "neutral"
}

function openProxyList() {
  router.push("/proxies")
}

function syncIssueAction(profile) {
  closeSyncDrawer()
  openRowProxyAction(profile)
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <div class="wt-resource-page resource-visual-approved">
      <ResourcePageHeader title="浏览器窗口" :description="pageDescription">
        <template v-if="isOwnDesktop" #actions>
          <t-button class="wt-secondary-button" variant="outline" @click="runSync">同步窗口</t-button>
          <t-button theme="primary" @click="openCreate">新增窗口</t-button>
        </template>
      </ResourcePageHeader>

      <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
      <FeedbackAlert
        v-if="bindingMissing"
        theme="warning"
        style="margin-bottom:16px"
        message="当前账号还没有绑定比特浏览器主账号，尚未建立窗口范围。请先在「个人信息」完成绑定，再同步窗口。"
      />

      <ResourceStatGrid :items="statItems" interactive :active-key="statusFilter" @select="toggleStatFilter" />

      <ResourceCard class="window-resource-card">
        <div class="filter-bar">
          <label class="wt-filter-field wt-filter-field--wide">
            <span class="wt-filter-field__label">搜索</span>
            <t-input v-model="filterDraft.search" clearable placeholder="名称 / ID / 比特窗口ID / 备注" @enter="submitSearch" />
          </label>
          <label v-if="isSenior" class="wt-filter-field">
            <span class="wt-filter-field__label">授权运营</span>
            <t-select v-model="memberFilter" :options="memberOptions" placeholder="全部" :clearable="false" @change="onSeniorMemberChange" />
          </label>
          <template v-if="isAdmin">
            <label class="wt-filter-field">
              <span class="wt-filter-field__label">运营团队</span>
              <t-select v-model="draftTeam" :options="teamOptions" clearable placeholder="全部" @change="onAdminTeamChange" />
            </label>
            <label class="wt-filter-field">
              <span class="wt-filter-field__label">授权运营</span>
              <t-select v-model="draftMember" :options="adminMemberOptions" clearable placeholder="全部" filterable @change="onAdminMemberChange" />
            </label>
          </template>
          <label class="wt-filter-field">
            <span class="wt-filter-field__label">代理方式</span>
            <t-select v-model="filterDraft.mode" clearable placeholder="全部">
              <t-option value="managed" label="系统托管" />
              <t-option value="self" label="比特自管" />
            </t-select>
          </label>
          <label class="wt-filter-field">
            <span class="wt-filter-field__label">业务状态</span>
            <t-select v-model="filterDraft.business" clearable placeholder="全部">
              <t-option value="enabled" label="启用" />
              <t-option value="disabled" label="停用" />
            </t-select>
          </label>
          <label v-if="isOwnDesktop" class="wt-filter-field">
            <span class="wt-filter-field__label">运行状态</span>
            <t-select v-model="filterDraft.running" clearable placeholder="全部">
              <t-option value="open" label="已打开" />
              <t-option value="closed" label="已关闭" />
              <t-option value="unknown" label="未知" />
            </t-select>
          </label>
          <div class="wt-filter-actions">
            <t-button theme="primary" @click="submitSearch">查询</t-button>
            <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
          </div>
        </div>

        <ResourceSelectionToolbar v-if="isOwnDesktop" :count="selectedRowKeys.length" unit="个窗口" :hint="batchHint" @clear="selectedRowKeys = []">
          <t-button theme="default" variant="outline" @click="openBatchProxy">批量设置代理</t-button>
          <t-button theme="default" variant="outline" :disabled="!batchOpenReady" @click="startBatchWindowFromSelection('open')">批量打开</t-button>
          <t-button theme="default" variant="outline" :disabled="!batchCloseReady" @click="startBatchWindowFromSelection('close')">批量关闭</t-button>
        </ResourceSelectionToolbar>

        <div class="table-scroll-wrap">
          <t-table
            class="wt-resource-table"
            :data="pagedProfiles"
            :columns="columns"
            row-key="id"
            size="small"
            hover
            v-model:selected-row-keys="selectedRowKeys"
            :scroll="tableScroll"
            empty="暂无符合条件的数据"
          >
            <template #id="{ row }"><span class="window-id">{{ row.id }}</span></template>
            <template #name="{ row }">
              <div class="window-name" :title="row.name || ''">{{ row.name || "-" }}</div>
            </template>
            <template #group_owner="{ row }">
              <span>{{ groupOwnerText(row) }}</span>
            </template>
            <template #team="{ row }"><span>{{ teamName(row) }}</span></template>
            <template #proxy_mode="{ row }">
              <ResourceStatusBadge :tone="isManaged(row) ? 'info' : 'neutral'" :label="proxyModeLabel(row)" />
            </template>
            <template #proxy_info="{ row }">
              <div class="window-address" :title="actualProxyText(row)">{{ row.proxy_host ? `${row.proxy_host}:${row.proxy_port || '—'}` : (actualProxyText(row) || (isManaged(row) ? "配置未核实" : "—")) }}</div>
              <div class="window-sub">{{ proxySubText(row) }}</div>
              <div v-if="anomalyReasons(row).length" class="window-issues" :title="anomalyReasons(row).join('、')">
                <span v-for="reason in anomalyReasons(row).slice(0, 2)" :key="reason" class="window-issue">{{ reason }}</span>
                <span v-if="anomalyReasons(row).length > 2" class="window-issue">另 {{ anomalyReasons(row).length - 2 }} 项</span>
              </div>
            </template>
            <template #accounts="{ row }">
              <div v-if="!accountsFor(row).length" class="window-sub">暂无关联账号</div>
              <template v-else>
                <div v-for="line in accountLines(row)" :key="line" class="window-account">{{ line }}</div>
                <div v-if="accountsFor(row).length > 2" class="window-sub">另 {{ accountsFor(row).length - 2 }} 个</div>
              </template>
            </template>
            <template #cloud_remark="{ row }">
              <span class="window-remark" :title="row.cloud_remark || ''">{{ row.cloud_remark || "-" }}</span>
            </template>
            <template #business_status="{ row }">
              <t-switch
                v-if="isOwnDesktop"
                :value="row.business_status === 'enabled'"
                :disabled="row.local_status !== 'active'"
                size="small"
                @change="(value) => requestBusinessChange(row, value)"
              />
              <ResourceStatusBadge
                v-else
                :tone="row.business_status === 'enabled' ? 'success' : 'neutral'"
                :label="row.business_status === 'enabled' ? '启用' : '停用'"
              />
            </template>
            <template #running="{ row }">
              <span class="running-state">
                <span class="running-dot" :class="{ 'is-open': runningState(row) === true }"></span>{{ runningLabel(row) }}
              </span>
            </template>
            <template #last_sync="{ row }"><span class="window-time">{{ formatTime(row.last_synced_at) }}</span></template>
            <template #op="{ row }">
              <ResourceRowActions>
                <t-button v-if="isOwnDesktop" class="is-primary"
                  size="small"
                  theme="primary"
                  variant="text"
                  :loading="operatingIds.includes(row.id)"
                  :disabled="rowOperable(row) === false"
                  @click="toggleRunning(row)"
                >{{ runningState(row) === undefined ? "刷新状态" : (runningState(row) ? "关闭" : "打开") }}</t-button>
                <t-button size="small" theme="primary" variant="text" @click="openDetail(row)">详情</t-button>
                <template v-if="isOwnDesktop">
                  <t-button
                    size="small"
                    theme="primary"
                    variant="text"
                    :disabled="!rowOperable(row)"
                    @click="openEdit(row)"
                  >编辑</t-button>
                  <t-button size="small" theme="primary" variant="text" @click="openRowProxyAction(row)">{{ hasRowIssue(row) ? "处理异常" : "管理代理" }}</t-button>
                </template>
                <t-button size="small" theme="primary" variant="text" @click="openNote(row)">运营备注</t-button>
              </ResourceRowActions>
            </template>
          </t-table>
        </div>
        <div class="pagination-bar">
          <t-pagination
            v-model:current="page"
            v-model:pageSize="pageSize"
            :total="filteredProfiles.length"
            :page-size-options="[10, 20, 50, 100]"
            show-jumper
          />
        </div>
      </ResourceCard>

      <ResourceOverlay v-model:visible="detailVisible" title="窗口详情" :footer="false" kind="drawer" :width="800">
        <template v-if="detailProfile">
          <FeedbackAlert
            v-if="detailProfile.local_status === 'local_missing'"
            theme="warning"
            style="margin-bottom:14px"
            message="该窗口在最近一次同步中未找到。系统记录、关联账号和历史都会保留；请在比特中确认窗口是否仍存在，恢复后重新同步。"
          />
          <ResourceSection title="窗口信息">
          <ResourceDescriptions size="small">
            <t-descriptions-item label="ID">{{ detailProfile.id }}</t-descriptions-item>
            <t-descriptions-item label="窗口名称">{{ detailProfile.name || "-" }}</t-descriptions-item>
            <t-descriptions-item label="比特窗口 ID">{{ detailProfile.bit_profile_id || "-" }}</t-descriptions-item>
            <t-descriptions-item label="比特分组">{{ detailProfile.group_name || detailProfile.group_id || "-" }}</t-descriptions-item>
            <t-descriptions-item label="比特备注">{{ detailProfile.remark || "-" }}</t-descriptions-item>
            <t-descriptions-item label="窗口状态">{{ detailProfile.local_status === "local_missing" ? "窗口缺失" : "在册" }}</t-descriptions-item>
            <t-descriptions-item v-if="isOwnDesktop" label="运行状态">{{ runningLabel(detailProfile) }}</t-descriptions-item>
            <t-descriptions-item label="最近同步">{{ formatTime(detailProfile.last_synced_at) }}</t-descriptions-item>
          </ResourceDescriptions>

          </ResourceSection>
          <ResourceSection title="代理">
          <ResourceDescriptions size="small">
            <t-descriptions-item label="代理方式">
              <ResourceStatusBadge :tone="isManaged(detailProfile) ? 'info' : 'neutral'" :label="proxyModeLabel(detailProfile)" />
            </t-descriptions-item>
            <t-descriptions-item label="当前代理">{{ actualProxyText(detailProfile) || (isManaged(detailProfile) ? "尚未核实" : "无代理 · 直连") }}</t-descriptions-item>
            <t-descriptions-item label="托管代理">
              <template v-if="managedProxy(detailProfile)">
                {{ managedProxy(detailProfile).name || "-" }}（{{ managedProxy(detailProfile).host }}:{{ managedProxy(detailProfile).port }}）
              </template>
              <span v-else>—</span>
            </t-descriptions-item>
            <t-descriptions-item label="配置结果">
              <template v-if="!isManaged(detailProfile)">由比特维护</template>
              <ResourceStatusBadge
                v-else-if="!managedProxy(detailProfile)"
                tone="danger"
                label="代理资源不可用"
              />
              <ResourceStatusBadge
                v-else
                :tone="proxyConfigMismatch(detailProfile, managedProxy(detailProfile)) ? 'warning' : 'success'"
                :label="proxyConfigMismatch(detailProfile, managedProxy(detailProfile)) ? '配置不一致' : '协议与地址记录一致'"
              />
            </t-descriptions-item>
            <t-descriptions-item v-if="isManaged(detailProfile) && managedProxy(detailProfile)" label="认证信息">未单独核对</t-descriptions-item>
            <t-descriptions-item label="最近窗口同步">{{ formatTime(detailProfile.last_synced_at) }}</t-descriptions-item>
          </ResourceDescriptions>

          <template v-if="isManaged(detailProfile) && managedProxy(detailProfile) && proxyConfigMismatch(detailProfile, managedProxy(detailProfile))">
            <div class="config-compare">
              <div class="config-card">
                <div class="config-card__title">托管配置</div>
                <div class="config-row"><span>协议</span><strong>{{ managedProxy(detailProfile).proxy_protocol || "-" }}</strong></div>
                <div class="config-row"><span>地址</span><strong>{{ managedProxy(detailProfile).host || "-" }}</strong></div>
                <div class="config-row"><span>端口</span><strong>{{ managedProxy(detailProfile).port || "-" }}</strong></div>
              </div>
              <div class="config-card config-card--mismatch">
                <div class="config-card__title">比特当前配置</div>
                <div class="config-row">
                  <span>协议</span>
                  <strong>{{ detailProfile.proxy_type || "-" }}<em v-if="String(detailProfile.proxy_type || '') !== String(managedProxy(detailProfile).proxy_protocol || '')">不同</em></strong>
                </div>
                <div class="config-row">
                  <span>地址</span>
                  <strong>{{ detailProfile.proxy_host || "-" }}<em v-if="String(detailProfile.proxy_host || '') !== String(managedProxy(detailProfile).host || '')">不同</em></strong>
                </div>
                <div class="config-row">
                  <span>端口</span>
                  <strong>{{ detailProfile.proxy_port || "-" }}<em v-if="Number(detailProfile.proxy_port || 0) !== Number(managedProxy(detailProfile).port || 0)">不同</em></strong>
                </div>
              </div>
            </div>
          </template>

          </ResourceSection>
          <ResourceSection :title="`关联账号 · ${accountsFor(detailProfile).length} 个`">
          <t-table
            :data="accountsFor(detailProfile)"
            row-key="id"
            size="small"
            :columns="[
              { colKey: 'id', title: 'ID', width: 80 },
              { colKey: 'platform', title: '平台', width: 90 },
              { colKey: 'name', title: '账号名称' },
              { colKey: 'login_status', title: '登录状态', width: 110 },
            ]"
            empty="暂无关联账号"
          >
            <template #platform="{ row }">{{ platformLabel(row.platform) }}</template>
            <template #name="{ row }">{{ row.name || row.platform_account_id || "未识别" }}</template>
            <template #login_status="{ row }"><ResourceStatusBadge :tone="row.login_status === 'normal' ? 'success' : 'warning'" :label="loginStatusText(row.login_status)" /></template>
          </t-table>

          </ResourceSection>
          <ResourceSection title="运营信息">
          <ResourceDescriptions size="small">
            <t-descriptions-item label="运营团队">{{ teamName(detailProfile) }}</t-descriptions-item>
            <t-descriptions-item label="授权运营">{{ ownerLabel(detailProfile.user_id) }}</t-descriptions-item>
            <t-descriptions-item label="业务启停">{{ detailProfile.business_status === "enabled" ? "启用" : "停用" }}</t-descriptions-item>
            <t-descriptions-item label="运营备注">{{ detailProfile.cloud_remark || "-" }}</t-descriptions-item>
          </ResourceDescriptions>
          </ResourceSection>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="noteVisible" title="运营备注" :width="560" :busy="noteSaving">
        <FeedbackAlert v-if="noteError" :message="noteError" theme="error" style="margin-bottom:12px" />
        <div class="window-note-context">
          <ResourceDescriptions size="small">
            <t-descriptions-item label="窗口">{{ noteProfile?.name || "-" }}</t-descriptions-item>
            <t-descriptions-item label="窗口 ID">{{ noteProfile?.id || "-" }}</t-descriptions-item>
          </ResourceDescriptions>
        </div>
        <ResourceForm class="window-note-form">
          <t-form-item label="运营备注">
            <t-textarea v-model="noteText" :autosize="{ minRows: 4, maxRows: 8 }" placeholder="填写运营备注" />
            <p class="note-help">供授权成员协作，不修改比特备注。</p>
          </t-form-item>
        </ResourceForm>
        <template #footer><t-button theme="default" variant="outline" :disabled="noteSaving" @click="noteVisible = false">取消</t-button><t-button theme="primary" :loading="noteSaving" @click="saveNote">保存</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="businessConfirmVisible" :title="businessNextEnabled ? '启用窗口' : '停用窗口'" :width="560" :busy="businessSaving">
        <ResourceDescriptions size="small">
          <t-descriptions-item label="窗口">{{ businessTarget?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="窗口 ID">{{ businessTarget?.id || "-" }}</t-descriptions-item>
          <t-descriptions-item label="业务启停">{{ businessNextEnabled ? "启用" : "停用" }}</t-descriptions-item>
        </ResourceDescriptions>
        <p class="form-hint">
          {{ businessNextEnabled
            ? "启用后窗口可用于后续业务操作，不会自动打开窗口，也不会改变代理关系。"
            : "停用不会关闭窗口、删除账号或清除代理关系；请确认窗口已关闭后再停用。" }}
        </p>
              <template #footer><t-button theme="default" variant="outline" :disabled="businessSaving" @click="businessConfirmVisible = false">取消</t-button><t-button theme="primary" :loading="businessSaving" @click="confirmBusinessChange">{{ businessNextEnabled ? '确认启用' : '确认停用' }}</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay
        v-model:visible="createVisible"
        title="新增窗口"
        :footer="true"
        kind="dialog" :width="560" :busy="progressVisible">
        <FeedbackAlert v-if="createError" :message="createError" theme="error" style="margin-bottom:12px" />
        <ResourceForm>
          <t-form-item label="窗口名称" required>
            <t-input v-model="createForm.name" placeholder="例如 B站主号 · 上海05" />
          </t-form-item>
          <t-form-item label="比特分组" required>
            <t-select v-model="createForm.group_id" :options="groupOptions(createGroups, createForm)" :loading="createGroupLoading" placeholder="请选择分组" @change="(value) => applyGroup(createForm, createGroups, value)" />
            <ResourceNotice v-if="!createGroupLoading && !createGroups.length">请先在比特中创建分组，再重新打开。</ResourceNotice>
          </t-form-item>
          <t-form-item label="比特备注"><t-textarea v-model="createForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" placeholder="仅写入比特窗口备注" /></t-form-item>
          <t-form-item label="代理方式">
            <ResourceChoiceCards v-model="createForm.proxy_intent" :options="[
              { value: 'self', label: '比特自管', description: '代理配置由比特维护' },
              { value: 'managed', label: '系统托管', description: '创建后选择托管代理' },
            ]" />
          </t-form-item>
        </ResourceForm>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" :disabled="progressVisible" @click="createVisible = false">{{ createCloudRetry ? "稍后处理" : "取消" }}</t-button>
            <t-button v-if="createUncertain" theme="primary" @click="createVisible = false; runSync()">同步窗口核实</t-button>
            <t-button v-else-if="createCloudRetry" theme="primary" :loading="progressVisible" @click="retryCreateCloud">重试同步记录</t-button>
            <t-button v-else theme="primary" :loading="progressVisible" @click="submitCreate">保存</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="createdDialogVisible" title="窗口已创建" :footer="true" :width="560">
        <div class="create-success">
          <div class="create-success__icon">✓</div>
          <ResourceDescriptions size="small">
            <t-descriptions-item label="窗口名称">{{ createdProfile?.name || createForm.name }}</t-descriptions-item>
            <t-descriptions-item label="ID">{{ createdProfile?.id || "-" }}</t-descriptions-item>
            <t-descriptions-item label="代理方式">{{ createdMode === "managed" ? "系统托管（待选择代理）" : "比特自管" }}</t-descriptions-item>
          </ResourceDescriptions>
          <ResourceNotice v-if="createdMode === 'managed'">选择代理后开始托管。</ResourceNotice>
        </div>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" @click="createdDialogVisible = false">完成</t-button>
            <t-button theme="primary" @click="openCreatedProxy">{{ createdMode === "managed" ? "选择代理" : "配置代理" }}</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay
        v-model:visible="editVisible"
        title="编辑窗口"
        :footer="true"
        kind="dialog" :width="560" :busy="progressVisible">
        <FeedbackAlert v-if="editError" :message="editError" theme="error" style="margin-bottom:12px" />
        <ResourceForm>
          <t-form-item label="窗口名称" required><t-input v-model="editForm.name" /></t-form-item>
          <t-form-item label="比特分组" required>
            <t-select v-model="editForm.group_id" :options="groupOptions(editGroups, editProfile)" :loading="editGroupLoading" @change="(value) => applyGroup(editForm, editGroups, value)" />
          </t-form-item>
          <t-form-item label="比特备注"><t-textarea v-model="editForm.remark" :autosize="{ minRows: 2, maxRows: 4 }" /></t-form-item>
        </ResourceForm>
        <ResourceNotice>保存后同步到比特；运营备注保持不变。</ResourceNotice>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" :disabled="progressVisible" @click="editVisible = false">取消</t-button>
            <t-button theme="primary" :loading="progressVisible" @click="submitEdit">保存</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="editResultVisible" :title="editResultKind === 'cloud_failed' ? '系统记录同步失败' : '窗口修改结果'" :footer="true" :width="800">
        <FeedbackAlert :message="editResultMessage" :theme="editResultKind === 'resolved' ? 'success' : 'warning'" style="margin-bottom:12px" />
        <t-table :data="editResultRows" row-key="field" size="small" :columns="[
          { colKey: 'field', title: '目标字段', width: 120 },
          { colKey: 'target', title: '目标值' },
          { colKey: 'actual', title: '实际结果' },
          { colKey: 'status', title: '状态', width: 110 },
        ]">
          <template #status="{ row }"><ResourceStatusBadge :tone="row.tone" :label="row.status" /></template>
        </t-table>
        <p class="form-hint">已核实一致的字段不会再次修改；未完成项会先读取实际值，再决定是否重试。</p>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" @click="closeEditResult">{{ editResultKind === 'resolved' ? '完成' : '稍后处理' }}</t-button>
            <t-button v-if="editResultKind === 'cloud_failed'" theme="primary" @click="retryEditCloud">重试同步</t-button>
            <t-button v-else-if="editResultKind !== 'resolved'" theme="primary" @click="verifyAndRetryEdit">核实并重试</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="manageVisible" title="管理代理" :footer="false" :width="640">
        <FeedbackAlert v-if="manageNote" :message="manageNote" theme="warning" style="margin-bottom:12px" />
        <ResourceDescriptions size="small">
          <t-descriptions-item label="窗口">{{ manageProfile?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="窗口 ID">{{ manageProfile?.id || "-" }}</t-descriptions-item>
          <t-descriptions-item label="管理方式">{{ proxyModeLabel(manageProfile || {}) }}</t-descriptions-item>
          <t-descriptions-item label="当前地址">{{ actualProxyText(manageProfile || {}) || "—" }}</t-descriptions-item>
        </ResourceDescriptions>
        <ResourceNotice v-if="accountsFor(manageProfile || {}).length" :details="accountTitle(manageProfile)">涉及 {{ accountsFor(manageProfile).length }} 个关联账号</ResourceNotice>
        <div class="manage-section">
          <div>
            <div class="manage-section__title">使用其他托管代理</div>
            <div class="manage-section__desc">选择可用线路，应用到此窗口。</div>
          </div>
          <t-button theme="primary" @click="closeManageToCandidates">{{ isManaged(manageProfile || {}) ? "更换代理" : "选择代理" }}</t-button>
        </div>
        <div v-if="isManaged(manageProfile || {})" class="manage-section">
          <div>
            <div class="manage-section__title">保留当前代理</div>
            <div class="manage-section__desc">由比特维护，不再占用系统额度。</div>
          </div>
          <t-button class="wt-secondary-button" variant="outline" @click="openReleaseConfirm(manageProfile)">改为比特自管</t-button>
        </div>
        <div v-if="actualProxyText(manageProfile || {})" class="manage-section">
          <div>
            <div class="manage-section__title">不再使用代理</div>
            <div class="manage-section__desc">清除代理，改用本机网络。</div>
          </div>
          <t-button class="wt-danger-button" theme="danger" variant="text" @click="openUnbindConfirm(manageProfile)">解绑并改为直连</t-button>
        </div>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="missingVisible" title="核实窗口缺失" :footer="true" kind="drawer" :width="640">
        <ResourceNotice tone="warning">同步未找到此窗口，请先在比特中确认是否仍然存在。</ResourceNotice>
        <h3 class="window-section">窗口信息</h3>
        <ResourceDescriptions bordered size="small">
          <t-descriptions-item label="窗口 ID">{{ missingProfile?.id || "-" }}</t-descriptions-item>
          <t-descriptions-item label="窗口名称">{{ missingProfile?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="托管代理">{{ managedProxy(missingProfile || {})?.name || "—" }}</t-descriptions-item>
          <t-descriptions-item label="可执行状态"><ResourceStatusBadge tone="warning" label="暂不可操作" /></t-descriptions-item>
        </ResourceDescriptions>
        <h3 class="window-section">受影响账号（{{ accountsFor(missingProfile || {}).length }}）</h3>
        <t-table :data="accountsFor(missingProfile || {})" row-key="id" size="small" :columns="[
          { colKey: 'platform', title: '平台', width: 90 },
          { colKey: 'name', title: '账号名称' },
          { colKey: 'login_status', title: '登录状态', width: 110 },
        ]" empty="暂无关联账号">
          <template #platform="{ row }">{{ platformLabel(row.platform) }}</template>
          <template #name="{ row }">{{ row.name || row.platform_account_id || "未识别" }}</template>
          <template #login_status="{ row }"><ResourceStatusBadge :tone="row.login_status === 'normal' ? 'success' : 'warning'" :label="loginStatusText(row.login_status)" /></template>
        </t-table>
        <h3 class="window-section">重要提示</h3>
        <ol class="window-tips">
          <li>请确认已登录正确的比特账号。</li>
          <li>在比特中查找此窗口，确认是否被删除或调整了权限。</li>
          <li>恢复后重新同步；账号和历史记录会保留，不会自动补建重复窗口。</li>
        </ol>
        <template #footer>
          <t-button theme="primary" @click="missingVisible = false; runSync()">重新同步</t-button>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="mismatchVisible" title="处理代理不一致" :footer="true" kind="drawer" :width="800">
        <ResourceNotice tone="warning">比特配置与托管配置不同，请选择下方处理方式。</ResourceNotice>
        <ResourceDescriptions size="small">
          <t-descriptions-item label="窗口">{{ mismatchProfile?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="窗口 ID">{{ mismatchProfile?.id || "-" }}</t-descriptions-item>
        </ResourceDescriptions>
        <div class="config-compare">
          <div class="config-card">
            <div class="config-card__title">托管配置</div>
            <div class="config-row"><span>协议</span><strong>{{ managedProxy(mismatchProfile || {})?.proxy_protocol || "-" }}</strong></div>
            <div class="config-row"><span>地址</span><strong>{{ managedProxy(mismatchProfile || {})?.host || "-" }}</strong></div>
            <div class="config-row"><span>端口</span><strong>{{ managedProxy(mismatchProfile || {})?.port || "-" }}</strong></div>
          </div>
          <div class="config-card config-card--mismatch">
            <div class="config-card__title">比特当前配置</div>
            <div class="config-row"><span>协议</span><strong>{{ mismatchProfile?.proxy_type || "-" }}</strong></div>
            <div class="config-row"><span>地址</span><strong>{{ mismatchProfile?.proxy_host || "-" }}</strong></div>
            <div class="config-row"><span>端口</span><strong>{{ mismatchProfile?.proxy_port || "-" }}</strong></div>
          </div>
        </div>
        <ResourceNotice v-if="accountsFor(mismatchProfile || {}).length" :details="accountTitle(mismatchProfile)" details-label="查看关联账号">涉及 {{ accountsFor(mismatchProfile).length }} 个关联账号。</ResourceNotice>
        <p v-if="managedProxy(mismatchProfile || {}) && !proxyResyncable(managedProxy(mismatchProfile || {}))" class="form-hint">
          该托管代理{{ proxyAvailabilityReason(managedProxy(mismatchProfile || {}), mismatchProfile) }}，不能重新同步；可改为比特自管或到代理管理更换代理。
        </p>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" @click="openReleaseConfirm(mismatchProfile)">改为比特自管</t-button>
            <t-button
              theme="primary"
              :disabled="!managedProxy(mismatchProfile || {}) || !proxyResyncable(managedProxy(mismatchProfile || {}))"
              @click="openResyncConfirm(mismatchProfile)"
            >重新同步托管代理</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="releaseConfirmVisible" title="改为比特自管" :width="560" >
        <ResourceNotice>保留当前代理，改由比特维护，并释放 1 个系统名额。</ResourceNotice>
        <ResourceDescriptions size="small">
          <t-descriptions-item label="窗口">{{ releaseProfile?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="保留地址">{{ actualProxyText(releaseProfile || {}) || "—" }}</t-descriptions-item>
          <t-descriptions-item label="解除托管代理">{{ managedProxy(releaseProfile || {})?.name || managedProxy(releaseProfile || {})?.host || "—" }}</t-descriptions-item>
        </ResourceDescriptions>
        <ResourceNotice v-if="accountsFor(releaseProfile || {}).length">涉及 {{ accountsFor(releaseProfile).length }} 个关联账号，账号与其他配置保持不变。</ResourceNotice>
              <template #footer><t-button theme="default" variant="outline"  @click="releaseConfirmVisible = false">取消</t-button><t-button theme="primary"  @click="confirmRelease">确认改为比特自管</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="unbindConfirmVisible" title="解绑并改为直连" :width="560" >
        <ResourceNotice tone="warning">将清除代理并改用本机网络。请先确认平台操作可以切换网络。</ResourceNotice>
        <ResourceDescriptions size="small">
          <t-descriptions-item label="窗口">{{ unbindProfile?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="当前代理">{{ actualProxyText(unbindProfile || {}) || "—" }}</t-descriptions-item>
          <t-descriptions-item label="变更后">无代理 · 直连</t-descriptions-item>
        </ResourceDescriptions>
        <ResourceNotice v-if="accountsFor(unbindProfile || {}).length">涉及 {{ accountsFor(unbindProfile).length }} 个关联账号。</ResourceNotice>
              <template #footer><t-button theme="default" variant="outline"  @click="unbindConfirmVisible = false">取消</t-button><t-button theme="danger"  @click="confirmUnbind">解绑并改为直连</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="resyncConfirmVisible" title="重新同步托管代理" :width="560" >
        <ResourceDescriptions size="small">
          <t-descriptions-item label="窗口">{{ resyncProfile?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="托管代理">{{ resyncProxy?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="应用地址">{{ resyncProxy?.host }}:{{ resyncProxy?.port }}</t-descriptions-item>
        </ResourceDescriptions>
        <ResourceNotice tone="warning">将替换当前代理；账号与其他配置保持不变。</ResourceNotice>
        <ResourceNotice v-if="accountsFor(resyncProfile || {}).length">涉及 {{ accountsFor(resyncProfile).length }} 个关联账号。</ResourceNotice>
              <template #footer><t-button theme="default" variant="outline"  @click="resyncConfirmVisible = false">取消</t-button><t-button theme="primary"  @click="confirmResync">确认同步</t-button></template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="candidateVisible" title="选择托管代理" :footer="true" :width="proxies.length ? 960 : 560">
        <FeedbackAlert v-if="candidateError" :message="candidateError" theme="error" style="margin-bottom:12px" />
        <ResourceDescriptions size="small">
          <t-descriptions-item label="窗口">{{ candidateProfile?.name || "-" }}</t-descriptions-item>
          <t-descriptions-item label="当前方式">{{ proxyModeLabel(candidateProfile || {}) }}</t-descriptions-item>
        </ResourceDescriptions>
        <div v-if="proxies.length" class="candidate-filters">
          <label class="candidate-filter candidate-filter--wide"><span>搜索代理</span><t-input v-model="candidateSearch" clearable placeholder="名称 / 地址 / 供应商" /></label>
          <label class="candidate-filter"><span>地区</span><t-select v-model="candidateRegion" :options="candidateRegionOptions" clearable placeholder="全部地区" /></label>
        </div>
        <ResourceEmptyState v-if="!proxies.length" title="当前团队暂无代理" description="添加代理资源后，即可为窗口选择线路。">
          <t-button theme="primary" @click="openProxyList">前往代理管理</t-button>
        </ResourceEmptyState>
        <ResourceEmptyState v-else-if="!candidateRows.length" title="没有符合条件的代理" description="试试其他名称、地址或地区。">
          <t-button theme="primary" variant="text" @click="candidateSearch = ''; candidateRegion = ''">清除筛选</t-button>
        </ResourceEmptyState>
        <t-table v-else :data="candidateRows" :max-height="320" :scroll="{ x: '800px' }" row-key="id" size="small" :columns="[
          { colKey: 'picked', title: '', width: 46 },
          { colKey: 'id', title: 'ID', width: 70 },
          { colKey: 'name', title: '代理名称 / 地址' },
          { colKey: 'region', title: '地区', width: 90 },
          { colKey: 'expires_at', title: '到期时间', width: 120 },
          { colKey: 'remaining', title: '剩余额度', width: 90 },
          { colKey: 'availability', title: '可用性', width: 110 },
        ]" empty="当前团队没有代理资源">
          <template #picked="{ row }">
            <t-radio :checked="candidateSelectedId === row.id" :disabled="!proxySelectable(row, candidateProfile)" @change="selectCandidate(row)" />
          </template>
          <template #name="{ row }">
            <div>{{ row.name || "-" }}</div>
            <div class="window-sub">{{ row.host }}:{{ row.port }}<span v-if="row.region"> · {{ row.region }}</span></div>
          </template>
          <template #expires_at="{ row }"><span :class="{ 'window-expired': proxyExpired(row) }">{{ formatDate(row.expires_at) }}</span></template>
          <template #remaining="{ row }">{{ proxyRemaining(row) }}</template>
          <template #availability="{ row }">
            <ResourceStatusBadge :tone="proxySelectable(row, candidateProfile) ? 'success' : 'neutral'" :label="proxyAvailabilityReason(row, candidateProfile)" />
          </template>
        </t-table>
        <ResourceNotice v-if="candidateRows.length && !candidateRows.some((row) => proxySelectable(row, candidateProfile))" tone="warning"
          details="代理需属于当前团队、已启用、未到期、检测正常且有剩余额度；具体原因见各行可用性。">当前代理均不可分配</ResourceNotice>
        <ResourceNotice v-if="candidateUsable" :details="accountTitle(candidateProfile)" details-label="查看关联账号">将替换此窗口当前代理。<span v-if="accountsFor(candidateProfile || {}).length">涉及 {{ accountsFor(candidateProfile).length }} 个关联账号。</span></ResourceNotice>
        <template #footer>
          <div class="candidate-footer">
            <t-button class="wt-secondary-button" variant="outline" @click="candidateVisible = false">取消</t-button>
            <span v-if="proxies.length" class="candidate-selection" role="status" :title="candidateSelected ? `已选择：${candidateSelected.name || candidateSelected.host}` : ''">{{ candidateSelected ? `已选择：${candidateSelected.name || candidateSelected.host}` : '尚未选择代理' }}</span>
            <t-button v-if="proxies.length" theme="primary" :disabled="!candidateUsable" @click="confirmCandidate">应用代理</t-button>
          </div>
        </template>
      </ResourceOverlay>

      <ResourceOverlay
        v-model:visible="syncVisible"
        title="同步窗口"
        :footer="syncDone || syncHasChanges"

        @close="closeSyncDrawer"
        kind="drawer" :width="860">
        <FeedbackAlert v-if="syncError" :message="syncError" theme="error" style="margin-bottom:12px" />
        <ResourceStatGrid :items="syncSummaryItems" />
        <template v-if="syncDone">
          <FeedbackAlert
            :theme="syncError ? 'error' : 'success'"
            style="margin-bottom:12px"
            :message="syncError || '本机窗口变化已按实际结果处理；下方列出仍需处理的窗口异常。'"
          />
          <t-table
            :data="allPendingProfiles"
            row-key="id"
            size="small"
            :columns="[
              { colKey: 'id', title: 'ID', width: 80 },
              { colKey: 'name', title: '窗口' },
              { colKey: 'issue', title: '结果' },
              { colKey: 'action', title: '操作', width: 90 },
            ]"
            empty="暂无待处理窗口"
          >
            <template #name="{ row }">{{ row.name || "-" }}</template>
            <template #issue="{ row }"><ResourceStatusBadge tone="warning" :label="anomalyReasons(row).join('、')" /></template>
            <template #action="{ row }"><t-button size="small" variant="text" theme="primary" @click="syncIssueAction(row)">处理</t-button></template>
          </t-table>
        </template>
        <template v-else>
          <div class="sync-section">
            <h3 class="window-section">新增窗口（已选 {{ syncSelectedAdded.length }} / {{ syncSelectableAdded.length }}）</h3>
            <p class="form-hint">本机发现但 Cloud 尚无关联的窗口。勾选后才会建立关联；不勾选不会认领，其他运营已登记的窗口不可选择。</p>
            <t-table
              :data="syncAddedProfiles"
              row-key="bit_profile_id"
              size="small"
              :columns="[
                { colKey: 'selected', title: '选择', width: 64 },
                { colKey: 'name', title: '窗口' },
                { colKey: 'bit_profile_id', title: '比特窗口 ID', width: 180 },
                { colKey: 'group_name', title: '分组', width: 120 },
                { colKey: 'notice', title: '说明' },
              ]"
              empty="没有新增窗口"
            >
              <template #selected="{ row }">
                <t-checkbox
                  :checked="syncSelectedAdded.includes(row.bit_profile_id)"
                  :disabled="Boolean(row.ownership_conflict)"
                  @change="(checked) => toggleSyncAdded(row, checked)"
                />
              </template>
              <template #group_name="{ row }">{{ row.group_name || row.group_id || "-" }}</template>
              <template #notice="{ row }">
                <span v-if="row.ownership_conflict" class="window-issue">{{ row.ownership_conflict }}</span>
                <span v-else class="window-sub">未建立关联</span>
              </template>
            </t-table>
          </div>

          <div v-if="syncChangedProfiles.length" class="sync-section">
            <h3 class="window-section">字段变更（{{ syncChangedProfiles.length }}）</h3>
            <p class="form-hint">名称、分组或比特备注已变化，接受后自动更新；运营备注、账号关系和业务启停不受影响。</p>
            <t-table :data="syncChangedProfiles" row-key="bit_profile_id" size="small" :columns="[
              { colKey: 'name', title: '窗口' },
              { colKey: 'bit_profile_id', title: '比特窗口 ID', width: 180 },
              { colKey: 'fields', title: '变化字段' },
            ]">
              <template #fields="{ row }">{{ changedFieldsText(row.bit_profile_id) }}</template>
            </t-table>
          </div>

          <div v-if="syncMissingProfiles.length" class="sync-section">
            <h3 class="window-section">本地缺失（{{ syncMissingProfiles.length }}）</h3>
            <p class="form-hint">接受后只标记为窗口缺失，不删除系统记录、关联账号或历史；恢复环境后可重新同步。</p>
            <t-table :data="syncMissingProfiles" row-key="bit_profile_id" size="small" :columns="[
              { colKey: 'name', title: '窗口' },
              { colKey: 'bit_profile_id', title: '比特窗口 ID', width: 180 },
            ]" />
          </div>

          <div v-if="syncProxyChanges.length" class="sync-section">
            <h3 class="window-section">代理变化（{{ syncProxyChanges.length }}）</h3>
            <p class="form-hint">已匹配到唯一代理的自动核对，已无代理的自动解除托管；新发现的地址需要勾选后才创建资源。冲突项不会自动处理。</p>
            <t-table :data="syncProxyChanges" row-key="profile_id" size="small" :columns="[
              { colKey: 'selected', title: '选择', width: 64 },
              { colKey: 'profile_id', title: '窗口 ID', width: 100 },
              { colKey: 'kind', title: '变化' },
              { colKey: 'detail', title: '说明' },
            ]">
              <template #selected="{ row }">
                <t-checkbox
                  v-if="row.kind === 'unknown'"
                  :checked="syncSelectedUnknown.includes(row.profile_id)"
                  @change="(checked) => toggleSyncUnknown(row, checked)"
                />
                <span v-else class="window-sub">—</span>
              </template>
              <template #kind="{ row }">
                <ResourceStatusBadge :tone="proxyChangeTone(row.kind)" :label="proxyChangeLabel(row.kind)" />
              </template>
              <template #detail="{ row }">
                <span v-if="row.kind === 'changed'">匹配到 {{ proxyById[row.target_proxy_id]?.name || row.target_proxy_id }}</span>
                <span v-else-if="row.kind === 'unbound'">比特中已无代理，接受后解除托管关系</span>
                <span v-else-if="row.kind === 'unknown'">{{ row.proxy_protocol }}://{{ row.host }}:{{ row.port }}（新发现）</span>
                <span v-else>{{ row.ownership_notice || "匹配到多个代理，不会自动处理" }}</span>
              </template>
            </t-table>
          </div>

          <FeedbackAlert
            v-if="!syncHasChanges && !syncDone"
            theme="info"
            message="本机窗口与 Cloud 记录已一致，没有需要接受的变化。"
          />
        </template>
        <template #footer>
          <t-space>
            <template v-if="syncDone">
              <t-button v-if="syncError" class="wt-secondary-button" variant="outline" @click="closeSyncDrawer(); runSync()">重新同步</t-button>
              <t-button theme="primary" @click="closeSyncDrawer">完成</t-button>
            </template>
            <template v-else>
              <t-button class="wt-secondary-button" variant="outline" @click="closeSyncDrawer">取消</t-button>
              <t-button theme="primary" @click="acceptSync">接受变化</t-button>
            </template>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="identityVisible" title="当前比特身份不匹配" :footer="true" :width="640">
        <div class="identity-block">
          <div class="identity-icon">!</div>
          <h3>本次同步已停止</h3>
          <p class="form-hint">为保护窗口与账号数据，本次未更新任何正式信息或代理关系。</p>
          <ResourceDescriptions size="small">
            <t-descriptions-item label="系统已绑定主账号">{{ boundIdentity?.main_user_id_masked || "-" }}</t-descriptions-item>
            <t-descriptions-item label="当前检测主账号">{{ identityMasked }}</t-descriptions-item>
            <t-descriptions-item label="本次窗口同步">未执行</t-descriptions-item>
            <t-descriptions-item label="已有窗口信息">未修改，不标记缺失</t-descriptions-item>
            <t-descriptions-item label="代理关系与配额">未修改</t-descriptions-item>
          </ResourceDescriptions>
          <FeedbackAlert
            theme="warning"
            style="margin-top:12px"
            message="请切换回已绑定的比特主账号再重新检测；如需换绑，请到「个人信息」完成确认，本次不会自动改绑。"
          />
        </div>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" @click="identityVisible = false">关闭</t-button>
            <t-button theme="primary" @click="identityVisible = false; runSync()">重新检测</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <ResourceOverlay v-model:visible="batchProxyVisible" title="批量设置代理" :footer="true" kind="drawer" :width="820">
        <ResourceNotice v-if="!batchProxyOptions.length" tone="warning" details="需同团队、已启用、未到期、检测正常且额度足够。">暂无可分配代理，请先到代理管理补充或检测。</ResourceNotice>
        <t-table :data="batchProxyRows" row-key="profile.id" size="small" :columns="[
          { colKey: 'id', title: 'ID', width: 80 },
          { colKey: 'name', title: '窗口' },
          { colKey: 'mode', title: '当前方式', width: 110 },
          { colKey: 'target', title: '目标代理', width: 260 },
        ]">
          <template #id="{ row }">{{ row.profile.id }}</template>
          <template #name="{ row }">
            <div>{{ row.profile.name || "-" }}</div>
            <div class="window-sub">关联 {{ accountsFor(row.profile).length }} 个账号</div>
          </template>
          <template #mode="{ row }"><ResourceStatusBadge :tone="isManaged(row.profile) ? 'info' : 'neutral'" :label="proxyModeLabel(row.profile)" /></template>
          <template #target="{ row }">
            <t-select v-model="row.targetId" :options="batchProxyOptions" size="small" placeholder="选择托管代理" />
          </template>
        </t-table>
        <ResourceNotice v-if="batchProxyIssue && batchProxyOptions.length" tone="warning">{{ batchProxyIssue }}</ResourceNotice>
        <ResourceNotice v-else-if="batchProxyOptions.length">涉及 {{ batchProxyRows.length }} 个窗口<span v-if="batchProxyAccountCount">、{{ batchProxyAccountCount }} 个平台账号</span>；按逐项核实结果生效。</ResourceNotice>
        <div v-if="!batchProxyOptions.length" class="sync-section">
          <t-button class="wt-secondary-button" variant="outline" @click="batchProxyVisible = false; openProxyList()">前往代理管理</t-button>
        </div>
        <template #footer>
          <t-space>
            <t-button class="wt-secondary-button" variant="outline" @click="batchProxyVisible = false">取消</t-button>
            <t-button theme="primary" :disabled="Boolean(batchProxyIssue)" @click="startBatchProxy">开始处理</t-button>
          </t-space>
        </template>
      </ResourceOverlay>

      <OperationProgressDialog
        :visible="progressVisible"
        :title="progressTitle"
        :phase="progressPhase"
        :items="progressItems"
        :running-label="progressRunningLabel"
        :status-labels="progressLabels"
        :retry-label="progressRetryLabel"
        :verify-label="progressVerifyLabel"
        @retry="retryFailed"
        @verify="verifyPending"
        @done="closeProgress"
      />
    </div>
  </t-loading>
</template>

<style scoped>
.window-resource-card { padding: 18px 20px; }
/* 备注弹窗：关联对象与编辑区分组，沿用公共描述和表单语义。 */
.window-note-context { margin-bottom: 24px; }
.window-note-context :deep(.t-descriptions__label), .window-note-context :deep(.t-descriptions__content) { padding-top: 4px; padding-bottom: 4px; font-size: 14px; line-height: 22px; }
.window-note-context :deep(tr:not(:last-child) .t-descriptions__label), .window-note-context :deep(tr:not(:last-child) .t-descriptions__content) { padding-bottom: 16px; }
.window-note-form :deep(.t-form__label) { color: var(--wt-text-tertiary); font-size: 14px; line-height: 22px; }
.window-note-form :deep(.t-form__controls-content) { gap: 8px; }
.window-note-form :deep(.t-textarea__inner) { font-size: 14px; line-height: 22px; }
.note-help { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; line-height: 20px; }

.window-address { font-size: 12px; font-variant-numeric: tabular-nums; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.window-id { color: var(--td-text-color-secondary); font-variant-numeric: tabular-nums; }
.window-name { color: var(--wt-text-primary); font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.window-sub { color: var(--td-text-color-placeholder); font-size: 12px; margin-top: 2px; }
.window-remark { display: block; max-width: 170px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.window-time { color: var(--td-text-color-secondary); font-variant-numeric: tabular-nums; }
.window-account { font-size: 12px; line-height: 1.6; }
.window-issues { display: flex; flex-wrap: wrap; gap: 4px 8px; margin-top: 3px; }
.window-issue { color: var(--td-warning-color); font-size: 12px; }
.window-expired { color: var(--td-error-color); }
.running-state { display: inline-flex; align-items: center; gap: 6px; color: var(--td-text-color-secondary); }
.running-dot { width: 7px; height: 7px; border-radius: 50%; border: 1px solid var(--td-text-color-placeholder); background: transparent; }
.running-dot.is-open { border-color: var(--td-success-color); background: var(--td-success-color); }
.form-hint { color: var(--td-text-color-placeholder); font-size: 12px; line-height: 1.5; margin-top: 4px; }
.window-section { margin: 18px 0 10px; font-size: 14px; font-weight: 600; color: var(--td-text-color-primary); }
.window-tips { margin: 0; padding-left: 20px; color: var(--td-text-color-secondary); line-height: 1.8; }
.config-compare { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 12px; }
.config-card { padding: 12px 14px; border: 1px solid var(--td-component-stroke); border-radius: 8px; background: var(--td-bg-color-container); }
.config-card--mismatch { border-color: var(--td-warning-color); background: var(--td-warning-color-1); }
.config-card__title { font-weight: 600; margin-bottom: 8px; }
.config-row { display: flex; justify-content: space-between; gap: 12px; padding: 3px 0; font-size: 13px; }
.config-row span { color: var(--td-text-color-secondary); }
.config-row strong { font-weight: 500; text-align: right; }
.config-row em { margin-left: 5px; padding: 0 4px; border-radius: 3px; color: var(--td-warning-color); background: var(--td-warning-color-1); font-size: 11px; font-style: normal; }
.create-success { text-align: center; }
.create-success__icon { width: 56px; height: 56px; margin: 8px auto 10px; border-radius: 50%; color: var(--td-success-color); background: var(--td-success-color-1); font-size: 28px; line-height: 56px; }
.create-success h3 { margin: 0 0 14px; }
.create-success :deep(.t-descriptions) { text-align: left; }
.manage-section { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 16px; margin-top: 10px; border: 1px solid var(--td-component-stroke); border-radius: 8px; }
.manage-section__title { font-weight: 600; }
.manage-section__desc { margin-top: 3px; color: var(--td-text-color-secondary); font-size: 12px; }
.candidate-filters { display: flex; flex-wrap: wrap; gap: 20px 24px; margin: 20px 0; }
.candidate-filter { display: flex; flex: 0 1 180px; flex-direction: column; gap: 8px; min-width: 0; font-size: 13px; font-weight: 500; }
.candidate-filter--wide { flex: 1 1 280px; }
.candidate-footer { display: flex; align-items: center; gap: 12px; width: 100%; min-width: 0; }
.candidate-selection { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--td-text-color-secondary); font-size: 14px; }
.candidate-filter :deep(.t-input__wrap), .candidate-filter :deep(.t-select__wrap) { min-width: 0; width: 100%; }
.sync-section { margin-top: 18px; }
.identity-block { text-align: center; }
.identity-icon { width: 52px; height: 52px; margin: 6px auto 10px; border-radius: 50%; color: var(--td-error-color); background: var(--td-error-color-1); font-size: 28px; line-height: 52px; }
.identity-block h3 { margin: 0 0 4px; }
.identity-block :deep(.t-descriptions) { margin-top: 14px; text-align: left; }
@media (max-width: 720px) { .config-compare { grid-template-columns: 1fr; } }
</style>
