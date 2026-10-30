<script setup>
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { computed, onMounted, ref, watch } from "vue"
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import { createSessionClient } from "../../../shared/api/session.js"
import { createUsersClient } from "../../../apps/cloud/pages/users/usersApi.js"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import { executionErrorMessage } from "../../../shared/utils/executionErrors.js"
import { loginStatusText } from "../../../shared/utils/mediaAccountLabels.js"
import { ACCOUNT_PLATFORM_OPTIONS, platformLabel } from "../../../shared/utils/platforms.js"
import { canCheckAccount as checkAvailability } from "./checkAvailability.js"
import { categoryTypeLabel } from "../../../shared/utils/categories.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue"
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue"
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue"
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue"
// Static import: a dynamic import() of a node_modules bare specifier does not
// resolve in the packaged Tauri WebView ("Module name ... does not resolve to a
// valid file"). invoke is only called inside __TAURI_INTERNALS__-guarded code,
// so importing it is inert in Cloud Web.
import { invoke } from "@tauri-apps/api/core"
import { useRouter } from "vue-router"

const accountClient = createMediaAccountClient()
const profileClient = createProfileBindingClient()
const sessionClient = createSessionClient()
const usersClient = createUsersClient()

const router = useRouter()
const isDesktop = window.__WT_MEDIA_APP__ === "desktop"

const user = ref(null)
const accounts = ref([])
const accountStats = ref({total:0,normal:0,abnormal:0,unbound:0})
const accountMembers = ref([])
const accountTeams = ref([])
const searchTeam = ref("")
const searchCategoryType = ref("")
const searchTagMatch = ref("any")
const profiles = ref([])
let bindingRequest = 0
let accountRequest = 0
const bindingUsage = ref([])
const bindingLoading = ref(false)
const bindingError = ref("")
const categories = ref([])
const loading = ref(true)
const error = ref("")

const searchView = ref("")
const searchText = ref("")
const searchPlatform = ref("")
const searchEnabled = ref("")
const searchLoginStatus = ref("")
const searchTags = ref([])
const searchCategoryIds = ref([])
const pagination = ref({ current: 1, pageSize: 20, total: 0, showJumper: false })

const showCreate = ref(false)
const createForm = ref(defaultCreateForm())
const creating = ref(false)
const createError = ref("")

const detailVisible = ref(false)
const detailAccount = ref(null)
const checkFailure = ref("")
const checkFailureEnv = ref(false)
const readingCookie = ref(false)
const showCookieDialog = ref(false)
const cookieDialog = ref(null)
const editVisible = ref(false)
const editAccount = ref(null)
const createCategoryType = ref("")
const editCategoryType = ref("")
const editRemark = ref("")
const editCategoryIds = ref([])
const bindAccount = ref(null)
const bindVisible = ref(false)
const bindProfileId = ref("")
const bindSearch = ref("")
const bindingSaving = ref(false)
const createdGuide = ref(null)
const boundGuide = ref(null)
const editTags = ref([])
const savingEdit = ref(false)
const editError = ref("")
const batchChecking = ref(false)
const batchResults = ref([])
const operatingProfileId = ref("")
const currentCheckingId = ref("")
const profileOpenMap = ref({})
const accountInfoPopup = ref(null)
const windowPopup = ref(null)
const allTagsPopup = ref(null)

const selectedAccountIds = ref([])
const selectedAccountCache = new Map()
const searchProfile = ref("")
// 使用人筛选：高级运营默认看自己，管理员默认看全部成员；普通运营由服务端强制
// 只看自己，因此不展示这个筛选项。
const searchOwner = ref("")

onMounted(async () => {
  try {
    user.value = await sessionClient.me()
    searchOwner.value = defaultOwnerFilter()
    await Promise.all([loadAuxiliaryData(), loadAccounts()])
  } catch (e) {
    error.value = e.message || "加载账号数据失败"
  } finally {
    loading.value = false
  }
})

const isAdmin = computed(() => user.value?.role === "admin")
const canFilterByOwner = computed(() => user.value?.role === "senior_operator" || user.value?.role === "admin")

function defaultOwnerFilter() {
  if (user.value?.role === "admin") return "all"
  if (user.value?.role === "senior_operator") return String(user.value?.id ?? "")
  return ""
}

watch(searchOwner, () => {
  pagination.value.current = 1
})

function defaultCreateForm() {
  return {
    categoryIds: [],
    platform: "bilibili",
    remark: "",
    tags: [],
  }
}

async function loadAuxiliaryData() {
  const [profileList, categoryList] = await Promise.all([
    profileClient.listProfiles().catch(() => []),
    usersClient.listCategories({ enabled: true }).catch(() => []),
  ])
  profiles.value = Array.isArray(profileList) ? profileList : []
  categories.value = Array.isArray(categoryList) ? categoryList : []
  if (canFilterByOwner.value) accountMembers.value = await usersClient.listUsers().catch(() => [])
  if (isAdmin.value) accountTeams.value = await usersClient.listTeams().catch(() => [])
}

async function loadAccounts({ resetPage = false } = {}) {
  const request = ++accountRequest
  error.value = ""
  const params = {}
  if (searchView.value) params.view = searchView.value
  if (searchText.value) params.search = searchText.value
  if (searchPlatform.value) params.platform = searchPlatform.value
  if (searchEnabled.value) params.enabled = searchEnabled.value === "enabled"
  if (searchLoginStatus.value) params.platform_login_status = searchLoginStatus.value
  if (searchCategoryIds.value.length) params.category_ids = searchCategoryIds.value
  if (searchTags.value.length) { params.tags = searchTags.value; params.tag_match = searchTagMatch.value }
  if (searchOwner.value && searchOwner.value !== "all") params.user_id = Number(searchOwner.value)
  if (searchTeam.value) params.team_id = Number(searchTeam.value)
  if (searchCategoryType.value) params.category_type = searchCategoryType.value
  if (resetPage) { pagination.value.current = 1; selectedAccountIds.value = [] }
  params.page = pagination.value.current
  params.page_size = pagination.value.pageSize
  if (searchProfile.value) params.profile_search = searchProfile.value
  try {
    const result = await accountClient.listPage(params)
    if (request !== accountRequest) return
    accounts.value = result.items
    result.items.forEach(account => selectedAccountCache.set(account.id,account))
    pagination.value.total = result.total
    accountStats.value = result.stats
    const lastPage = Math.max(1,Math.ceil(result.total / pagination.value.pageSize))
    if (pagination.value.current > lastPage) {pagination.value.current=lastPage;await loadAccounts()}
  } catch (e) {
    error.value = e.message || "加载媒体账号失败"
  }
}

function onPageChange(pageInfo) {
  pagination.value.current = pageInfo.current
  pagination.value.pageSize = pageInfo.pageSize
  loadAccounts()
}

function resetFilters(reload = true) {
  searchView.value = ""
  pagination.value.current = 1
  searchTeam.value = ""
  searchCategoryType.value = ""
  searchTagMatch.value = "any"
  searchText.value = ""
  searchPlatform.value = ""
  searchEnabled.value = ""
  searchLoginStatus.value = ""
  searchTags.value = []
  searchCategoryIds.value = []
  searchProfile.value = ""
  searchOwner.value = defaultOwnerFilter()
  if (reload) loadAccounts()
}

function applyStatFilter(key) {
 resetFilters(false)
 searchView.value = ({normal:"normal",abnormal:"attention",unbound:"unbound"})[key] || ""
 loadAccounts({resetPage:true})
}


function openCreate() {
  tagSearch.value = ""
  createForm.value = defaultCreateForm()
  createCategoryType.value = ""
  createError.value = ""
  showCreate.value = true
}

