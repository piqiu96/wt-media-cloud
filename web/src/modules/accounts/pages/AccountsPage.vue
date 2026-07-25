<script setup>
import { computed, onMounted, ref } from "vue"
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import { createSessionClient } from "../../../shared/api/session.js"
import { createUsersClient } from "../../../apps/cloud/pages/users/usersApi.js"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"

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
const batchChecking = ref(false)
const batchResults = ref([])

const tagInput = ref("")
const selectedAccountIds = ref([])
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
  if (searchTags.value) params.anyTags = searchTags.value.split(",").map(t => t.trim()).filter(Boolean)
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

function profileForAccount(account) {
  return profiles.value.find(item => item.id === account?.browser_profile_id)
}

function executableText(account) {
  if (account.business_status !== "enabled") return "不可执行：账号未启用"
  if (!account.game_id) return "不可执行：未绑定游戏"
  if (!account.browser_profile_id) return "不可执行：未绑定窗口"
  const profile = profileForAccount(account)
  if (profile && profile.local_status !== "active") return "不可执行：绑定窗口已停用"
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

function canCheckAccount(account) {
  return isDesktop && account && account.business_status === "enabled" && !!account.game_id && !!account.browser_profile_id && canOperateBoundWindow(account)
}

function canOperateBoundWindow(account) {
  const profile = profileForAccount(account)
  return isDesktop && account?.browser_profile_id && profile?.bit_profile_id && profile.local_status === "active"
}

function cloudBaseUrl() {
  if (typeof window === "undefined") return "http://127.0.0.1:18080"
  const origin = window.location?.origin || "http://127.0.0.1:18080"
  if (origin === "http://127.0.0.1:5174" || origin === "http://localhost:5174") {
    return "http://127.0.0.1:18080"
  }
  return origin
}

async function desktopLocalAgentService() {
  if (typeof window === "undefined" || !window.__TAURI_INTERNALS__) {
    throw new Error("账号检查只能在 Desktop 客户端执行")
  }
  const { invoke } = await import("@tauri-apps/api/core")
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
  })
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
  error.value = ""
  checkNotice.value = ""
  try {
    const service = await desktopLocalAgentService()
    await refreshRuntimeWithCooldown(service)
    await service.profileOpen(profile.bit_profile_id)
    checkNotice.value = `已打开绑定窗口：${profile.name || profile.bit_profile_id}`
  } catch (e) {
    error.value = e.message || "打开绑定窗口失败"
  }
}

async function closeAccountProfile(account) {
  const profile = profileForAccount(account)
  if (!canOperateBoundWindow(account)) return
  error.value = ""
  checkNotice.value = ""
  try {
    const service = await desktopLocalAgentService()
    await refreshRuntimeWithCooldown(service)
    await service.profileClose(profile.bit_profile_id)
    checkNotice.value = `已关闭绑定窗口：${profile.name || profile.bit_profile_id}`
  } catch (e) {
    error.value = e.message || "关闭绑定窗口失败"
  }
}

