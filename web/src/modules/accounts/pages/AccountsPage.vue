<script setup>
import { computed, onMounted, ref } from "vue"
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import { createSessionClient } from "../../../shared/api/session.js"
import { createUsersClient } from "../../../apps/cloud/pages/users/usersApi.js"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"
// Static import: a dynamic import() of a node_modules bare specifier does not
// resolve in the packaged Tauri WebView ("Module name ... does not resolve to a
// valid file"). invoke is only called inside __TAURI_INTERNALS__-guarded code,
// so importing it is inert in Cloud Web.
import { invoke } from "@tauri-apps/api/core"

const accountClient = createMediaAccountClient()
const profileClient = createProfileBindingClient()
const sessionClient = createSessionClient()
const usersClient = createUsersClient()

const isDesktop = window.__WT_MEDIA_APP__ === "desktop"

const user = ref(null)
const accounts = ref([])
const profiles = ref([])
const games = ref([])
const loading = ref(true)
const error = ref("")

const searchText = ref("")
const searchPlatform = ref("")
const searchBizStatus = ref("")
const searchLoginStatus = ref("")
const searchTags = ref("")
const searchGameId = ref("")
const pagination = ref({ current: 1, pageSize: 20 })

const showCreate = ref(false)
const createForm = ref(defaultCreateForm())
const creating = ref(false)

const detailVisible = ref(false)
const detailAccount = ref(null)
const detailRemark = ref("")
const detailProfileId = ref("")
const savingDetail = ref(false)
const checkingAccount = ref(false)
const checkNotice = ref("")
const readingCookie = ref(false)
const cookieNotice = ref("")
const showCookieDialog = ref(false)
const cookieDialog = ref({ original_cookie: "", active_cookie: "" })
const editVisible = ref(false)
const editAccount = ref(null)
const editRemark = ref("")
const editGameId = ref("")
const editTagsText = ref("")
const savingEdit = ref(false)
const batchChecking = ref(false)
const batchResults = ref([])
const operatingProfileId = ref("")
const currentCheckingId = ref("")
const profileOpenMap = ref({})
const accountInfoPopup = ref(null)
const windowPopup = ref(null)
const allTagsPopup = ref(null)

const tagInput = ref("")
const selectedAccountIds = ref([])
const tagManageVisible = ref(false)
const tagManageList = ref([])

let lastRuntimeRefreshAt = 0
let lastRuntimeStatus = null

onMounted(async () => {
  try {
    user.value = await sessionClient.me()
    await Promise.all([loadAuxiliaryData(), loadAccounts()])
  } catch (e) {
    error.value = e.message || "加载账号数据失败"
  } finally {
    loading.value = false
  }
})

function defaultCreateForm() {
  return {
    userId: "",
    gameId: "",
    platform: "bilibili",
    browserProfileId: "",
    remark: "",
    tagsText: "",
  }
}

async function loadAuxiliaryData() {
  const [profileList, gameList] = await Promise.all([
    profileClient.listProfiles().catch(() => []),
    usersClient.listGames({ status: "enabled" }).catch(() => []),
  ])
  profiles.value = Array.isArray(profileList) ? profileList : []
  games.value = Array.isArray(gameList) ? gameList : []
  if (!createForm.value.gameId) {
    createForm.value.gameId = (user.value?.game_ids || [])[0] || games.value[0]?.id || ""
  }
}

async function loadAccounts() {
  error.value = ""
  const params = {}
  if (searchText.value) params.search = searchText.value
  if (searchPlatform.value) params.platform = searchPlatform.value
  if (searchBizStatus.value) params.businessStatus = searchBizStatus.value
  if (searchLoginStatus.value) params.loginStatus = searchLoginStatus.value
  if (searchGameId.value) params.game_id = searchGameId.value
  if (searchTags.value) params.anyTags = [searchTags.value]
  try {
    accounts.value = await accountClient.list(params)
  } catch (e) {
    error.value = e.message || "加载媒体账号失败"
  }
}

function resetFilters() {
  searchText.value = ""
  searchPlatform.value = ""
  searchBizStatus.value = ""
  searchLoginStatus.value = ""
  searchTags.value = ""
  searchGameId.value = ""
  loadAccounts()
}


function openCreate() {
  createForm.value = defaultCreateForm()
  createForm.value.gameId = (user.value?.game_ids || [])[0] || games.value[0]?.id || ""
  showCreate.value = true
}

async function createAccount() {
  error.value = ""
  creating.value = true
  try {
    await accountClient.create({
      userId: user.value?.role === "admin" ? Number(createForm.value.userId) || undefined : undefined,
      gameId: createForm.value.gameId,
      platform: createForm.value.platform,
      browserProfileId: isDesktop ? createForm.value.browserProfileId : "",
      remark: createForm.value.remark,
      tags: splitTags(createForm.value.tagsText),
    })
    showCreate.value = false
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "创建媒体账号失败"
  } finally {
    creating.value = false
  }
}

function openDetail(account) {
  detailAccount.value = account
  detailRemark.value = account.remark || ""
  detailProfileId.value = account.browser_profile_id || ""
  checkNotice.value = ""
  detailVisible.value = true
}

function accountDisplayName(account) {
  if (!account) return "-"
  if (account.name) return account.name
  if (account.platform_account_id) return account.platform_account_id
  return account.identification_status !== "identified" ? "待识别账号" : "未识别"
}

async function copyAccountId(account) {
  if (!account?.id) return
  try {
    await navigator.clipboard.writeText(account.id)
    cookieNotice.value = "已复制系统账号 ID"
  } catch (e) {
    error.value = "复制失败，请手动选择"
  }
}

function openAccountInfoPopup(account) {
  accountInfoPopup.value = account
}

function showAllTags(account) {
  allTagsPopup.value = account
}

function openWindowPopup(account) {
  windowPopup.value = account
}

function profileIsOpen(account) {
  const profile = profileForAccount(account)
  if (!profile?.bit_profile_id) return false
  return Boolean(profileOpenMap.value[profile.bit_profile_id])
}

async function toggleWindow(account) {
  const profile = profileForAccount(account)
  if (!profile?.bit_profile_id) return
  const isOpen = profileIsOpen(account)
  try {
    if (isOpen) {
      await closeAccountProfile(account)
      profileOpenMap.value[profile.bit_profile_id] = false
    } else {
      await openAccountProfile(account)
      profileOpenMap.value[profile.bit_profile_id] = true
    }
  } catch (e) {
    error.value = e.message || "窗口操作失败"
  }
}