async function createAccount() {
  error.value = ""
  const categoryError = validateCategorySelection(createForm.value.categoryIds)
  if (categoryError) {
    createError.value = categoryError
    return
  }
  creating.value = true
  try {
    const created = await accountClient.create({
      category_ids: createForm.value.categoryIds,
      platform: createForm.value.platform,
      remark: createForm.value.remark,
      tags: createForm.value.tags || [],
    })
    showCreate.value = false
    createdGuide.value = created
    await loadAccounts()
  } catch (e) {
    createError.value = e.message || "创建媒体账号失败"
  } finally {
    creating.value = false
  }
}

function openDetail(account) {
  detailAccount.value = account
  checkFailure.value = ""
  checkFailureEnv.value = false
  detailVisible.value = true
}

function accountDisplayName(account) {
  if (!account) return "-"
  if (account.platform_name) return account.platform_name
  if (account.platform_account_id) return account.platform_account_id
  return account.identification_status !== "identified" ? "待识别账号" : "未识别"
}

async function copyAccountId(account) {
  if (!account?.id) return
  try {
    await navigator.clipboard.writeText(account.id)
    feedback.toast.success("已复制系统账号 ID")
  } catch (e) {
    feedback.toast.error("复制失败，请手动选择")
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

function profileWindowStateText(account) {
  const profile = profileForAccount(account)
  if (!profile) return "窗口未同步"
  if (profile.local_state === "local_missing") return "窗口缺失"
  if (profile.enabled === false) return "窗口已停用"
  return "窗口可使用"
}

function profileProxyModeText(account) {
  const profile = profileForAccount(account)
  if (!profile) return "代理未知"
  if (profile.proxy_id) return "系统托管代理"
  if (profile.bit_proxy_host) return "比特自管代理"
  return "直连"
}

function profileIsOpen(account) {
  const profile = profileForAccount(account)
  if (!profile?.bit_profile_id) return false
  return Boolean(profileOpenMap.value[profile.bit_profile_id])
}

async function toggleWindow(account) {
  if (!canOperateLocalAccount(account)) {
    feedback.toast.info(localOperationHint(account))
    return
  }
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
    feedback.toast.error(e.message || "窗口操作失败")
  }
}

async function checkFromRow(account) {
  if (!canCheckAccount(account)) {
    feedback.toast.info(executableText(account))
    return
  }
  currentCheckingId.value = account.id
  try {
    const service = await desktopLocalAgentService()
    const updated = await runSingleAccountCheck(account, service)
    const idx = accounts.value.findIndex(a => a.id === account.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated }
    feedback.toast.success(`检查完成：${loginStatusText(updated.platform_login_status)}`)
    await loadAccounts()
  } catch (e) {
    // 单账号检查失败：行状态置「检查失败」+ 打开详情展示失败原因（不使用页面级横幅）
    const idx = accounts.value.findIndex(a => a.id === account.id)
    const failed = idx >= 0 ? { ...accounts.value[idx], last_check_status: "failed", last_check_message: e.message || "环境不可用" } : account
    if (idx >= 0) accounts.value[idx] = failed
    feedback.toast.error(e.message || "检查失败")
    openDetail(failed)
    checkFailure.value = `检查失败：${e.message || "环境不可用"}`
    checkFailureEnv.value = isEnvironmentError(e?.message)
  } finally {
    currentCheckingId.value = ""
  }
}

function openEdit(account) {
  tagSearch.value = ""
  editAccount.value = account
  editCategoryType.value = selectedCategoryType(account.category_ids)
  editRemark.value = account.remark || ""
  editCategoryIds.value = Array.isArray(account.category_ids) ? [...account.category_ids] : []
  editTags.value = [...(account.tags || [])]
  editError.value = ""
  editVisible.value = true
}

async function saveEdit() {
  if (!editAccount.value) return
  const categoryError = validateCategorySelection(editCategoryIds.value)
  if (categoryError) {
    editError.value = categoryError
    return
  }
  savingEdit.value = true
  error.value = ""
  try {
    let updated = await accountClient.update(editAccount.value.id, {
      tags: editTags.value.map(t => String(t).trim()).filter(Boolean),
      remark: editRemark.value,
      category_ids: editCategoryIds.value,
    })
    const newTags = updated.tags || []
    const idx = accounts.value.findIndex(a => a.id === editAccount.value.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated, tags: newTags }
    editVisible.value = false
    await loadAccounts()
  } catch (e) {
    editError.value = e.message || "保存失败"
  } finally {
    savingEdit.value = false
  }
}

async function toggleEnabled(account) {
  const target = !account.enabled
  if (!await feedback.confirmWarning({ title: target ? "启用账号" : "停用账号", message: `${target ? "启用" : "停用"}「${accountDisplayName(account)}」？${target ? "" : "停用后将不参与后续任务。"}`, confirmText: target ? "启用" : "停用", danger: !target })) return
  try {
    const updated = await accountClient.update(account.id, { enabled: target })
    const idx = accounts.value.findIndex(a => a.id === account.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated }
  } catch (e) {
    feedback.toast.error(e.message || "状态切换失败")
  }
}

function rowMoreOptions(account) {
 return [...(canOperateLocalAccount(account) ? [{content: account.browser_profile_id ? "更换窗口" : "绑定窗口", value:"bind"}] : []), ...(!isAdmin.value && isOwnAccount(account) ? [{content:"查看 Cookie",value:"credentials"}] : []),{content:account.enabled ? "停用" : "启用",value:"enabled"}]
}
function onRowMore(option, account) { if (option.value === "bind") openBind(account); else if (option.value === "credentials") openCookieDialog(account); else if (option.value === "enabled") toggleEnabled(account) }
function splitTags(value) {
  return String(value || "").split(",").map(t => t.trim()).filter(Boolean)
}


function selectedCategoryType(categoryIds) {
  const values = Array.isArray(categoryIds) ? categoryIds : []
  return categories.value.find((category) => values.includes(category.id))?.type || ""
}

function categoryOptionDisabled(category, categoryIds) {
  const selectedType = selectedCategoryType(categoryIds)
  return Boolean(selectedType && category.type !== selectedType && !(categoryIds || []).includes(category.id))
}

function validateCategorySelection(categoryIds) {
  const selected = new Set(categoryIds || [])
  const types = new Set(categories.value.filter((category) => selected.has(category.id)).map((category) => category.type))
  return types.size > 1 ? "同一个社媒账号的多个类目必须属于同一一级类型" : ""
}

function categoryNames(categoryIds) {
  const values = Array.isArray(categoryIds) ? categoryIds : []
  if (!values.length) return "未绑定类目"
  return values.map(categoryId => {
    const category = categories.value.find(item => item.id === categoryId)
    return category ? category.name : `${categoryId}（超出用户类目范围）`
  }).join("、")
}

function profileName(profileId) {
  const profile = profiles.value.find(item => item.id === profileId)
  if (!profile) return "未命名窗口"
  return profile.bit_name || "未命名窗口"
}

function profileForAccount(account) {
  return profiles.value.find(item => item.id === account?.browser_profile_id)
}

function isOwnAccount(account) {
  return Boolean(account) && String(account.user_id ?? "") === String(user.value?.id ?? "")
}

// 本地操作按账号归属：普通运营可操作自己的账号，高级运营只能操作本人创建的账号，
// 管理员一律只读。服务端 StartLocalAccountCheck / StartCookieRead 用同一套归属判定。
function canOperateLocalAccount(account) {
  if (!isDesktop || !account) return false
  if (user.value?.role === "operator") return isOwnAccount(account)
  if (user.value?.role === "senior_operator") return isOwnAccount(account)
  return false
}

function localOperationHint(account) {
  if (!isDesktop) return "本地操作只能在 Desktop 客户端执行"
  if (user.value?.role === "admin") return "管理员不执行本地操作"
  if (!canOperateLocalAccount(account)) return "仅账号本人可执行本地操作"
  return ""
}

function executableText(account) {
  const roleHint = isDesktop ? localOperationHint(account) : ""
  if (roleHint) return `不可执行：${roleHint}`
  if (!account.enabled) return "不可执行：账号已停用"
  if (!(account.category_ids || []).length) return "不可执行：未绑定类目"
  if (!account.browser_profile_id) return "不可执行：未绑定窗口"
  const profile = profileForAccount(account)
  if (profile && (profile.local_state !== "active" || !profile.enabled)) return "不可执行：绑定窗口已停用"
  if (account.platform_login_status !== "normal") return "待检查：需要真实账号检查"
  return "可用于运营任务"
}

function enabledText(enabled) {
  return enabled ? "启用" : "停用"
}

// 账号状态：真实平台状态，由 platform_login_status + identification_status 派生（与业务状态独立）
// 颜色语义遵循视觉规范 6.2（正常=绿、未检查=灰、待处理=橙、错误/冲突/失效/受限=红）
const ACCOUNT_STATUS_MAP = {
  pending: { label: "待上号", color: "gray" },
  normal: { label: "正常", color: "green" },
  not_logged_in: { label: "未登录", color: "orange" },
  expired: { label: "登录失效", color: "red" },
  account_mismatch: { label: "账号不匹配", color: "red" },
  restricted: { label: "账号受限", color: "red" },
  check_failed: { label: "检查失败", color: "red" },
}

function accountStatusKey(account) {
  if (!account) return "pending"
  if (account.last_check_status === "failed") return "check_failed"
  const idStatus = account.identification_status || ""
  const login = account.platform_login_status || ""
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

// 账号状态 → t-tag theme 映射（视觉规范 6.2 颜色语义）
function accountStatusTheme(account) {
  return { gray: "default", green: "success", orange: "warning", red: "danger" }[accountStatusColor(account)] || "default"
}

function isEnvironmentError(message) {
  return /可信绑定|环境确认|timeout|timed out|BitBrowser/.test(message || "")
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

function profileOptionLabel(profile) {
  if (!profile) return ""
  const name = profile.bit_name || "未命名窗口"
  const parts = []
  if (profile.bit_seq) parts.push(String(profile.bit_seq))
  parts.push(name)
  if (profile.id) parts.push(String(profile.id))
  return parts.join(" - ")
}


function canCheckAccount(account) {
  return checkAvailability({ isDesktop, isOwner: canOperateLocalAccount(account), account, windowAvailable: canOperateBoundWindow(account) })
}

function canOperateBoundWindow(account) {
  const profile = profileForAccount(account)
  return isDesktop && account?.browser_profile_id && profile?.bit_profile_id && profile.local_state === "active" && profile.enabled !== false
}

// 他人账号也要露出窗口按钮（禁用 + 提示），否则高级运营／管理员看不到「为什么不能操作」。
function showWindowToggle(account) {
  if (!isDesktop || !account?.browser_profile_id) return false
  return canOperateBoundWindow(account) || !canOperateLocalAccount(account)
}

async function desktopLocalAgentService() {
  if (typeof window === "undefined" || !window.__TAURI_INTERNALS__) {
    throw new Error("账号检查只能在 Desktop 客户端执行")
  }
  return createLocalAgentService({ invoke })
}

async function runSingleAccountCheck(account, service) {
  const start = await accountClient.check(account.id)
  let localResult
  try {
    localResult = await service.accountCheck({
      operationId: start.operation_id,
      bitProfileId: start.bit_profile_id,
      platform: start.platform,
      expectedPlatformAccountId: start.expected_platform_account_id || "",
    })
  } catch (error) {
    try {
      await accountClient.submitCheckResult(account.id, {
        operation_id: start.operation_id, execution_status: "failed",
        message: error.message || "环境不可用",
      })
    } catch (reportError) {
      throw new Error(`${error.message || "检查失败"}；结果保存失败：${reportError.message || "请刷新后核实"}`)
    }
    throw error
  }
  return accountClient.submitCheckResult(account.id, {
    operation_id: start.operation_id,
    execution_status: "completed",
    platform_account_id: localResult.platform_account_id,
    platform_name: localResult.name,
    platform_avatar_url: localResult.avatar_url,
    platform_login_status: localResult.login_status,
    message: localResult.message,
    check_items: localResult.check_items || [],
  })
}

async function readProfileCookieFor(account) {
  const target = account || cookieDialog.value?.account
  if (!target?.id || !target.browser_profile_id) return
  if (!canOperateLocalAccount(target)) {
    feedback.toast.info(localOperationHint(target))
    return
  }
  if (!canOperateBoundWindow(target)) {
    feedback.toast.info("绑定窗口不可用，无法读取 Cookie")
    return
  }
  readingCookie.value = true
  error.value = ""
  try {
    const service = await desktopLocalAgentService()
    const start = await accountClient.startCookieReadSync(target.id)
    const localResult = await service.cookieRead({
      operationId: start.operation_id,
      bitProfileId: start.bit_profile_id,
    })
    const cookies = localResult.cookies || []
    const updated = await accountClient.submitCookieReadResult(target.id, {
      operation_id: start.operation_id,
      cookies,
    })
    const idx = accounts.value.findIndex(a => a.id === target.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated }
    if (cookieDialog.value?.account?.id === target.id) {
      cookieDialog.value = { ...cookieDialog.value, ...updated, account: { ...cookieDialog.value.account, ...updated } }
    }
    feedback.toast.success(`已从 Profile 读回 ${cookies.length} 条真实 Cookie 并更新 active_cookie`)
  } catch (e) {
    feedback.toast.error(e.message || "读取 Cookie 失败")
  } finally {
    readingCookie.value = false
  }
}

async function openCookieDialog(account) {
  if (!account?.id) return
  try {
    const data = await accountClient.fetchCookies(account.id)
    cookieDialog.value = {
      account,
      original_cookie: (data && data.original_cookie) || "",
      active_cookie: (data && data.active_cookie) || "",
    }
    showCookieDialog.value = true
  } catch (e) {
    feedback.toast.error(e.message || "获取 Cookie 失败")
  }
}

async function copyCookie(field) {
  const value = cookieDialog.value?.[field]
  if (!value) {
    feedback.toast.info("当前 Cookie 为空，无可复制内容")
    return
  }
  try {
    await navigator.clipboard.writeText(value)
    feedback.toast.success(`已复制 ${field === "active_cookie" ? "Active" : "原始"} Cookie 到剪贴板`)
  } catch (e) {
    feedback.toast.error("复制失败，请手动选择复制")
  }
}

function accountLabel(account) {
  return account?.platform_name || account?.platform_account_id || account?.id || "未知账号"
}

function selectedAccounts() {
  return selectedAccountIds.value.map(id => selectedAccountCache.get(id)).filter(Boolean)
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
  if (!candidates.length) {
    feedback.toast.error("请先勾选要检查的账号")
    return
  }
  batchChecking.value = true
  const nonOwnedCount = candidates.filter(account => !canOperateLocalAccount(account)).length
  const skippedOwnershipText = nonOwnedCount
    ? isAdmin.value
      ? `，管理员不执行本地操作，已跳过 ${nonOwnedCount} 条`
      : `，已跳过 ${nonOwnedCount} 条非本人账号`
    : ""
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
        item.message = `检查完成：${loginStatusText(updated.platform_login_status)}`
        item.account = { ...item.account, ...updated }
      } catch (e) {
        item.status = "failed"
        item.message = e.message || "检查失败"
      }
    }
    const stats = batchStats()
    const summary = `批量检查完成：成功 ${stats.success}，失败 ${stats.failed}，跳过 ${stats.skipped}${skippedOwnershipText}`
    if (stats.failed > 0 || stats.success === 0) {
      feedback.toast.info(summary)
    } else {
      feedback.toast.success(summary)
    }
    await loadAccounts()
  } catch (e) {
    feedback.toast.error(e.message || "批量检查失败")
  } finally {
    batchChecking.value = false
  }
}

