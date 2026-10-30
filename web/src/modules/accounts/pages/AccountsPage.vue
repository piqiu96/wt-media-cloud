<script setup>
import BusinessSelect from "../../../shared/ui/fields/BusinessSelect.vue"
import ResourceSelectionToolbar from "../../../shared/ui/resource/ResourceSelectionToolbar.vue"
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue"
import ResourceForm from "../../../shared/ui/resource/ResourceForm.vue"
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue"
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue"
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue"
import CategoryFields from "../../../shared/ui/fields/CategoryFields.vue"
import TagInput from "../../../shared/ui/fields/TagInput.vue"
import { opColumnWidth } from "../../../shared/ui/resource/opColumnWidth.js"

import AccountImportDialog from './AccountImportDialog.vue'
import {createMediaAccountImportClient} from '../../../shared/api/mediaAccountImports.js'

import { executeCookieUpdate, reconcileCookieUpdate } from "./cookieUpdates.js"
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
import { CATEGORY_TYPE_LABELS } from "../../../shared/utils/categories.js"
import OperationProgressDialog from "../../../shared/ui/feedback/OperationProgressDialog.vue"
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

const importClient=createMediaAccountImportClient()
const importVisible=ref(false),importId=ref(''),importHistoryOnly=ref(false),recentImport=ref(null)
async function loadRecentImport(){if(!isDesktop||isAdmin.value)return;try{recentImport.value=(await importClient.list()).find(b=>b.counts.pending+b.counts.failed+b.counts.uncertain+b.counts.running>0)||null}catch{}}
function openImports(id='',historyOnly=false){importId.value=id;importHistoryOnly.value=historyOnly;importVisible.value=true}
async function importsChanged(){await Promise.all([loadAccounts(),loadAuxiliaryData(),loadRecentImport()])}
async function viewImportedAccount(id){try{detailAccount.value=await accountClient.get(id);detailVisible.value=true;importVisible.value=false}catch(e){feedback.toast.error(e.message||'读取账号失败')}}
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
const updatingCookie = ref(false)
const cookieUpdateVisible = ref(false)
const cookieInput = ref("")
const cookieError = ref("")
const cookieRequestKey = ref("")
const cookieOperation = ref(null)
const cookieShown = ref({active_cookie:false,original_cookie:false})
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
const enabledConfirming = ref(new Set())
const enabledPending = ref(new Set())
const showAdvancedFilters = ref(false)
const batchCheckVisible = ref(false)
const batchChecking = ref(false)
const batchResults = ref([])
const operatingProfileId = ref("")
const currentCheckingId = ref("")
const profileOpenMap = ref({})
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
    await Promise.all([loadAuxiliaryData(), loadAccounts(),loadRecentImport()])
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

watch(searchTeam, () => {
  if (searchOwner.value !== "all" && !ownerOptions.value.some(o => o.value === searchOwner.value)) searchOwner.value = "all"
})

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
    selectedAccountIds.value = selectedAccountIds.value.filter(id => result.items.some(row => row.id === id))
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
  selectedAccountIds.value = []
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

async function toggleEnabled(account, target) {
  account = accounts.value.find(row => row.id === account.id)
  if (!account || typeof target !== "boolean" || target === account.enabled) return
  if (enabledPending.value.has(account.id) || enabledConfirming.value.has(account.id)) return
  enabledConfirming.value.add(account.id)
  try {
    if (!await feedback.confirmWarning({ title: target ? "启用账号" : "停用账号", message: `${target ? "启用" : "停用"}「${accountDisplayName(account)}」？${target ? "" : "停用后将不参与后续任务。"}`, confirmText: target ? "启用" : "停用", danger: false })) return
    enabledConfirming.value.delete(account.id)
    enabledPending.value.add(account.id)
    const updated = await accountClient.update(account.id, { enabled: target })
    const idx = accounts.value.findIndex(a => a.id === account.id)
    if (idx >= 0) accounts.value[idx] = { ...accounts.value[idx], ...updated }
    feedback.toast.success(target ? "账号已启用" : "账号已停用")
  } catch (e) {
    feedback.toast.error(e.message || "状态切换失败")
  } finally { enabledPending.value.delete(account.id); enabledConfirming.value.delete(account.id) }
}

function rowMoreOptions(account) {
 return [...(canOperateLocalAccount(account) ? [{content: account.browser_profile_id ? "更换窗口" : "绑定窗口", value:"bind"}] : []), ...(!isAdmin.value && isOwnAccount(account) ? [{content:"账号凭证",value:"credentials"}] : [])]
}
function usesRowMore(account) {
 const base = 2 + (canOperateLocalAccount(account) ? 1 : 0) + (canOperateLocalAccount(account) && showWindowToggle(account) ? 1 : 0)
 return base + rowMoreOptions(account).length > 5
}
function onRowMore(option, account) { if (option.value === "bind") openBind(account); else if (option.value === "credentials") openCookieDialog(account) }