async function checkFromRow(account) {
  if (!canCheckAccount(account)) {
    error.value = executableText(account)
    return
  }
  currentCheckingId.value = account.id
  error.value = ""
  try {
    const service = await desktopLocalAgentService()
    const updated = await runSingleAccountCheck(account, service)
    const idx = accounts.value.findIndex(a => a.id === account.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated }
    checkNotice.value = `检查完成：${loginStatusText(updated.login_status)}`
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "账号检查失败"
  } finally {
    currentCheckingId.value = ""
  }
}

function openTagManage() {
  const count = {}
  accounts.value.forEach(a => (a.tags || []).forEach(t => { count[t] = (count[t] || 0) + 1 }))
  tagManageList.value = Object.entries(count).map(([name, num]) => ({ name, num })).sort((a, b) => b.num - a.num)
  tagManageVisible.value = true
}

async function deleteTagFromAll(tag) {
  try {
    const affected = accounts.value.filter(a => (a.tags || []).includes(tag))
    if (!affected.length) return
    await accountClient.removeTags(affected.map(a => a.id), [tag])
    await loadAccounts()
    await openTagManage()
  } catch (e) {
    error.value = e.message || "删除标签失败"
  }
}

async function batchSetBizStatus(status) {
  const ids = selectedAccountIds.value
  if (!ids.length) return
  try {
    await Promise.all(ids.map(id => accountClient.update(id, { businessStatus: status })))
    selectedAccountIds.value = []
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "批量状态更新失败"
  }
}

function openEdit(account) {
  editAccount.value = account
  editRemark.value = account.remark || ""
  editGameId.value = account.game_id || ""
  editTagsText.value = (account.tags || []).join(",")
  editVisible.value = true
}

async function saveEdit() {
  if (!editAccount.value) return
  savingEdit.value = true
  error.value = ""
  try {
    const updated = await accountClient.update(editAccount.value.id, {
      remark: editRemark.value,
      gameId: editGameId.value || "",
    })
    // 标签差异：先加后删
    const oldTags = new Set(editAccount.value.tags || [])
    const newTags = splitTags(editTagsText.value)
    const toAdd = newTags.filter(t => !oldTags.has(t))
    const toRemove = [...oldTags].filter(t => !newTags.includes(t))
    if (toAdd.length) await accountClient.addTags([editAccount.value.id], toAdd)
    if (toRemove.length) await accountClient.removeTags([editAccount.value.id], toRemove)
    const idx = accounts.value.findIndex(a => a.id === editAccount.value.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated, tags: newTags }
    editVisible.value = false
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "保存失败"
  } finally {
    savingEdit.value = false
  }
}

async function toggleBizStatus(account) {
  const target = account.business_status === "enabled" ? "disabled" : "enabled"
  try {
    const updated = await accountClient.update(account.id, { businessStatus: target })
    const idx = accounts.value.findIndex(a => a.id === account.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated }
  } catch (e) {
    error.value = e.message || "状态切换失败"
  }
}

async function saveDetail() {
  if (!detailAccount.value) return
  savingDetail.value = true
  error.value = ""
  try {
    let updated = await accountClient.update(detailAccount.value.id, { remark: detailRemark.value })
    if (isDesktop && detailProfileId.value !== (detailAccount.value.browser_profile_id || "")) {
      updated = detailProfileId.value
        ? await accountClient.bindProfile(detailAccount.value.id, detailProfileId.value)
        : await accountClient.unbindProfile(detailAccount.value.id)
    }
    detailAccount.value = { ...detailAccount.value, ...updated }
    await loadAccounts()
    detailVisible.value = false
  } catch (e) {
    error.value = e.message || "保存账号失败"
  } finally {
    savingDetail.value = false
  }
}

async function updateStatus(account, bizStatus) {
  error.value = ""
  try {
    await accountClient.update(account.id, { businessStatus: bizStatus })
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "更新状态失败"
  }
}

async function changeTags(remove = false) {
  const tags = splitTags(tagInput.value)
  if (!selectedAccountIds.value.length || !tags.length) return
  error.value = ""
  try {
    if (remove) await accountClient.removeTags(selectedAccountIds.value, tags)
    else await accountClient.addTags(selectedAccountIds.value, tags)
    tagInput.value = ""
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "更新标签失败"
  }
}

function splitTags(value) {
  return String(value || "").split(",").map(t => t.trim()).filter(Boolean)
}

function gameName(gameId) {
  return games.value.find(game => game.id === gameId)?.name || gameId || "-"
}

function profileLabel(profileId) {
  const profile = profiles.value.find(item => item.id === profileId)
  if (!profile) return profileId || "-"
  const name = profile.name || "未命名窗口"
  const bitId = profile.bit_profile_id || "-"
  return `系统ID ${profile.id} / ${name} / BitBrowser ${bitId}`
}

function profileName(profileId) {
  const profile = profiles.value.find(item => item.id === profileId)
  if (!profile) return "未命名窗口"
  return profile.name || "未命名窗口"
}

function profileForAccount(account) {
  return profiles.value.find(item => item.id === account?.browser_profile_id)
}

function executableText(account) {
  if (account.business_status !== "enabled") return "不可执行：账号已停用"
  if (!account.game_id) return "不可执行：未绑定游戏"
  if (!account.browser_profile_id) return "不可执行：未绑定窗口"
  const profile = profileForAccount(account)
  if (profile && (profile.local_status !== "active" || profile.business_status === "disabled")) return "不可执行：绑定窗口已停用"
  if (account.login_status !== "normal") return "待检查：需要真实账号检查"
  return "可进入后续预检"
}

function loginStatusText(status) {
  return {
    unknown: "待检查",
    normal: "登录正常",
    not_logged_in: "未登录",
    verification_needed: "需要验证码",
    expired: "已失效",
    restricted: "受限",
    account_mismatch: "账号不一致",
    environment_error: "环境异常",
  }[status] || status || "-"
}

function bizStatusText(status) {
  return {
    enabled: "启用",
    disabled: "停用",
  }[status] || status || "-"
}

// 账号状态：真实平台状态，由 login_status + identification_status 派生（与业务状态独立）
const ACCOUNT_STATUS_MAP = {
  pending: { label: "待上号", color: "gray" },
  normal: { label: "正常", color: "green" },
  not_logged_in: { label: "未登录", color: "orange" },
  expired: { label: "登录失效", color: "orange" },
  account_mismatch: { label: "账号不匹配", color: "red" },
  restricted: { label: "账号受限", color: "red" },
  check_failed: { label: "检查失败", color: "red" },
}

