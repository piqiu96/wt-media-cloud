<script setup>
import { computed, onMounted, ref, watch } from "vue"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import { createUsersClient } from "../../../apps/cloud/pages/users/usersApi.js"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import { createProxyClient } from "../../../shared/api/proxy.js"
import { createSessionClient } from "../../../shared/api/session.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue"
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue"
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue"
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue"
import { isDesktop } from "../../../utils.js"
// Static import: a dynamic import() of a node_modules bare specifier does not
// resolve in the packaged Tauri WebView ("Module name ... does not resolve to a
// valid file"). invoke is only called inside __TAURI_INTERNALS__-guarded code,
// so importing it is inert in Cloud Web.
import { invoke } from "@tauri-apps/api/core"

const bindingClient = createProfileBindingClient()
const proxyClient = createProxyClient()
const sessionClient = createSessionClient()
const usersClient = createUsersClient()
const profiles = ref([])
const scans = ref([])
const loading = ref(true)
const error = ref("")
const taskNotice = ref("")
const operatingProfileId = ref("")
const filterId = ref("")
const filterName = ref("")
const filterGroup = ref("")
const filterBitId = ref("")
const filterRemark = ref("")
const statusFilter = ref("")
const businessFilter = ref("")
const runningFilter = ref("")
const userFilter = ref("")
const pagination = ref({ current: 1, pageSize: 20, total: 0 })
// 运行状态本地跟踪：key=profile.id → true(打开)/false(关闭)；未知则为 undefined
const openStates = ref({})
const selectedRowKeys = ref([])
const batchOperating = ref("")

// Create dialog
const showCreate = ref(false)
const creating = ref(false)
const newProfile = ref({ name: "", group_id: "", group_name: "", remark: "" })
const profileGroups = ref([])
const loadingGroups = ref(false)
const createError = ref("")

// Edit dialog (Cloud remark)
const showEdit = ref(false)
const editing = ref(false)
const editProfile = ref(null)
const editRemark = ref("")
const editError = ref("")

// Detail drawer
const detailVisible = ref(false)
const detailProfile = ref(null)
const showAssign = ref(false)
const assigning = ref(false)
const assignProfile = ref(null)
const assignUserId = ref("")
const operatorUsers = ref([])

// Scan
const scanning = ref(false)
const acceptingScan = ref(false)
const restoringCloud = ref(false)
const scanDetailVisible = ref(false)
const currentScan = ref(null)
const identityError = ref("")
const selectedTab = ref("changed")
const acceptConfirmVisible = ref(false)
const acceptConfirmMessage = ref("")
const restoreConfirmVisible = ref(false)
const restoreConfirmMessage = ref("")
const restoreTargets = ref([])
const proxyDialogVisible = ref(false)
const proxyBinding = ref(false)
const proxyProfile = ref(null)
const proxyId = ref("")
const availableProxies = ref([])
const proxyById = ref({})
const proxyBindingHint = ref("")
const batchProxyDialogVisible = ref(false)
const batchProxyBinding = ref(false)
const batchProxyTargets = ref([])
const batchProxyId = ref("")
const batchProxyCandidates = ref([])
const batchProxyBindingHint = ref("")
const localProxyScan = ref(null)
let lastRuntimeRefreshAt = 0
let lastRuntimeStatus = null

const currentUser = ref(null)
const isDesktopClient = computed(() => isDesktop())
const isAdmin = computed(() => currentUser.value?.role === "admin")
const operatorOptions = computed(() => operatorUsers.value.map((user) => ({
  label: `${user.username}（UID ${user.id}）`,
  value: String(user.id),
})))
const groupOptions = computed(() => profileGroups.value.map((group) => ({
  label: `${group.name || group.id}（${group.id}）`,
  value: group.id,
})))
const groupFilterOptions = computed(() => {
  const seen = new Set()
  return profiles.value
    .filter((p) => {
      if (!p.group_name || seen.has(p.group_name)) return false
      seen.add(p.group_name)
      return true
    })
    .map((p) => ({ label: p.group_name, value: p.group_name }))
})
const userFilterOptions = computed(() => {
  const seen = new Set()
  const list = []
  if (currentUser.value) {
    list.push({ label: currentUser.value.username || `UID ${currentUser.value.id}`, value: String(currentUser.value.id) })
    seen.add(String(currentUser.value.id))
  }
  for (const u of operatorUsers.value) {
    const key = String(u.id)
    if (seen.has(key)) continue
    seen.add(key)
    list.push({ label: u.username || `UID ${u.id}`, value: key })
  }
  return list
})
const filteredProfiles = computed(() => {
  const id = filterId.value.trim().toLowerCase()
  const name = filterName.value.trim().toLowerCase()
  const group = filterGroup.value.trim().toLowerCase()
  const bitId = filterBitId.value.trim().toLowerCase()
  const remark = filterRemark.value.trim().toLowerCase()
  return profiles.value.filter((profile) => {
    if (statusFilter.value && profile.local_status !== statusFilter.value) return false
    if (businessFilter.value && profile.business_status !== businessFilter.value) return false
    if (runningFilter.value === "open" && openStates.value[profile.id] !== true) return false
    if (runningFilter.value === "closed" && openStates.value[profile.id] !== false) return false
    if (userFilter.value && String(profile.user_id || "") !== userFilter.value) return false
    if (id && !String(profile.id || "").toLowerCase().includes(id)) return false
    if (name && !String(profile.name || "").toLowerCase().includes(name)) return false
    if (group && ![profile.group_id, profile.group_name].some((value) => String(value || "").toLowerCase().includes(group))) return false
    if (bitId && !String(profile.bit_profile_id || "").toLowerCase().includes(bitId)) return false
    if (remark && !String(profile.remark || "").toLowerCase().includes(remark)) return false
    return true
  }).sort((a, b) => (Number(b.seq) || 0) - (Number(a.seq) || 0))
})

const pagedProfiles = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return filteredProfiles.value.slice(start, start + pagination.value.pageSize)
})

const profileStatItems = computed(() => [
  { key: "total", label: "窗口总数", value: profiles.value.length, tone: "info" },
  { key: "open", label: "运行中", value: profiles.value.filter((profile) => isWindowOpen(profile)).length, tone: "success" },
  { key: "bound_proxy", label: "已绑代理", value: profiles.value.filter((profile) => profile.proxy_id).length, tone: "info" },
  { key: "attention", label: "需关注", value: profiles.value.filter((profile) => profile.local_status !== "active" || profile.business_status === "disabled").length, tone: "warning" },
])

function applyProfileStatFilter(key) {
  businessFilter.value = ""
  runningFilter.value = ""
  statusFilter.value = ""
  if (key === "open") runningFilter.value = "open"
  if (key === "bound_proxy") filterRemark.value = ""
  if (key === "attention") statusFilter.value = "local_missing"
}

watch(filteredProfiles, (list) => {
  const maxPage = Math.max(1, Math.ceil(list.length / (pagination.value.pageSize || 20)))
  if (pagination.value.current > maxPage) pagination.value.current = maxPage
}, { immediate: true })

async function desktopLocalAgentService() {
  if (typeof window === "undefined" || !window.__TAURI_INTERNALS__) {
    throw new Error("BitBrowser 扫描只能在 Desktop 客户端执行")
  }
  return createLocalAgentService({ invoke })
}

