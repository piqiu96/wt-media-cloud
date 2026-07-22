<script setup>
import { computed, onMounted, ref } from "vue"
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js"
import { createSessionClient } from "../../../shared/api/session.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"

const accountClient = createMediaAccountClient()
const sessionClient = createSessionClient()

// --- State ---
const user = ref(null)
const accounts = ref([])
const loading = ref(true)
const error = ref("")

// Search / Filter
const searchPlatform = ref("")
const searchBizStatus = ref("")
const searchTags = ref("")
const gameId = ref("")

// Create account form
const showCreate = ref(false)
const targetUserId = ref("")
const createPlatform = ref("douyin")
const originalCookie = ref("")
const creating = ref(false)

// Batch tag
const tagInput = ref("")
const selectedAccountIds = ref([])

// Detail drawer
const detailVisible = ref(false)
const detailAccount = ref(null)
const identifyName = ref("")
const identifyPlatformAccountId = ref("")
const savingIdentify = ref(false)

onMounted(async () => {
  try {
    user.value = await sessionClient.me()
    gameId.value = (user.value.game_ids || [])[0] || ""
    await loadAccounts()
  } catch {
    user.value = null
  } finally {
    loading.value = false
  }
})

// --- Data ---
async function loadAccounts() {
  error.value = ""
  try {
    const params = {}
    if (searchPlatform.value) params.platform = searchPlatform.value
    if (searchBizStatus.value) params.businessStatus = searchBizStatus.value
    if (searchTags.value) params.anyTags = searchTags.value.split(",").map(t => t.trim()).filter(Boolean)
    accounts.value = await accountClient.list(params)
  } catch (e) {
    error.value = e.message
  }
}

// --- Create ---
async function createAccount() {
  error.value = ""
  creating.value = true
  try {
    await accountClient.create({
      userId: user.value?.role === "admin" ? Number(targetUserId.value) || undefined : undefined,
      gameId: gameId.value,
      platform: createPlatform.value,
      originalCookie: originalCookie.value,
    })
    originalCookie.value = ""
    showCreate.value = false
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  } finally {
    creating.value = false
  }
}

