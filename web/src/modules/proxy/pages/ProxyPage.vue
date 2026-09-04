<script setup>
import { onMounted, ref } from "vue"
import { createProxyClient } from "../../../shared/api/proxy.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"

const proxyClient = createProxyClient()
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

// Detail drawer
const detailVisible = ref(false)
const detailProxy = ref(null)

// Quota dialog
const quotaVisible = ref(false)
const quotaProxyId = ref("")
const quotaMax = ref(1)
const savingQuota = ref(false)

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
  showCreate.value = true
}

async function createProxy() {
  if (!createForm.value.host.trim() || !Number(createForm.value.port)) {
    error.value = "请填写代理地址和端口"
    return
  }
  creating.value = true
  try {
    await proxyClient.create({ ...createForm.value, host: createForm.value.host.trim(), port: Number(createForm.value.port) })
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

const columns = [
  { colKey: "host", title: "地址", width: 200 },
  { colKey: "proxy_protocol", title: "协议", width: 80 },
  { colKey: "region", title: "地区", width: 100 },
  { colKey: "supplier", title: "供应商", width: 100 },
  { colKey: "business_status", title: "状态", width: 90 },
	{ colKey: "max_profile_count", title: "最大窗口数", width: 110 },
	{ colKey: "last_check_result", title: "检测结果", width: 100 },
  { colKey: "expires_at", title: "到期时间", width: 140 },
  { colKey: "op", title: "操作", width: 180 },
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
		<template #max_profile_count="{ row }">{{ row.max_profile_count || 3 }}</template>
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
          <t-button size="small" variant="text" theme="danger" @click="deleteProxy(row)">删除</t-button>
        </t-space>
      </template>
    </t-table>

    <t-dialog v-model:visible="showCreate" header="新增代理" :confirm-btn="{ loading: creating, content: '创建' }" @confirm="createProxy">
      <t-form label-width="84px">
        <t-form-item label="协议"><t-select v-model="createForm.proxy_protocol"><t-option value="http" label="HTTP" /><t-option value="https" label="HTTPS" /><t-option value="socks5" label="SOCKS5" /></t-select></t-form-item>
        <t-form-item label="地址"><t-input v-model="createForm.host" placeholder="例如 127.0.0.1" /></t-form-item>
        <t-form-item label="端口"><t-input-number v-model="createForm.port" :min="1" :max="65535" /></t-form-item>
        <t-form-item label="用户名"><t-input v-model="createForm.username" /></t-form-item>
        <t-form-item label="密码"><t-input v-model="createForm.password" type="password" /></t-form-item>
        <t-form-item label="地区"><t-input v-model="createForm.region" /></t-form-item>
        <t-form-item label="供应商"><t-input v-model="createForm.supplier" /></t-form-item>
        <t-form-item label="备注"><t-textarea v-model="createForm.remark" /></t-form-item>
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
  </t-loading>
</template>

<style scoped>
.search-bar { margin-bottom: 12px; }
.action-bar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.proxy-addr { font-weight: 500; }
.proxy-user { color: var(--td-text-color-placeholder); font-size: 12px; margin-top: 2px; }
</style>