function cloudBaseUrl() {
  if (typeof window === "undefined") return "http://127.0.0.1:18080"
  const origin = window.location?.origin || "http://127.0.0.1:18080"
  // Packaged Desktop runs on http://tauri.localhost, which is NOT the Cloud
  // API host; the local Cloud server is always the API base.
  return origin.startsWith("http://127.0.0.1:18080") ? origin : "http://127.0.0.1:18080"
}

async function currentLocalNodeId() {
  const service = await desktopLocalAgentService()
  const status = await refreshRuntimeWithCooldown(service)
  return status.node_id || ""
}

async function refreshRuntimeWithCooldown(service, { force = false } = {}) {
  const now = Date.now()
  if (!force && lastRuntimeStatus?.node_id && now - lastRuntimeRefreshAt < 60_000) {
    return lastRuntimeStatus
  }
  const localStatus = await service.status()
  try {
    const refreshed = await service.refreshRuntime({ cloudBaseUrl: cloudBaseUrl() })
    lastRuntimeStatus = refreshed
    lastRuntimeRefreshAt = now
    return refreshed
  } catch {
    // 返回本地状态，由调用方根据 node_id 判断可信与友好提示，而不是抛原始错误。
    taskNotice.value = "Cloud可信状态刷新暂时失败；将由本地状态与本机预检判断是否可执行。"
    lastRuntimeStatus = localStatus
    lastRuntimeRefreshAt = now
    return localStatus
  }
}

function diffCount(scan) {
  const diff = getDiff(scan)
  return (diff.added?.length || 0) + (diff.changed?.length || 0) + (diff.missing?.length || 0)
}

function hasDiff(scan) {
  return diffCount(scan) > 0
}

function localTrustMessage(e) {
  const message = String(e?.message || e || "")
  if (e.errcode === 10001 || e.errcode === 23003 || e.errcode === 11001) {
    return "当前电脑尚未完成本地环境确认，暂时不能扫描或操作浏览器窗口。请到 Desktop「环境状态」页刷新本机状态后重试。"
  }
  if (message.includes("timed out") || message.includes("timeout")) {
    return "BitBrowser窗口操作超时：Local Agent 已请求 BitBrowser，但 BitBrowser 未在限定时间内返回。请查看 Agent 日志确认是否已延迟打开，或稍后重试。"
  }
  return message || "本机操作失败，请查看 Agent 日志。"
}

onMounted(async () => {
  try {
    currentUser.value = await sessionClient.me()
    if (currentUser.value?.role === "admin") {
      await loadOperatorUsers()
    }
  } catch {}
  await loadProfiles()
})

async function loadOperatorUsers() {
  try {
    const users = await usersClient.listUsers({ role: "operator", status: "enabled" })
    operatorUsers.value = Array.isArray(users) ? users.filter((user) => user.role === "operator" && user.status === "enabled") : []
  } catch (e) {
    error.value = e.message
  }
}

async function loadProfiles() {
  error.value = ""
  loading.value = true
  try {
    const [listedProfiles, proxyResult] = await Promise.all([bindingClient.listProfiles(), proxyClient.list()])
    profiles.value = listedProfiles
    const all = Array.isArray(proxyResult) ? proxyResult : (proxyResult?.list || [])
    proxyById.value = Object.fromEntries(all.map((item) => [item.id, item]))
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function createProfile() {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web只展示云端已保存的浏览器窗口信息；新建窗口请在Desktop客户端执行。"
    return
  }
  createError.value = ""
  if (!newProfile.value.name.trim()) {
    createError.value = "请填写窗口名称。"
    return
  }
  if (!newProfile.value.group_id) {
    createError.value = "请先选择真实BitBrowser分组。"
    return
  }
  creating.value = true
  error.value = ""
  try {
    const localAgent = await desktopLocalAgentService()
    const status = await refreshRuntimeWithCooldown(localAgent, { force: true })
    if (!status.node_id) {
      throw new Error("当前电脑尚未完成本地环境确认，请先到 Desktop「环境状态」页绑定当前比特浏览器账号。")
    }
    const created = await localAgent.profileCreate(newProfile.value)
    const readback = created.snapshot || {}
    const scan = await bindingClient.submit({
      main_user_id: readback.main_user_id || currentUser.value?.bit_main_user_id || currentUser.value?.id,
      profiles: readback.profiles || [],
    }, { nodeId: status.node_id })
    await bindingClient.confirm(scan.id || scan.scan_id, { nodeId: status.node_id })
    taskNotice.value = `已在BitBrowser创建窗口并读回同步到Cloud：${created.bit_profile_id}`
    showCreate.value = false
    newProfile.value = { name: "", group_id: "", group_name: "", remark: "" }
    identityError.value = ""
    await loadProfiles()
  } catch (e) {
    if (e.errcode === 23002 || e.errcode === 23003) {
      identityError.value = "当前比特浏览器登录账号与本系统用户绑定账号不一致。为避免数据错乱，请切换回正确的比特浏览器账号后重试。"
    }
    createError.value = localTrustMessage(e)
    error.value = createError.value
  } finally {
    creating.value = false
  }
}

function isWindowOpen(profile) {
  return openStates.value[profile.id] === true
}

function openStateLabel(profile) {
  if (isWindowOpen(profile)) return "打开"
  if (openStates.value[profile.id] === false) return "关闭"
  return "未知"
}

async function openProfile(profile) {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web不能打开本机BitBrowser窗口；请在Desktop客户端执行。"
    return
  }
  const bitProfileId = String(profile.bit_profile_id || "").trim()
  if (!bitProfileId) {
    error.value = "打开窗口失败：Cloud记录缺少BitBrowser窗口ID，请先扫描并同步本机窗口。"
    return
  }
  if (isWindowOpen(profile)) {
    taskNotice.value = "该窗口已处于打开状态。"
    return
  }
  operatingProfileId.value = `open:${bitProfileId}`
  error.value = ""
  taskNotice.value = `正在打开BitBrowser窗口：${profile.name || bitProfileId}`
  try {
    const localAgent = await desktopLocalAgentService()
    const result = await localAgent.profileOpen(bitProfileId)
    openStates.value[profile.id] = true
    taskNotice.value = `已打开BitBrowser窗口：${profile.name || result.bit_profile_id}`
  } catch (e) {
    taskNotice.value = ""
    error.value = localTrustMessage(e)
  } finally {
    operatingProfileId.value = ""
  }
}

async function closeProfile(profile) {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web不能关闭本机BitBrowser窗口；请在Desktop客户端执行。"
    return
  }
  const bitProfileId = String(profile.bit_profile_id || "").trim()
  if (!bitProfileId) {
    error.value = "关闭窗口失败：Cloud记录缺少BitBrowser窗口ID，请先扫描并同步本机窗口。"
    return
  }
  if (openStates.value[profile.id] === false) {
    taskNotice.value = "该窗口已处于关闭状态。"
    return
  }
  operatingProfileId.value = `close:${bitProfileId}`
  error.value = ""
  taskNotice.value = `正在关闭BitBrowser窗口：${profile.name || bitProfileId}`
  try {
    const localAgent = await desktopLocalAgentService()
    const result = await localAgent.profileClose(bitProfileId)
    openStates.value[profile.id] = false
    taskNotice.value = `已关闭BitBrowser窗口：${profile.name || result.bit_profile_id}`
  } catch (e) {
    taskNotice.value = ""
    error.value = localTrustMessage(e)
  } finally {
    operatingProfileId.value = ""
  }
}

