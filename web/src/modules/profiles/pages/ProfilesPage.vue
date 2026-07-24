<script setup>
import { computed, onMounted, ref } from "vue"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import { createUsersClient } from "../../../apps/cloud/pages/users/usersApi.js"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import { createSessionClient } from "../../../shared/api/session.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"
import { isDesktop } from "../../../utils.js"

const bindingClient = createProfileBindingClient()
const sessionClient = createSessionClient()
const usersClient = createUsersClient()
const profiles = ref([])
const scans = ref([])
const loading = ref(true)
const error = ref("")
const taskNotice = ref("")

// Create dialog
const showCreate = ref(false)
const creating = ref(false)
const newProfile = ref({ name: "", group_name: "", seq: 1 })

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
let lastRuntimeRefreshAt = 0
let lastRuntimeStatus = null

const currentUser = ref(null)
const isDesktopClient = computed(() => isDesktop())
const isAdmin = computed(() => currentUser.value?.role === "admin")
const operatorOptions = computed(() => operatorUsers.value.map((user) => ({
  label: `${user.username}（UID ${user.id}）`,
  value: String(user.id),
})))

async function desktopLocalAgentService() {
  if (typeof window === "undefined" || !window.__TAURI_INTERNALS__) {
    throw new Error("BitBrowser 扫描只能在 Desktop 客户端执行")
  }
  const { invoke } = await import("@tauri-apps/api/core")
  return createLocalAgentService({ invoke })
}

function cloudBaseUrl() {
  if (typeof window === "undefined") return "http://127.0.0.1:18080"
  const origin = window.location?.origin || "http://127.0.0.1:18080"
  if (origin === "http://127.0.0.1:5174" || origin === "http://localhost:5174") {
    return "http://127.0.0.1:18080"
  }
  return origin
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
  } catch (e) {
    if (localStatus?.node_id) {
      taskNotice.value = "本机状态已读取，Cloud可信状态刷新暂时失败；将继续由Cloud预检判断是否可执行。"
      lastRuntimeStatus = localStatus
      lastRuntimeRefreshAt = now
      return localStatus
    }
    throw e
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
  if (e.errcode === 10001 || e.errcode === 23003 || e.errcode === 11001) {
    return "当前电脑尚未完成本地环境确认，暂时不能扫描或操作浏览器窗口。请到 Desktop「环境状态」页刷新本机状态后重试。"
  }
  return e.message
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
    profiles.value = await bindingClient.listProfiles()
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
  creating.value = true
  try {
    const nodeId = await currentLocalNodeId()
    const task = await bindingClient.createProfile(newProfile.value, { nodeId })
    taskNotice.value = `已创建 Profile 任务（${task.task_id || "待执行"}），需要 Local Agent 执行后才会出现在列表。`
    showCreate.value = false
    newProfile.value = { name: "", group_name: "", seq: 1 }
    identityError.value = ""
    await loadProfiles()
  } catch (e) {
    if (e.errcode === 23002 || e.errcode === 23003) {
      identityError.value = "当前比特浏览器登录账号与本系统用户绑定账号不一致。为避免数据错乱，请切换回正确的比特浏览器账号后重试。"
    }
    error.value = localTrustMessage(e)
  } finally {
    creating.value = false
  }
}

async function openProfile(profile) {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web不能打开本机BitBrowser窗口；请在Desktop客户端执行。"
    return
  }
  try {
    const nodeId = await currentLocalNodeId()
    const task = await bindingClient.openProfile(profile.id, { nodeId })
    taskNotice.value = `已创建打开任务（${task.task_id || "待执行"}）。`
  } catch (e) {
    error.value = localTrustMessage(e)
  }
}

async function closeProfile(profile) {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web不能关闭本机BitBrowser窗口；请在Desktop客户端执行。"
    return
  }
  try {
    const nodeId = await currentLocalNodeId()
    const task = await bindingClient.closeProfile(profile.id, { nodeId })
    taskNotice.value = `已创建关闭任务（${task.task_id || "待执行"}）。`
  } catch (e) {
    error.value = localTrustMessage(e)
  }
}

async function deleteProfile(profile) {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web不处理本机窗口变更；窗口失效或归档请在后续Desktop闭环中处理。"
    return
  }
  if (!confirm(`确定删除 Profile "${profile.name}"？`)) return
  try {
    await bindingClient.deleteProfile(profile.id)
    await loadProfiles()
  } catch (e) {
    error.value = e.message
  }
}