// --- Batch Tags ---
async function changeTags(remove = false) {
  const tags = tagInput.value.split(",").map(t => t.trim()).filter(Boolean)
  if (!selectedAccountIds.value.length || !tags.length) return
  error.value = ""
  try {
    if (remove) await accountClient.removeTags(selectedAccountIds.value, tags)
    else await accountClient.addTags(selectedAccountIds.value, tags)
    tagInput.value = ""
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}

// --- Status Operation ---
async function updateStatus(account, bizStatus) {
  error.value = ""
  try {
    await accountClient.update(account.id, { businessStatus: bizStatus })
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}

// --- Detail Drawer ---
function openDetail(account) {
  detailAccount.value = account
  identifyName.value = account.name || ""
  identifyPlatformAccountId.value = account.platform_account_id || ""
  detailVisible.value = true
}

async function saveIdentify() {
  if (!detailAccount.value) return
  savingIdentify.value = true
  try {
    await accountClient.identify(detailAccount.value.id, {
      platformAccountId: identifyPlatformAccountId.value,
      name: identifyName.value,
    })
    await loadAccounts()
    if (detailAccount.value) {
      detailAccount.value.name = identifyName.value
      detailAccount.value.platform_account_id = identifyPlatformAccountId.value
    }
  } catch (e) {
    error.value = e.message
  } finally {
    savingIdentify.value = false
  }
}

// --- Cookie Import / Export ---
const importCookieText = ref("")
const showCookieImport = ref(false)
const importingCookie = ref(false)

function openCookieImport(account) {
  detailAccount.value = account
  importCookieText.value = ""
  showCookieImport.value = true
}

async function importCookie() {
  if (!detailAccount.value || !importCookieText.value) return
  importingCookie.value = true
  try {
    await accountClient.update(detailAccount.value.id, { loginStatus: "unknown" })
    importCookieText.value = ""
    showCookieImport.value = false
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  } finally {
    importingCookie.value = false
  }
}

async function exportOriginalCookie(account) {
  try {
    const cookies = await accountClient.fetchCookies(account.id)
    const c = cookies?.original_cookie
    if (!c) { alert("无原始 Cookie"); return }
    await navigator.clipboard.writeText(c)
    alert("原始 Cookie 已复制到剪贴板")
  } catch (e) {
    alert("获取 Cookie 失败: " + e.message)
  }
}

async function exportActiveCookie(account) {
  try {
    const cookies = await accountClient.fetchCookies(account.id)
    const c = cookies?.active_cookie
    if (!c) { alert("无活跃 Cookie"); return }
    await navigator.clipboard.writeText(c)
    alert("活跃 Cookie 已复制到剪贴板")
  } catch (e) {
    alert("获取 Cookie 失败: " + e.message)
  }
}

// --- Stats ---

function formatTime(t) {
  if (!t) return "-"
  return new Date(t).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

// --- Stats ---
const stats = computed(() => {
  const all = accounts.value
  return {
    total: all.length,
    active: all.filter(a => a.business_status === "active").length,
    disabled: all.filter(a => a.business_status === "disabled").length,
    identified: all.filter(a => a.identification_status === "identified").length,
    pendingId: all.filter(a => a.identification_status === "pending_identification").length,
  }
})

// --- Columns ---
const columns = [
  { colKey: "name", title: "账号", width: 200 },
  { colKey: "platform", title: "平台", width: 90 },
  { colKey: "game_id", title: "游戏", width: 100 },
  { colKey: "business_status", title: "业务状态", width: 100 },
  { colKey: "identification_status", title: "识别状态", width: 100 },
  { colKey: "login_status", title: "登录状态", width: 110 },
  { colKey: "op", title: "操作", width: 160 },
]
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />

    <!-- 统计卡片 -->
    <t-row :gutter="16" class="stats-row">
      <t-col :span="4">
        <t-card><template #title>总账号</template><div class="stat-num">{{ stats.total }}</div></t-card>
      </t-col>
      <t-col :span="4">
        <t-card><template #title>已启用</template><div class="stat-num">{{ stats.active }}</div></t-card>
      </t-col>
      <t-col :span="4">
        <t-card><template #title>已识别</template><div class="stat-num">{{ stats.identified }}</div></t-card>
      </t-col>
      <t-col :span="4">
        <t-card><template #title>待识别</template><div class="stat-num">{{ stats.pendingId }}</div></t-card>
      </t-col>
      <t-col :span="4">
        <t-card><template #title>已停用</template><div class="stat-num">{{ stats.disabled }}</div></t-card>
      </t-col>
    </t-row>

    <!-- 搜索/过滤栏 -->
    <t-card class="search-bar" :bordered="true">
      <t-form layout="inline">
        <t-form-item label="平台">
          <t-select v-model="searchPlatform" placeholder="全部" clearable style="width:120px">
            <t-option value="douyin" label="抖音" />
            <t-option value="bilibili" label="B站" />
            <t-option value="baijiahao" label="百家号" />
          </t-select>
        </t-form-item>
        <t-form-item label="业务状态">
          <t-select v-model="searchBizStatus" placeholder="全部" clearable style="width:120px">
            <t-option value="active" label="启用" />
            <t-option value="disabled" label="停用" />
            <t-option value="retired" label="退役" />
          </t-select>
        </t-form-item>
        <t-form-item label="标签">
          <t-input v-model="searchTags" placeholder="用逗号分隔" clearable style="width:160px" />
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" @click="loadAccounts">查询</t-button>
          <t-button @click="() => { searchPlatform=''; searchBizStatus=''; searchTags=''; loadAccounts() }">重置</t-button>
        </t-form-item>
      </t-form>
    </t-card>

    <!-- 操作栏 -->
    <div class="action-bar">
      <t-space>
        <t-button theme="primary" @click="showCreate = true">新建账号</t-button>
        <t-button variant="outline" @click="loadAccounts">刷新</t-button>
      </t-space>
      <t-space v-if="selectedAccountIds.length" class="batch-actions">
        <t-input v-model="tagInput" placeholder="标签，多个用逗号分隔" style="width:200px" />
        <t-button size="small" @click="changeTags(false)">添加标签</t-button>
        <t-button size="small" @click="changeTags(true)">移除标签</t-button>
        <span class="selected-count">已选 {{ selectedAccountIds.length }} 项</span>
      </t-space>
    </div>

    <!-- 表格 -->
    <t-table
      :data="accounts"
      :columns="columns"
      :selected-row-keys="selectedAccountIds"
      :row-key="(r) => r.id"
      @select-change="(keys) => { selectedAccountIds = keys }"
      size="small"
      hover
      :pagination="{ pageSize: 50, total: accounts.length }"
      empty="暂无媒体账号"
    >
      <template #name="{ row }">
        <div>
          <div class="account-name">{{ row.name || row.platform_account_id || '待识别' }}</div>
          <div class="account-tags">
            <t-tag v-for="tag in row.tags || []" :key="tag" size="small" variant="light">{{ tag }}</t-tag>
          </div>
        </div>
      </template>
      <template #platform="{ row }">
        {{ { douyin: '抖音', bilibili: 'B站', baijiahao: '百家号' }[row.platform] || row.platform }}
      </template>
      <template #business_status="{ row }">
        <BusinessStatus :status="row.business_status === 'active' ? 'normal' : row.business_status === 'disabled' ? 'paused' : 'expired'" />
      </template>
      <template #identification_status="{ row }">
        <BusinessStatus :status="row.identification_status === 'identified' ? 'success' : 'pending_review'" :label="row.identification_status" />
      </template>
      <template #login_status="{ row }">
        <BusinessStatus :status="row.login_status === 'normal' ? 'normal' : row.login_status === 'not_logged_in' ? 'warning' : 'error'" :label="row.login_status" />
      </template>
      <template #op="{ row }">
        <t-space>
          <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
          <t-dropdown :options="[
            { value: 'active', label: '启用', disabled: row.business_status === 'active' },
            { value: 'disabled', label: '停用', disabled: row.business_status === 'disabled' },
            { value: 'retired', label: '退役', disabled: row.business_status === 'retired' },
          ]" @click="(v) => updateStatus(row, v)">
            <t-button size="small" variant="text">状态</t-button>
          </t-dropdown>
        </t-space>
      </template>
    </t-table>

    <!-- 新建账号弹窗 -->
    <t-dialog v-model:visible="showCreate" header="新建账号" @confirm="createAccount" :confirm-btn="{ loading: creating, theme: 'primary' }">
      <t-form @submit="createAccount">
        <t-form-item v-if="user?.role === 'admin'" label="归属用户 UID">
          <t-input-number v-model="targetUserId" :min="1" placeholder="留空则归当前用户" />
        </t-form-item>
        <t-form-item label="平台">
          <t-select v-model="createPlatform">
            <t-option value="douyin" label="抖音" />
            <t-option value="bilibili" label="B站" />
            <t-option value="baijiahao" label="百家号" />
          </t-select>
        </t-form-item>
        <t-form-item label="原始 Cookie（可选）">
          <t-textarea v-model="originalCookie" :rows="3" placeholder="Cookie 仅随本次请求提交，服务端不会持久保存" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="账号详情" :size="'500px'" destroy-on-close>
      <t-descriptions v-if="detailAccount" :column="1" bordered size="small">
        <t-descriptions-item label="账号 ID">{{ detailAccount.id }}</t-descriptions-item>
        <t-descriptions-item label="用户名">{{ detailAccount.name || '-' }}</t-descriptions-item>
        <t-descriptions-item label="平台">{{ detailAccount.platform }}</t-descriptions-item>
        <t-descriptions-item label="游戏">{{ detailAccount.game_id || '-' }}</t-descriptions-item>
        <t-descriptions-item label="业务状态">
          <BusinessStatus :status="detailAccount.business_status === 'active' ? 'normal' : 'expired'" :label="detailAccount.business_status" />
        </t-descriptions-item>
        <t-descriptions-item label="识别状态">
          <BusinessStatus :status="detailAccount.identification_status === 'identified' ? 'success' : 'pending_review'" :label="detailAccount.identification_status" />
        </t-descriptions-item>
        <t-descriptions-item label="登录状态">
          <BusinessStatus :status="detailAccount.login_status === 'normal' ? 'normal' : 'warning'" :label="detailAccount.login_status" />
        </t-descriptions-item>
        <t-descriptions-item label="Profile ID">{{ detailAccount.browser_profile_id || '-' }}</t-descriptions-item>
        <t-descriptions-item label="Cookie 状态">{{ detailAccount.cookie_status || '无' }}</t-descriptions-item>
        <t-descriptions-item label="活跃 Cookie 更新">{{ detailAccount.active_cookie_updated_at ? formatTime(detailAccount.active_cookie_updated_at) : '从未' }}</t-descriptions-item>
      </t-descriptions>

      <template #footer>
        <t-space>
          <t-button variant="outline" @click="detailVisible = false">关闭</t-button>
          <t-button variant="outline" @click="exportOriginalCookie(detailAccount)">导出原始 CK</t-button>
          <t-button variant="outline" @click="exportActiveCookie(detailAccount)">导出活跃 CK</t-button>
          <t-button @click="openCookieImport(detailAccount)">导入 CK</t-button>
          <t-button v-if="detailAccount?.identification_status !== 'identified'" theme="primary" :loading="savingIdentify" @click="saveIdentify">保存识别</t-button>
        </t-space>
      </template>
    </t-drawer>

    <!-- Cookie 导入弹窗 -->
    <t-dialog v-model:visible="showCookieImport" header="导入 Cookie" @confirm="importCookie" :confirm-btn="{ loading: importingCookie, theme: 'primary' }">
      <p style="margin-bottom:8px; color:var(--td-text-color-secondary); font-size:13px">粘贴 Cookie 文本，将更新此账号的原始 Cookie</p>
      <t-textarea v-model="importCookieText" :rows="5" placeholder="粘贴 Cookie..." />
    </t-dialog>
  </t-loading>
</template>

<style scoped>
.stats-row { margin-bottom: 16px; }
.stat-num { font-size: 24px; font-weight: 700; line-height: 1.2; }
.search-bar { margin-bottom: 12px; }
.action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.batch-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.selected-count {
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
.account-name { font-weight: 500; }
.account-tags { display: flex; gap: 4px; margin-top: 4px; flex-wrap: wrap; }
</style>