async function batchOpenWindows() {
  const targets = profiles.value.filter((p) => selectedRowKeys.value.includes(p.id) && !isWindowOpen(p))
  await runBatchWindow(targets, "open")
}

async function batchCloseWindows() {
  const targets = profiles.value.filter((p) => selectedRowKeys.value.includes(p.id) && openStates.value[p.id] !== false)
  await runBatchWindow(targets, "close")
}

async function runBatchWindow(targets, action) {
  if (!isDesktopClient.value) {
    error.value = "批量操作请在Desktop客户端执行。"
    return
  }
  if (!targets.length) {
    taskNotice.value = "没有需要操作的已选窗口。"
    return
  }
  batchOperating.value = action
  error.value = ""
  let ok = 0
  let failed = 0
  try {
    const localAgent = await desktopLocalAgentService()
    for (const profile of targets) {
      const bitProfileId = String(profile.bit_profile_id || "").trim()
      if (!bitProfileId) { failed += 1; continue }
      try {
        if (action === "open") {
          await localAgent.profileOpen(bitProfileId)
          openStates.value[profile.id] = true
        } else {
          await localAgent.profileClose(bitProfileId)
          openStates.value[profile.id] = false
        }
        ok += 1
      } catch {
        failed += 1
      }
    }
    taskNotice.value = `批量${action === "open" ? "打开" : "关闭"}完成：成功 ${ok} 个，失败 ${failed} 个。`
  } catch (e) {
    error.value = localTrustMessage(e)
  } finally {
    batchOperating.value = ""
    selectedRowKeys.value = []
  }
}

function businessStatusLabel(status) {
  return status === "disabled" ? "已停用" : "启用"
}

async function toggleBusinessStatus(profile) {
  const next = profile.business_status === "disabled" ? "enabled" : "disabled"
  operatingProfileId.value = `biz:${profile.id}`
  error.value = ""
  const label = next === "disabled" ? "停用" : "启用"
  try {
    await bindingClient.updateProfile(profile.id, { business_status: next })
    taskNotice.value = `已${label}窗口 "${profile.name || profile.bit_profile_id}"：${label === "停用" ? "将不再参与社媒账号匹配、不可打开/关闭，但窗口仍会被扫描显示。" : "已恢复参与账号匹配与执行。"}`
    await loadProfiles()
  } catch (e) {
    error.value = e.message
  } finally {
    operatingProfileId.value = ""
  }
}

function remarkDisplay(profile) {
  const parts = []
  if (profile.remark) parts.push(profile.remark)
  if (profile.cloud_remark) parts.push(profile.cloud_remark)
  return parts.join("\n")
}

function openEdit(profile) {
  editProfile.value = profile
  editRemark.value = profile.cloud_remark || ""
  editError.value = ""
  showEdit.value = true
}

async function saveEdit() {
  if (!editProfile.value) return
  editing.value = true
  editError.value = ""
  try {
    await bindingClient.updateProfile(editProfile.value.id, { cloud_remark: editRemark.value })
    taskNotice.value = "已更新Cloud窗口备注（BitBrowser备注保持不变）。"
    showEdit.value = false
    await loadProfiles()
  } catch (e) {
    editError.value = e.message
  } finally {
    editing.value = false
  }
}

async function openProxyBinding(profile) {
  if (!isDesktopClient.value) return
  proxyProfile.value = profile
  proxyId.value = profile.proxy_id || ""
  try {
    const result = await proxyClient.list({ business_status: "active" })
    const all = Array.isArray(result) ? result : (result?.list || [])
    proxyById.value = Object.fromEntries(all.map((item) => [item.id, item]))
    availableProxies.value = all.filter((item) => isProxyAssignable(item) && (item.remaining_profile_count > 0 || item.id === profile.proxy_id))
    proxyBindingHint.value = availableProxies.value.length
      ? (proxyNeedsSync(profile) ? "当前关联代理的连接信息已变更；请先检测正常，再重新确认写入并读回。" : "")
      : "暂无可绑定代理：仅已启用、检测正常、未过期且有剩余配额的代理可选择。请到代理管理页编辑或批量检测。"
    proxyDialogVisible.value = true
  } catch (e) {
    error.value = e.message || "无法读取可绑定代理"
  }
}

async function confirmProxyBinding() {
  if (!proxyProfile.value) return
  if (proxyId.value && !availableProxies.value.some((item) => item.id === proxyId.value)) {
    error.value = "所选代理尚不可绑定，请先完成检测并确认有剩余配额。"
    return
  }
  proxyBinding.value = true
  error.value = ""
  try {
    if (proxyId.value) {
      const profile = await proxyClient.assign(proxyId.value, proxyProfile.value.id)
      taskNotice.value = `代理已写入 BitBrowser 并读回验证：${profile.name || profile.bit_profile_id}`
    } else if (proxyProfile.value.proxy_id) {
      await proxyClient.unbind(proxyProfile.value.proxy_id, proxyProfile.value.id)
      taskNotice.value = "已解绑代理，并完成 BitBrowser 无代理读回验证。"
    }
    proxyDialogVisible.value = false
    await loadProfiles()
  } catch (e) {
    error.value = e.message || "代理写入或读回失败"
  } finally {
    proxyBinding.value = false
  }
}

function selectedProfilesForProxyBinding() {
  return profiles.value.filter((profile) => (
    selectedRowKeys.value.includes(profile.id)
    && profile.local_status === "active"
    && profile.business_status !== "disabled"
  ))
}

async function openBatchProxyBinding() {
  if (!isDesktopClient.value) {
    error.value = "批量绑定代理请在Desktop客户端执行。"
    return
  }
  const targets = selectedProfilesForProxyBinding()
  if (!targets.length) {
    error.value = "请选择至少一个可用浏览器窗口后再绑定代理。"
    return
  }
  error.value = ""
  batchProxyTargets.value = targets
  batchProxyId.value = ""
  batchProxyCandidates.value = []
  batchProxyBindingHint.value = "正在按所选窗口数量、检测状态和剩余配额获取推荐代理。"
  try {
    const result = await proxyClient.recommend(targets.map((profile) => profile.id))
    const candidates = Array.isArray(result) ? result : (result?.list || [])
    batchProxyCandidates.value = candidates
    batchProxyId.value = candidates[0]?.id || ""
    batchProxyBindingHint.value = candidates.length
      ? "已默认选择首个推荐代理；你也可以手动调整为其他合格候选。确认后会逐窗口写入 BitBrowser 并读回验证。"
      : "暂无可绑定代理：候选必须已启用、检测正常、未过期，并能容纳全部所选窗口。"
    batchProxyDialogVisible.value = true
  } catch (e) {
    batchProxyBindingHint.value = ""
    error.value = e.message || "无法读取代理推荐"
  }
}

