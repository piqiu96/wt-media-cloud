<script setup>
import { computed, onMounted, ref } from "vue"
import { createProxyClient } from "../../../shared/api/proxy.js"
import BusinessStatus from "../../../shared/ui/BusinessStatus.vue"
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue"
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue"
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue"
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue"

const proxyClient = createProxyClient()
const proxies = ref([])
const loading = ref(true)
const error = ref("")
const taskNotice = ref("")
const failedCheckProxy = ref(null)
const queuedTaskId = ref("")
const selectedRowKeys = ref([])
const batchChecking = ref(false)
const batchCheckResult = ref("")
const pagination = ref({ current: 1, pageSize: 50, total: 0 })

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
const editVisible = ref(false)
const editing = ref(false)
const editProxy = ref(null)
const editForm = ref(emptyProxyForm())

// Detail drawer
const detailVisible = ref(false)
const detailProxy = ref(null)
const boundProfiles = ref([])
const loadingBindings = ref(false)

// Quota dialog
const quotaVisible = ref(false)
const quotaProxyId = ref("")
const quotaMax = ref(1)
const savingQuota = ref(false)

const proxyStatItems = computed(() => [
  { key: "total", label: "代理总数", value: proxies.value.length, tone: "info" },
  { key: "available", label: "可用代理", value: proxies.value.filter((proxy) => proxy.business_status === "active" && proxy.last_check_result === "ok").length, tone: "success" },
  { key: "attention", label: "异常代理", value: proxies.value.filter((proxy) => proxy.last_check_result && proxy.last_check_result !== "ok").length, tone: "danger" },
  { key: "bound", label: "已绑定窗口", value: proxies.value.reduce((total, proxy) => total + Number(proxy.assigned_profile_count || 0), 0), tone: "warning" },
])

const pagedProxies = computed(() => {
  const start = (pagination.value.current - 1) * pagination.value.pageSize
  return proxies.value.slice(start, start + pagination.value.pageSize)
})


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
    pagination.value.total = proxies.value.length
    pagination.value.current = 1
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function deleteProxy(proxy) {
	if ((proxy.assigned_profile_count || 0) > 0) {
		error.value = `该代理仍关联 ${proxy.assigned_profile_count} 个浏览器窗口，请先到浏览器窗口页解绑或更换。`
		return
	}
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
  return { source_type: "static", proxy_address: "", extract_url: "", proxy_protocol: "http", host: "", port: 0, username: "", password: "", region: "", supplier: "", remark: "" }
}

function applyParsedAddress(form, parsed) {
  form.proxy_protocol = parsed.proxy_protocol || form.proxy_protocol
  form.host = parsed.host || ""
  form.port = Number(parsed.port) || 0
  form.username = parsed.username || ""
  form.password = parsed.password || ""
}

async function parseAddress(form) {
  if (!form.proxy_address?.trim()) return
  try {
    applyParsedAddress(form, await proxyClient.parseAddress(form.proxy_address.trim()))
  } catch (e) {
    error.value = e.message || "代理地址无法解析"
  }
}

async function previewExtract(form) {
  if (!form.extract_url?.trim()) {
    error.value = "请填写提取 URL"
    return
  }
  try {
    applyParsedAddress(form, await proxyClient.previewExtract({ extract_url: form.extract_url.trim(), proxy_protocol: form.proxy_protocol }))
    taskNotice.value = "已提取一条代理地址；确认创建后才会写入台账。"
  } catch (e) {
    error.value = e.message || "动态代理提取失败"
  }
}

function openCreate() {
  createForm.value = emptyProxyForm()
  showCreate.value = true
}

function openEdit(proxy) {
	editProxy.value = proxy
	editForm.value = {
		source_type: proxy.source_type || "static", proxy_address: "", extract_url: "",
		proxy_protocol: proxy.proxy_protocol || "http", host: proxy.host || "", port: Number(proxy.port) || 0,
		username: proxy.username || "", password: "", region: proxy.region || "", supplier: proxy.supplier || "", remark: proxy.remark || "",
	}
	editVisible.value = true
}