async function openAccountProfile(account) {
  const profile = profileForAccount(account)
  if (!canOperateBoundWindow(account)) return
  const bitProfileId = String(profile.bit_profile_id || "").trim()
  if (!bitProfileId) {
    feedback.toast.error("打开绑定窗口失败：Cloud记录缺少BitBrowser窗口ID，请先扫描并同步本机窗口。")
    return
  }
  operatingProfileId.value = `open:${bitProfileId}`
  try {
    const service = await desktopLocalAgentService()
    const result = await service.profileOpen(bitProfileId)
    feedback.toast.success(`已打开绑定窗口：${profile.bit_name || result.bit_profile_id}`)
  } catch (e) {
    feedback.toast.error(executionErrorMessage(e))
  } finally {
    operatingProfileId.value = ""
  }
}

async function closeAccountProfile(account) {
  const profile = profileForAccount(account)
  if (!canOperateBoundWindow(account)) return
  const bitProfileId = String(profile.bit_profile_id || "").trim()
  if (!bitProfileId) {
    feedback.toast.error("关闭绑定窗口失败：Cloud记录缺少BitBrowser窗口ID，请先扫描并同步本机窗口。")
    return
  }
  operatingProfileId.value = `close:${bitProfileId}`
  try {
    const service = await desktopLocalAgentService()
    const result = await service.profileClose(bitProfileId)
    feedback.toast.success(`已关闭绑定窗口：${profile.bit_name || result.bit_profile_id}`)
  } catch (e) {
    feedback.toast.error(executionErrorMessage(e))
  } finally {
    operatingProfileId.value = ""
  }
}