async function confirmBatchProxyBinding() {
  if (!batchProxyTargets.value.length) return
  if (!batchProxyId.value || !batchProxyCandidates.value.some((proxy) => proxy.id === batchProxyId.value)) {
    error.value = "请选择系统推荐的合格代理后再确认。"
    return
  }
  batchProxyBinding.value = true
  error.value = ""
  try {
    const result = await proxyClient.assignBatch(batchProxyId.value, batchProxyTargets.value.map((profile) => profile.id))
    const succeeded = Array.isArray(result?.succeeded) ? result.succeeded.length : 0
    const failed = Array.isArray(result?.failed) ? result.failed.length : 0
    taskNotice.value = `批量绑定代理完成：成功 ${succeeded} 个，失败 ${failed} 个。每个成功窗口均已写入 BitBrowser 并读回验证。`
    batchProxyDialogVisible.value = false
    selectedRowKeys.value = []
    await loadProfiles()
  } catch (e) {
    error.value = e.message || "批量代理写入或读回失败"
  } finally {
    batchProxyBinding.value = false
  }
}


function openDetail(profile) {
  detailProfile.value = profile
  detailVisible.value = true
}

async function openCreateDialog() {
  showCreate.value = true
  createError.value = ""
  if (!isDesktopClient.value) return
  loadingGroups.value = true
  try {
    const localAgent = await desktopLocalAgentService()
    const result = await localAgent.profileGroups()
    profileGroups.value = result?.data?.groups || result?.groups || []
    if (!profileGroups.value.length) {
      createError.value = "未读取到BitBrowser分组，请确认BitBrowser已登录并至少存在一个分组。"
    }
  } catch (e) {
    createError.value = localTrustMessage(e)
    error.value = createError.value
  } finally {
    loadingGroups.value = false
  }
}

async function triggerScan() {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web只展示Cloud已保存信息；扫描BitBrowser窗口请在Desktop客户端执行。"
    return
  }
  scanning.value = true
  error.value = ""
  try {
    const localAgent = await desktopLocalAgentService()
    const status = await refreshRuntimeWithCooldown(localAgent, { force: true })
    if (!status.node_id) {
      throw new Error("当前电脑尚未完成本地环境确认，请先到 Desktop「环境状态」页绑定当前比特浏览器账号。")
    }
    const snapshot = await localAgent.profileScan()
    const scan = await bindingClient.submit({
      main_user_id: snapshot.main_user_id || currentUser.value?.id,
      profiles: snapshot.profiles || [],
    }, { nodeId: status.node_id })
    currentScan.value = scan
    localProxyScan.value = await proxyClient.previewLocalScan(scan.id || scan.scan_id)
    scanDetailVisible.value = true
    selectedTab.value = "changed"
    identityError.value = ""
    await loadProfiles()
  } catch (e) {
    if (e.errcode === 23002 || e.errcode === 23003) {
      identityError.value = "当前比特浏览器登录账号与本系统用户绑定账号不一致。为避免数据错乱，请切换回正确的比特浏览器账号后重试。"
    }
    error.value = e.message
  } finally {
    scanning.value = false
  }
}

async function reviewScan(scan) {
  try {
    currentScan.value = await bindingClient.review(scan.id || scan.scan_id)
    scanDetailVisible.value = true
  } catch (e) {
    error.value = e.message
  }
}

async function acceptLocalChanges() {
  if (!currentScan.value) return
  if (!isDesktopClient.value) {
    error.value = "Cloud Web不处理本机扫描差异；请在Desktop客户端接受本地变化。"
    return
  }
  const diff = getDiff(currentScan.value)
  const total = diffCount(currentScan.value)
  if (total === 0) {
    taskNotice.value = "本机窗口与Cloud记录一致，无需处理。"
    return
  }
  const message = total > 0
    ? `确定接受本机扫描结果并更新Cloud窗口镜像？本次会处理 ${total} 项差异（含BitBrowser备注），但不会覆盖授权用户、媒体账号绑定、游戏、标签、Cloud备注、Cookie 和业务状态。`
    : "本次扫描没有差异，确认后只会记录本次扫描已处理。"
  acceptConfirmMessage.value = message
  acceptConfirmVisible.value = true
}

async function confirmAcceptLocalChanges() {
  if (!currentScan.value) return
  acceptingScan.value = true
  error.value = ""
  try {
    const nodeId = await currentLocalNodeId()
    currentScan.value = await bindingClient.confirm(currentScan.value.id || currentScan.value.scan_id, { nodeId })
    if (localProxyScan.value?.scan_id) {
      await proxyClient.confirmLocalScan(localProxyScan.value.scan_id, nodeId)
    }
    taskNotice.value = "已接受本地变化，Cloud浏览器窗口镜像已按允许字段更新。"
    acceptConfirmVisible.value = false
    await loadProfiles()
  } catch (e) {
    error.value = localTrustMessage(e)
  } finally {
    acceptingScan.value = false
  }
}

function restoreTargetsFromCurrentScan() {
  const diff = getDiff(currentScan.value)
  const targetBitIds = new Set([
    ...(diff.changed || []).map((item) => item.bit_profile_id).filter(Boolean),
    ...(diff.missing || []).map((item) => item.bit_profile_id).filter(Boolean),
  ])
  const byBitId = new Map(profiles.value.map((profile) => [profile.bit_profile_id, profile]))
  return [...targetBitIds]
    .map((bitProfileId) => byBitId.get(bitProfileId))
    .filter(Boolean)
    .map((profile) => ({
      bit_profile_id: profile.bit_profile_id,
      name: profile.name || "",
      seq: Number.isFinite(Number(profile.seq)) ? Number(profile.seq) : null,
      group_id: profile.group_id || "",
      group_name: profile.group_name || "",
      proxy_type: profile.proxy_type || "",
      proxy_host: profile.proxy_host || "",
      proxy_port: Number.isFinite(Number(profile.proxy_port)) ? Number(profile.proxy_port) : null,
      remark: profile.remark || "",
    }))
}

async function restoreCloudConfig() {
  if (!currentScan.value) return
  if (!isDesktopClient.value) {
    error.value = "Cloud Web不能恢复本机BitBrowser配置；请在Desktop客户端执行。"
    return
  }
  const targets = restoreTargetsFromCurrentScan()
  if (!targets.length) {
    error.value = "当前扫描没有可恢复的Cloud窗口配置；本地新增窗口只能选择接受本地变化。"
    return
  }
  restoreTargets.value = targets
  restoreConfirmMessage.value = `确定将 ${targets.length} 个Cloud已保存窗口配置写回本机BitBrowser，并在写回后重新读回验证？本操作不会写入账号、Cookie、授权用户或业务状态。`
  restoreConfirmVisible.value = true
}

async function confirmRestoreCloudConfig() {
  if (!currentScan.value || !restoreTargets.value.length) return
  restoringCloud.value = true
  error.value = ""
  try {
    const localAgent = await desktopLocalAgentService()
    const status = await refreshRuntimeWithCooldown(localAgent, { force: true })
    if (!status.node_id) {
      throw new Error("当前电脑尚未完成本地环境确认，请先到 Desktop「环境状态」页绑定当前比特浏览器账号。")
    }
    const result = await localAgent.profileRestore(restoreTargets.value)
    const readback = result.snapshot || {}
    currentScan.value = await bindingClient.submit({
      main_user_id: readback.main_user_id || currentUser.value?.id,
      profiles: readback.profiles || [],
    }, { nodeId: status.node_id })
    taskNotice.value = `已恢复 ${result.restored_count || restoreTargets.value.length} 个窗口的Cloud配置，并已读回验证。`
    restoreConfirmVisible.value = false
    restoreTargets.value = []
    selectedTab.value = "changed"
    await loadProfiles()
  } catch (e) {
    error.value = localTrustMessage(e)
  } finally {
    restoringCloud.value = false
  }
}