function accountStatusKey(account) {
  if (!account) return "pending"
  const idStatus = account.identification_status || ""
  const login = account.login_status || ""
  if (login === "normal") return "normal"
  switch (login) {
    case "not_logged_in": return "not_logged_in"
    case "expired": return "expired"
    case "account_mismatch": return "account_mismatch"
    case "restricted": case "verification_needed": return "restricted"
    case "environment_error": case "unknown": return "check_failed"
    default:
      // 未检查/待识别 → 待上号
      return idStatus !== "identified" ? "pending" : "check_failed"
  }
}

function accountStatusText(account) {
  return ACCOUNT_STATUS_MAP[accountStatusKey(account)]?.label || "待上号"
}

function accountStatusColor(account) {
  return ACCOUNT_STATUS_MAP[accountStatusKey(account)]?.color || "gray"
}

function relativeTime(ts) {
  if (!ts) return "从未检查"
  const diff = Date.now() - new Date(ts).getTime()
  if (diff < 0) return "刚刚"
  const min = Math.floor(diff / 60000)
  if (min < 1) return "刚刚"
  if (min < 60) return `${min}分钟前`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}小时前`
  const day = Math.floor(hr / 24)
  if (day === 1) return "昨天"
  return `${day}天前 · 建议检查`
}

function platformLabel(platform) {
  return { bilibili: "哔哩", baijiahao: "百度" }[platform] || platform || "-"
}

function profileOptionLabel(profile) {
  if (!profile) return ""
  const name = profile.name || profileLabel(profile.id)
  const parts = []
  if (profile.seq) parts.push(String(profile.seq))
  parts.push(name)
  if (profile.id) parts.push(String(profile.id))
  return parts.join(" - ")
}

function checkItemStatusText(status) {
  return {
    pass: "通过",
    fail: "未通过",
    skip: "跳过",
    na: "不适用",
  }[status] || status || "-"
}

function canCheckAccount(account) {
  // draft（待识别）与 enabled 均可发起检查：draft 需要检查完成真实识别后升级
  return isDesktop && account && (account.business_status === "enabled" || account.business_status === "draft") && !!account.game_id && !!account.browser_profile_id && canOperateBoundWindow(account)
}

function canOperateBoundWindow(account) {
  const profile = profileForAccount(account)
  return isDesktop && account?.browser_profile_id && profile?.bit_profile_id && profile.local_status === "active" && profile.business_status !== "disabled"
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

function cloudBaseUrl() {
  if (typeof window === "undefined") return "http://127.0.0.1:18080"
  const origin = window.location?.origin || "http://127.0.0.1:18080"
  // Packaged Desktop runs on http://tauri.localhost, which is NOT the Cloud
  // API host; the local Cloud server is always the API base.
  return origin.startsWith("http://127.0.0.1:18080") ? origin : "http://127.0.0.1:18080"
}

async function desktopLocalAgentService() {
  if (typeof window === "undefined" || !window.__TAURI_INTERNALS__) {
    throw new Error("账号检查只能在 Desktop 客户端执行")
  }
  return createLocalAgentService({ invoke })
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
  } catch (e) {
    if (localStatus?.node_id) {
      checkNotice.value = "本机状态已读取，Cloud可信状态刷新暂时失败；将继续由Cloud预检判断是否可执行。"
      lastRuntimeStatus = localStatus
      lastRuntimeRefreshAt = now
      return localStatus
    }
    throw e
  }
}

async function checkDetailAccount() {
  if (!detailAccount.value || !canCheckAccount(detailAccount.value)) return
  checkingAccount.value = true
  checkNotice.value = ""
  error.value = ""
  try {
    const service = await desktopLocalAgentService()
    const updated = await runSingleAccountCheck(detailAccount.value, service)
    detailAccount.value = { ...detailAccount.value, ...updated }
    checkNotice.value = `检查完成：${loginStatusText(updated.login_status)}`
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "账号检查失败"
  } finally {
    checkingAccount.value = false
  }
}

async function runSingleAccountCheck(account, service) {
  const status = await refreshRuntimeWithCooldown(service)
  const nodeId = status.node_id || ""
  if (!nodeId) {
    throw new Error("当前电脑尚未完成可信绑定，请先到环境状态页重新检测并绑定。")
  }
  const start = await accountClient.check(account.id, { nodeId })
  const localResult = await service.accountCheck({
    cloudBaseUrl: cloudBaseUrl(),
    taskId: start.task_id,
    bitProfileId: start.bit_profile_id,
    platform: start.platform,
    expectedPlatformAccountId: start.expected_platform_account_id || "",
  })
  return accountClient.submitCheckResult(account.id, {
    taskId: start.task_id,
    platformAccountId: localResult.platform_account_id,
    name: localResult.name,
    avatarUrl: localResult.avatar_url,
    loginStatus: localResult.login_status,
    message: localResult.message,
    checkItems: localResult.check_items || [],
  })
}

async function readProfileCookie() {
  if (!detailAccount.value || !detailAccount.value.browser_profile_id) return
  readingCookie.value = true
  cookieNotice.value = ""
  error.value = ""
  try {
    const service = await desktopLocalAgentService()
    const status = await refreshRuntimeWithCooldown(service)
    const nodeId = status.node_id || ""
    if (!nodeId) throw new Error("当前电脑尚未完成可信绑定，请先到环境状态页重新检测并绑定。")
    const start = await accountClient.startCookieReadSync(detailAccount.value.id, { nodeId })
    const localResult = await service.cookieRead({
      cloudBaseUrl: cloudBaseUrl(),
      taskId: start.task_id,
      bitProfileId: start.bit_profile_id,
    })
    const cookies = localResult.cookies || []
    const updated = await accountClient.submitCookieReadResult(detailAccount.value.id, {
      taskId: start.task_id,
      cookies,
    })
    detailAccount.value = { ...detailAccount.value, ...updated }
    cookieNotice.value = `已从 Profile 读回 ${cookies.length} 条真实 Cookie 并更新 active_cookie`
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "读取 Cookie 失败"
  } finally {
    readingCookie.value = false
  }
}

async function viewExportCookie() {
  if (!detailAccount.value) return
  try {
    const data = await accountClient.fetchCookies(detailAccount.value.id)
    cookieDialog.value = data || { original_cookie: "", active_cookie: "" }
    showCookieDialog.value = true
  } catch (e) {
    error.value = e.message || "获取 Cookie 失败"
  }
}

async function copyCookie(field) {
  const value = cookieDialog.value?.[field]
  if (!value) {
    error.value = "当前 Cookie 为空，无可复制内容"
    return
  }
  try {
    await navigator.clipboard.writeText(value)
    cookieNotice.value = `已复制 ${field === "active_cookie" ? "Active" : "原始"} Cookie 到剪贴板`
  } catch (e) {
    error.value = "复制失败，请手动选择复制"
  }
}

function accountLabel(account) {
  return account?.name || account?.platform_account_id || account?.id || "未知账号"
}

function selectedAccounts() {
  const selected = new Set(selectedAccountIds.value)
  return accounts.value.filter(account => selected.has(account.id))
}

function batchStats() {
  return {
    total: batchResults.value.length,
    success: batchResults.value.filter(item => item.status === "success").length,
    failed: batchResults.value.filter(item => item.status === "failed").length,
    skipped: batchResults.value.filter(item => item.status === "skipped").length,
    pending: batchResults.value.filter(item => item.status === "pending").length,
  }
}

async function runBatchCheck({ retryFailedOnly = false } = {}) {
  if (!isDesktop || batchChecking.value) return
  const candidates = retryFailedOnly
    ? batchResults.value.filter(item => item.status === "failed").map(item => item.account)
    : selectedAccounts()
  if (!candidates.length) return
  batchChecking.value = true
  error.value = ""
  checkNotice.value = ""
  const initialResults = candidates.map(account => {
    if (!canCheckAccount(account)) {
      return { account, status: "skipped", message: executableText(account) }
    }
    return { account, status: "pending", message: "等待检查" }
  })
  batchResults.value = retryFailedOnly
    ? batchResults.value.map(existing => {
        const retry = initialResults.find(item => item.account.id === existing.account.id)
        return retry || existing
      })
    : initialResults
  try {
    const service = await desktopLocalAgentService()
    for (const item of batchResults.value) {
      if (!candidates.some(account => account.id === item.account.id) || item.status !== "pending") continue
      try {
        const updated = await runSingleAccountCheck(item.account, service)
        item.status = "success"
        item.message = `检查完成：${loginStatusText(updated.login_status)}`
        item.account = { ...item.account, ...updated }
      } catch (e) {
        item.status = "failed"
        item.message = e.message || "检查失败"
      }
    }
    const stats = batchStats()
    checkNotice.value = `批量检查完成：成功 ${stats.success}，失败 ${stats.failed}，跳过 ${stats.skipped}`
    await loadAccounts()
  } catch (e) {
    error.value = e.message || "批量检查失败"
  } finally {
    batchChecking.value = false
  }
}

async function openAccountProfile(account) {
  const profile = profileForAccount(account)
  if (!canOperateBoundWindow(account)) return
  const bitProfileId = String(profile.bit_profile_id || "").trim()
  if (!bitProfileId) {
    error.value = "打开绑定窗口失败：Cloud记录缺少BitBrowser窗口ID，请先扫描并同步本机窗口。"
    return
  }
  operatingProfileId.value = `open:${bitProfileId}`
  error.value = ""
  checkNotice.value = `正在打开绑定窗口：${profile.name || bitProfileId}`
  try {
    const service = await desktopLocalAgentService()
    const result = await service.profileOpen(bitProfileId)
    checkNotice.value = `已打开绑定窗口：${profile.name || result.bit_profile_id}`
  } catch (e) {
    checkNotice.value = ""
    error.value = localTrustMessage(e)
  } finally {
    operatingProfileId.value = ""
  }
}

async function closeAccountProfile(account) {
  const profile = profileForAccount(account)
  if (!canOperateBoundWindow(account)) return
  const bitProfileId = String(profile.bit_profile_id || "").trim()
  if (!bitProfileId) {
    error.value = "关闭绑定窗口失败：Cloud记录缺少BitBrowser窗口ID，请先扫描并同步本机窗口。"
    return
  }
  operatingProfileId.value = `close:${bitProfileId}`
  error.value = ""
  checkNotice.value = `正在关闭绑定窗口：${profile.name || bitProfileId}`
  try {
    const service = await desktopLocalAgentService()
    const result = await service.profileClose(bitProfileId)
    checkNotice.value = `已关闭绑定窗口：${profile.name || result.bit_profile_id}`
  } catch (e) {
    checkNotice.value = ""
    error.value = localTrustMessage(e)
  } finally {
    operatingProfileId.value = ""
  }
}

function formatTime(t) {
  if (!t) return "-"
  return new Date(t).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

const activeProfiles = computed(() => profiles.value.filter(profile => profile.local_status === "active" && profile.business_status !== "disabled"))

const availableTags = computed(() => {
  const set = new Set()
  accounts.value.forEach(a => (a.tags || []).forEach(t => set.add(t)))
  return [...set].sort()
})

const stats = computed(() => ({
  total: accounts.value.length,
  enabled: accounts.value.filter(a => a.business_status === "enabled").length,
  disabled: accounts.value.filter(a => a.business_status === "disabled").length,
  normal: accounts.value.filter(a => accountStatusKey(a) === "normal").length,
  abnormal: accounts.value.filter(a => ["restricted", "account_mismatch", "check_failed"].includes(accountStatusKey(a))).length,
  pending: accounts.value.filter(a => ["pending", "not_logged_in", "expired"].includes(accountStatusKey(a))).length,
}))

const currentBatchStats = computed(batchStats)
const hasBatchFailures = computed(() => batchResults.value.some(item => item.status === "failed"))

const columns = [
  { colKey: "id", title: "ID", width: 70 },
  { colKey: "platform", title: "平台", width: 80 },
  { colKey: "account_info", title: "账号信息", width: 200 },
  { colKey: "game_id", title: "游戏", width: 90 },
  { colKey: "tags", title: "标签", width: 150 },
  { colKey: "window_info", title: "浏览器窗口", width: 190 },
  { colKey: "business_status", title: "业务状态", width: 80 },
  { colKey: "account_status", title: "账号状态", width: 90 },
  { colKey: "last_checked_at", title: "最近检查", width: 120 },
  { colKey: "op", title: "操作", width: 420, fixed: "right" },
]
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-alert
      v-if="!isDesktop"
        message="Cloud Web 只展示 Cloud 已保存的账号与窗口绑定信息；打开/关闭窗口、绑定/换绑和检查/同步账号信息请在 Desktop 端完成。"
      theme="info"
      style="margin-bottom:16px"
    />

    <t-row :gutter="16" class="stats-row">
      <t-col :span="4"><t-card><template #title>总账号</template><div class="stat-num">{{ stats.total }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>启用账号</template><div class="stat-num">{{ stats.enabled }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>停用账号</template><div class="stat-num">{{ stats.disabled }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>正常账号</template><div class="stat-num">{{ stats.normal }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>异常账号</template><div class="stat-num">{{ stats.abnormal }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>待检查</template><div class="stat-num">{{ stats.pending }}</div></t-card></t-col>
    </t-row>

    <t-card class="search-bar" :bordered="true">
      <t-form layout="inline">
        <t-form-item label="搜索">
          <t-input v-model="searchText" placeholder="账号、UID、备注" clearable style="width:180px" />
        </t-form-item>
        <t-form-item label="游戏">
          <t-select v-model="searchGameId" placeholder="全部" clearable style="width:140px">
            <t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" />
          </t-select>
        </t-form-item>
        <t-form-item label="平台">
          <t-select v-model="searchPlatform" placeholder="全部" clearable style="width:120px">
            <t-option value="bilibili" label="哔哩" />
            <t-option value="baijiahao" label="百度" />
          </t-select>
        </t-form-item>
        <t-form-item label="业务状态">
          <t-select v-model="searchBizStatus" placeholder="全部" clearable style="width:120px">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
          </t-select>
        </t-form-item>
        <t-form-item label="账号状态">
          <t-select v-model="searchLoginStatus" placeholder="全部" clearable style="width:130px">
            <t-option value="normal" label="正常" />
            <t-option value="not_logged_in" label="未登录" />
            <t-option value="expired" label="登录失效" />
            <t-option value="account_mismatch" label="账号不匹配" />
            <t-option value="restricted" label="账号受限" />
            <t-option value="environment_error" label="检查失败" />
          </t-select>
        </t-form-item>
        <t-form-item label="标签">
          <t-select v-model="searchTags" placeholder="全部" clearable style="width:140px">
            <t-option v-for="t in availableTags" :key="t" :value="t" :label="t" />
          </t-select>
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" @click="loadAccounts">查询</t-button>
          <t-button @click="resetFilters">重置</t-button>
        </t-form-item>
      </t-form>
    </t-card>

    <div class="action-bar">
      <t-space>
        <t-button theme="primary" @click="openCreate">新增账号</t-button>
        <t-button variant="outline" @click="openTagManage">标签管理</t-button>
        <t-button variant="outline" @click="() => { loadAuxiliaryData(); loadAccounts() }">刷新</t-button>
      </t-space>
      <t-space class="batch-actions">
        <t-input v-model="tagInput" placeholder="标签，多个用逗号分隔" style="width:180px" :disabled="!selectedAccountIds.length" />
        <t-button size="small" :disabled="!selectedAccountIds.length" @click="changeTags(false)">添加标签</t-button>
        <t-button size="small" :disabled="!selectedAccountIds.length" @click="changeTags(true)">移除标签</t-button>
        <t-button size="small" variant="outline" :disabled="!selectedAccountIds.length" @click="batchSetBizStatus('enabled')">批量启用</t-button>
        <t-button size="small" variant="outline" :disabled="!selectedAccountIds.length" @click="batchSetBizStatus('disabled')">批量停用</t-button>
        <t-button v-if="isDesktop" size="small" theme="primary" :loading="batchChecking" :disabled="!selectedAccountIds.length" @click="runBatchCheck()">批量检查/同步</t-button>
        <span class="selected-count">已选 {{ selectedAccountIds.length }} 项</span>
      </t-space>
    </div>

    <t-card v-if="batchResults.length" title="批量检查结果" :bordered="true" class="batch-result-card">
      <div class="batch-summary">
        共 {{ currentBatchStats.total }} 项，成功 {{ currentBatchStats.success }}，失败 {{ currentBatchStats.failed }}，跳过 {{ currentBatchStats.skipped }}，等待 {{ currentBatchStats.pending }}
        <t-button v-if="hasBatchFailures" size="small" variant="outline" :loading="batchChecking" @click="runBatchCheck({ retryFailedOnly: true })">重试失败项</t-button>
      </div>
      <div class="batch-result-list">
        <div v-for="item in batchResults" :key="item.account.id" class="batch-result-item">
          <span class="batch-account">{{ accountLabel(item.account) }}</span>
          <BusinessStatus
            :status="item.status === 'success' ? 'normal' : item.status === 'pending' ? 'pending_review' : item.status === 'skipped' ? 'paused' : 'warning'"
            :label="{ success: '成功', failed: '失败', skipped: '跳过', pending: '等待' }[item.status]"
          />
          <span class="batch-message">{{ item.message }}</span>
        </div>
      </div>
    </t-card>

    <t-table
      :data="accounts"
      :columns="columns"
      :selected-row-keys="selectedAccountIds"
      :row-key="(r) => r.id"
      @select-change="(keys) => { selectedAccountIds = keys }"
      size="small"
      hover
      v-model:pagination="pagination"
      :pagination="{ ...pagination, total: accounts.length, showJumper: true }"
      empty="暂无媒体账号"
    >
      <template #id="{ row }"><span class="account-id" @click="copyAccountId(row)">{{ row.id }}</span></template>
      <template #platform="{ row }"><span>{{ platformLabel(row.platform) }}</span></template>
      <template #account_info="{ row }">
        <div>
          <a class="account-name-link" @click="openAccountInfoPopup(row)">{{ accountDisplayName(row) }}</a>
          <div class="account-sub">UID：{{ row.platform_account_id || '--' }}</div>
        </div>
      </template>
      <template #game_id="{ row }">{{ gameName(row.game_id) }}</template>
      <template #tags="{ row }">
        <div class="account-tags">
          <t-tag v-for="tag in (row.tags || []).slice(0, 2)" :key="tag" size="small" variant="light">{{ tag }}</t-tag>
          <t-tag v-if="(row.tags || []).length > 2" size="small" variant="light" class="tags-more" @click="showAllTags(row)">+{{ (row.tags || []).length - 2 }}</t-tag>
        </div>
      </template>
      <template #window_info="{ row }">
        <div v-if="row.browser_profile_id" class="window-cell" @click="openWindowPopup(row)">
          <div class="window-proxy"><span class="proxy-dot proxy-ok">●</span> 代理正常</div>
          <div class="window-proxy"><span class="proxy-dot proxy-ok">●</span> 窗口正常</div>
          <div class="window-name window-name-link">{{ profileName(row.browser_profile_id) }}</div>
        </div>
        <div v-else class="window-cell"><span class="window-unbound">未绑定窗口</span></div>
      </template>
      <template #business_status="{ row }">
        <span class="biz-status" :class="row.business_status === 'enabled' ? 'biz-enabled' : 'biz-disabled'">{{ bizStatusText(row.business_status) }}</span>
      </template>
      <template #account_status="{ row }">
        <span class="account-status" :class="'status-' + accountStatusColor(row)">{{ accountStatusText(row) }}</span>
      </template>
      <template #last_checked_at="{ row }">
        <span :title="row.last_checked_at ? formatTime(row.last_checked_at) : ''">{{ relativeTime(row.last_checked_at) }}</span>
      </template>
      <template #op="{ row }">
        <t-space size="small" class="op-cell">
          <template v-if="isDesktop && canOperateBoundWindow(row)">
            <t-button size="small" variant="text" :loading="operatingProfileId === `open:${profileForAccount(row)?.bit_profile_id || ''}` || operatingProfileId === `close:${profileForAccount(row)?.bit_profile_id || ''}`" @click="toggleWindow(row)">{{ profileIsOpen(row) ? '关闭窗口' : '打开窗口' }}</t-button>
          </template>
          <t-button size="small" theme="primary" variant="text" :loading="currentCheckingId === row.id" @click="checkFromRow(row)">检查</t-button>
          <t-button size="small" variant="text" @click="openDetail(row)">查看</t-button>
          <t-button size="small" variant="text" @click="openEdit(row)">编辑</t-button>
          <t-button size="small" variant="text" @click="toggleBizStatus(row)">{{ row.business_status === 'enabled' ? '停用' : '启用' }}</t-button>
        </t-space>
      </template>
    </t-table>

    <!-- 账号信息弹窗（点击昵称） -->
    <t-dialog :visible="accountInfoPopup !== null" @close="accountInfoPopup = null" header="账号信息" :footer="false" width="480px">
      <div v-if="accountInfoPopup" class="info-popup">
        <div class="info-head">
          <t-avatar v-if="accountInfoPopup.avatar_url" :image="accountInfoPopup.avatar_url" size="medium" />
          <div>
            <div class="info-name">{{ accountInfoPopup.name || '待识别账号' }}</div>
            <div class="info-sub">{{ platformLabel(accountInfoPopup.platform) }}</div>
          </div>
        </div>
        <t-descriptions :column="1" bordered size="small" style="margin-top:12px">
          <t-descriptions-item label="系统账号ID">{{ accountInfoPopup.id }}</t-descriptions-item>
          <t-descriptions-item label="平台UID">{{ accountInfoPopup.platform_account_id || '--' }}</t-descriptions-item>
          <t-descriptions-item label="平台昵称">{{ accountInfoPopup.name || '--' }}</t-descriptions-item>
          <t-descriptions-item label="所属游戏">{{ gameName(accountInfoPopup.game_id) }}</t-descriptions-item>
          <t-descriptions-item label="标签">{{ (accountInfoPopup.tags || []).join('、') || '--' }}</t-descriptions-item>
          <t-descriptions-item label="业务状态">{{ bizStatusText(accountInfoPopup.business_status) }}</t-descriptions-item>
          <t-descriptions-item label="账号状态">{{ accountStatusText(accountInfoPopup) }}</t-descriptions-item>
          <t-descriptions-item label="最近检查">{{ accountInfoPopup.last_checked_at ? formatTime(accountInfoPopup.last_checked_at) : '从未检查' }}</t-descriptions-item>
          <t-descriptions-item label="最近同步资料">{{ accountInfoPopup.last_checked_at ? formatTime(accountInfoPopup.last_checked_at) : '--' }}</t-descriptions-item>
          <t-descriptions-item label="备注">{{ accountInfoPopup.remark || '--' }}</t-descriptions-item>
        </t-descriptions>
        <div style="margin-top:12px; text-align:right">
          <t-button variant="outline" @click="openDetail(accountInfoPopup); accountInfoPopup = null">查看完整详情</t-button>
          <t-button style="margin-left:8px" @click="accountInfoPopup = null">关闭</t-button>
        </div>
      </div>
    </t-dialog>

    <!-- 浏览器窗口弹窗（点击窗口名） -->
    <t-dialog :visible="windowPopup !== null" @close="windowPopup = null" header="浏览器窗口信息" :footer="false" width="480px">
      <div v-if="windowPopup" class="info-popup">
        <div class="info-name">{{ profileName(windowPopup.browser_profile_id) }}</div>
        <t-descriptions :column="1" bordered size="small" style="margin-top:12px">
          <t-descriptions-item label="窗口名称">{{ profileName(windowPopup.browser_profile_id) }}</t-descriptions-item>
          <t-descriptions-item label="浏览器窗口状态"><span class="status-green">● 正常</span> <span class="info-tip">窗口存在，可以正常连接</span></t-descriptions-item>
          <t-descriptions-item label="代理状态"><span class="info-tip">代理管理未接入（M2-C）</span></t-descriptions-item>
          <t-descriptions-item label="当前运行状态">{{ profileIsOpen(windowPopup) ? '已打开' : '已关闭' }}</t-descriptions-item>
          <t-descriptions-item label="最近同步">{{ windowPopup.last_checked_at ? formatTime(windowPopup.last_checked_at) : '--' }}</t-descriptions-item>
        </t-descriptions>
        <div style="margin-top:12px; text-align:right">
          <t-button v-if="canOperateBoundWindow(windowPopup)" variant="outline" :loading="operatingProfileId.startsWith('open:') || operatingProfileId.startsWith('close:')" @click="toggleWindow(windowPopup)">{{ profileIsOpen(windowPopup) ? '关闭窗口' : '打开窗口' }}</t-button>
          <t-button variant="outline" style="margin-left:8px" @click="openDetail(windowPopup); windowPopup = null">查看窗口详情</t-button>
          <t-button style="margin-left:8px" @click="windowPopup = null">关闭</t-button>
        </div>
      </div>
    </t-dialog>

    <!-- 标签管理弹窗 -->
    <t-dialog :visible="tagManageVisible" header="标签管理" :footer="false" width="420px" @close="tagManageVisible = false">
      <div class="info-tip" style="margin-bottom:8px">本运营的标签库（无全局库）；删除标签会从所有账号移除。</div>
      <div v-if="!tagManageList.length" class="info-tip" style="padding:12px 0">暂无标签</div>
      <div v-for="item in tagManageList" :key="item.name" class="tag-manage-row">
        <t-tag size="medium" variant="light">{{ item.name }}</t-tag>
        <span class="tag-count">{{ item.num }} 个账号</span>
        <t-button size="small" variant="text" theme="danger" @click="deleteTagFromAll(item.name)">删除</t-button>
      </div>
      <div class="info-tip" style="margin-top:8px">新增标签：先在列表勾选账号，点批量「添加标签」；或在编辑弹窗给账号加标签。</div>
    </t-dialog>

    <!-- 全部标签弹窗 -->
    <t-dialog :visible="allTagsPopup !== null" @close="allTagsPopup = null" header="全部标签" :footer="false" width="360px">
      <div v-if="allTagsPopup">
        <t-space wrap>
          <t-tag v-for="tag in (allTagsPopup.tags || [])" :key="tag" size="medium" variant="light">{{ tag }}</t-tag>
          <span v-if="!(allTagsPopup.tags || []).length" class="info-tip">暂无标签</span>
        </t-space>
        <div style="margin-top:12px; text-align:right">
          <t-button @click="allTagsPopup = null">关闭</t-button>
        </div>
      </div>
    </t-dialog>

    <t-dialog v-model:visible="showCreate" header="新增账号" @confirm="createAccount" :confirm-btn="{ loading: creating, theme: 'primary' }">
      <t-form @submit.prevent="createAccount">
        <t-form-item v-if="user?.role === 'admin'" label="归属用户 UID">
          <t-input-number v-model="createForm.userId" :min="1" placeholder="留空则归当前用户" />
        </t-form-item>
        <t-form-item label="游戏">
          <t-select v-model="createForm.gameId" clearable placeholder="可先不绑定；启用执行前必须绑定游戏">
            <t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" />
          </t-select>
        </t-form-item>
        <t-form-item label="平台">
          <t-select v-model="createForm.platform">
            <t-option value="bilibili" label="哔哩" />
            <t-option value="baijiahao" label="百度" />
          </t-select>
        </t-form-item>
        <t-form-item v-if="isDesktop" label="绑定窗口">
          <t-select v-model="createForm.browserProfileId" clearable placeholder="可先不绑定">
            <t-option v-for="profile in activeProfiles" :key="profile.id" :value="profile.id" :label="profileOptionLabel(profile)" />
          </t-select>
        </t-form-item>
        <t-form-item label="标签">
          <t-input v-model="createForm.tagsText" placeholder="多个标签用逗号分隔" />
        </t-form-item>
        <t-form-item label="备注">
          <t-textarea v-model="createForm.remark" :rows="3" placeholder="账号来源、用途或注意事项" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 编辑账号（纯资料，不含打开关闭/检查/Cookie） -->
    <t-dialog v-model:visible="editVisible" header="编辑账号" :confirm-btn="{ content: '保存', loading: savingEdit, theme: 'primary' }" @confirm="saveEdit">
      <t-form v-if="editAccount" label-width="90px">
        <t-form-item label="账号">{{ accountDisplayName(editAccount) }}（{{ platformLabel(editAccount.platform) }}）</t-form-item>
        <t-form-item label="所属游戏">
          <t-select v-model="editGameId" clearable placeholder="选择游戏">
            <t-option v-for="game in games" :key="game.id" :value="game.id" :label="game.name" />
          </t-select>
        </t-form-item>
        <t-form-item label="标签">
          <t-input v-model="editTagsText" placeholder="多个标签用逗号分隔" />
        </t-form-item>
        <t-form-item label="备注">
          <t-textarea v-model="editRemark" :rows="3" placeholder="账号备注" />
        </t-form-item>
        <div class="form-tip">平台 UID/昵称/头像/账号状态由检查同步，不在此编辑。</div>
      </t-form>
    </t-dialog>

    <t-drawer v-model:visible="detailVisible" header="账号详情" :size="'560px'" destroy-on-close>
      <t-descriptions v-if="detailAccount" :column="1" bordered size="small">
        <t-descriptions-item label="账号ID">{{ detailAccount.id }}</t-descriptions-item>
        <t-descriptions-item label="平台">{{ detailAccount.platform }}</t-descriptions-item>
        <t-descriptions-item label="游戏">{{ gameName(detailAccount.game_id) }}</t-descriptions-item>
        <t-descriptions-item label="绑定窗口">{{ detailAccount.browser_profile_id ? profileLabel(detailAccount.browser_profile_id) : '未绑定' }}</t-descriptions-item>
        <t-descriptions-item label="平台账号UID">{{ detailAccount.platform_account_id || '未回填' }}</t-descriptions-item>
        <t-descriptions-item label="账号名称">{{ detailAccount.name || '未回填' }}</t-descriptions-item>
        <t-descriptions-item label="登录状态">{{ loginStatusText(detailAccount.login_status) }}</t-descriptions-item>
        <t-descriptions-item label="可执行结论">{{ executableText(detailAccount) }}</t-descriptions-item>
        <t-descriptions-item label="最近检查">{{ detailAccount.last_checked_at ? formatTime(detailAccount.last_checked_at) : '尚未检查' }}</t-descriptions-item>
      </t-descriptions>
      <div v-if="detailAccount?.check_items?.length" class="check-items-box" style="margin-top:12px">
        <div class="check-items-title">检查明细（8 项）</div>
        <div v-for="item in detailAccount.check_items" :key="item.key" class="check-item">
          <span class="check-item-status" :class="'status-' + (item.status || 'na')">{{ checkItemStatusText(item.status) }}</span>
          <span class="check-item-label">{{ item.label }}</span>
          <span v-if="item.message" class="check-item-msg">{{ item.message }}</span>
        </div>
      </div>
      <t-alert v-if="checkNotice" :message="checkNotice" theme="success" style="margin-top:12px" />
      <t-alert v-if="cookieNotice" :message="cookieNotice" theme="success" style="margin-top:12px" />

      <t-form v-if="detailAccount" class="detail-form" label-width="92px">
        <t-form-item label="备注">
          <t-textarea v-model="detailRemark" :rows="3" />
        </t-form-item>
        <t-form-item v-if="isDesktop" label="绑定窗口">
          <t-select v-model="detailProfileId" clearable placeholder="选择本人授权窗口">
            <t-option v-for="profile in activeProfiles" :key="profile.id" :value="profile.id" :label="profileOptionLabel(profile)" />
          </t-select>
          <div class="form-tip">绑定或换绑后，登录状态会回到“待检查”，需要后续账号检查重新确认。</div>
        </t-form-item>
        <t-form-item v-else label="操作边界">
          <div class="form-tip">Cloud Web 只查看绑定关系；请在 Desktop 端绑定、解绑或换绑窗口。</div>
        </t-form-item>
        <t-form-item v-if="isDesktop" label="账号同步">
          <div class="form-tip">如已在 BitBrowser 窗口中人工登录，请返回这里点击“检查/同步账号信息”，系统会读取真实平台身份并回填 Cloud。</div>
        </t-form-item>
      </t-form>

      <template #footer>
        <t-space>
          <t-button variant="outline" @click="detailVisible = false">关闭</t-button>
          <t-button v-if="canCheckAccount(detailAccount)" variant="outline" :loading="checkingAccount" @click="checkDetailAccount">检查/同步账号信息</t-button>
          <t-button variant="outline" @click="viewExportCookie">查看/导出 Cookie</t-button>
          <t-button v-if="isDesktop && canOperateBoundWindow(detailAccount)" variant="outline" :loading="readingCookie" @click="readProfileCookie">从 Profile 读真实 Cookie</t-button>
          <t-button theme="primary" :loading="savingDetail" @click="saveDetail">保存</t-button>
        </t-space>
      </template>
    </t-drawer>

    <t-dialog v-model:visible="showCookieDialog" header="账号 Cookie（敏感数据）" :cancel-btn="{ content: '关闭' }">
      <p><strong>原始 Cookie：</strong></p>
      <pre class="cookie-box">{{ cookieDialog.original_cookie || '（空）' }}</pre>
      <p><strong>当前真实 Cookie（active）：</strong></p>
      <pre class="cookie-box">{{ cookieDialog.active_cookie || '（空）' }}</pre>
      <div class="form-tip">Cookie 为敏感数据，请勿泄露。复制后建议及时清理剪贴板；导出仅用于授权用途。</div>
      <t-space style="margin-top:8px">
        <t-button size="small" @click="copyCookie('active_cookie')">复制 Active Cookie</t-button>
        <t-button size="small" variant="outline" @click="copyCookie('original_cookie')">复制原始 Cookie</t-button>
      </t-space>
    </t-dialog>
  </t-loading>
</template>

<style scoped>
.stats-row { margin-bottom: 12px; }
.stat-num { font-size: 18px; font-weight: 600; line-height: 1.2; }
.stats-row :deep(.t-card__body) { padding: 10px 12px; }
.stats-row :deep(.t-card__title) { font-size: 13px; }
.search-bar { margin-bottom: 12px; }
.batch-result-card { margin-bottom: 12px; }
.cookie-box {
  max-height: 160px;
  overflow: auto;
  background: var(--td-bg-color-container-hover, #f5f5f5);
  padding: 8px;
  border-radius: 4px;
  font-size: 12px;
  word-break: break-all;
  white-space: pre-wrap;
}
.account-id { cursor: pointer; color: var(--td-brand-color, #0052d9); }
.account-name-link { cursor: pointer; color: var(--td-brand-color, #0052d9); font-weight: 500; }
.window-name-link { cursor: pointer; color: var(--td-brand-color, #0052d9); }
.account-sub { color: var(--td-text-color-secondary, #666); font-size: 12px; }
.account-remark { color: var(--td-text-color-secondary, #666); font-size: 12px; }
.account-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.tags-more { cursor: pointer; }
.window-cell { cursor: pointer; }
.window-proxy { font-size: 12px; }
.proxy-dot { margin-right: 2px; }
.proxy-dot.proxy-ok { color: var(--td-success-color, #2ba471); }
.window-name { font-size: 13px; }
.window-unbound { color: var(--td-text-color-secondary, #666); }
.biz-status { font-size: 13px; }
.biz-enabled { color: var(--td-success-color, #2ba471); }
.biz-disabled { color: var(--td-text-color-secondary, #666); }
.account-status { font-size: 13px; padding: 1px 8px; border-radius: 3px; }
.account-status.status-gray { background: #f0f0f0; color: #666; }
.account-status.status-green { background: #e8f7ef; color: #2ba471; }
.account-status.status-orange { background: #fef3e8; color: #e37318; }
.account-status.status-red { background: #fdecee; color: #d54941; }
.op-cell { white-space: nowrap; }
.info-popup .info-head { display: flex; gap: 10px; align-items: center; }
.info-popup .info-name { font-size: 16px; font-weight: 600; }
.info-popup .info-sub { color: var(--td-text-color-secondary, #666); font-size: 13px; }
.info-tip { color: var(--td-text-color-secondary, #666); font-size: 12px; }
.status-green { color: var(--td-success-color, #2ba471); }
.check-items-box {
  border: 1px solid var(--td-component-border, #ddd);
  border-radius: 4px;
  padding: 8px;
}
.check-items-title { font-weight: 600; margin-bottom: 6px; font-size: 13px; }
.check-item { display: flex; gap: 8px; align-items: center; padding: 3px 0; font-size: 13px; }
.check-item-status { width: 48px; text-align: center; border-radius: 3px; padding: 1px 4px; font-size: 12px; flex-shrink: 0; }
.check-item-status.status-pass { background: #e8f7ef; color: #2ba471; }
.check-item-status.status-fail { background: #fdecee; color: #d54941; }
.check-item-status.status-na, .check-item-status.status-skip { background: #f0f0f0; color: #666; }
.check-item-label { flex-shrink: 0; }
.check-item-msg { color: #666; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tag-manage-row { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
.tag-count { color: var(--td-text-color-secondary, #666); font-size: 12px; flex: 1; }
.batch-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}
.batch-result-list { display: grid; gap: 6px; }
.batch-result-item {
  display: grid;
  grid-template-columns: minmax(140px, 1fr) 80px minmax(200px, 2fr);
  align-items: center;
  gap: 8px;
}
.batch-account { font-weight: 500; }
.batch-message { color: var(--td-text-color-secondary); }
.action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.batch-actions { display: flex; align-items: center; gap: 8px; }
.selected-count,
.account-sub,
.form-tip {
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
.account-name { font-weight: 500; }
.account-tags { display: flex; gap: 4px; margin-top: 4px; flex-wrap: wrap; }
.detail-form { margin-top: 16px; }
</style>