function openDetail(profile) {
  detailProfile.value = profile
  detailVisible.value = true
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

async function rejectScan() {
  if (!currentScan.value) return
  currentScan.value = null
  scanDetailVisible.value = false
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
    ? `确定接受本机扫描结果并更新Cloud窗口镜像？本次会处理 ${total} 项差异，但不会覆盖授权用户、媒体账号绑定、游戏、标签、备注、Cookie 和业务状态。`
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
  const user = operatorUsers.value.find((item) => Number(item.id) === Number(userID))
  return user ? `${user.username}（UID ${user.id}）` : `UID ${userID || "-"}`
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
  return new Date(t).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}

const columns = [
  { colKey: "name", title: "名称", width: 160 },
  { colKey: "bit_profile_id", title: "Bit ID", width: 100 },
  { colKey: "group_name", title: "分组", width: 100 },
  { colKey: "user_id", title: "授权用户", width: 120 },
  { colKey: "local_status", title: "状态", width: 90 },
  { colKey: "last_synced_at", title: "同步", width: 130 },
  { colKey: "op", title: "操作", width: 200 },
]

// Profile lookup from scan profiles list
function profileName(scan, bitProfileId) {
  const p = scan?.profiles?.find(p => p.bit_profile_id === bitProfileId)
  return p?.name || p?.profile_name || bitProfileId.slice(0, 12)
}
function profileGroup(scan, bitProfileId) {
  const p = scan?.profiles?.find(p => p.bit_profile_id === bitProfileId)
  return p?.group_name || p?.groupName || '-'
}
function profileProxy(scan, bitProfileId) {
  const p = scan?.profiles?.find(p => p.bit_profile_id === bitProfileId)
  if (!p?.proxy_host) return '-'
  return (p.proxy_type || 'http') + '://' + p.proxy_host + ':' + (p.proxy_port || 0)
}
function profileRemark(scan, bitProfileId) {
  const p = scan?.profiles?.find(p => p.bit_profile_id === bitProfileId)
  return p?.remark || '-'
}

const diffColumns = [
  { colKey: "bit_profile_id", title: "Profile ID", width: 120 },
  { colKey: "name", title: "名称", width: 140 },
  { colKey: "group_name", title: "分组", width: 100 },
  { colKey: "proxy", title: "代理", width: 140 },
  { colKey: "remark", title: "备注", width: 120 },
]
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-alert v-if="taskNotice" :message="taskNotice" theme="info" style="margin-bottom:16px" closable @close="taskNotice=''" />
    <t-alert v-if="identityError" :message="identityError" theme="warning" style="margin-bottom:16px" closable @close="identityError=''" />
    <t-alert
      v-if="!isDesktopClient"
      message="当前为Cloud Web：仅展示Cloud已保存的浏览器窗口信息。扫描、Diff处理、打开/关闭和创建窗口请在Desktop客户端执行。"
      theme="info"
      style="margin-bottom:16px"
    />

    <div class="action-bar">
      <t-space>
        <t-button v-if="isDesktopClient" theme="primary" @click="showCreate = true">新建窗口</t-button>
        <t-button v-if="isDesktopClient" :loading="scanning" @click="triggerScan">扫描本机窗口</t-button>
        <t-button variant="outline" @click="loadProfiles">刷新</t-button>
      </t-space>
    </div>

    <t-card title="浏览器窗口" :bordered="true">
      <t-table
        :data="profiles"
        :columns="columns"
        row-key="id"
        size="small"
        hover
        :pagination="{ pageSize: 50, total: profiles.length }"
        empty="暂无浏览器窗口"
      >
        <template #name="{ row }">
          <div class="profile-name">{{ row.name || '-' }}</div>
        </template>
        <template #local_status="{ row }">
          <BusinessStatus :status="row.local_status === 'active' ? 'normal' : 'stopped'" :label="row.local_status" />
        </template>
        <template #user_id="{ row }">
          {{ operatorLabel(row.user_id) }}
        </template>
        <template #last_synced_at="{ row }">{{ formatTime(row.last_synced_at) }}</template>
        <template #op="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
            <t-button v-if="isAdmin" size="small" variant="text" @click="openAssignProfile(row)">分配</t-button>
            <template v-if="isDesktopClient">
              <t-button size="small" variant="text" @click="openProfile(row)">打开</t-button>
              <t-button size="small" variant="text" @click="closeProfile(row)">关闭</t-button>
              <t-button size="small" variant="text" theme="danger" @click="deleteProfile(row)">删除</t-button>
            </template>
          </t-space>
        </template>
      </t-table>
    </t-card>

    <!-- 新建窗口 -->
    <t-dialog v-model:visible="showCreate" header="新建窗口" @confirm="createProfile" :confirm-btn="{ loading: creating, theme: 'primary' }">
      <t-form>
        <t-form-item label="名称">
          <t-input v-model="newProfile.name" placeholder="窗口名称" />
        </t-form-item>
        <t-form-item label="分组">
          <t-input v-model="newProfile.group_name" placeholder="如: 运营组/抖音组" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- 详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="浏览器窗口详情" :size="'480px'" destroy-on-close>
      <t-descriptions v-if="detailProfile" :column="1" bordered size="small">
        <t-descriptions-item label="ID">{{ detailProfile.id }}</t-descriptions-item>
        <t-descriptions-item label="BitBrowser窗口ID">{{ detailProfile.bit_profile_id }}</t-descriptions-item>
        <t-descriptions-item label="名称">{{ detailProfile.name || '-' }}</t-descriptions-item>
        <t-descriptions-item label="分组">{{ detailProfile.group_name || '-' }}</t-descriptions-item>
        <t-descriptions-item label="已保存主账号">{{ maskMainUserId(detailProfile.main_user_id) }}</t-descriptions-item>
        <t-descriptions-item label="授权用户">{{ operatorLabel(detailProfile.user_id) }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <BusinessStatus :status="detailProfile.local_status === 'active' ? 'normal' : 'stopped'" :label="detailProfile.local_status" />
        </t-descriptions-item>
        <t-descriptions-item label="代理">{{ detailProfile.proxy_host ? detailProfile.proxy_type + '://' + detailProfile.proxy_host + ':' + detailProfile.proxy_port : '-' }}</t-descriptions-item>
        <t-descriptions-item label="备注">{{ detailProfile.remark || '-' }}</t-descriptions-item>
        <t-descriptions-item label="最后同步">{{ formatTime(detailProfile.last_synced_at) }}</t-descriptions-item>
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
          message="接受本地变化后，会更新Cloud窗口镜像中的名称、分组、代理摘要、运行状态等允许字段；不会覆盖授权用户、媒体账号绑定、游戏、标签、备注、Cookie 和业务状态。"
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
            <t-table v-if="getDiff(currentScan).changed?.length" :data="getDiff(currentScan).changed.map(d => ({...d, name: profileName(currentScan, d.bit_profile_id), group_name: profileGroup(currentScan, d.bit_profile_id), proxy: profileProxy(currentScan, d.bit_profile_id), remark: profileRemark(currentScan, d.bit_profile_id)}))" :columns="diffColumns" size="small">
              <template #name="{ row }">{{ row.name || row.bit_profile_id }}</template>
              <template #group_name="{ row }">{{ row.group_name }}</template>
              <template #proxy="{ row }">{{ row.proxy }}</template>
              <template #remark="{ row }">{{ row.remark }}</template>
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
      </div>
      <template #footer>
        <t-space>
          <t-button variant="outline" @click="scanDetailVisible = false">关闭</t-button>
          <t-button v-if="currentScan?.status === 'ready' && hasDiff(currentScan)" theme="primary" :loading="acceptingScan" @click="acceptLocalChanges">接受本地变化</t-button>
          <t-button v-if="currentScan?.status === 'ready' && hasDiff(currentScan)" theme="default" :loading="restoringCloud" @click="restoreCloudConfig">恢复Cloud配置并读回验证</t-button>
          <t-button v-if="currentScan?.status === 'ready' && hasDiff(currentScan)" theme="default" @click="rejectScan" style="margin-left:8px">取消变更</t-button>
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
        message="接受后只更新Cloud窗口镜像中的允许字段，不会覆盖授权用户、媒体账号绑定、游戏、标签、备注、Cookie和业务状态。"
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
  </t-loading>
</template>

<style scoped>
.action-bar { margin-bottom: 12px; }
.profile-name { font-weight: 500; }
</style>