async function saveEdit() {
	if (!editProxy.value || !editForm.value.host.trim() || !Number(editForm.value.port)) {
		error.value = "请填写代理地址和端口"
		return
	}
	editing.value = true
	try {
		await proxyClient.update(editProxy.value.id, { ...editForm.value, host: editForm.value.host.trim(), port: Number(editForm.value.port) })
		const bound = editProxy.value.assigned_profile_count || 0
		taskNotice.value = bound > 0
			? `代理已更新；${bound} 个关联窗口的代理配置待同步，请到浏览器窗口页重新写入并读回验证。`
			: "代理已更新，检测结果已清空，请重新检测后再绑定窗口。"
		editVisible.value = false
		await loadProxies()
	} catch (e) {
		error.value = e.message || "更新代理失败"
	} finally {
		editing.value = false
	}
}

async function createProxy() {
  if (!createForm.value.host.trim() || !Number(createForm.value.port)) {
    error.value = "请填写代理地址和端口"
    return
  }
  creating.value = true
  try {
    await proxyClient.create({ ...createForm.value, host: createForm.value.host.trim(), port: Number(createForm.value.port) })
    taskNotice.value = "代理台账已创建；请到浏览器窗口页绑定并同步到 BitBrowser。"
    showCreate.value = false
    await loadProxies()
  } catch (e) {
    error.value = e.message || "创建代理失败"
  } finally {
    creating.value = false
  }
}