function formatTime(t) {
  if (!t) return "-"
  return new Date(t).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

const activeProfiles = computed(() => profiles.value.filter(profile => String(profile.user_id) === String(user.value?.id)))
async function loadBindingUsage(platform) {
 const request=++bindingRequest
 bindingLoading.value=true;bindingError.value=""
 try {const usage=await accountClient.list({platform,user_id:user.value.id});if (request===bindingRequest) bindingUsage.value=usage}
 catch (e) {if(request===bindingRequest) bindingError.value=e.message || "暂时无法核对窗口占用"}
 finally {if(request===bindingRequest) bindingLoading.value=false}
}

function bindingUnavailableReason(profile) {
 if (bindingLoading.value) return "正在核对窗口"
 if (bindingError.value) return "暂时无法核对"
 if (profile.enabled === false) return "窗口已停用"
 if (profile.local_state !== "active") return "本机窗口不可用"
 const exceptId=bindAccount.value?.id
 if (bindingUsage.value.some(account => account.id !== exceptId && String(account.browser_profile_id) === String(profile.id))) return "已绑定同平台账号"
 return ""
}

const bindingCandidates = computed(() => activeProfiles.value.filter(profile =>
  [profile.bit_name, profile.bit_seq, profile.bit_group_name].join(" ").toLowerCase().includes(bindSearch.value.trim().toLowerCase())))
function openBind(account) {
  if (!canOperateLocalAccount(account)) return
  createdGuide.value = null
  windowPopup.value = null
  bindAccount.value = account
  bindProfileId.value = ""
  bindSearch.value = ""
  bindingError.value = ""
  bindVisible.value = true
  loadBindingUsage(account.platform)
}
async function saveBinding() {
  if (bindingSaving.value || !bindAccount.value) return
  const profile = activeProfiles.value.find(p => p.id === bindProfileId.value)
  if (!profile || bindingUnavailableReason(profile)) { feedback.toast.error("请选择一个可关联的窗口"); return }
  if (bindAccount.value.browser_profile_id && !await feedback.confirmWarning({title:"确认更换窗口？", message:`将使用「${profile.bit_name}」，原窗口的登录信息保留。`,confirmText:"确认更换"})) return
  bindingSaving.value = true
  try {
    const updated = await accountClient.bindProfile(bindAccount.value.id, profile.id)
    bindVisible.value = false
    boundGuide.value = updated
    await loadAccounts()
  } catch (e) { bindingError.value = e.message || "窗口未绑定，请重试" }
  finally { bindingSaving.value = false }
}


const availableTags = computed(() => {
  const set = new Set()
  accounts.value.forEach(a => (a.tags || []).forEach(t => set.add(t)))
  return [...set].sort()
})

const tagOptions = computed(() => availableTags.value.map(t => ({ label: t, value: t })))
const tagSearch = ref("")
const tagCreateCandidate = computed(() => {
  const v = tagSearch.value.trim()
  if (!v) return ""
  return availableTags.value.includes(v) ? "" : v
})
function onTagSearch(ctx) {
  tagSearch.value = typeof ctx === "string" ? ctx : (ctx?.value ?? "")
}
function commitNewTag(list, value) {
  const v = String(value ?? "").trim()
  if (!v || list.includes(v)) return
  list.push(v)
  tagSearch.value = ""
}
function onTagEnter(list, ctx) {
  const v = String(ctx?.inputValue ?? ctx?.value ?? tagSearch.value ?? "").trim()
  if (v) commitNewTag(list, v)
}

// 使用人选项来自已加载的账号目录，不额外请求；高级运营自己即使还没有账号也要能选回自己。
const ownerOptions = computed(() => {
  const options = new Map()
  const selfId = String(user.value?.id ?? "")
  if (selfId && user.value?.role === "senior_operator") {
    options.set(selfId, user.value?.nickname || user.value?.username || "本人")
  }
  for (const account of accountMembers.value) {
    const value = String(account.id ?? "")
    if (!value || options.has(value)) continue
    options.set(value, account.nickname || account.username || `用户 ${value}`)
  }
  return [...options].map(([value, label]) => ({ value, label }))
})

// 服务端已按角色划定可见范围（普通运营强制自己、高级运营/管理员为团队范围），
// 这里只在已加载数据上再按使用人收窄，不扩大可见边界。
const ownerFilteredAccounts = computed(() => {
  if (!canFilterByOwner.value || !searchOwner.value || searchOwner.value === "all") return accounts.value
  return accounts.value.filter(account => String(account.user_id ?? "") === searchOwner.value)
})

const resourceAccountStats = computed(() => [
  { key: "total", label: "账号总数", value: accountStats.value.total, tone: "info" },
  { key: "normal", label: "正常账号", value: accountStats.value.normal, tone: "success" },
  { key: "abnormal", label: "异常账号", value: accountStats.value.abnormal, tone: "danger" },
  { key: "unbound", label: "未绑定环境", value: accountStats.value.unbound, tone: "warning" },
])

const currentBatchStats = computed(batchStats)
const hasBatchFailures = computed(() => batchResults.value.some(item => item.status === "failed"))
const pagedAccounts = computed(() => accounts.value)

const columns = [
  { colKey: "row-select", type: "multiple", width: 40 },
  { colKey: "id", title: "ID", width: 60 },
  { colKey: "platform", title: "平台", width: 70 },
  { colKey: "account_info", title: "账号信息", width: 190 },
  { colKey: "user_name", title: "使用人", width: 100 },
  { colKey: "category_ids", title: "类目", width: 160 },
  { colKey: "tags", title: "标签", width: 140 },
  { colKey: "window_info", title: "浏览器窗口", width: 180 },
  { colKey: "enabled", title: "业务状态", width: 80, minWidth: 80 },
  { colKey: "account_status", title: "账号状态", width: 90, minWidth: 90 },
  { colKey: "last_check_at", title: "最近检查", width: 110, minWidth: 110 },
  { colKey: "op", title: "操作", width: 340, fixed: "right" },
]
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
  <div class="wt-resource-page">
    <ResourcePageHeader title="社媒账号" description="管理账号资产、浏览器环境关联和账号检查状态">
      <template #actions>
        <t-button v-if="!isAdmin" theme="primary" @click="openCreate">新增账号</t-button>
        <t-button v-if="isDesktop && !isAdmin" variant="outline" class="wt-secondary-button" :loading="batchChecking" @click="runBatchCheck()">批量检查</t-button>
        <t-button class="wt-secondary-button" variant="outline" @click="() => { loadAuxiliaryData(); loadAccounts() }">刷新</t-button>
      </template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <FeedbackAlert
      v-if="!isDesktop"
        message="Cloud Web 只展示 Cloud 已保存的账号与窗口绑定信息；打开/关闭窗口、绑定/换绑和检查/同步账号信息请在 Desktop 端完成。"
      theme="info"
      style="margin-bottom:16px"
    />

    <ResourceStatGrid :items="resourceAccountStats" interactive @select="applyStatFilter" />

    <ResourceCard class="account-resource-card">
    <div v-if="searchView" style="margin-bottom:12px"><t-tag closable @close="searchView='';loadAccounts({resetPage:true})">{{ {normal:"正常账号",attention:"待处理账号",unbound:"未绑定窗口"}[searchView] }}</t-tag></div>
    <div class="filter-bar">
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">综合搜索</span><t-input v-model="searchText" placeholder="搜索账号、UID、备注" clearable /></label>
        <label v-if="canFilterByOwner" class="wt-filter-field"><span class="wt-filter-field__label">使用人</span><t-select v-model="searchOwner" placeholder="全部">
            <t-option value="all" label="全部成员" />
            <t-option v-for="option in ownerOptions" :key="option.value" :value="option.value" :label="option.label" />
          </t-select></label>
        <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">团队</span><t-select v-model="searchTeam" clearable placeholder="全部团队"><t-option v-for="team in accountTeams" :key="team.id" :value="String(team.id)" :label="team.name" /></t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">一级分类</span><t-select v-model="searchCategoryType" clearable placeholder="全部" @change="searchCategoryIds=[]"><t-option value="game" label="游戏" /><t-option value="ai" label="AI" /><t-option value="anime" label="二次元" /></t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span><t-select v-model="searchCategoryIds" multiple clearable placeholder="全部">
            <t-option v-for="category in categories.filter(c => !searchCategoryType || c.type === searchCategoryType)" :key="category.id" :value="category.id" :label="category.name" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">平台</span><t-select v-model="searchPlatform" placeholder="全部" clearable>
            <t-option v-for="option in ACCOUNT_PLATFORM_OPTIONS.filter(o => ['bilibili','baidu'].includes(o.value))" :key="option.value" :value="option.value" :label="option.label" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">业务状态</span><t-select v-model="searchEnabled" placeholder="全部" clearable>
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">账号状态</span><t-select v-model="searchLoginStatus" @change="searchView=''" placeholder="全部" clearable>
            <t-option value="normal" label="正常" />
            <t-option value="not_logged_in" label="未登录" />
            <t-option value="expired" label="登录失效" />
            <t-option value="account_mismatch" label="账号不匹配" />
            <t-option value="restricted" label="账号受限" />
            <t-option value="environment_error" label="检查失败" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">标签</span><t-select v-model="searchTags" multiple filterable placeholder="全部" clearable>
            <t-option v-for="t in availableTags" :key="t" :value="t" :label="t" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">标签匹配</span><t-select v-model="searchTagMatch"><t-option value="any" label="任一标签" /><t-option value="all" label="全部标签" /><t-option value="exclude" label="排除标签" /></t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">窗口</span><t-input v-model="searchProfile" placeholder="窗口名 / 序号 / BitID / ID" clearable /></label>
        <div class="wt-filter-actions">
          <t-button theme="primary" @click="loadAccounts({ resetPage: true })">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
        </div>
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

    <div class="table-scroll-wrap">
    <t-table class="wt-resource-table"
      :data="pagedAccounts"
      :columns="columns"
      v-model:selected-row-keys="selectedAccountIds"
      row-key="id"
      size="small"
      hover
      :scroll="{ x: 'max-content' }"
      empty="暂无媒体账号"
    >
      <template #id="{ row }"><span class="account-id" :title="`系统账号ID：${row.id}（点击复制）`" @click="copyAccountId(row)">{{ row.id }}</span></template>
      <template #platform="{ row }"><span>{{ platformLabel(row.platform) }}</span></template>
      <template #account_info="{ row }">
        <div>
          <a class="account-name-link" @click="openAccountInfoPopup(row)">{{ accountDisplayName(row) }}</a>
          <div class="account-sub">UID：{{ row.platform_account_id || '--' }}</div>
        </div>
      </template>
      <template #user_name="{ row }">{{ row.user_name || '--' }}</template>
      <template #category_ids="{ row }">{{ categoryNames(row.category_ids) }}</template>
      <template #tags="{ row }">
        <div class="account-tags">
          <t-tag v-for="tag in (row.tags || []).slice(0, 2)" :key="tag" size="small" variant="light">{{ tag }}</t-tag>
          <t-tag v-if="(row.tags || []).length > 2" size="small" variant="light" class="tags-more" @click="showAllTags(row)">+{{ (row.tags || []).length - 2 }}</t-tag>
        </div>
      </template>
      <template #window_info="{ row }">
        <div v-if="row.browser_profile_id" class="window-cell" @click="openWindowPopup(row)">
          <div class="window-proxy"><span class="proxy-dot">●</span> {{ profileProxyModeText(row) }}</div>
          <div class="window-proxy"><span class="proxy-dot">●</span> {{ profileWindowStateText(row) }}</div>
          <div class="window-name window-name-link">{{ profileName(row.browser_profile_id) }}</div>
        </div>
        <div v-else class="window-cell"><span class="window-unbound">未绑定窗口</span></div>
      </template>
      <template #enabled="{ row }">
        <ResourceStatusBadge :tone="row.enabled ? 'success' : 'neutral'" :label="enabledText(row.enabled)" />
      </template>
      <template #account_status="{ row }">
        <ResourceStatusBadge :tone="accountStatusTheme(row) === 'success' ? 'success' : accountStatusTheme(row) === 'danger' ? 'danger' : accountStatusTheme(row) === 'warning' ? 'warning' : 'neutral'" :label="accountStatusText(row)" />
      </template>
      <template #last_check_at="{ row }">
        <span :title="row.last_check_at ? formatTime(row.last_check_at) : ''">{{ relativeTime(row.last_check_at) }}</span>
      </template>
      <template #op="{ row }">
        <t-space size="small" class="op-cell">
          <t-button v-if="canOperateLocalAccount(row)" size="small" theme="primary" :loading="currentCheckingId === row.id" :disabled="batchChecking || Boolean(currentCheckingId) || (isDesktop && !canOperateLocalAccount(row))" :title="isDesktop ? localOperationHint(row) : ''" @click="checkFromRow(row)">检查</t-button>
          <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">查看</t-button>
          <t-button v-if="canOperateLocalAccount(row) && showWindowToggle(row)" size="small" class="wt-secondary-button" variant="outline" :disabled="!canOperateLocalAccount(row) || !canOperateBoundWindow(row) || Boolean(operatingProfileId)" :title="localOperationHint(row)" @click="toggleWindow(row)">{{ profileIsOpen(row) ? '关闭窗口' : '打开窗口' }}</t-button>
          <t-button size="small" class="wt-secondary-button" variant="outline" @click="openEdit(row)">编辑</t-button>
          <t-dropdown :options="rowMoreOptions(row)" @click="(option) => onRowMore(option, row)"><t-button size="small" variant="text">更多</t-button></t-dropdown>
        </t-space>
      </template>
    </t-table>
    </div>
    <div class="pagination-bar">
      <t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" @change="onPageChange" :total="pagination.total" :page-size-options="[10, 20, 50, 100]" />
    </div>
    </ResourceCard>

    <!-- 账号信息弹窗（点击昵称） -->
    <t-dialog :visible="accountInfoPopup !== null" @close="accountInfoPopup = null" header="账号信息" :footer="false" width="480px">
      <div v-if="accountInfoPopup" class="info-popup">
        <div class="info-head">
          <t-avatar v-if="accountInfoPopup.platform_avatar_url" :image="accountInfoPopup.platform_avatar_url" size="medium" />
          <div>
            <div class="info-name">{{ accountInfoPopup.platform_name || '待识别账号' }}</div>
            <div class="info-sub">{{ platformLabel(accountInfoPopup.platform) }}</div>
          </div>
        </div>
        <t-descriptions :column="1" bordered size="small" style="margin-top:12px">
          <t-descriptions-item label="系统账号ID">{{ accountInfoPopup.id }}</t-descriptions-item>
          <t-descriptions-item label="平台UID">{{ accountInfoPopup.platform_account_id || '--' }}</t-descriptions-item>
          <t-descriptions-item label="平台昵称">{{ accountInfoPopup.platform_name || '--' }}</t-descriptions-item>
          <t-descriptions-item label="使用人">{{ accountInfoPopup.user_name || '--' }}</t-descriptions-item>
          <t-descriptions-item label="所属类目">{{ categoryNames(accountInfoPopup.category_ids) }}</t-descriptions-item>
          <t-descriptions-item label="标签">{{ (accountInfoPopup.tags || []).join('、') || '--' }}</t-descriptions-item>
          <t-descriptions-item label="业务状态">{{ enabledText(accountInfoPopup.enabled) }}</t-descriptions-item>
          <t-descriptions-item label="账号状态">{{ accountStatusText(accountInfoPopup) }}</t-descriptions-item>
          <t-descriptions-item label="最近检查">{{ accountInfoPopup.last_check_at ? formatTime(accountInfoPopup.last_check_at) : '从未检查' }}</t-descriptions-item>
          <t-descriptions-item label="最近同步资料">{{ accountInfoPopup.last_check_at ? formatTime(accountInfoPopup.last_check_at) : '--' }}</t-descriptions-item>
          <t-descriptions-item label="备注">{{ accountInfoPopup.remark || '--' }}</t-descriptions-item>
        </t-descriptions>
      </div>
    </t-dialog>

    <!-- 浏览器窗口弹窗（点击窗口名） -->
    <t-dialog :visible="windowPopup !== null" @close="windowPopup = null" header="浏览器窗口信息" :footer="false" width="480px">
      <div v-if="windowPopup" class="info-popup">
        <div class="info-name">{{ profileName(windowPopup.browser_profile_id) }}</div>
        <t-descriptions :column="1" bordered size="small" style="margin-top:12px">
          <t-descriptions-item label="窗口名称">{{ profileName(windowPopup.browser_profile_id) }}</t-descriptions-item>
          <t-descriptions-item label="系统窗口ID">{{ profileForAccount(windowPopup)?.id || '--' }}</t-descriptions-item>
          <t-descriptions-item label="BitBrowser ID">
            <span class="window-bit-id" :title="profileForAccount(windowPopup)?.bit_profile_id || ''">{{ profileForAccount(windowPopup)?.bit_profile_id || '--' }}</span>
          </t-descriptions-item>
          <t-descriptions-item label="浏览器窗口状态">{{ profileWindowStateText(windowPopup) }}</t-descriptions-item>
          <t-descriptions-item label="代理状态">{{ profileProxyModeText(windowPopup) }}</t-descriptions-item>
          <t-descriptions-item label="当前运行状态">{{ profileIsOpen(windowPopup) ? '已打开' : '已关闭' }}</t-descriptions-item>
          <t-descriptions-item label="最近同步">{{ profileForAccount(windowPopup)?.last_synced_at ? formatTime(profileForAccount(windowPopup).last_synced_at) : '--' }}</t-descriptions-item>
        </t-descriptions>
        <div style="margin-top:12px; text-align:right">
          <t-button v-if="canOperateBoundWindow(windowPopup)" variant="outline" :disabled="!canOperateLocalAccount(windowPopup)" :title="localOperationHint(windowPopup)" :loading="operatingProfileId.startsWith('open:') || operatingProfileId.startsWith('close:')" @click="toggleWindow(windowPopup)">{{ profileIsOpen(windowPopup) ? '关闭窗口' : '打开窗口' }}</t-button>
        </div>
      </div>
    </t-dialog>

    <!-- 全部标签弹窗 -->
    <t-dialog :visible="allTagsPopup !== null" @close="allTagsPopup = null" header="全部标签" :footer="false" width="360px">
      <div v-if="allTagsPopup">
        <t-space wrap>
          <t-tag v-for="tag in (allTagsPopup.tags || [])" :key="tag" size="medium" variant="light">{{ tag }}</t-tag>
          <span v-if="!(allTagsPopup.tags || []).length" class="info-tip">暂无标签</span>
        </t-space>
      </div>
    </t-dialog>

    <t-dialog v-model:visible="showCreate" header="新增账号" width="640px" @confirm="createAccount" :confirm-btn="{ loading: creating, theme: 'primary' }">
      <FeedbackAlert v-if="createError" :message="createError" theme="error" style="margin-bottom:12px" />
      <t-form @submit.prevent="createAccount">
        <div class="form-section">基本信息</div>
        <t-form-item label="分类">
          <t-space direction="vertical" style="width:100%">
            <t-select v-model="createCategoryType" clearable placeholder="选择一级分类" @change="createForm.categoryIds = []">
              <t-option value="game" label="游戏" /><t-option value="ai" label="AI" /><t-option value="anime" label="二次元" />
            </t-select>
            <t-select v-model="createForm.categoryIds" multiple clearable :disabled="!createCategoryType" placeholder="选择二级分类（可多选）">
              <t-option v-for="category in categories.filter(c => c.type === createCategoryType)" :key="category.id" :value="category.id" :label="category.name" />
            </t-select>
          </t-space>
        </t-form-item>
        <t-form-item label="平台">
          <t-select v-model="createForm.platform">
            <t-option v-for="option in ACCOUNT_PLATFORM_OPTIONS.filter(o => ['bilibili','baidu'].includes(o.value))" :key="option.value" :value="option.value" :label="option.label" />
          </t-select>
        </t-form-item>
        <div class="form-section">业务分类</div>
        <t-form-item label="标签">
          <div class="tag-editor">
            <t-select
              v-model="createForm.tags"
              multiple
              filterable
              :options="tagOptions"
              placeholder="输入搜索已有标签，或输入新标签后回车创建"
              v-model:input-value="tagSearch"
              @input-change="onTagSearch"
              @enter="ctx => onTagEnter(createForm.tags, ctx)"
              clearable
            >
              <template #panelBottomContent>
                <div v-if="tagCreateCandidate" class="tag-create-option" @mousedown.prevent="commitNewTag(createForm.tags, tagCreateCandidate)">
                  <t-icon name="add" /> 创建标签「{{ tagCreateCandidate }}」
                </div>
              </template>
            </t-select>
          </div>
        </t-form-item>
        <div class="form-section">备注</div>
        <t-form-item label="备注">
          <t-textarea v-model="createForm.remark" :rows="3" placeholder="账号来源、用途或注意事项" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog :visible="createdGuide !== null" @close="createdGuide = null" header="账号已保存" width="560px" :footer="false">
      <p>可以继续绑定浏览器窗口，也可以稍后从账号列表继续。</p>
      <t-space style="display:flex;justify-content:flex-end;margin-top:24px">
        <t-button variant="outline" @click="createdGuide = null">稍后处理</t-button>
        <t-button v-if="canOperateLocalAccount(createdGuide)" theme="primary" @click="openBind(createdGuide)">绑定窗口</t-button>
      </t-space>
    </t-dialog>
    <t-dialog v-model:visible="bindVisible" :header="bindAccount?.browser_profile_id ? '更换浏览器窗口' : '绑定浏览器窗口'" width="960px" :confirm-btn="{content:bindAccount?.browser_profile_id ? '确认更换' : '绑定',loading:bindingSaving}" @confirm="saveBinding">
      <p>{{ accountDisplayName(bindAccount) }} · {{ platformLabel(bindAccount?.platform) }}</p>
      <p class="form-tip">关联已有窗口不会修改窗口内的登录信息。绑定后检查并识别当前账号。</p>
      <FeedbackAlert v-if="bindingError" :message="bindingError" theme="error" />
      <t-input v-model="bindSearch" clearable placeholder="搜索窗口名称 / 序号 / 分组" style="margin:16px 0" />
      <t-table :data="bindingCandidates" row-key="id" size="small" :loading="bindingLoading" :max-height="360" :pagination="null" :columns="[{colKey:'select',title:'',width:48},{colKey:'bit_name',title:'窗口名称'},{colKey:'bit_seq',title:'序号',width:80},{colKey:'bit_group_name',title:'分组'},{colKey:'usage',title:'使用情况',width:180}]">
        <template #select="{row}"><t-radio :checked="bindProfileId === row.id" :disabled="Boolean(bindingUnavailableReason(row))" :aria-label="'选择' + row.bit_name" @change="bindProfileId = row.id" /></template>
        <template #usage="{row}"><span :style="{color:bindingUnavailableReason(row) ? 'var(--td-text-color-secondary)' : 'var(--td-success-color)'}">{{ bindingUnavailableReason(row) || (row.id === bindAccount?.browser_profile_id ? '当前账号已绑定' : '可关联') }}</span></template>
      </t-table>
    </t-dialog>
    <t-dialog :visible="boundGuide !== null" @close="boundGuide = null" header="窗口已绑定" width="560px" :footer="false">
      <p>可以打开窗口完成登录，也可以直接检查已登录的账号。</p>
      <t-space style="display:flex;justify-content:flex-end;margin-top:24px">
        <t-button variant="outline" @click="boundGuide = null">稍后处理</t-button>
        <t-button @click="checkFromRow(boundGuide); boundGuide = null">检查账号</t-button>
        <t-button theme="primary" @click="!profileIsOpen(boundGuide) && toggleWindow(boundGuide); boundGuide = null">打开窗口</t-button>
      </t-space>
    </t-dialog>

    <!-- 编辑账号（纯资料，不含打开关闭/检查/Cookie） -->
    <t-dialog v-model:visible="editVisible" header="编辑资料" width="640px" :confirm-btn="{ content: '保存', loading: savingEdit, theme: 'primary' }" @confirm="saveEdit">
      <FeedbackAlert v-if="editError" :message="editError" theme="error" style="margin-bottom:12px" />
      <t-form v-if="editAccount" label-width="90px">
        <div class="form-section">基本信息</div>
        <t-form-item label="账号">{{ accountDisplayName(editAccount) }}（{{ platformLabel(editAccount.platform) }}）</t-form-item>
        <t-form-item label="分类">
          <t-space direction="vertical" style="width:100%">
            <t-select v-model="editCategoryType" clearable placeholder="选择一级分类" @change="editCategoryIds = []">
              <t-option value="game" label="游戏" /><t-option value="ai" label="AI" /><t-option value="anime" label="二次元" />
            </t-select>
            <t-select v-model="editCategoryIds" multiple clearable :disabled="!editCategoryType" placeholder="选择二级分类（可多选）">
              <t-option v-for="category in categories.filter(c => c.type === editCategoryType)" :key="category.id" :value="category.id" :label="category.name" />
            </t-select>
          </t-space>
        </t-form-item>
        <div class="form-section">业务分类</div>
        <t-form-item label="标签">
          <div class="tag-editor">
            <t-select
              v-model="editTags"
              multiple
              filterable
              :options="tagOptions"
              placeholder="输入搜索已有标签，或输入新标签后回车创建"
              v-model:input-value="tagSearch"
              @input-change="onTagSearch"
              @enter="ctx => onTagEnter(editTags, ctx)"
              clearable
            >
              <template #panelBottomContent>
                <div v-if="tagCreateCandidate" class="tag-create-option" @mousedown.prevent="commitNewTag(editTags, tagCreateCandidate)">
                  <t-icon name="add" /> 创建标签「{{ tagCreateCandidate }}」
                </div>
              </template>
            </t-select>
          </div>
        </t-form-item>
        <div class="form-section">备注</div>
        <t-form-item label="备注">
          <t-textarea v-model="editRemark" :rows="3" placeholder="账号备注" />
        </t-form-item>
        <div class="form-tip">平台 UID/头像/账号状态由检查回填，不在此编辑。</div>
      </t-form>
    </t-dialog>

    <t-drawer :close-btn="true" v-model:visible="detailVisible" header="账号详情" :size="'800px'" destroy-on-close :footer="false">
      <div v-if="detailAccount" class="account-detail-heading">
        <t-avatar size="56px" :image="detailAccount.platform_avatar_url || undefined">{{ accountDisplayName(detailAccount).slice(0,1) }}</t-avatar>
        <div><h2>{{ accountDisplayName(detailAccount) }}</h2><p>{{ platformLabel(detailAccount.platform) }} · ID {{ detailAccount.id }} · UID {{ detailAccount.platform_account_id || '尚未识别' }}</p>
          <t-space size="small"><ResourceStatusBadge :tone="detailAccount.enabled ? 'success' : 'neutral'" :label="enabledText(detailAccount.enabled)" /><ResourceStatusBadge :tone="accountStatusTheme(detailAccount) === 'success' ? 'success' : accountStatusTheme(detailAccount) === 'danger' ? 'danger' : 'warning'" :label="accountStatusText(detailAccount)" /></t-space>
        </div>
      </div>
      <div class="form-section">业务信息</div>
      <t-descriptions v-if="detailAccount" :column="1" bordered size="small">
        <t-descriptions-item label="业务分类">{{ categoryNames(detailAccount.category_ids) }}</t-descriptions-item>
        <t-descriptions-item label="业务标签"><t-space size="small" wrap><t-tag v-for="tag in detailAccount.tags" :key="tag" size="small" variant="light">{{ tag }}</t-tag><span v-if="!detailAccount.tags?.length">—</span></t-space></t-descriptions-item>
        <t-descriptions-item label="运营备注">{{ detailAccount.remark || '—' }}</t-descriptions-item>
        <t-descriptions-item v-if="canFilterByOwner" label="使用人">{{ detailAccount.user_name || '—' }}</t-descriptions-item>
      </t-descriptions>
      <div class="form-section">关联窗口</div>
      <t-descriptions v-if="detailAccount" :column="1" bordered size="small">
        <t-descriptions-item label="浏览器窗口">{{ detailAccount.browser_profile_id ? profileName(detailAccount.browser_profile_id) : '未绑定' }}</t-descriptions-item>
        <t-descriptions-item label="窗口分组">{{ profileForAccount(detailAccount)?.bit_group_name || '—' }}</t-descriptions-item>
      </t-descriptions>
      <div class="form-section">账号检查</div>
      <t-descriptions v-if="detailAccount" :column="1" bordered size="small">
        <t-descriptions-item label="登录状态">{{ loginStatusText(detailAccount.platform_login_status) }}</t-descriptions-item>
        <t-descriptions-item label="可执行性">{{ executableText(detailAccount) }}</t-descriptions-item>
        <t-descriptions-item label="最近检查">{{ detailAccount.last_check_at ? formatTime(detailAccount.last_check_at) : '尚未检查' }}</t-descriptions-item>
        <t-descriptions-item label="账号资料">{{ detailAccount.platform_account_id ? '已取得平台身份' : '尚未识别' }}</t-descriptions-item>
      </t-descriptions>
      <FeedbackAlert v-if="detailAccount?.last_check_status === 'failed'" theme="error" :message="detailAccount.last_check_message || '本次检查失败，保留上次登录结论'" style="margin-top:12px" />
      <div v-if="detailAccount?.platform_login_status === 'account_mismatch' && detailAccount?.last_check_identity" class="identity-comparison">
        <div><strong>已记录账号</strong><p>{{ detailAccount.platform_name || '待识别' }}</p><p>UID {{ detailAccount.platform_account_id || '—' }}</p></div>
        <div><strong>窗口当前账号</strong><p>{{ detailAccount.last_check_identity.platform_name || '未取得昵称' }}</p><p>UID {{ detailAccount.last_check_identity.platform_account_id || '—' }}</p></div>
      </div>
      <FeedbackAlert v-if="checkFailure" theme="error" style="margin-top:12px">
        <template #message>{{ checkFailure }}</template>
        <template #operation>
          <t-button v-if="checkFailureEnv" size="small" variant="outline" @click="router.push('/personal-info')">前往个人中心检查设备</t-button>
        </template>
      </FeedbackAlert>
    </t-drawer>

    <t-dialog v-model:visible="showCookieDialog" width="640px" header="账号 Cookie" :footer="false">
      <p v-if="cookieDialog?.account"><strong>账号：</strong>{{ accountDisplayName(cookieDialog.account) }}</p>
      <p><strong>原始 Cookie：</strong></p>
      <pre class="cookie-box">{{ cookieDialog?.original_cookie || '（空）' }}</pre>
      <p><strong>当前真实 Cookie（active）：</strong></p>
      <pre class="cookie-box">{{ cookieDialog?.active_cookie || '（空）' }}</pre>
      <div class="form-tip">Cookie 为敏感数据，请勿泄露。复制后建议及时清理剪贴板；导出仅用于授权用途。</div>
      <t-space style="margin-top:8px">
        <t-button v-if="isDesktop && cookieDialog?.account?.browser_profile_id" size="small" :disabled="!canOperateLocalAccount(cookieDialog?.account) || !canOperateBoundWindow(cookieDialog?.account)" :title="localOperationHint(cookieDialog?.account)" :loading="readingCookie" @click="readProfileCookieFor(cookieDialog.account)">从 Profile 读真实 Cookie</t-button>
        <t-button size="small" @click="copyCookie('active_cookie')">复制 Active Cookie</t-button>
        <t-button size="small" variant="outline" @click="copyCookie('original_cookie')">复制原始 Cookie</t-button>
      </t-space>
    </t-dialog>
  </div>
  </t-loading>
</template>

<style scoped>
.account-detail-heading { display:flex;gap:16px;align-items:center;margin-bottom:24px; }
.account-detail-heading h2 { margin:0;font-size:20px; }
.account-detail-heading p { margin:6px 0 10px;font-size:12px;color:var(--td-text-color-secondary); }
.account-resource-card { padding: 18px 20px; }
.form-section { font-weight: 600; font-size: 13px; color: var(--td-text-color-secondary); margin: 12px 0 6px; }
.form-section:first-child { margin-top: 0; }
.stats-row { display: flex; align-items: stretch; flex-wrap: wrap; gap: 12px; margin-bottom: 16px; }
.stats-group { display: flex; align-items: center; color: var(--td-text-color-secondary, #666); font-size: 12px; padding: 0 4px; white-space: nowrap; }
.stat-card { flex: 1 1 118px; cursor: pointer; min-width: 118px; min-height: 72px; max-height: 80px; }
.stat-card :deep(.t-card__body) { padding: 12px 16px; }
.stat-label { font-size: 12px; color: var(--td-text-color-secondary, #666); }
.stat-num { font-size: 20px; font-weight: 600; line-height: 1.2; margin-top: 2px; }
.batch-result-card { margin-bottom: 16px; }
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
.account-id { cursor: pointer; color: var(--td-brand-color, #0052d9); max-width: 56px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-block; vertical-align: bottom; }
.account-name-link { cursor: pointer; color: var(--td-brand-color, #0052d9); font-weight: 500; }
.window-name-link { cursor: pointer; color: var(--td-brand-color, #0052d9); }
.account-sub { color: var(--td-text-color-secondary, #666); font-size: 12px; }
.account-remark { color: var(--td-text-color-secondary, #666); font-size: 12px; }
.account-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.tags-more { cursor: pointer; }
.tag-editor { width: 100%; }
.tag-editor :deep(.t-select) { width: 100%; }
.tag-create-option { display: flex; align-items: center; gap: 4px; padding: 8px 12px; cursor: pointer; color: var(--td-brand-color, #0052d9); font-size: 14px; }
.tag-create-option:hover { background: var(--td-bg-color-container-hover, #f3f3f3); }
.window-cell { cursor: pointer; }
.window-proxy { font-size: 12px; }
.proxy-dot { margin-right: 2px; }
.proxy-dot.proxy-ok { color: var(--td-success-color, #2ba471); }
.window-name { font-size: 13px; }
.window-unbound { color: var(--td-text-color-secondary, #666); }
.window-binding-name { font-size: 13px; font-weight: 500; }
.window-binding-meta { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--td-text-color-secondary, #666); margin-top: 4px; }
.window-bit-id { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 360px; display: inline-block; vertical-align: bottom; }
.op-cell { white-space: nowrap; }
.info-popup .info-head { display: flex; gap: 10px; align-items: center; }
.info-popup .info-name { font-size: 16px; font-weight: 600; }
.info-popup .info-sub { color: var(--td-text-color-secondary, #666); font-size: 13px; }
.info-tip { color: var(--td-text-color-secondary, #666); font-size: 12px; }
.status-green { color: var(--td-success-color, #2ba471); }
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
  margin-bottom: 16px;
}
.account-sub,
.form-tip {
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
.account-name { font-weight: 500; }
.account-tags { display: flex; gap: 4px; margin-top: 4px; flex-wrap: wrap; }
.detail-form { margin-top: 16px; }
</style>