function selectedCategoryType(categoryIds) {
  const values = Array.isArray(categoryIds) ? categoryIds : []
  return categories.value.find((category) => values.includes(category.id))?.type || ""
}


function validateCategorySelection(categoryIds) {
  const selected = new Set(categoryIds || [])
  const types = new Set(categories.value.filter((category) => selected.has(category.id)).map((category) => category.type))
  return types.size > 1 ? "同一个社媒账号的多个类目必须属于同一一级类型" : ""
}

function categoryNames(categoryIds) {
  const values = Array.isArray(categoryIds) ? categoryIds : []
  if (!values.length) return "未选择"
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
// 管理员一律只读。服务端 StartLocalAccountCheck / StartCookieUpdate 用同一套归属判定。
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
  if (!account.enabled) return "不可执行：账号已停用"
  if (!(account.category_ids || []).length) return "不可执行：未绑定类目"
  if (!account.browser_profile_id) return "不可执行：未绑定窗口"
  const profile = profileForAccount(account)
  if (profile && (profile.local_state !== "active" || !profile.enabled)) return "不可执行：绑定窗口已停用"
  if (account.last_check_status === "failed") return "最近检查未完成"
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

async function openCookieDialog(account) {
  if (!account?.id) return
  try {
    const data = await accountClient.credentials(account.id)
    cookieDialog.value = {...data, account}
    cookieOperation.value = data.current_update?.operation_id ? data.current_update : null
    cookieShown.value = {active_cookie:false,original_cookie:false}
    showCookieDialog.value = true
  } catch (e) { feedback.toast.error(e.message || "获取 Cookie 失败") }
}
function openCookieUpdate() {
  cookieInput.value="";cookieError.value="";cookieRequestKey.value="";showCookieDialog.value=false;cookieUpdateVisible.value=true
}
const cookieUnresolved = computed(()=>['running','uncertain'].includes(cookieOperation.value?.status))
async function finishCookieFeedback(receipt) {
  cookieOperation.value=receipt
  if (receipt.status==='succeeded') {
    cookieInput.value="";cookieUpdateVisible.value=false
    feedback.toast.success("登录 Cookie 已更新，账号检查完成")
    await loadAccounts()
  } else if(receipt.status==='failed') {
    cookieError.value=receipt.message || "本次更新失败，请重试";cookieRequestKey.value="";feedback.toast.error(cookieError.value)
  } else {
    cookieError.value=receipt.message || "更新结果待确认，请核实当前登录状态";feedback.toast.info(cookieError.value)
  }
}
async function submitCookieUpdate(reconcile=false) {
  const target=cookieDialog.value?.account
  if(updatingCookie.value||!target||!canOperateBoundWindow(target))return
  if(!reconcile) {
    if(!cookieInput.value.trim()){cookieError.value="请输入新的 Cookie";return}
    if(!await feedback.confirmWarning({title:"更新当前账号的登录 Cookie？",message:`将更新「${accountDisplayName(target)}」在「${profileForAccount(target)?.bit_name || "绑定窗口"}」内的 ${platformLabel(target.platform)} 登录信息。`,confirmText:"更新并检查"}))return
  }
  updatingCookie.value=true;cookieError.value=""
  try {
    const local=await desktopLocalAgentService()
    if(!cookieRequestKey.value)cookieRequestKey.value=crypto.randomUUID()
    const receipt=reconcile ? await reconcileCookieUpdate(accountClient,local,target.id,cookieOperation.value.operation_id) : await executeCookieUpdate(accountClient,local,target.id,{request_key:cookieRequestKey.value,expected_operation_id:cookieOperation.value?.operation_id || "",input_cookie:cookieInput.value},r=>cookieOperation.value=r)
    await finishCookieFeedback(receipt)
  } catch(e) {
    cookieError.value=e.message || "本次操作未完成，请核实后继续"
    // A lost response may already have persisted the operation. Reopen reads its receipt.
    try { const data=await accountClient.credentials(target.id);cookieOperation.value=data.current_update?.operation_id?data.current_update:cookieOperation.value } catch {}
    feedback.toast.error(cookieError.value)
  } finally {updatingCookie.value=false}
}
async function viewExistingCookieAccount() {
 try {const account=await accountClient.get(cookieOperation.value.existing_account_id);cookieUpdateVisible.value=false;showCookieDialog.value=false;detailAccount.value=account;detailVisible.value=true}
 catch(e){feedback.toast.error(e.message || "读取已有账号失败")}
}
async function copyCookie(field) {
 const value=cookieDialog.value?.[field];if(!value)return
 try {await navigator.clipboard.writeText(value);feedback.toast.success("已复制 Cookie")}
 catch {cookieShown.value[field]=true;feedback.toast.error("复制失败，请手动选择复制")}
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
  batchCheckVisible.value = true
  batchChecking.value = true
  const nonOwnedCount = candidates.filter(account => !canOperateLocalAccount(account)).length
  const skippedOwnershipText = nonOwnedCount
    ? isAdmin.value
      ? `，管理员不执行本地操作，已跳过 ${nonOwnedCount} 条`
      : `，已跳过 ${nonOwnedCount} 条非本人账号`
    : ""
  const initialResults = candidates.map(account => {
    if (!canCheckAccount(account)) {
      return { account, status: "skipped", message: localOperationHint(account) || executableText(account) }
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
      item.status = "processing"
      item.message = "正在检查账号"
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
      feedback.toast.error(summary)
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

// 使用人选项来自已加载的账号目录，不额外请求；高级运营自己即使还没有账号也要能选回自己。
const ownerOptions = computed(() => {
  const options = new Map()
  const selfId = String(user.value?.id ?? "")
  if (selfId && user.value?.role === "senior_operator") {
    options.set(selfId, user.value?.nickname || user.value?.username || "本人")
  }
  for (const account of accountMembers.value.filter(member => !searchTeam.value || String(member.team_id) === String(searchTeam.value))) {
    const value = String(account.id ?? "")
    if (!value || options.has(value)) continue
    options.set(value, account.nickname || account.username || `用户 ${value}`)
  }
  return [...options].map(([value, label]) => ({ value, label }))
})


const resourceAccountStats = computed(() => [
  { key: "total", label: "账号总数", value: accountStats.value.total, tone: "info" },
  { key: "normal", label: "正常账号", value: accountStats.value.normal, tone: "success" },
  { key: "abnormal", label: "异常账号", value: accountStats.value.abnormal, tone: "danger" },
  { key: "unbound", label: "未绑定窗口", value: accountStats.value.unbound, tone: "warning" },
])

const pagedAccounts = computed(() => accounts.value)

const columns = computed(() => [
  ...(isDesktop && !isAdmin.value ? [{ colKey: "row-select", type: "multiple", width: 40, fixed: "left" }] : []),
  { colKey: "id", title: "ID", width: 60 },
  { colKey: "platform", title: "平台", width: 70 },
  { colKey: "account_info", title: "账号信息", width: 220 },
  ...(isAdmin.value ? [{ colKey: "team_name", title: "运营团队", width: 120 }] : []),
  ...(canFilterByOwner.value ? [{ colKey: "user_name", title: "使用人", width: 100 }] : []),
  { colKey: "window_info", title: "浏览器窗口", width: 180 },
  { colKey: "category_ids", title: "业务分类", width: 160 },
  { colKey: "tags", title: "标签", width: 140 },
  { colKey: "account_status", title: "登录状态", width: 90, minWidth: 90 },
  { colKey: "executable", title: "可执行性", width: 130 },
  { colKey: "remark", title: "运营备注", width: 160, ellipsis: true },
  { colKey: "enabled", title: "启用", width: 80, minWidth: 80 },
  { colKey: "last_check_at", title: "最近检查", width: 110, minWidth: 110 },
  { colKey: "op", title: "操作", width: opColumnWidth(!isDesktop || isAdmin.value ? ["详情", "编辑"] : ["检查", "详情", "打开窗口", "编辑", "更多"], {variant:"text", primary:true, loading:true}), fixed: "right" },
])
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
  <div class="wt-resource-page resource-visual-approved account-page">
    <ResourcePageHeader title="社媒账号">
      <template #actions>
        <t-button v-if="!isAdmin" theme="primary" @click="openCreate">新增账号</t-button>
        <t-button v-if="isDesktop && !isAdmin" variant="outline" @click="openImports()">批量CK接入</t-button>
        <t-button v-if="isDesktop && !isAdmin" variant="text" @click="openImports('',true)">接入记录</t-button>
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

    <ResourceStatGrid :items="resourceAccountStats" :active-key="({normal: 'normal', attention: 'abnormal', unbound: 'unbound'})[searchView] || 'total'" interactive @select="applyStatFilter" />
    <div v-if="recentImport" class="import-resume"><span>最近一次接入还有 {{ recentImport.counts.pending+recentImport.counts.failed+recentImport.counts.uncertain+recentImport.counts.running }} 项需要处理</span><t-button variant="text" @click="openImports(recentImport.id)">继续处理</t-button></div>

    <ResourceCard class="account-resource-card">
    <div v-if="searchView" style="margin-bottom:12px"><t-tag closable @close="searchView='';loadAccounts({resetPage:true})">{{ {normal:"正常账号",attention:"待处理账号",unbound:"未绑定窗口"}[searchView] }}</t-tag></div>
    <div class="filter-bar">
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">搜索</span><t-input v-model="searchText" placeholder="搜索账号、UID、备注" clearable /></label>
        <label v-if="canFilterByOwner" class="wt-filter-field"><span class="wt-filter-field__label">使用人</span><BusinessSelect kind="member" filter v-model="searchOwner">
            <t-option value="all" label="全部" />
            <t-option v-for="option in ownerOptions" :key="option.value" :value="option.value" :label="option.label" />
          </BusinessSelect></label>
        <label v-if="isAdmin" class="wt-filter-field"><span class="wt-filter-field__label">运营团队</span><BusinessSelect kind="team" filter v-model="searchTeam" clearable><t-option v-for="team in accountTeams" :key="team.id" :value="String(team.id)" :label="team.name" /></BusinessSelect></label>
        <CategoryFields v-model="searchCategoryIds" v-model:type="searchCategoryType" :categories="categories" filter class="account-category-filter" />
        <label class="wt-filter-field"><span class="wt-filter-field__label">平台</span><t-select v-model="searchPlatform" placeholder="全部" clearable>
            <t-option v-for="option in ACCOUNT_PLATFORM_OPTIONS.filter(o => ['bilibili','baidu'].includes(o.value))" :key="option.value" :value="option.value" :label="option.label" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">启用状态</span><t-select v-model="searchEnabled" placeholder="全部" clearable>
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">登录状态</span><t-select v-model="searchLoginStatus" @change="searchView=''" placeholder="全部" clearable>
            <t-option value="normal" label="正常" />
            <t-option value="not_logged_in" label="未登录" />
            <t-option value="expired" label="登录失效" />
            <t-option value="account_mismatch" label="账号不匹配" />
            <t-option value="restricted" label="账号受限" />
            <t-option value="environment_error" label="检查失败" />
        </t-select></label>
        <template v-if="showAdvancedFilters">
        <label class="wt-filter-field"><span class="wt-filter-field__label">标签</span><t-select v-model="searchTags" multiple filterable placeholder="全部" clearable>
            <t-option v-for="t in availableTags" :key="t" :value="t" :label="t" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">标签匹配</span><t-select v-model="searchTagMatch" placeholder="全部"><t-option value="any" label="任一标签" /><t-option value="all" label="全部标签" /><t-option value="exclude" label="排除标签" /></t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">窗口</span><t-input v-model="searchProfile" placeholder="窗口名 / 序号 / BitID / ID" clearable /></label>
        </template>
        <div class="wt-filter-actions">
          <t-button theme="primary" @click="loadAccounts({ resetPage: true })">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="resetFilters">重置</t-button>
          <t-button variant="text" @click="showAdvancedFilters = !showAdvancedFilters">{{ showAdvancedFilters ? "收起筛选" : "更多筛选" }}</t-button>
        </div>
    </div>

    <ResourceSelectionToolbar v-if="isDesktop && !isAdmin" :count="selectedAccountIds.length" unit="个账号" @clear="selectedAccountIds = []">
      <t-button theme="default" variant="outline" :loading="batchChecking" :disabled="Boolean(currentCheckingId)" @click="runBatchCheck()">批量检查</t-button>
    </ResourceSelectionToolbar>
    <OperationProgressDialog :visible="batchCheckVisible" title="批量检查账号" retry-label="重试失败项" :phase="batchChecking ? 'running' : 'result'" running-label="正在检查账号"
      :items="batchResults.map(item => ({key: item.account.id, label: accountLabel(item.account), status: item.status, reason: item.message}))"
      @retry="runBatchCheck({retryFailedOnly:true})" @done="batchCheckVisible = false" />

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
      <template #id="{ row }"><span class="account-id">{{ row.id }}</span></template>
      <template #platform="{ row }"><span>{{ platformLabel(row.platform) }}</span></template>
      <template #account_info="{ row }">
        <div class="account-identity">
          <t-avatar size="32px" :image="row.platform_avatar_url || undefined">{{ accountDisplayName(row).slice(0,1) }}</t-avatar>
          <div class="account-identity__text"><strong class="account-name" :title="accountDisplayName(row)">{{ accountDisplayName(row) }}</strong>
          <div class="account-sub" :title="`UID ${row.platform_account_id || '—'}`">UID {{ row.platform_account_id || '—' }}</div></div>
        </div>
      </template>
      <template #team_name="{ row }">{{ accountTeams.find(t => String(t.id) === String(row.team_id))?.name || accountMembers.find(m => String(m.id) === String(row.user_id))?.team_name || "—" }}</template>
      <template #user_name="{ row }">{{ row.user_name || '--' }}</template>
      <template #category_ids="{ row }"><div class="account-category"><small>{{ CATEGORY_TYPE_LABELS[selectedCategoryType(row.category_ids)] || "—" }}</small><span>{{ categoryNames(row.category_ids) }}</span></div></template>
      <template #tags="{ row }">
        <div class="account-tags">
          <t-tag v-for="tag in (row.tags || []).slice(0, 2)" :key="tag" size="small" variant="light">{{ tag }}</t-tag>
          <t-tag v-if="(row.tags || []).length > 2" size="small" variant="light" class="tags-more" @click="showAllTags(row)">+{{ (row.tags || []).length - 2 }}</t-tag>
        </div>
      </template>
      <template #window_info="{ row }">
        <div v-if="row.browser_profile_id" class="window-cell" @click="openWindowPopup(row)">
          <t-button variant="text" size="small" class="window-name-link">{{ profileName(row.browser_profile_id) }}</t-button>
        </div>
        <div v-else class="window-cell"><span class="window-unbound">未绑定窗口</span></div>
      </template>
      <template #executable="{ row }"><span :class="{'account-attention': executableText(row) !== '可用于运营任务'}">{{ executableText(row).replace(/^不可执行：|^待检查：/, "") }}</span></template>
      <template #enabled="{ row }">
        <t-switch :value="row.enabled" :loading="enabledPending.has(row.id)" :disabled="enabledPending.has(row.id) || enabledConfirming.has(row.id)" :aria-label="`${accountDisplayName(row)}${row.enabled ? '停用' : '启用'}`" @change="value => toggleEnabled(row, value)" />
      </template>
      <template #account_status="{ row }">
        <ResourceStatusBadge :tone="accountStatusTheme(row) === 'success' ? 'success' : accountStatusTheme(row) === 'danger' ? 'danger' : accountStatusTheme(row) === 'warning' ? 'warning' : 'neutral'" :label="accountStatusText(row)" />
      </template>
      <template #last_check_at="{ row }">
        <span :title="row.last_check_at ? formatTime(row.last_check_at) : ''">{{ relativeTime(row.last_check_at) }}</span>
      </template>
      <template #op="{ row }">
        <ResourceRowActions>
          <t-button v-if="canOperateLocalAccount(row)" size="small" variant="text" class="is-primary" :loading="currentCheckingId === row.id" :disabled="batchChecking || Boolean(currentCheckingId) || (isDesktop && !canOperateLocalAccount(row))" :title="isDesktop ? localOperationHint(row) : ''" @click="checkFromRow(row)">检查</t-button>
          <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
          <t-button v-if="canOperateLocalAccount(row) && showWindowToggle(row)" size="small" variant="text" :disabled="!canOperateLocalAccount(row) || !canOperateBoundWindow(row) || Boolean(operatingProfileId)" :title="localOperationHint(row)" @click="toggleWindow(row)">{{ profileIsOpen(row) ? '关闭窗口' : '打开窗口' }}</t-button>
          <t-button size="small" variant="text" @click="openEdit(row)">编辑</t-button>
          <template v-if="!usesRowMore(row)"><t-button v-for="action in rowMoreOptions(row)" :key="action.value" size="small" variant="text" @click="onRowMore(action, row)">{{ action.content }}</t-button></template>
          <t-dropdown v-else :options="rowMoreOptions(row)" @click="(option) => onRowMore(option, row)"><t-button size="small" variant="text">更多</t-button></t-dropdown>
        </ResourceRowActions>
      </template>
    </t-table>
    </div>
    <div class="pagination-bar">
      <t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" @change="onPageChange" :total="pagination.total" :page-size-options="[10, 20, 50, 100]" />
    </div>
    </ResourceCard>

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

    <AccountImportDialog v-model:visible="importVisible" :initial-id="importId" :history-only="importHistoryOnly" :categories="categories" :get-local="desktopLocalAgentService" @changed="importsChanged" @view-account="viewImportedAccount" />
    <ResourceOverlay v-model:visible="showCreate" title="新增账号" :width="640" :busy="creating">
      <FeedbackAlert v-if="createError" :message="createError" theme="error" style="margin-bottom:12px" />
      <ResourceForm>
        <t-form-item label="业务分类"><CategoryFields v-model="createForm.categoryIds" v-model:type="createCategoryType" :categories="categories" /></t-form-item>
        <t-form-item label="平台">
          <t-select v-model="createForm.platform">
            <t-option v-for="option in ACCOUNT_PLATFORM_OPTIONS.filter(o => ['bilibili','baidu'].includes(o.value))" :key="option.value" :value="option.value" :label="option.label" />
          </t-select>
        </t-form-item>
        <t-form-item label="标签"><TagInput v-model="createForm.tags" /></t-form-item>
        <t-form-item label="备注">
          <t-textarea v-model="createForm.remark" :rows="3" placeholder="账号来源、用途或注意事项" />
        </t-form-item>
      </ResourceForm>
    <template #footer><t-button variant="outline" :disabled="creating" @click="showCreate = false">取消</t-button><t-button theme="primary" :loading="creating" @click="createAccount">新增</t-button></template>
    </ResourceOverlay>

    <t-dialog :visible="createdGuide !== null" @close="createdGuide = null" header="账号已保存" width="560px" :footer="false">
      <p>可以继续绑定浏览器窗口，也可以稍后从账号列表继续。</p>
      <t-space style="display:flex;justify-content:flex-end;margin-top:24px">
        <t-button variant="outline" @click="createdGuide = null">稍后处理</t-button>
        <t-button v-if="canOperateLocalAccount(createdGuide)" theme="primary" @click="openBind(createdGuide)">绑定窗口</t-button>
      </t-space>
    </t-dialog>
    <ResourceOverlay v-model:visible="bindVisible" :title="bindAccount?.browser_profile_id ? '更换浏览器窗口' : '绑定浏览器窗口'" :width="960" :busy="bindingSaving">
      <p>{{ accountDisplayName(bindAccount) }} · {{ platformLabel(bindAccount?.platform) }}</p>
      <p class="form-tip">关联已有窗口不会修改窗口内的登录信息。绑定后检查并识别当前账号。</p>
      <FeedbackAlert v-if="bindingError" :message="bindingError" theme="error" />
      <t-input v-model="bindSearch" clearable placeholder="搜索窗口名称 / 序号 / 分组" style="margin:16px 0" />
      <t-table :data="bindingCandidates" row-key="id" size="small" :loading="bindingLoading" :max-height="360" :pagination="null" :columns="[{colKey:'select',title:'',width:48},{colKey:'bit_name',title:'窗口名称'},{colKey:'bit_seq',title:'序号',width:80},{colKey:'bit_group_name',title:'分组'},{colKey:'usage',title:'使用情况',width:180}]">
        <template #select="{row}"><t-radio :checked="bindProfileId === row.id" :disabled="Boolean(bindingUnavailableReason(row))" :aria-label="'选择' + row.bit_name" @change="bindProfileId = row.id" /></template>
        <template #usage="{row}"><span :style="{color:bindingUnavailableReason(row) ? 'var(--td-text-color-secondary)' : 'var(--td-success-color)'}">{{ bindingUnavailableReason(row) || (row.id === bindAccount?.browser_profile_id ? '当前账号已绑定' : '可关联') }}</span></template>
      </t-table>
    <template #footer><t-button variant="outline" :disabled="bindingSaving" @click="bindVisible = false">取消</t-button><t-button theme="primary" :loading="bindingSaving" @click="saveBinding">绑定</t-button></template></ResourceOverlay>
    <t-dialog :visible="boundGuide !== null" @close="boundGuide = null" header="窗口已绑定" width="560px" :footer="false">
      <p>可以打开窗口完成登录，也可以直接检查已登录的账号。</p>
      <t-space style="display:flex;justify-content:flex-end;margin-top:24px">
        <t-button variant="outline" @click="boundGuide = null">稍后处理</t-button>
        <t-button @click="checkFromRow(boundGuide); boundGuide = null">检查账号</t-button>
        <t-button theme="primary" @click="!profileIsOpen(boundGuide) && toggleWindow(boundGuide); boundGuide = null">打开窗口</t-button>
      </t-space>
    </t-dialog>

    <!-- 编辑账号（纯资料，不含打开关闭/检查/Cookie） -->
    <ResourceOverlay v-model:visible="editVisible" title="编辑资料" :width="640" :busy="savingEdit">
      <FeedbackAlert v-if="editError" :message="editError" theme="error" style="margin-bottom:12px" />
      <ResourceForm>
        <t-form-item label="账号">{{ accountDisplayName(editAccount) }}（{{ platformLabel(editAccount.platform) }}）</t-form-item>
        <t-form-item label="业务分类"><CategoryFields v-model="editCategoryIds" v-model:type="editCategoryType" :categories="categories" /></t-form-item>
        <t-form-item label="标签"><TagInput v-model="editTags" /></t-form-item>
        <t-form-item label="备注">
          <t-textarea v-model="editRemark" :rows="3" placeholder="账号备注" />
        </t-form-item>

      </ResourceForm>
    <template #footer><t-button variant="outline" :disabled="savingEdit" @click="editVisible = false">取消</t-button><t-button theme="primary" :loading="savingEdit" @click="saveEdit">保存</t-button></template>
    </ResourceOverlay>

    <ResourceOverlay v-model:visible="detailVisible" title="账号详情" kind="drawer" :width="800" :footer="false">
      <div v-if="detailAccount" class="account-detail-heading">
        <t-avatar size="56px" :image="detailAccount.platform_avatar_url || undefined">{{ accountDisplayName(detailAccount).slice(0,1) }}</t-avatar>
        <div><h2>{{ accountDisplayName(detailAccount) }}</h2><p>{{ platformLabel(detailAccount.platform) }} · ID {{ detailAccount.id }} · UID {{ detailAccount.platform_account_id || '尚未识别' }}</p>
          <t-space size="small"><ResourceStatusBadge :tone="detailAccount.enabled ? 'success' : 'neutral'" :label="enabledText(detailAccount.enabled)" /><ResourceStatusBadge :tone="accountStatusTheme(detailAccount) === 'success' ? 'success' : accountStatusTheme(detailAccount) === 'danger' ? 'danger' : 'warning'" :label="accountStatusText(detailAccount)" /></t-space>
        </div>
      </div>
      <ResourceSection title="业务信息">
      <ResourceDescriptions v-if="detailAccount">
        <t-descriptions-item label="业务分类">{{ categoryNames(detailAccount.category_ids) }}</t-descriptions-item>
        <t-descriptions-item label="业务标签"><t-space size="small" wrap><t-tag v-for="tag in detailAccount.tags" :key="tag" size="small" variant="light">{{ tag }}</t-tag><span v-if="!detailAccount.tags?.length">—</span></t-space></t-descriptions-item>
        <t-descriptions-item label="运营备注">{{ detailAccount.remark || '—' }}</t-descriptions-item>
        <t-descriptions-item v-if="canFilterByOwner" label="使用人">{{ detailAccount.user_name || '—' }}</t-descriptions-item>
      </ResourceDescriptions>
      </ResourceSection><ResourceSection title="关联窗口">
      <ResourceDescriptions v-if="detailAccount">
        <t-descriptions-item label="浏览器窗口">{{ detailAccount.browser_profile_id ? profileName(detailAccount.browser_profile_id) : '未绑定' }}</t-descriptions-item>
        <t-descriptions-item label="窗口分组">{{ profileForAccount(detailAccount)?.bit_group_name || '—' }}</t-descriptions-item>
      </ResourceDescriptions>
      </ResourceSection><ResourceSection title="账号检查">
      <ResourceDescriptions v-if="detailAccount">
        <t-descriptions-item label="登录状态">{{ loginStatusText(detailAccount.platform_login_status) }}</t-descriptions-item>
        <t-descriptions-item label="可执行性">{{ executableText(detailAccount) }}</t-descriptions-item>
        <t-descriptions-item label="最近检查">{{ detailAccount.last_check_at ? formatTime(detailAccount.last_check_at) : '尚未检查' }}</t-descriptions-item>
        <t-descriptions-item label="账号资料">{{ detailAccount.platform_account_id ? '已取得平台身份' : '尚未识别' }}</t-descriptions-item>
      </ResourceDescriptions>
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
    </ResourceSection>
</ResourceOverlay>

    <ResourceOverlay v-model:visible="showCookieDialog" title="账号凭证" :width="640">
      <div class="account-object"><strong>{{ accountDisplayName(cookieDialog?.account) }}</strong><span>{{ platformLabel(cookieDialog?.account?.platform) }} · UID {{ cookieDialog?.account?.platform_account_id || '待识别' }}</span></div>
      <section v-for="field in ['active_cookie','original_cookie']" :key="field" class="credential-section">
        <h4>{{ field==='active_cookie' ? '当前登录 Cookie' : '首次导入 Cookie' }}</h4>
        <pre class="cookie-box">{{ !cookieDialog?.[field] ? '暂无 Cookie' : cookieShown[field] ? cookieDialog[field] : '••••••••••••••••••••••••••••••••' }}</pre>
        <div class="credential-actions"><t-button variant="text" size="small" :disabled="!cookieDialog?.[field]" @click="cookieShown[field]=!cookieShown[field]">{{ cookieShown[field]?'隐藏':'显示' }}</t-button><t-button variant="text" size="small" :disabled="!cookieDialog?.[field]" @click="copyCookie(field)">复制</t-button></div>
        <div v-if="field==='active_cookie'" class="form-tip">最近同步：{{ cookieDialog?.cookie_updated_at ? new Date(cookieDialog.cookie_updated_at).toLocaleString() : '尚未同步' }}</div>
      </section>
      <template #footer><t-button theme="primary" :disabled="!isDesktop || !canOperateBoundWindow(cookieDialog?.account)" @click="openCookieUpdate">{{ cookieUnresolved ? '核实更新结果' : '更新登录 Cookie' }}</t-button></template>
    </ResourceOverlay>
    <ResourceOverlay v-model:visible="cookieUpdateVisible" title="更新登录 Cookie" :width="640" :busy="updatingCookie">
      <div class="account-object"><strong>{{ accountDisplayName(cookieDialog?.account) }}</strong><span>{{ platformLabel(cookieDialog?.account?.platform) }} · UID {{ cookieDialog?.account?.platform_account_id || '待识别' }}</span></div>
      <p class="form-tip">目标窗口：{{ profileForAccount(cookieDialog?.account)?.bit_name || '未绑定' }}</p>
      <template v-if="!cookieUnresolved"><label class="field-label">新的 Cookie</label><t-textarea v-model="cookieInput" :autosize="{minRows:7,maxRows:10}" placeholder="粘贴此账号的新 Cookie" :disabled="updatingCookie" /><p class="form-tip">更新当前平台的登录信息，完成后检查账号身份。首次导入的 Cookie 保留。</p></template>
      <FeedbackAlert v-if="cookieError || cookieUnresolved" :theme="cookieUnresolved?'warning':'error'" :message="cookieError || cookieOperation?.message || '更新结果待确认，请核实当前登录状态'" />
      <template #footer><t-button v-if="cookieUnresolved" variant="outline" :disabled="updatingCookie" @click="openAccountProfile(cookieDialog?.account)">打开窗口</t-button><t-button v-if="cookieOperation?.existing_account_id" variant="outline" @click="viewExistingCookieAccount">查看已有账号</t-button><t-button variant="outline" :disabled="updatingCookie" @click="cookieUpdateVisible=false">取消</t-button><t-button theme="primary" :loading="updatingCookie" @click="submitCookieUpdate(cookieUnresolved)">{{ updatingCookie ? '处理中…' : cookieUnresolved ? '核实更新结果' : '更新并检查' }}</t-button></template>
    </ResourceOverlay>
  </div>
  </t-loading>
</template>

<style scoped>
.account-sub { color:var(--wt-text-tertiary);font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis; }
.account-identity { display:flex;align-items:center;gap:10px; }
.account-identity__text { min-width:0; }
.account-name { display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap; }
.account-category { display:flex;flex-direction:column;gap:4px; }
.account-category small { color:var(--wt-text-tertiary);font-size:12px; }

.account-attention { color: var(--wt-warning); }

.account-category-filter { flex: 0 1 384px; }
.account-page :deep(.resource-row-actions .t-button) { min-height: 32px; height: 32px; font-size: 13px; }
.account-page :deep(.wt-filter-actions) { align-self: flex-end; }
.account-name { color: var(--wt-text-primary); }
.identity-comparison { display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:16px; }
.identity-comparison > div { padding:16px;border:1px solid var(--wt-border);border-radius:6px; }

.import-resume{display:flex;align-items:center;justify-content:space-between;padding:8px 16px;margin-bottom:16px;border:1px solid var(--td-component-border);border-radius:6px;background:var(--td-bg-color-container);font-size:13px}
.account-object{display:flex;flex-direction:column;gap:6px;margin-bottom:20px}.account-object span{font-size:12px;color:var(--td-text-color-secondary)}.credential-section{position:relative;margin-bottom:20px}.credential-section h4{font-size:14px;margin:0 0 10px}.credential-actions{display:flex;justify-content:flex-end;gap:4px}.field-label{display:block;margin-bottom:8px}.dialog-footer{display:flex;justify-content:flex-end;gap:8px;margin-top:24px}
.account-detail-heading { display:flex;gap:16px;align-items:center;margin-bottom:24px; }
.account-detail-heading h2 { margin:0;font-size:20px; }
.account-detail-heading p { margin:6px 0 10px;font-size:12px;color:var(--td-text-color-secondary); }
.account-resource-card { padding: 18px 20px; }
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
.account-id { color: var(--wt-text-tertiary); max-width: 56px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-block; vertical-align: bottom; }

.window-name-link { cursor: pointer; color: var(--td-brand-color, #0052d9); }

.account-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.tags-more { cursor: pointer; }
.window-cell { cursor: pointer; }
.window-unbound { color: var(--td-text-color-secondary, #666); }
.window-binding-name { font-size: 13px; font-weight: 500; }
.window-binding-meta { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--td-text-color-secondary, #666); margin-top: 4px; }
.window-bit-id { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 360px; display: inline-block; vertical-align: bottom; }
.info-tip { color: var(--td-text-color-secondary, #666); font-size: 12px; }
.form-tip {
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
.account-name { font-weight: 500; }
.account-tags { display: flex; gap: 4px; margin-top: 4px; flex-wrap: wrap; }
</style>