async function openDetail(proxy) {
  detailProxy.value = proxy
  boundProfiles.value = []
  detailVisible.value = true
  loadingBindings.value = true
  try {
    const result = await proxyClient.listBindings(proxy.id)
    boundProfiles.value = Array.isArray(result) ? result : []
  } catch (e) {
    error.value = e.message || "关联窗口查询失败"
  } finally {
    loadingBindings.value = false
  }
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

async function refreshDynamicProxy(proxy) {
  try {
    await proxyClient.refreshDynamic(proxy.id)
    taskNotice.value = (proxy.assigned_profile_count || 0) > 0 ? "动态代理已提取并检测；关联窗口待到浏览器窗口页重新同步。" : "动态代理已提取并检测。"
    await loadProxies()
    await openDetail(proxy)
  } catch (e) {
    error.value = e.message || "动态代理刷新失败"
  }
}

async function batchCheck() {
	const targets = proxies.value.filter((proxy) => selectedRowKeys.value.includes(proxy.id))
	if (!targets.length) return
	batchChecking.value = true
	batchCheckResult.value = ""
	let passed = 0
	let failed = 0
	for (const proxy of targets) {
		try {
			const result = await proxyClient.check(proxy.id)
			if (result.last_check_result === "ok") passed += 1
			else failed += 1
		} catch {
			failed += 1
		}
	}
	selectedRowKeys.value = []
	batchChecking.value = false
	batchCheckResult.value = `批量检测完成：${passed} 条正常，${failed} 条失败。`
	await loadProxies()
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
  { colKey: "row-select", type: "multiple", width: 48 },
  { colKey: "host", title: "地址", width: 200 },
  { colKey: "proxy_protocol", title: "协议", width: 80 },
  { colKey: "region", title: "地区", width: 100 },
  { colKey: "supplier", title: "供应商", width: 100 },
  { colKey: "business_status", title: "状态", width: 90 },
	{ colKey: "max_profile_count", title: "窗口容量", width: 170 },
	{ colKey: "bindings", title: "绑定窗口", width: 120 },
	{ colKey: "last_check_result", title: "检测结果", width: 100 },
  { colKey: "expires_at", title: "到期时间", width: 140 },
	{ colKey: "op", title: "操作", width: 340, fixed: "right" },
]

function formatTime(t) {
  if (!t) return "-"
  return new Date(t).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })
}
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
  <div class="wt-resource-page">
    <ResourcePageHeader title="代理管理" description="管理代理资源池、连通性和窗口绑定">
      <template #actions>
        <t-button theme="primary" @click="openCreate">新增代理</t-button>
        <t-button class="wt-secondary-button" variant="outline" @click="loadProxies">刷新</t-button>
      </template>
    </ResourcePageHeader>
    <ResourceStatGrid :items="proxyStatItems" />
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
    <t-alert v-if="failedCheckProxy" theme="warning" style="margin-bottom:16px">
      同步检测失败时才需要后台任务：
      <t-button size="small" variant="text" @click="queueBackgroundCheck">后台重试</t-button>
    </t-alert>
    <t-alert v-if="taskNotice" :message="taskNotice" theme="info" style="margin-bottom:16px" closable @close="taskNotice=''" />
		<t-alert v-if="batchCheckResult" :message="batchCheckResult" theme="info" style="margin-bottom:16px" closable @close="batchCheckResult=''" />
    <t-button v-if="queuedTaskId" size="small" variant="outline" style="margin:-8px 0 16px" @click="$router.push(`/execute-tasks?task_id=${encodeURIComponent(queuedTaskId)}`)">查看任务进度</t-button>

    <ResourceCard class="proxy-resource-card">
    <!-- 搜索/过滤栏 -->
    <div class="filter-bar">
      <t-space wrap>
        <label class="wt-filter-field"><span class="wt-filter-field__label">代理状态</span><t-select v-model="searchBizStatus" placeholder="状态" clearable style="width:100%">
          <t-option value="active" label="正常" />
          <t-option value="paused" label="停用" />
          <t-option value="expired" label="过期" />
        </t-select></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">供应商</span><t-input v-model="searchSupplier" placeholder="供应商" clearable style="width:100%" /></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">地区</span><t-input v-model="searchRegion" placeholder="地区" clearable style="width:100%" /></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">搜索</span><t-input v-model="searchText" placeholder="IP / 备注" clearable style="width:100%" /></label>
        <div class="wt-filter-actions">
          <t-button theme="primary" @click="loadProxies">查询</t-button>
          <t-button class="wt-secondary-button" variant="outline" @click="() => { searchBizStatus=''; searchSupplier=''; searchRegion=''; searchText=''; loadProxies() }">重置</t-button>
        </div>
      </t-space>
    </div>

    <!-- 操作栏 -->
    <div class="action-bar wt-resource-actions">
      <t-space>
        <t-button class="wt-secondary-button" variant="outline" @click="showImport = true">批量导入</t-button>
			<t-button class="wt-secondary-button" variant="outline" :loading="batchChecking" :disabled="!selectedRowKeys.length" @click="batchCheck">批量检测</t-button>
      </t-space>
    </div>

    <!-- 表格 -->
    <div class="table-scroll-wrap">
    <t-table class="wt-resource-table"
      :data="pagedProxies"
      :columns="columns"
      row-key="id"
      size="small"
      hover
			v-model:selected-row-keys="selectedRowKeys"
      :scroll="{ x: 'max-content' }"
      empty="暂无代理"
    >
      <template #host="{ row }">
        <div>
          <div class="proxy-addr">{{ row.host }}:{{ row.port }}</div>
          <div v-if="row.username" class="proxy-user">{{ row.username }}</div>
        </div>
      </template>
      <template #business_status="{ row }">
        <ResourceStatusBadge :tone="row.business_status === 'active' ? 'success' : 'warning'" :label="row.business_status === 'active' ? '可用' : row.business_status === 'paused' ? '停用' : '过期'" />
      </template>
      <template #last_check_result="{ row }">
        <ResourceStatusBadge v-if="row.last_check_result === 'ok'" tone="success" label="正常" />
        <ResourceStatusBadge v-else-if="row.last_check_result" tone="danger" :label="row.last_check_result" />
        <ResourceStatusBadge v-else tone="neutral" label="未检测" />
      </template>
      <template #max_profile_count="{ row }">{{ row.assigned_profile_count || 0 }} / {{ row.max_profile_count || 3 }} 已用，{{ row.remaining_profile_count ?? (row.max_profile_count || 3) }} 剩余</template>
      <template #bindings="{ row }"><t-button variant="text" size="small" @click="openDetail(row)">已绑定 {{ row.assigned_profile_count || 0 }} 个</t-button></template>
      <template #expires_at="{ row }">
        <span :style="row.expires_at && new Date(row.expires_at) < new Date() ? 'color:var(--td-error-color)' : ''">
          {{ formatTime(row.expires_at) }}
        </span>
      </template>
      <template #op="{ row }">
        <t-space size="small" class="op-cell">
          <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDetail(row)">详情</t-button>
          <t-button size="small" theme="primary" @click="triggerCheck(row)">检测</t-button>
          <t-button size="small" class="wt-secondary-button" variant="outline" @click="openEdit(row)">编辑</t-button>
          <t-button size="small" class="wt-secondary-button" variant="outline" @click="openQuota(row)">设置配额</t-button>
          <t-button size="small" class="wt-secondary-button wt-danger-button" variant="outline" @click="deleteProxy(row)">删除</t-button>
        </t-space>
      </template>
    </t-table>
    </div>
    <div class="pagination-bar">
      <t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="pagination.total" :page-size-options="[10, 20, 50, 100]" show-jumper />
    </div>
    </ResourceCard>

    <t-dialog v-model:visible="showCreate" header="新增代理" :confirm-btn="{ loading: creating, content: '创建' }" @confirm="createProxy">
      <t-form label-width="84px">
        <t-form-item label="代理方式"><t-radio-group v-model="createForm.source_type"><t-radio value="static">静态地址</t-radio><t-radio value="api">动态 API 提取</t-radio></t-radio-group></t-form-item>
        <template v-if="createForm.source_type === 'static'">
          <t-form-item label="代理地址"><t-input v-model="createForm.proxy_address" placeholder="host:port 或 socks5://账号:密码@host:port" @blur="parseAddress(createForm)" /></t-form-item>
        </template>
        <template v-else>
          <t-form-item label="提取 URL"><t-input v-model="createForm.extract_url" placeholder="供应商 API 提取链接" /></t-form-item>
          <t-form-item label=""><t-button size="small" @click="previewExtract(createForm)">测试提取</t-button></t-form-item>
        </template>
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

    <t-dialog v-model:visible="editVisible" header="编辑代理" :confirm-btn="{ loading: editing, content: '保存' }" @confirm="saveEdit">
      <t-alert v-if="(editProxy?.assigned_profile_count || 0) > 0" theme="warning" style="margin-bottom:12px">保存不会自动改写 BitBrowser；{{ editProxy.assigned_profile_count }} 个关联窗口将标记为代理配置待同步，请到浏览器窗口页重新写入并读回。</t-alert>
      <t-form label-width="84px">
        <t-form-item label="协议"><t-select v-model="editForm.proxy_protocol"><t-option value="http" label="HTTP" /><t-option value="https" label="HTTPS" /><t-option value="socks5" label="SOCKS5" /></t-select></t-form-item>
        <t-form-item label="地址"><t-input v-model="editForm.host" placeholder="例如 127.0.0.1（不含协议、账号和端口）" /></t-form-item>
        <t-form-item label="端口"><t-input-number v-model="editForm.port" :min="1" :max="65535" /></t-form-item>
        <t-form-item label="用户名"><t-input v-model="editForm.username" /></t-form-item>
        <t-form-item label="密码"><t-input v-model="editForm.password" type="password" placeholder="留空表示不修改" /></t-form-item>
        <t-form-item label="地区"><t-input v-model="editForm.region" /></t-form-item>
        <t-form-item label="供应商"><t-input v-model="editForm.supplier" /></t-form-item>
        <t-form-item label="备注"><t-textarea v-model="editForm.remark" /></t-form-item>
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
      <t-divider>绑定窗口</t-divider>
      <t-loading :loading="loadingBindings"><t-table :data="boundProfiles" size="small" :columns="[{ colKey: 'name', title: '窗口' }, { colKey: 'bit_profile_id', title: 'BitBrowser ID' }, { colKey: 'user_id', title: '用户' }, { colKey: 'local_status', title: '状态' }]" row-key="id" /></t-loading>
      <t-button v-if="detailProxy?.source_type === 'api'" style="margin-top:12px" @click="refreshDynamicProxy(detailProxy)">提取新地址并检测</t-button>
    </t-drawer>

    <!-- 配额弹窗 -->
    <t-dialog v-model:visible="quotaVisible" header="设置最大窗口数" @confirm="saveQuota" :confirm-btn="{ loading: savingQuota, theme: 'primary' }">
      <t-form>
        <t-form-item label="最大 Profile 数">
          <t-input-number v-model="quotaMax" :min="1" :max="100" />
        </t-form-item>
      </t-form>
    </t-dialog>

  </div>
  </t-loading>
</template>

<style scoped>
.proxy-resource-card { padding: 18px 20px; }
.action-bar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.op-cell { white-space: nowrap; }
.proxy-addr { font-weight: 500; }
.proxy-user { color: var(--td-text-color-placeholder); font-size: 12px; margin-top: 2px; }
</style>
