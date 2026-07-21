<script setup>
import { onMounted, ref } from "vue"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import { createSessionClient } from "../../../shared/api/session.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"

const bindingClient = createProfileBindingClient()
const sessionClient = createSessionClient()
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

// Scan
const scanning = ref(false)
const scanDetailVisible = ref(false)
const currentScan = ref(null)
const identityError = ref("")
const selectedTab = ref("changed")

let currentUser = null

onMounted(async () => {
  try {
    currentUser = await sessionClient.me()
  } catch {}
  await loadProfiles()
})

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
  creating.value = true
  try {
    const task = await bindingClient.createProfile(newProfile.value)
    taskNotice.value = `已创建 Profile 任务（${task.task_id || "待执行"}），需要 Local Agent 执行后才会出现在列表。`
    showCreate.value = false
    newProfile.value = { name: "", group_name: "", seq: 1 }
    identityError.value = ""
    await loadProfiles()
  } catch (e) {
    if (e.errcode === 23002 || e.errcode === 23003) {
      identityError.value = "当前比特浏览器登录账号与本系统用户绑定账号不一致。为避免数据错乱，请切换回正确的比特浏览器账号后重试。"
    }
    error.value = e.message
  } finally {
    creating.value = false
  }
}

async function openProfile(profile) {
  try {
    const task = await bindingClient.openProfile(profile.id)
    taskNotice.value = `已创建打开任务（${task.task_id || "待执行"}）。`
  } catch (e) {
    error.value = e.message
  }
}

async function closeProfile(profile) {
  try {
    const task = await bindingClient.closeProfile(profile.id)
    taskNotice.value = `已创建关闭任务（${task.task_id || "待执行"}）。`
  } catch (e) {
    error.value = e.message
  }
}

async function deleteProfile(profile) {
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
  scanning.value = true
  error.value = ""
  try {
    // Detect environment: Desktop (has local Agent) or Cloud
    const isDesktop = typeof window !== "undefined" && (
      window.__TAURI_INTERNALS__ || location.port === "5174"
    )
    if (!isDesktop) {
      throw new Error("BitBrowser 扫描只能在 Desktop 端执行")
    }

    // 1. Call Local Agent directly (Desktop only — same machine, no CORS for localhost)
    let snapshot
    const scanResp = await fetch("http://127.0.0.1:8765/api/v1/bit-browser/profile-scans", {
      method: "POST",
      mode: "cors",
    })
    if (!scanResp.ok) {
      const body = await scanResp.json().catch(() => ({}))
      const msg = body?.error?.message || body?.error?.code || "Agent 不可达"
      throw new Error(`BitBrowser 扫描失败: ${msg}`)
    }
    snapshot = await scanResp.json()

    // 2. Submit scan result to Cloud for diff
    const scan = await bindingClient.submit({
      main_user_id: snapshot.main_user_id || currentUser?.id,
      profiles: snapshot.profiles || [],
    })
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

async function confirmScan() {
  if (!currentScan.value) return
  try {
    await bindingClient.confirm(currentScan.value.id)
    currentScan.value = null
    scanDetailVisible.value = false
    await loadProfiles()
  } catch (e) {
    error.value = e.message
  }
}

async function rejectScan() {
  if (!currentScan.value) return
  currentScan.value = null
  scanDetailVisible.value = false
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

    <div class="action-bar">
      <t-space>
        <t-button theme="primary" @click="showCreate = true">新建窗口</t-button>
        <t-button :loading="scanning" @click="triggerScan">触发扫描</t-button>
        <t-button variant="outline" @click="loadProfiles">刷新</t-button>
      </t-space>
    </div>

    <t-card title="浏览器窗口 (Profile)" :bordered="true">
      <t-table
        :data="profiles"
        :columns="columns"
        row-key="id"
        size="small"
        hover
        :pagination="{ pageSize: 50, total: profiles.length }"
        empty="暂无 Profile"
      >
        <template #name="{ row }">
          <div class="profile-name">{{ row.name || '-' }}</div>
        </template>
        <template #local_status="{ row }">
          <BusinessStatus :status="row.local_status === 'active' ? 'normal' : 'stopped'" :label="row.local_status" />
        </template>
        <template #last_synced_at="{ row }">{{ formatTime(row.last_synced_at) }}</template>
        <template #op="{ row }">
          <t-space>
            <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
            <t-button size="small" variant="text" @click="openProfile(row)">打开</t-button>
            <t-button size="small" variant="text" @click="closeProfile(row)">关闭</t-button>
            <t-button size="small" variant="text" theme="danger" @click="deleteProfile(row)">删除</t-button>
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
    <t-drawer v-model:visible="detailVisible" header="Profile 详情" :size="'480px'" destroy-on-close>
      <t-descriptions v-if="detailProfile" :column="1" bordered size="small">
        <t-descriptions-item label="ID">{{ detailProfile.id }}</t-descriptions-item>
        <t-descriptions-item label="BitBrowser ID">{{ detailProfile.bit_profile_id }}</t-descriptions-item>
        <t-descriptions-item label="名称">{{ detailProfile.name || '-' }}</t-descriptions-item>
        <t-descriptions-item label="分组">{{ detailProfile.group_name || '-' }}</t-descriptions-item>
        <t-descriptions-item label="主账号">{{ detailProfile.main_user_id }}</t-descriptions-item>
        <t-descriptions-item label="状态">
          <BusinessStatus :status="detailProfile.local_status === 'active' ? 'normal' : 'stopped'" :label="detailProfile.local_status" />
        </t-descriptions-item>
        <t-descriptions-item label="代理">{{ detailProfile.proxy_host ? detailProfile.proxy_type + '://' + detailProfile.proxy_host + ':' + detailProfile.proxy_port : '-' }}</t-descriptions-item>
        <t-descriptions-item label="备注">{{ detailProfile.remark || '-' }}</t-descriptions-item>
        <t-descriptions-item label="最后同步">{{ formatTime(detailProfile.last_synced_at) }}</t-descriptions-item>
      </t-descriptions>
    </t-drawer>

    <!-- 扫描 Diff 抽屉 -->
    <t-drawer v-model:visible="scanDetailVisible" header="扫描结果" :size="'700px'" destroy-on-close>
      <div v-if="currentScan">
        <t-alert :message="'状态: ' + currentScan.status + ' | 时间: ' + formatTime(currentScan.created_at)" theme="info" style="margin-bottom:16px" />
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
          <t-button v-if="currentScan?.status === 'ready'" theme="primary" @click="confirmScan">确认变更</t-button>
          <t-button v-if="currentScan?.status === 'ready'" theme="default" @click="rejectScan" style="margin-left:8px">取消变更</t-button>
        </t-space>
      </template>
    </t-drawer>
  </t-loading>
</template>

<style scoped>
.action-bar { margin-bottom: 12px; }
.profile-name { font-weight: 500; }
</style>
