<script setup>
import { computed, onMounted, ref } from "vue"
import { createLocalAgentService } from "../../../apps/desktop/features/local-agent/service.js"
import { createProxyClient } from "../../../shared/api/proxy.js"
import { createProfileBindingClient } from "../../../shared/api/profileBindings.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"
import { isDesktop } from "../../../utils.js"
import { invoke } from "@tauri-apps/api/core"

const proxyClient = createProxyClient()
const profileClient = createProfileBindingClient()
const proxies = ref([])
const loading = ref(true)
const error = ref("")
const taskNotice = ref("")
const failedCheckProxy = ref(null)
const queuedTaskId = ref("")

// Search
const searchSupplier = ref("")
const searchBizStatus = ref("")
const searchRegion = ref("")
const searchText = ref("")

// Import
const importText = ref("")
const showImport = ref(false)
const importPreview = ref([])
const importing = ref(false)
const showCreate = ref(false)
const creating = ref(false)
const createForm = ref(emptyProxyForm())
const createSyncToProfile = ref(false)
const createProfileId = ref("")
const createProfiles = ref([])
const loadingCreateProfiles = ref(false)

// Detail drawer
const detailVisible = ref(false)
const detailProxy = ref(null)

// Quota dialog
const quotaVisible = ref(false)
const quotaProxyId = ref("")
const quotaMax = ref(1)
const savingQuota = ref(false)

// Assignment dialog
const assignVisible = ref(false)
const assignProxy = ref(null)
const assignProfileId = ref("")
const assigning = ref(false)
const activeProfiles = ref([])
const isDesktopClient = computed(() => isDesktop())
const localScanVisible = ref(false)
const localScanLoading = ref(false)
const localScanConfirming = ref(false)
const localScanRestoring = ref(false)
const localProxyScan = ref(null)
const localScanNodeId = ref("")

onMounted(() => { loadProxies() })