function formatTime(t) {
  if (!t) return "-"
  return new Date(t).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

const activeProfiles = computed(() => profiles.value.filter(profile => profile.local_status === "active"))

const stats = computed(() => ({
  total: accounts.value.length,
  enabled: accounts.value.filter(a => a.business_status === "enabled").length,
  unbound: accounts.value.filter(a => !a.browser_profile_id).length,
  normal: accounts.value.filter(a => a.login_status === "normal").length,
  unknown: accounts.value.filter(a => a.login_status === "unknown").length,
}))

const currentBatchStats = computed(batchStats)
const hasBatchFailures = computed(() => batchResults.value.some(item => item.status === "failed"))

const columns = [
  { colKey: "id", title: "系统ID", width: 130 },
  { colKey: "name", title: "账号", width: 210 },
  { colKey: "platform", title: "平台", width: 90 },
  { colKey: "platform_account_id", title: "平台UID", width: 130 },
  { colKey: "game_id", title: "游戏", width: 120 },
  { colKey: "browser_profile_id", title: "绑定窗口", width: 260 },
  { colKey: "login_status", title: "登录状态", width: 120 },
  { colKey: "business_status", title: "业务状态", width: 100 },
  { colKey: "last_checked_at", title: "最近检查", width: 130 },
  { colKey: "executable", title: "可执行结论", width: 180 },
  { colKey: "op", title: "操作", width: 260 },
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
      <t-col :span="4"><t-card><template #title>已启用</template><div class="stat-num">{{ stats.enabled }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>未绑定窗口</template><div class="stat-num">{{ stats.unbound }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>登录正常</template><div class="stat-num">{{ stats.normal }}</div></t-card></t-col>
      <t-col :span="4"><t-card><template #title>待检查</template><div class="stat-num">{{ stats.unknown }}</div></t-card></t-col>
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
            <t-option value="bilibili" label="B站" />
            <t-option value="baijiahao" label="百家号" />
            <t-option value="douyin" label="抖音" />
          </t-select>
        </t-form-item>
        <t-form-item label="业务状态">
          <t-select v-model="searchBizStatus" placeholder="全部" clearable style="width:120px">
            <t-option value="enabled" label="启用" />
            <t-option value="disabled" label="停用" />
            <t-option value="retired" label="退役" />
          </t-select>
        </t-form-item>
        <t-form-item label="登录状态">
          <t-select v-model="searchLoginStatus" placeholder="全部" clearable style="width:130px">
            <t-option value="unknown" label="待检查" />
            <t-option value="normal" label="登录正常" />
            <t-option value="not_logged_in" label="未登录" />
            <t-option value="expired" label="已失效" />
            <t-option value="account_mismatch" label="账号不一致" />
          </t-select>
        </t-form-item>
        <t-form-item label="标签">
          <t-input v-model="searchTags" placeholder="逗号分隔" clearable style="width:140px" />
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
        <t-button variant="outline" @click="() => { loadAuxiliaryData(); loadAccounts() }">刷新</t-button>
      </t-space>
      <t-space v-if="selectedAccountIds.length" class="batch-actions">
        <t-input v-model="tagInput" placeholder="标签，多个用逗号分隔" style="width:200px" />
        <t-button size="small" @click="changeTags(false)">添加标签</t-button>
        <t-button size="small" @click="changeTags(true)">移除标签</t-button>
        <t-button v-if="isDesktop" size="small" theme="primary" :loading="batchChecking" @click="runBatchCheck()">批量检查/同步</t-button>
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
      <template #id="{ row }">{{ row.id }}</template>
      <template #name="{ row }">
        <div>
          <t-avatar v-if="row.avatar_url" :image="row.avatar_url" size="small" style="margin-right:6px" />
          <div class="account-name">{{ row.name || row.platform_account_id || '待检查账号' }}</div>
          <div class="account-sub">{{ row.remark || row.id }}</div>
          <div class="account-tags">
            <t-tag v-for="tag in row.tags || []" :key="tag" size="small" variant="light">{{ tag }}</t-tag>
          </div>
        </div>
      </template>
      <template #platform="{ row }">{{ { douyin: '抖音', bilibili: 'B站', baijiahao: '百家号' }[row.platform] || row.platform }}</template>
      <template #platform_account_id="{ row }">{{ row.platform_account_id || '未回填' }}</template>
      <template #game_id="{ row }">{{ gameName(row.game_id) }}</template>
      <template #browser_profile_id="{ row }">{{ row.browser_profile_id ? profileLabel(row.browser_profile_id) : '未绑定' }}</template>
      <template #login_status="{ row }">
        <BusinessStatus :status="row.login_status === 'normal' ? 'normal' : row.login_status === 'unknown' ? 'pending_review' : 'warning'" :label="loginStatusText(row.login_status)" />
      </template>
      <template #business_status="{ row }">
        <BusinessStatus :status="row.business_status === 'enabled' ? 'normal' : row.business_status === 'disabled' ? 'paused' : 'expired'" :label="row.business_status" />
      </template>
      <template #last_checked_at="{ row }">{{ row.last_checked_at ? formatTime(row.last_checked_at) : '尚未检查' }}</template>
      <template #executable="{ row }">{{ executableText(row) }}</template>
      <template #op="{ row }">
        <t-space>
          <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
          <template v-if="isDesktop">
            <t-button size="small" variant="text" :disabled="!canOperateBoundWindow(row)" @click="openAccountProfile(row)">打开窗口</t-button>
            <t-button size="small" variant="text" :disabled="!canOperateBoundWindow(row)" @click="closeAccountProfile(row)">关闭窗口</t-button>
          </template>
          <t-dropdown :options="[
            { value: 'enabled', label: '启用', disabled: row.business_status === 'enabled' },
            { value: 'disabled', label: '停用', disabled: row.business_status === 'disabled' },
            { value: 'retired', label: '退役', disabled: row.business_status === 'retired' },
          ]" @click="(v) => updateStatus(row, v)">
            <t-button size="small" variant="text">状态</t-button>
          </t-dropdown>
        </t-space>
      </template>
    </t-table>

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
            <t-option value="bilibili" label="B站" />
            <t-option value="baijiahao" label="百家号" />
            <t-option value="douyin" label="抖音" />
          </t-select>
        </t-form-item>
        <t-form-item v-if="isDesktop" label="绑定窗口">
          <t-select v-model="createForm.browserProfileId" clearable placeholder="可先不绑定">
            <t-option v-for="profile in activeProfiles" :key="profile.id" :value="profile.id" :label="profileLabel(profile.id)" />
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
      <t-alert v-if="checkNotice" :message="checkNotice" theme="success" style="margin-top:12px" />

      <t-form v-if="detailAccount" class="detail-form" label-width="92px">
        <t-form-item label="备注">
          <t-textarea v-model="detailRemark" :rows="3" />
        </t-form-item>
        <t-form-item v-if="isDesktop" label="绑定窗口">
          <t-select v-model="detailProfileId" clearable placeholder="选择本人授权窗口">
            <t-option v-for="profile in activeProfiles" :key="profile.id" :value="profile.id" :label="profileLabel(profile.id)" />
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
          <t-button v-if="canOperateBoundWindow(detailAccount)" variant="outline" @click="openAccountProfile(detailAccount)">打开窗口</t-button>
          <t-button v-if="canOperateBoundWindow(detailAccount)" variant="outline" @click="closeAccountProfile(detailAccount)">关闭窗口</t-button>
          <t-button v-if="canCheckAccount(detailAccount)" variant="outline" :loading="checkingAccount" @click="checkDetailAccount">检查/同步账号信息</t-button>
          <t-button theme="primary" :loading="savingDetail" @click="saveDetail">保存</t-button>
        </t-space>
      </template>
    </t-drawer>
  </t-loading>
</template>

<style scoped>
.stats-row { margin-bottom: 16px; }
.stat-num { font-size: 24px; font-weight: 700; line-height: 1.2; }
.search-bar { margin-bottom: 12px; }
.batch-result-card { margin-bottom: 12px; }
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