function openAssignProfile(profile) {
  assignProfile.value = profile
  assignUserId.value = profile.user_id ? String(profile.user_id) : ""
  showAssign.value = true
}

async function assignOwner() {
  if (!assignProfile.value || !assignUserId.value) {
    error.value = "请选择要授权的普通运营用户。"
    return
  }
  assigning.value = true
  error.value = ""
  try {
    const updated = await bindingClient.assignProfileOwner(assignProfile.value.id, assignUserId.value)
    taskNotice.value = "已更新Cloud窗口授权关系。本操作不会修改本机BitBrowser窗口或媒体账号绑定。"
    showAssign.value = false
    const index = profiles.value.findIndex((profile) => profile.id === updated.id)
    if (index >= 0) profiles.value.splice(index, 1, updated)
    else await loadProfiles()
  } catch (e) {
    error.value = e.message
  } finally {
    assigning.value = false
  }
}

function operatorLabel(userID) {
  const nid = Number(userID)
  if (currentUser.value && nid === Number(currentUser.value.id)) return "我"
  const user = operatorUsers.value.find((item) => Number(item.id) === nid)
  return user ? user.username : (userID ? `UID ${userID}` : "-")
}

function maskMainUserId(value) {
  if (!value) return "-"
  const text = String(value)
  if (text.length <= 8) return text
  return `${text.slice(0, 4)}…${text.slice(-4)}`
}

function getDiff(scan) {
  const src = scan?.diff || scan?.diff_json
  if (!src) return { added: [], changed: [], missing: [] }
  if (typeof src === "string") {
    try { return JSON.parse(src) } catch { return { added: [], changed: [], missing: [] } }
  }
  // Convert flat diff array to grouped format { added: [...], changed: [...], missing: [...] }
  if (Array.isArray(src)) {
    const grouped = { added: [], changed: [], missing: [] }
    for (const item of src) {
      if (item.kind === "added") grouped.added.push(item)
      else if (item.kind === "changed") grouped.changed.push(item)
      else if (item.kind === "missing") grouped.missing.push(item)
    }
    return grouped
  }
  return src
}