async function loadProxies() {
  error.value = ""
  loading.value = true
  try {
    const params = {}
    if (searchBizStatus.value) params.business_status = searchBizStatus.value
    if (searchSupplier.value) params.supplier = searchSupplier.value
    if (searchRegion.value) params.region = searchRegion.value
    if (searchText.value) params.search = searchText.value
    const result = await proxyClient.list(params)
    // Be defensive while older Cloud instances may still return data: null.
    proxies.value = Array.isArray(result) ? result : (Array.isArray(result?.list) ? result.list : [])
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function updateStatus(proxy, status) {
  try {
    await proxyClient.updateStatus(proxy.id, status)
    await loadProxies()
  } catch (e) {
    error.value = e.message
  }
}

async function deleteProxy(proxy) {
  if (!confirm(`确定删除代理 ${proxy.host}:${proxy.port}？`)) return
  try {
    await proxyClient.delete(proxy.id)
    await loadProxies()
  } catch (e) {
    error.value = e.message
  }
}

function onFileSelected(e) {
  const file = e.target?.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    importText.value = reader.result
    e.target.value = ''
  }
  reader.readAsText(file)
}

async function previewImport() {
  const lines = importText.value.split("\n").map(l => l.trim()).filter(Boolean)
  if (!lines.length) return
  try {
    const result = await proxyClient.previewImport(lines)
    importPreview.value = result.parsed || []
  } catch (e) {
    error.value = e.message
  }
}

async function confirmImport() {
	importing.value = true
	try {
		const lines = importText.value.split("\n").map(l => l.trim()).filter(Boolean)
		await proxyClient.bulkImport(lines)
		await loadProxies()
    showImport.value = false
    importText.value = ""
    importPreview.value = []
  } catch (e) {
    error.value = e.message
  } finally {
    importing.value = false
  }
}

function emptyProxyForm() {
  return { proxy_protocol: "http", host: "", port: 0, username: "", password: "", region: "", supplier: "", remark: "" }
}

function openCreate() {
  createForm.value = emptyProxyForm()
  createSyncToProfile.value = false
  createProfileId.value = ""
  showCreate.value = true
}

async function loadCreateProfiles() {
  if (!isDesktopClient.value) return
  loadingCreateProfiles.value = true
  try {
    const profiles = await profileClient.listProfiles()
    createProfiles.value = Array.isArray(profiles) ? profiles.filter(profile => profile.local_status === "active" && !profile.proxy_id) : []
  } catch (e) {
    error.value = e.message || "无法读取可同步窗口"
  } finally {
    loadingCreateProfiles.value = false
  }
}

async function createProxy() {
  if (!createForm.value.host.trim() || !Number(createForm.value.port)) {
    error.value = "请填写代理地址和端口"
    return
  }
  creating.value = true
  try {
    if (createSyncToProfile.value && !createProfileId.value) {
      error.value = "请选择要同步的浏览器窗口"
      return
    }
    const created = await proxyClient.create({ ...createForm.value, host: createForm.value.host.trim(), port: Number(createForm.value.port) })
    if (createSyncToProfile.value) {
      const profile = await proxyClient.assign(created.id, createProfileId.value)
      taskNotice.value = `代理已创建并同步写入 BitBrowser，读回验证窗口：${profile.name || profile.bit_profile_id}`
    } else {
      taskNotice.value = "代理台账已创建；可在列表中选择“同步至窗口”写入 BitBrowser。"
    }
    showCreate.value = false
    await loadProxies()
  } catch (e) {
    error.value = e.message || "创建代理失败"
  } finally {
    creating.value = false
  }
}

function openDetail(proxy) {
  detailProxy.value = proxy
  detailVisible.value = true
}

async function triggerCheck(proxy) {
  try {
    const result = await proxyClient.check(proxy.id)
    failedCheckProxy.value = null
    taskNotice.value = `代理检测完成：${result.last_check_result || "未知"}`
    await loadProxies()
  } catch (e) {
    error.value = e.message
    failedCheckProxy.value = proxy
  }
}

async function queueBackgroundCheck() {
  if (!failedCheckProxy.value) return
  try {
    const task = await proxyClient.backgroundCheck(failedCheckProxy.value.id)
    queuedTaskId.value = task.task_id || ""
    taskNotice.value = `已创建后台检测任务（${task.task_id || "待执行"}）。`
    failedCheckProxy.value = null
  } catch (e) {
    error.value = e.message
  }
}

function openQuota(proxy) {
	quotaProxyId.value = proxy.id
	quotaMax.value = proxy.max_profile_count || 3
  quotaVisible.value = true
}

async function saveQuota() {
  savingQuota.value = true
  try {
		await proxyClient.setMaxProfileCount(quotaProxyId.value, quotaMax.value)
    quotaVisible.value = false
  } catch (e) {
    error.value = e.message
  } finally {
    savingQuota.value = false
  }
}

async function openAssign(proxy) {
  assignProxy.value = proxy
  assignProfileId.value = ""
  try {
    const profiles = await profileClient.listProfiles()
    activeProfiles.value = Array.isArray(profiles) ? profiles.filter(profile => profile.local_status === "active" && !profile.proxy_id) : []
    assignVisible.value = true
  } catch (e) {
    error.value = e.message || "无法读取可分配窗口"
  }
}

async function assignProxyToProfile() {
  if (!assignProxy.value || !assignProfileId.value) {
    error.value = "请选择窗口"
    return
  }
  assigning.value = true
  try {
    const profile = await proxyClient.assign(assignProxy.value.id, assignProfileId.value)
    taskNotice.value = `代理已写入并读回验证：${profile.name || profile.bit_profile_id}`
    assignVisible.value = false
    await loadProxies()
  } catch (e) {
    error.value = e.message || "代理写入或读回失败"
  } finally {
    assigning.value = false
  }
}

function cloudBaseUrl() {
  if (typeof window === "undefined") return "http://127.0.0.1:18080"
  return window.location?.origin === "http://127.0.0.1:18080" ? window.location.origin : "http://127.0.0.1:18080"
}

async function desktopLocalAgentService() {
  if (typeof window === "undefined" || !window.__TAURI_INTERNALS__) {
    throw new Error("扫描本机代理只能在 Desktop 客户端执行")
  }
  return createLocalAgentService({ invoke })
}

async function scanLocalProxies() {
  if (!isDesktopClient.value) {
    error.value = "Cloud Web只展示代理台账；扫描本机代理请在Desktop客户端执行。"
    return
  }
  localScanLoading.value = true
  error.value = ""
  try {
    const localAgent = await desktopLocalAgentService()
    const status = await localAgent.refreshRuntime({ cloudBaseUrl: cloudBaseUrl() })
    if (!status.node_id) throw new Error("当前电脑尚未完成本地环境确认，请先刷新 Desktop 环境状态。")
    const snapshot = await localAgent.profileScan()
    const scan = await profileClient.submit({ main_user_id: snapshot.main_user_id, profiles: snapshot.profiles || [] }, { nodeId: status.node_id })
    localScanNodeId.value = status.node_id
    localProxyScan.value = await proxyClient.previewLocalScan(scan.id || scan.scan_id)
    localScanVisible.value = true
  } catch (e) {
    error.value = e.message || "扫描本机代理失败"
  } finally {
    localScanLoading.value = false
  }
}

async function confirmLocalProxyScan() {
  if (!localProxyScan.value?.scan_id) return
  localScanConfirming.value = true
  try {
    const result = await proxyClient.confirmLocalScan(localProxyScan.value.scan_id, localScanNodeId.value)
    localProxyScan.value = result
    localScanVisible.value = false
    const changes = result.changes || []
    taskNotice.value = changes.length ? `已确认 ${changes.length} 项本机代理变化；未知代理已创建为待补充记录。` : "本机代理与Cloud记录一致，无需更新。"
    await loadProxies()
  } catch (e) {
    error.value = e.message || "确认本机代理变化失败"
  } finally {
    localScanConfirming.value = false
  }
}

async function restoreCloudProxyConfiguration() {
  const changes = localProxyScan.value?.changes || []
  const targets = changes.filter(change => change.current_proxy_id && change.profile_id)
  if (!targets.length) {
    error.value = "当前差异没有可恢复的 Cloud 代理关联"
    return
  }
  localScanRestoring.value = true
  let succeeded = 0
  const failures = []
  try {
    for (const target of targets) {
      try {
        await proxyClient.assign(target.current_proxy_id, target.profile_id)
        succeeded += 1
      } catch (e) {
        failures.push(`${target.bit_profile_id || target.profile_id}：${e.message || '写入失败'}`)
      }
    }
    if (failures.length) {
      error.value = `恢复完成：成功 ${succeeded} 项，失败 ${failures.length} 项。${failures.join('；')}`
    } else {
      taskNotice.value = `已恢复 ${succeeded} 个窗口的 Cloud 代理配置，并逐项完成 BitBrowser 读回验证。`
      await scanLocalProxies()
    }
  } finally {
    localScanRestoring.value = false
  }
}

const columns = [
  { colKey: "host", title: "地址", width: 200 },
  { colKey: "proxy_protocol", title: "协议", width: 80 },
  { colKey: "region", title: "地区", width: 100 },
  { colKey: "supplier", title: "供应商", width: 100 },
  { colKey: "business_status", title: "状态", width: 90 },
	{ colKey: "max_profile_count", title: "窗口容量", width: 170 },
	{ colKey: "last_check_result", title: "检测结果", width: 100 },
  { colKey: "expires_at", title: "到期时间", width: 140 },
	{ colKey: "op", title: "操作", width: 220 },
]

function formatTime(t) {
  if (!t) return "-"
  return new Date(t).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-alert v-if="failedCheckProxy" theme="warning" style="margin-bottom:16px">
      同步检测失败时才需要后台任务：
      <t-button size="small" variant="text" @click="queueBackgroundCheck">后台重试</t-button>
    </t-alert>
    <t-alert v-if="taskNotice" :message="taskNotice" theme="info" style="margin-bottom:16px" closable @close="taskNotice=''" />
    <t-button v-if="queuedTaskId" size="small" variant="outline" style="margin:-8px 0 16px" @click="$router.push(`/execute-tasks?task_id=${encodeURIComponent(queuedTaskId)}`)">查看任务进度</t-button>

    <!-- 搜索/过滤栏 -->
    <t-card class="search-bar" :bordered="true">
      <t-form layout="inline">
        <t-form-item label="状态">
          <t-select v-model="searchBizStatus" placeholder="全部" clearable style="width:120px">
            <t-option value="active" label="正常" />
            <t-option value="paused" label="停用" />
            <t-option value="expired" label="过期" />
          </t-select>
        </t-form-item>
        <t-form-item label="供应商">
          <t-input v-model="searchSupplier" placeholder="筛选" clearable style="width:140px" />
        </t-form-item>
        <t-form-item label="地区">
          <t-input v-model="searchRegion" placeholder="筛选" clearable style="width:140px" />
        </t-form-item>
        <t-form-item label="搜索">
          <t-input v-model="searchText" placeholder="IP/备注" clearable style="width:140px" />
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" @click="loadProxies">查询</t-button>
          <t-button @click="() => { searchBizStatus=''; searchSupplier=''; searchRegion=''; searchText=''; loadProxies() }">重置</t-button>
        </t-form-item>
      </t-form>
    </t-card>

    <!-- 操作栏 -->
    <div class="action-bar">
      <t-space>
		<t-button theme="primary" @click="openCreate">新增代理</t-button>
        <t-button theme="primary" @click="showImport = true">批量导入</t-button>
        <t-button v-if="isDesktopClient" variant="outline" :loading="localScanLoading" @click="scanLocalProxies">扫描本机代理</t-button>
        <t-button variant="outline" @click="loadProxies">刷新</t-button>
      </t-space>
    </div>

    <!-- 表格 -->
    <t-table
      :data="proxies"
      :columns="columns"
      row-key="id"
      size="small"
      hover
      :pagination="{ pageSize: 50, total: proxies.length }"
      empty="暂无代理"
    >
      <template #host="{ row }">
        <div>
          <div class="proxy-addr">{{ row.host }}:{{ row.port }}</div>
          <div v-if="row.username" class="proxy-user">{{ row.username }}</div>
        </div>
      </template>
      <template #business_status="{ row }">
        <BusinessStatus :status="row.business_status === 'active' ? 'normal' : row.business_status === 'paused' ? 'paused' : 'expired'" />
      </template>
      <template #last_check_result="{ row }">
        <BusinessStatus v-if="row.last_check_result === 'ok'" status="success" label="正常" />
        <BusinessStatus v-else-if="row.last_check_result" status="error" :label="row.last_check_result" />
        <span v-else style="color:var(--td-text-color-placeholder)">未检测</span>
      </template>
		<template #max_profile_count="{ row }">{{ row.assigned_profile_count || 0 }} / {{ row.max_profile_count || 3 }} 已用，{{ row.remaining_profile_count ?? (row.max_profile_count || 3) }} 剩余</template>
      <template #expires_at="{ row }">
        <span :style="row.expires_at && new Date(row.expires_at) < new Date() ? 'color:var(--td-error-color)' : ''">
          {{ formatTime(row.expires_at) }}
        </span>
      </template>
      <template #op="{ row }">
        <t-space>
          <t-button size="small" variant="text" @click="openDetail(row)">详情</t-button>
          <t-dropdown :options="[
            { value: 'active', label: '启用', disabled: row.business_status === 'active' },
            { value: 'paused', label: '停用', disabled: row.business_status === 'paused' },
            { value: 'expired', label: '标记过期', disabled: row.business_status === 'expired' },
          ]" @click="(v) => updateStatus(row, v)">
            <t-button size="small" variant="text">状态</t-button>
          </t-dropdown>
          <t-button size="small" variant="text" @click="triggerCheck(row)">检测</t-button>
          <t-button size="small" variant="text" @click="openQuota(row)">配额</t-button>
				<t-button size="small" variant="text" :disabled="row.business_status !== 'active' || (row.remaining_profile_count ?? (row.max_profile_count || 3)) <= 0" @click="openAssign(row)">同步至窗口</t-button>
          <t-button size="small" variant="text" theme="danger" @click="deleteProxy(row)">删除</t-button>
        </t-space>
      </template>
    </t-table>

    <t-dialog v-model:visible="showCreate" header="新增代理" :confirm-btn="{ loading: creating, content: createSyncToProfile ? '创建并同步' : '创建' }" @confirm="createProxy">
      <t-form label-width="84px">
        <t-form-item label="协议"><t-select v-model="createForm.proxy_protocol"><t-option value="http" label="HTTP" /><t-option value="https" label="HTTPS" /><t-option value="socks5" label="SOCKS5" /></t-select></t-form-item>
        <t-form-item label="地址"><t-input v-model="createForm.host" placeholder="例如 127.0.0.1" /></t-form-item>
        <t-form-item label="端口"><t-input-number v-model="createForm.port" :min="1" :max="65535" /></t-form-item>
        <t-form-item label="用户名"><t-input v-model="createForm.username" /></t-form-item>
        <t-form-item label="密码"><t-input v-model="createForm.password" type="password" /></t-form-item>
        <t-form-item label="地区"><t-input v-model="createForm.region" /></t-form-item>
        <t-form-item label="供应商"><t-input v-model="createForm.supplier" /></t-form-item>
        <t-form-item label="备注"><t-textarea v-model="createForm.remark" /></t-form-item>
        <template v-if="isDesktopClient">
          <t-form-item label="同步 BitBrowser"><t-switch v-model="createSyncToProfile" @change="loadCreateProfiles" /></t-form-item>
          <t-form-item v-if="createSyncToProfile" label="目标窗口">
            <t-select v-model="createProfileId" :loading="loadingCreateProfiles" placeholder="选择未绑定代理的可用窗口">
              <t-option v-for="profile in createProfiles" :key="profile.id" :value="profile.id" :label="`${profile.name || '未命名窗口'} (${profile.bit_profile_id})`" />
            </t-select>
          </t-form-item>
        </template>
      </t-form>
    </t-dialog>

    <!-- 批量导入弹窗 -->
    <t-dialog v-model:visible="showImport" header="批量导入代理" width="640px"
      @confirm="confirmImport"
      :confirm-btn="importPreview.length ? { loading: importing, theme: 'primary', content: '确认导入' } : null"
    >
      <p style="margin-bottom:8px; color:var(--td-text-color-secondary); font-size:13px">
        支持格式: host:port、host:port:user:pass、protocol://user:pass@host:port，每行一个
      </p>
      <div style="display:flex; gap:8px; align-items:center; margin-bottom:8px">
        <t-button size="small" variant="outline" @click="$refs.fileInput.click()">选择文件</t-button>
        <span style="font-size:12px; color:var(--td-text-color-secondary)">支持 .txt / .csv 格式</span>
        <input ref="fileInput" type="file" accept=".txt,.csv" style="display:none" @change="onFileSelected" />
      </div>
      <t-textarea v-model="importText" :rows="6" placeholder="粘贴代理文本，每行一个，或点击上方选择文件导入..." />
      <t-button v-if="importText.trim()" size="small" style="margin-top:8px" @click="previewImport">预览解析</t-button>

      <t-table v-if="importPreview.length" :data="importPreview" :columns="[
        { colKey: 'raw', title: '原文', width: 200 },
        { colKey: 'error', title: '解析结果' },
      ]" size="small" style="margin-top:12px">
        <template #raw="{ row }">{{ row.raw }}</template>
        <template #error="{ row }">
          <t-tag v-if="row.parsed" theme="success" size="small">可导入</t-tag>
          <t-tag v-else theme="danger" size="small">{{ row.error }}</t-tag>
        </template>
      </t-table>
    </t-dialog>

    <!-- 详情抽屉 -->
    <t-drawer v-model:visible="detailVisible" header="代理详情" :size="'480px'" destroy-on-close>
      <t-descriptions v-if="detailProxy" :column="1" bordered size="small">
        <t-descriptions-item label="ID">{{ detailProxy.id }}</t-descriptions-item>
        <t-descriptions-item label="地址">{{ detailProxy.host }}:{{ detailProxy.port }}</t-descriptions-item>
        <t-descriptions-item label="协议">{{ detailProxy.proxy_protocol }}</t-descriptions-item>
        <t-descriptions-item label="用户名">{{ detailProxy.username || '-' }}</t-descriptions-item>
        <t-descriptions-item label="地区">{{ detailProxy.region || '-' }}</t-descriptions-item>
        <t-descriptions-item label="供应商">{{ detailProxy.supplier || '-' }}</t-descriptions-item>
        <t-descriptions-item label="业务状态">
          <BusinessStatus :status="detailProxy.business_status === 'active' ? 'normal' : detailProxy.business_status === 'paused' ? 'paused' : 'expired'" />
        </t-descriptions-item>
        <t-descriptions-item label="到期时间">{{ formatTime(detailProxy.expires_at) }}</t-descriptions-item>
        <t-descriptions-item label="最后检测">{{ formatTime(detailProxy.last_check_at) }}</t-descriptions-item>
        <t-descriptions-item label="检测结果">{{ detailProxy.last_check_result || '-' }}</t-descriptions-item>
        <t-descriptions-item label="出口 IP">{{ detailProxy.observed_exit_ip || '-' }}</t-descriptions-item>
        <t-descriptions-item label="备注">{{ detailProxy.remark || '-' }}</t-descriptions-item>
      </t-descriptions>
    </t-drawer>

    <!-- 配额弹窗 -->
    <t-dialog v-model:visible="quotaVisible" header="设置最大窗口数" @confirm="saveQuota" :confirm-btn="{ loading: savingQuota, theme: 'primary' }">
      <t-form>
        <t-form-item label="最大 Profile 数">
          <t-input-number v-model="quotaMax" :min="1" :max="100" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog v-model:visible="assignVisible" header="分配代理到窗口" @confirm="assignProxyToProfile" :confirm-btn="{ loading: assigning, theme: 'primary', content: '写入并读回验证' }">
      <t-alert theme="info" style="margin-bottom:12px">确认后会同步写入 BitBrowser；只有读回地址、端口和协议一致，Cloud 才会更新正式关联。</t-alert>
      <t-form label-width="84px">
        <t-form-item label="代理"><span>{{ assignProxy?.host }}:{{ assignProxy?.port }}</span></t-form-item>
        <t-form-item label="窗口">
          <t-select v-model="assignProfileId" placeholder="选择未绑定代理的可用窗口">
            <t-option v-for="profile in activeProfiles" :key="profile.id" :value="profile.id" :label="`${profile.name || '未命名窗口'} (${profile.bit_profile_id})`" />
          </t-select>
        </t-form-item>
      </t-form>
    </t-dialog>

    <t-dialog v-model:visible="localScanVisible" header="本机代理扫描差异" width="760px" @confirm="confirmLocalProxyScan" :confirm-btn="{ loading: localScanConfirming, theme: 'primary', content: '接受本机代理变化' }">
      <t-alert theme="info" style="margin-bottom:12px">扫描结果来自当前 BitBrowser 窗口；预览不会修改台账。确认后只回写无歧义的正式代理关系，未知代理会创建为“待补充、未检测”。</t-alert>
      <t-table :data="localProxyScan?.changes || []" row-key="profile_id" size="small" :columns="[
        { colKey: 'kind', title: '差异', width: 120 },
        { colKey: 'bit_profile_id', title: '窗口' },
        { colKey: 'host', title: '本机代理' },
        { colKey: 'current_proxy_id', title: '当前关联' },
      ]" empty="本机代理与Cloud记录一致">
        <template #kind="{ row }"><t-tag :theme="row.kind === 'conflict' ? 'danger' : row.kind === 'unknown' ? 'warning' : 'primary'">{{ { changed: '已更换', unbound: '已解绑', unknown: '未登记代理', conflict: '匹配冲突' }[row.kind] || row.kind }}</t-tag></template>
        <template #host="{ row }">{{ row.host ? `${row.proxy_protocol}://${row.host}:${row.port}` : '-' }}</template>
        <template #current_proxy_id="{ row }">{{ row.current_proxy_id || '-' }}</template>
      </t-table>
      <t-button style="margin-top:12px" variant="outline" :loading="localScanRestoring" :disabled="!(localProxyScan?.changes || []).some(change => change.current_proxy_id)" @click="restoreCloudProxyConfiguration">恢复 Cloud 代理配置（逐项写入并读回）</t-button>
    </t-dialog>
  </t-loading>
</template>

<style scoped>
.search-bar { margin-bottom: 12px; }
.action-bar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.proxy-addr { font-weight: 500; }
.proxy-user { color: var(--td-text-color-placeholder); font-size: 12px; margin-top: 2px; }
</style>