function formatTime(t) {
  if (!t) return "-"
  return new Date(t).toLocaleString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

function proxySummary(profile) {
  if (!profile?.proxy_host) return "-"
  return `${profile.proxy_type || "http"}://${profile.proxy_host}:${profile.proxy_port || 0}`
}

function isProxyAssignable(proxy) {
  if (!proxy || proxy.business_status !== "active" || proxy.last_check_result !== "ok") return false
  return !proxy.expires_at || new Date(proxy.expires_at) > new Date()
}

function proxyNeedsSync(profile) {
  if (!profile?.proxy_id) return false
  const proxy = proxyById.value[profile.proxy_id]
  if (!proxy) return false
  return proxy.proxy_protocol !== profile.proxy_type || proxy.host !== profile.proxy_host || Number(proxy.port) !== Number(profile.proxy_port)
}

function statusLabel(status) {
  return {
    active: "可用",
    local_missing: "本机缺失",
    archived: "已归档",
  }[status] || status || "-"
}

function bitStatusLabel(status) {
  if (!status) return "-"
  return String(status) === "0" ? "未运行" : "运行中"
}

const columns = [
  { colKey: "row-select", type: "multiple", width: 40 },
  { colKey: "id", title: "ID", width: 70, sortable: true },
  { colKey: "seq", title: "比特序号", width: 80, sortable: true },
  { colKey: "name", title: "名称", width: 160 },
  { colKey: "group_name", title: "分组", width: 100 },
  { colKey: "proxy", title: "代理", width: 100 },
  { colKey: "remark", title: "备注", width: 180 },
  { colKey: "business_status", title: "状态", width: 70 },
  { colKey: "running", title: "运行", width: 60 },
  { colKey: "last_synced_at", title: "同步时间", width: 130 },
  { colKey: "op", title: "操作", width: 200 },
]

// Profile lookup from scan profiles list
// 从本次扫描候选或 Cloud 窗口列表取值（缺失项在 BitBrowser 已删，仅在 Cloud 有值）。
function profileFieldValue(bitProfileId, field) {
  const scanP = currentScan.value?.profiles?.find((p) => p.bit_profile_id === bitProfileId)
  if (scanP && scanP[field] !== undefined && scanP[field] !== null && scanP[field] !== '') return scanP[field]
  const cloudP = profiles.value.find((p) => p.bit_profile_id === bitProfileId)
  if (cloudP && cloudP[field] !== undefined && cloudP[field] !== null && cloudP[field] !== '') return cloudP[field]
  return undefined
}
function profileName(scan, bitProfileId) {
  return profileFieldValue(bitProfileId, 'name') || bitProfileId.slice(0, 12)
}
function profileGroup(scan, bitProfileId) {
  return profileFieldValue(bitProfileId, 'group_name') || '-'
}
function profileProxy(scan, bitProfileId) {
  const host = profileFieldValue(bitProfileId, 'proxy_host')
  if (!host) return '-'
  const type = profileFieldValue(bitProfileId, 'proxy_type') || 'http'
  const port = profileFieldValue(bitProfileId, 'proxy_port')
  return `${type}://${host}:${port || 0}`
}
function profileRemark(scan, bitProfileId) {
  return profileFieldValue(bitProfileId, 'remark') || '-'
}

const CHANGED_FIELD_LABELS = {
  name: '名称', group_name: '分组', proxy_host: '代理', remark: '备注', seq: '比特序号',
  main_user_id: '主账号', profile_user_id: '子账号', group_id: '分组ID', proxy_type: '代理类型', proxy_port: '代理端口',
}
// 变更字段明细：字段: 旧值 → 新值（Cloud 旧值 vs 扫描新值）
function changedFieldDetails(scan, bitProfileId) {
  const item = (getDiff(scan).changed || []).find((d) => d.bit_profile_id === bitProfileId)
  const fields = item?.fields || []
  if (!fields.length) return ''
  const cloud = profiles.value.find((p) => p.bit_profile_id === bitProfileId) || {}
  const scanP = scan?.profiles?.find((p) => p.bit_profile_id === bitProfileId) || {}
  return fields.map((f) => {
    const label = CHANGED_FIELD_LABELS[f] || f
    const format = (src) => (f === 'proxy_host' ? `${src.proxy_type || 'http'}://${src.proxy_host}:${src.proxy_port || 0}` : src[f]) || '-'
    return `${label}: ${format(cloud)} → ${format(scanP)}`
  }).join('\n')
}

const diffColumns = [
  { colKey: "bit_profile_id", title: "Profile ID", width: 120 },
  { colKey: "name", title: "名称", width: 140 },
  { colKey: "group_name", title: "分组", width: 100 },
  { colKey: "proxy", title: "代理", width: 140 },
  { colKey: "remark", title: "备注", width: 120 },
]

const changedColumns = [...diffColumns, { colKey: "fields", title: "变更字段", width: 240 }]

</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
  <div class="wt-resource-page">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-alert v-if="taskNotice" :message="taskNotice" theme="info" style="margin-bottom:16px" closable @close="taskNotice=''" />
    <t-alert v-if="identityError" :message="identityError" theme="warning" style="margin-bottom:16px" closable @close="identityError=''" />
    <t-alert
      v-if="!isDesktopClient"
      message="当前为Cloud Web：仅展示Cloud已保存的浏览器窗口信息。扫描、Diff处理、打开/关闭和创建窗口请在Desktop客户端执行。"
      theme="info"
      style="margin-bottom:16px"
    />

    <ResourcePageHeader title="浏览器窗口" description="管理 BitBrowser 浏览环境、窗口状态和账号绑定">
      <template #actions>
        <t-button v-if="isDesktopClient" theme="primary" @click="openCreateDialog">新建窗口</t-button>
        <t-button v-if="isDesktopClient" class="wt-secondary-button" variant="outline" :loading="scanning" @click="triggerScan">扫描本机窗口</t-button>
        <template v-if="isDesktopClient">
          <t-dropdown trigger="click">
            <t-button class="wt-secondary-button" variant="outline" :disabled="!selectedRowKeys.length || Boolean(batchOperating)">批量操作</t-button>
            <t-dropdown-menu>
              <t-dropdown-item @click="batchOpenWindows">批量打开</t-dropdown-item>
              <t-dropdown-item @click="batchCloseWindows">批量关闭</t-dropdown-item>
              <t-dropdown-item :disabled="batchProxyBinding" @click="openBatchProxyBinding">批量绑定代理</t-dropdown-item>
            </t-dropdown-menu>
          </t-dropdown>
        </template>
        <t-button class="wt-secondary-button" variant="outline" @click="loadProfiles">刷新</t-button>
      </template>
    </ResourcePageHeader>

    <ResourceStatGrid :items="profileStatItems" @select="applyProfileStatFilter" />

    <ResourceCard class="window-resource-card">
      <div class="filter-bar">
        <t-space wrap>
          <t-input v-model="filterId" clearable placeholder="ID" style="width:90px" />
          <t-input v-model="filterName" clearable placeholder="名称" style="width:120px" />
          <t-select v-model="filterGroup" clearable placeholder="分组" style="width:140px" :options="groupFilterOptions" filterable />
          <t-input v-model="filterBitId" clearable placeholder="Bit ID" style="width:170px" />
          <t-input v-model="filterRemark" clearable placeholder="备注" style="width:110px" />
          <t-select v-model="businessFilter" clearable placeholder="状态" style="width:90px">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select>
          <t-select v-model="runningFilter" clearable placeholder="运行" style="width:90px">
            <t-option value="open" label="打开" />
            <t-option value="closed" label="关闭" />
          </t-select>
          <t-select v-model="userFilter" clearable placeholder="授权用户" style="width:120px" :options="userFilterOptions" filterable />
          <t-select v-model="statusFilter" clearable placeholder="Cloud状态" style="width:110px">
            <t-option value="active" label="可用" />
            <t-option value="local_missing" label="本机缺失" />
          </t-select>
        </t-space>
      </div>
      <div class="table-scroll-wrap">
      <t-table
        class="wt-resource-table"
        :data="pagedProfiles"
        :columns="columns"
        row-key="id"
        size="small"
        hover
        :scroll="{ x: 'max-content' }"
        v-model:selected-row-keys="selectedRowKeys"
        empty="暂无浏览器窗口"
      >
        <template #id="{ row }">
          <span>{{ row.id }}</span>
        </template>
        <template #name="{ row }">
          <div class="profile-name">{{ row.name || '-' }}</div>
        </template>
        <template #proxy="{ row }">
          <div>{{ proxySummary(row) }}</div>
          <ResourceStatusBadge v-if="proxyNeedsSync(row)" tone="warning" label="待同步" />
        </template>
        <template #remark="{ row }">
          <div class="remark-cell">
            <div v-if="row.remark" class="remark-bit">{{ row.remark }}</div>
            <div v-if="row.cloud_remark" class="remark-cloud">{{ row.cloud_remark }}</div>
            <span v-if="!row.remark && !row.cloud_remark">-</span>
          </div>
        </template>
        <template #business_status="{ row }">
          <ResourceStatusBadge :tone="row.business_status === 'disabled' ? 'danger' : 'success'" :label="businessStatusLabel(row.business_status)" />
        </template>
        <template #running="{ row }">
          <ResourceStatusBadge :tone="isWindowOpen(row) ? 'success' : (openStates[row.id] === false ? 'neutral' : 'warning')" :label="openStateLabel(row)" />
        </template>
        <template #last_synced_at="{ row }">{{ formatTime(row.last_synced_at) }}</template>
        <template #op="{ row }">
          <t-space class="wt-resource-actions">
            <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
            <t-button v-if="isDesktopClient && !isWindowOpen(row)" size="small" theme="primary" :loading="operatingProfileId === `open:${row.bit_profile_id}`" :disabled="row.business_status === 'disabled' || row.local_status !== 'active' || Boolean(operatingProfileId)" @click="openProfile(row)">打开</t-button>
            <t-dropdown v-if="isDesktopClient || isAdmin" trigger="click">
              <t-button size="small" class="wt-secondary-button" variant="outline">更多</t-button>
              <t-dropdown-menu>
                <t-dropdown-item v-if="isAdmin" @click="openAssignProfile(row)">分配</t-dropdown-item>
                <template v-if="isDesktopClient">
                  <t-dropdown-item :disabled="!isWindowOpen(row) || row.business_status === 'disabled' || row.local_status !== 'active' || Boolean(operatingProfileId)" @click="closeProfile(row)">关闭</t-dropdown-item>
                  <t-dropdown-item :disabled="row.business_status === 'disabled' || row.local_status !== 'active' || Boolean(operatingProfileId)" @click="openProxyBinding(row)">绑定代理</t-dropdown-item>
                  <t-dropdown-item :disabled="Boolean(operatingProfileId)" @click="openEdit(row)">编辑</t-dropdown-item>
                  <t-dropdown-item :disabled="row.local_status !== 'active' || Boolean(operatingProfileId)" @click="toggleBusinessStatus(row)">{{ row.business_status === 'disabled' ? '启用' : '停用' }}</t-dropdown-item>
                </template>
              </t-dropdown-menu>
            </t-dropdown>
          </t-space>
        </template>
      </t-table>
      </div>
      <div class="pagination-bar">
        <t-pagination
          v-model:current="pagination.current"
          v-model:pageSize="pagination.pageSize"
          :total="filteredProfiles.length"
          :page-size-options="[10, 20, 50, 100]"
          show-jumper
        />
      </div>
    </ResourceCard>

    <!-- 新建窗口 -->
    <t-dialog v-model:visible="showCreate" header="新建BitBrowser窗口" @confirm="createProfile" :confirm-btn="{ loading: creating, theme: 'primary', content: '创建并同步' }">
      <t-alert
        message="新建窗口会先调用本机BitBrowser创建，创建成功后立即读回并同步Cloud镜像；不会只创建Cloud假记录。"
        theme="info"
        style="margin-bottom:12px"
      />
      <t-form>
        <t-form-item label="名称">
          <t-input v-model="newProfile.name" placeholder="窗口名称" />
        </t-form-item>
        <t-form-item label="BitBrowser分组">
          <t-select
            v-model="newProfile.group_id"
            :loading="loadingGroups"
            :options="groupOptions"
            placeholder="请选择真实BitBrowser分组"
            filterable
            @change="value => { const group = profileGroups.find(item => item.id === value); newProfile.group_name = group?.name || '' }"
          />
        </t-form-item>
        <t-form-item label="备注">
          <t-input v-model="newProfile.remark" placeholder="可选，写入BitBrowser备注" />
        </t-form-item>
      </t-form>
      <t-alert v-if="createError" :message="createError" theme="error" style="margin-top:12px" />
    </t-dialog>

    <t-dialog v-model:visible="proxyDialogVisible" header="绑定代理并同步 BitBrowser" @confirm="confirmProxyBinding" :confirm-btn="{ loading: proxyBinding, theme: 'primary', content: '写入并读回验证' }">
      <t-alert theme="info" style="margin-bottom:12px">选择代理后会写入当前窗口的 BitBrowser 配置；只有协议、地址和端口读回一致，Cloud 才更新正式关联与配额。</t-alert>
		<t-alert v-if="proxyBindingHint" :message="proxyBindingHint" theme="warning" style="margin-bottom:12px" />
      <t-form label-width="84px">
        <t-form-item label="窗口"><span>{{ proxyProfile?.name || proxyProfile?.bit_profile_id }}</span></t-form-item>
        <t-form-item label="代理">
          <t-select v-model="proxyId" clearable placeholder="留空则解绑当前代理">
            <t-option v-for="item in availableProxies" :key="item.id" :value="item.id" :label="`${item.proxy_protocol}://${item.host}:${item.port}（剩余 ${item.remaining_profile_count ?? 0}）`" />
          </t-select>
			<t-empty v-if="!availableProxies.length" description="暂无可绑定代理" style="margin-top:12px" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog v-model:visible="batchProxyDialogVisible" header="批量绑定代理并同步 BitBrowser" @confirm="confirmBatchProxyBinding" :confirm-btn="{ loading: batchProxyBinding, theme: 'primary', content: '确认逐项写入并读回' }">
      <t-alert theme="info" style="margin-bottom:12px">系统先按已启用、检测正常、未过期及总剩余配额推荐代理。确认后逐窗口执行写入与读回；失败窗口不会建立Cloud正式关联。</t-alert>
      <t-alert v-if="batchProxyBindingHint" :message="batchProxyBindingHint" :theme="batchProxyCandidates.length ? 'info' : 'warning'" style="margin-bottom:12px" />
      <t-form label-width="84px">
        <t-form-item label="已选窗口"><span>{{ batchProxyTargets.length }} 个</span></t-form-item>
        <t-form-item label="代理">
          <t-select v-model="batchProxyId" placeholder="选择推荐代理">
            <t-option v-for="item in batchProxyCandidates" :key="item.id" :value="item.id" :label="`${item.proxy_protocol}://${item.host}:${item.port}（剩余 ${item.remaining_profile_count ?? 0}）`" />
          </t-select>
          <t-empty v-if="!batchProxyCandidates.length" description="暂无可容纳全部所选窗口的代理" style="margin-top:12px" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 编辑窗口 -->
    <t-dialog v-model:visible="showEdit" header="编辑浏览器窗口" @confirm="saveEdit" :confirm-btn="{ loading: editing, theme: 'primary', content: '保存' }">
      <t-alert
        message="仅更新Cloud窗口备注（扫描不会覆盖备注）；BitBrowser侧窗口配置（名称/分组/代理）的编辑为后续能力。"
        theme="info"
        style="margin-bottom:12px"
      />
      <t-form>
        <t-form-item label="窗口">
          <span>{{ editProfile?.name || editProfile?.bit_profile_id || '-' }}</span>
        </t-form-item>
        <t-form-item label="Cloud备注">
          <t-input v-model="editRemark" placeholder="窗口Cloud备注" />
        </t-form-item>
      </t-form>
      <t-alert v-if="editError" :message="editError" theme="error" style="margin-top:12px" />
    </t-dialog>

    <!-- 详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="浏览器窗口详情" :size="'560px'" destroy-on-close :footer="false">
      <t-descriptions v-if="detailProfile" :column="1" bordered size="small">
        <t-descriptions-item label="ID">{{ detailProfile.id }}</t-descriptions-item>
        <t-descriptions-item label="BitBrowser窗口ID">{{ detailProfile.bit_profile_id }}</t-descriptions-item>
        <t-descriptions-item label="名称">{{ detailProfile.name || '-' }}</t-descriptions-item>
        <t-descriptions-item label="分组">{{ detailProfile.group_name || '-' }}</t-descriptions-item>
        <t-descriptions-item label="比特序号">{{ detailProfile.seq || '-' }}</t-descriptions-item>
        <t-descriptions-item label="业务状态">{{ businessStatusLabel(detailProfile.business_status) }}</t-descriptions-item>
        <t-descriptions-item label="Cloud状态">
          <BusinessStatus :status="detailProfile.local_status === 'active' ? 'normal' : 'stopped'" :label="statusLabel(detailProfile.local_status)" />
        </t-descriptions-item>
        <t-descriptions-item label="BitBrowser状态">{{ bitStatusLabel(detailProfile.bit_status) }}</t-descriptions-item>
        <t-descriptions-item label="已保存主账号">{{ maskMainUserId(detailProfile.main_user_id) }}</t-descriptions-item>
        <t-descriptions-item label="授权用户">{{ operatorLabel(detailProfile.user_id) }}</t-descriptions-item>
        <t-descriptions-item label="代理">{{ proxySummary(detailProfile) }}</t-descriptions-item>
        <t-descriptions-item label="备注"><span class="detail-remark">{{ remarkDisplay(detailProfile) || '-' }}</span></t-descriptions-item>
        <t-descriptions-item label="最后同步">{{ formatTime(detailProfile.last_synced_at) }}</t-descriptions-item>
        <t-descriptions-item label="创建时间">{{ formatTime(detailProfile.created_at) }}</t-descriptions-item>
        <t-descriptions-item label="更新时间">{{ formatTime(detailProfile.updated_at) }}</t-descriptions-item>
      </t-descriptions>
    </t-drawer>

    <!-- Cloud窗口授权 -->
    <t-dialog
      v-model:visible="showAssign"
      header="分配浏览器窗口"
      :confirm-btn="{ loading: assigning, theme: 'primary' }"
      @confirm="assignOwner"
    >
      <t-alert
        message="只更新Cloud窗口授权关系，不操作本机BitBrowser。已绑定媒体账号的窗口不能直接分配。"
        theme="warning"
        style="margin-bottom:12px"
      />
      <t-form>
        <t-form-item label="窗口">
          <span>{{ assignProfile?.name || assignProfile?.bit_profile_id || '-' }}</span>
        </t-form-item>
        <t-form-item label="授权给">
          <t-select v-model="assignUserId" :options="operatorOptions" placeholder="选择普通运营" filterable />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 扫描 Diff 抽屉 -->
    <t-drawer v-model:visible="scanDetailVisible" header="本机扫描结果" :size="'700px'" destroy-on-close>
      <div v-if="currentScan">
        <t-alert :message="'状态: ' + currentScan.status + ' | 时间: ' + formatTime(currentScan.created_at)" theme="info" style="margin-bottom:16px" />
        <t-alert
          v-if="hasDiff(currentScan)"
          message="接受本地变化后，会更新Cloud窗口镜像中的名称、分组、代理摘要、BitBrowser备注等允许字段；不会覆盖授权用户、媒体账号绑定、游戏、标签、Cloud备注、Cookie 和业务状态。"
          theme="warning"
          style="margin-bottom:16px"
        />
        <t-alert
          v-else
          message="本机窗口与Cloud记录一致，无需处理。"
          theme="success"
          style="margin-bottom:16px"
        />
        <t-tabs v-model="selectedTab" :default-value="'changed'">
          <t-tab-panel value="added" label="新增">
            <t-table v-if="getDiff(currentScan).added?.length" :data="getDiff(currentScan).added.map(d => ({...d, name: profileName(currentScan, d.bit_profile_id), group_name: profileGroup(currentScan, d.bit_profile_id), proxy: profileProxy(currentScan, d.bit_profile_id), remark: profileRemark(currentScan, d.bit_profile_id)}))" :columns="diffColumns" size="small">
              <template #name="{ row }">{{ row.name || row.bit_profile_id }}</template>
              <template #group_name="{ row }">{{ row.group_name }}</template>
              <template #proxy="{ row }">{{ row.proxy }}</template>
              <template #remark="{ row }">{{ row.remark }}</template>
            </t-table>
            <t-empty v-else description="无新增" />
          </t-tab-panel>
          <t-tab-panel value="changed" label="变更">
            <t-table v-if="getDiff(currentScan).changed?.length" :data="getDiff(currentScan).changed.map(d => ({...d, name: profileName(currentScan, d.bit_profile_id), group_name: profileGroup(currentScan, d.bit_profile_id), proxy: profileProxy(currentScan, d.bit_profile_id), remark: profileRemark(currentScan, d.bit_profile_id)}))" :columns="changedColumns" size="small">
              <template #name="{ row }">{{ row.name || row.bit_profile_id }}</template>
              <template #group_name="{ row }">{{ row.group_name }}</template>
              <template #proxy="{ row }">{{ row.proxy }}</template>
              <template #remark="{ row }">{{ row.remark }}</template>
              <template #fields="{ row }">
                <div class="diff-fields">{{ changedFieldDetails(currentScan, row.bit_profile_id) || '-' }}</div>
              </template>
            </t-table>
            <t-empty v-else description="无变更" />
          </t-tab-panel>
          <t-tab-panel value="missing" label="缺失">
            <t-table v-if="getDiff(currentScan).missing?.length" :data="getDiff(currentScan).missing.map(d => ({...d, name: profileName(currentScan, d.bit_profile_id), group_name: profileGroup(currentScan, d.bit_profile_id), proxy: profileProxy(currentScan, d.bit_profile_id), remark: profileRemark(currentScan, d.bit_profile_id)}))" :columns="diffColumns" size="small">
              <template #name="{ row }">{{ row.name || row.bit_profile_id }}</template>
              <template #group_name="{ row }">{{ row.group_name }}</template>
              <template #proxy="{ row }">{{ row.proxy }}</template>
              <template #remark="{ row }">{{ row.remark }}</template>
            </t-table>
            <t-empty v-else description="无缺失" />
          </t-tab-panel>
        </t-tabs>
        <t-divider>代理关系差异</t-divider>
        <t-table :data="localProxyScan?.changes || []" row-key="profile_id" size="small" :columns="[
          { colKey: 'kind', title: '差异', width: 120 }, { colKey: 'bit_profile_id', title: '窗口' }, { colKey: 'host', title: '本机代理' }, { colKey: 'current_proxy_id', title: '当前关联' },
        ]" empty="本机代理与Cloud记录一致">
          <template #kind="{ row }"><t-tag :theme="row.kind === 'conflict' ? 'danger' : row.kind === 'unknown' ? 'warning' : 'primary'">{{ { changed: '已更换', unbound: '已解绑', unknown: '未登记代理', conflict: '匹配冲突' }[row.kind] || row.kind }}</t-tag></template>
          <template #host="{ row }">{{ row.host ? `${row.proxy_protocol}://${row.host}:${row.port}` : '-' }}</template>
          <template #current_proxy_id="{ row }">{{ row.current_proxy_id || '-' }}</template>
        </t-table>
      </div>
      <template #footer>
        <t-space>
          <t-button variant="outline" @click="scanDetailVisible = false">关闭</t-button>
          <t-button v-if="currentScan?.status === 'ready' && hasDiff(currentScan)" theme="primary" :loading="acceptingScan" @click="acceptLocalChanges">接受本地变化</t-button>
          <t-button v-if="currentScan?.status === 'ready' && hasDiff(currentScan)" theme="default" :loading="restoringCloud" @click="restoreCloudConfig">恢复Cloud配置并读回验证</t-button>
        </t-space>
      </template>
    </t-drawer>

    <t-dialog
      v-model:visible="acceptConfirmVisible"
      header="接受本地变化"
      :confirm-btn="{ loading: acceptingScan, theme: 'primary', content: '确认接受' }"
      :cancel-btn="{ disabled: acceptingScan }"
      @confirm="confirmAcceptLocalChanges"
    >
      <t-alert
        theme="warning"
        message="接受后只更新Cloud窗口镜像中的允许字段（含BitBrowser备注），不会覆盖授权用户、媒体账号绑定、游戏、标签、Cloud备注、Cookie和业务状态。"
        style="margin-bottom:12px"
      />
      <p>{{ acceptConfirmMessage }}</p>
    </t-dialog>

    <t-dialog
      v-model:visible="restoreConfirmVisible"
      header="恢复Cloud配置"
      :confirm-btn="{ loading: restoringCloud, theme: 'primary', content: '确认恢复' }"
      :cancel-btn="{ disabled: restoringCloud }"
      @confirm="confirmRestoreCloudConfig"
    >
      <t-alert
        theme="warning"
        message="恢复会把Cloud已保存的窗口配置写回本机BitBrowser，并在写回后重新扫描验证；不会写入账号、Cookie、授权用户或业务状态。"
        style="margin-bottom:12px"
      />
      <p>{{ restoreConfirmMessage }}</p>
    </t-dialog>
  </div>
  </t-loading>
</template>

<style scoped>
.filter-bar { margin-bottom: 12px; }
.profile-name { font-weight: 500; }
.remark-cell { line-height: 1.5; }
.remark-bit { color: #999; }
.remark-cloud { color: #0052d9; }
.detail-remark { white-space: pre-line; }
.diff-fields { white-space: pre-line; color: #e34d59; line-height: 1.6; }
.table-scroll-wrap { overflow-x: auto; width: 100%; }
.pagination-bar { margin-top: 12px; display: flex; justify-content: flex-end; }
.window-resource-card { padding: 18px 20px; }
.window-resource-card :deep(.resource-status-badge) { margin-top: 4px; }
</style>
