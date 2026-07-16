<script setup>
import { ref, computed } from "vue"
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js"
import { createProxyClient } from "../../../shared/api/proxy.js"

const accountClient = createMediaAccountClient()
const proxyClient = createProxyClient()
const error = ref("")

// Step 1: Cookie Input
const cookieText = ref("")
const parsedCookies = ref([])
const step = ref(1)
const importPlatform = ref("douyin")
const importGameId = ref("")

// Step 2: Preview + Proxy Assignment
const proxies = ref([])
const openingRows = ref([])
const executing = ref(false)

function parseCookies() {
  error.value = ""
  const lines = cookieText.value.split("\n").map(l => l.trim()).filter(Boolean)
  parsedCookies.value = lines.map((raw, i) => ({
    id: `row-${i}`,
    raw,
    platform: importPlatform.value,
    game_id: importGameId.value,
    status: "pending",
    account: null,
    error: "",
  }))
  step.value = 2
}

async function previewOpening() {
  error.value = ""
  try {
    proxies.value = await proxyClient.list({ business_status: "active" })
  } catch (e) {
    error.value = e.message
  }

  openingRows.value = parsedCookies.value.map((row, i) => ({
    ...row,
    proxy: proxies.value[i % Math.max(proxies.value.length, 1)] || null,
  }))
}

async function executeOpening() {
  executing.value = true
  error.value = ""

  // Check proxy capacity before starting
  if (proxies.value.length < openingRows.value.length) {
    const ok = confirm(`活跃代理数量 (${proxies.value.length}) 不足待开户数量 (${openingRows.value.length})。继续执行后部分账号将无代理可用，是否继续？`)
    if (!ok) {
      executing.value = false
      return
    }
  }

  for (const row of openingRows.value) {
    row.status = "creating"
    try {
      // Step 1: Create media account
      const account = await accountClient.create({
        gameId: row.game_id,
        platform: row.platform,
        originalCookie: row.raw,
      })
      row.account = account
      row.status = "created"
    } catch (e) {
      row.status = "failed"
      row.error = e.message
    }
  }
  executing.value = false
}

async function retryFailed() {
  const failed = openingRows.value.filter(r => r.status === "failed")
  if (!failed.length) return
  executing.value = true
  for (const row of failed) {
    row.status = "creating"
    row.error = ""
    try {
      const account = await accountClient.create({
        gameId: row.game_id,
        platform: row.platform,
        originalCookie: row.raw,
      })
      row.account = account
      row.status = "created"
    } catch (e) {
      row.status = "failed"
      row.error = e.message
    }
  }
  executing.value = false
}

const stats = computed(() => {
  const rows = openingRows.value
  return {
    total: rows.length,
    created: rows.filter(r => r.status === "created").length,
    failed: rows.filter(r => r.status === "failed").length,
    pending: rows.filter(r => r.status === "pending").length,
  }
})
</script>

<template>
  <t-loading :loading="executing" :show-overlay="true" size="large">
    <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />

    <t-steps :current="step - 1" style="margin-bottom:24px">
      <t-step-item title="导入 Cookie" />
      <t-step-item title="预览分配" />
      <t-step-item title="执行开户" />
    </t-steps>

    <!-- Step 1: Cookie Import -->
    <t-card v-if="step === 1" title="导入 Cookie" :bordered="true">
      <t-form layout="inline" style="margin-bottom:16px">
        <t-form-item label="平台">
          <t-select v-model="importPlatform" style="width:120px">
            <t-option value="douyin" label="抖音" />
            <t-option value="bilibili" label="B站" />
            <t-option value="baijiahao" label="百家号" />
          </t-select>
        </t-form-item>
        <t-form-item label="游戏">
          <t-input v-model="importGameId" placeholder="游戏 ID" style="width:160px" />
        </t-form-item>
      </t-form>
      <t-textarea v-model="cookieText" :rows="8" placeholder="粘贴 Cookie，每行一条..." />
      <t-button theme="primary" style="margin-top:12px" :disabled="!cookieText.trim()" @click="parseCookies">解析预览</t-button>
    </t-card>

    <!-- Step 2: Preview -->
    <t-card v-if="step === 2" title="分配预览" :bordered="true">
      <t-table :data="openingRows" :columns="[
        { colKey: 'raw', title: 'Cookie', width: 200 },
        { colKey: 'platform', title: '平台', width: 80 },
        { colKey: 'proxy', title: '分配代理' },
        { colKey: 'status', title: '状态', width: 100 },
      ]" size="small" hover>
        <template #raw="{ row }">
          <span :title="row.raw">{{ row.raw.substring(0, 40) }}...</span>
        </template>
        <template #proxy="{ row }">
          <span>{{ row.proxy ? row.proxy.host + ':' + row.proxy.port : '待分配' }}</span>
        </template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'pending' ? 'default' : row.status === 'created' ? 'success' : 'danger'" size="small">{{ row.status }}</t-tag>
        </template>
      </t-table>
      <t-space style="margin-top:16px">
        <t-button variant="outline" @click="step = 1">返回</t-button>
        <t-button theme="primary" @click="executeOpening">开始执行</t-button>
      </t-space>
    </t-card>

    <!-- Step 3: Results -->
    <t-card v-if="step === 3 || (step === 2 && !executing && openingRows.some(r => r.status !== 'pending'))" title="执行结果" :bordered="true">
      <t-row :gutter="16" style="margin-bottom:16px">
        <t-col :span="4"><t-card><template #title>总数</template><div class="stat-num">{{ stats.total }}</div></t-card></t-col>
        <t-col :span="4"><t-card><template #title>已创建</template><div class="stat-num" style="color:var(--td-success-color)">{{ stats.created }}</div></t-card></t-col>
        <t-col :span="4"><t-card><template #title>失败</template><div class="stat-num" style="color:var(--td-error-color)">{{ stats.failed }}</div></t-card></t-col>
      </t-row>
      <t-table :data="openingRows" :columns="[
        { colKey: 'raw', title: 'Cookie', width: 200 },
        { colKey: 'platform', title: '平台', width: 80 },
        { colKey: 'status', title: '状态', width: 100 },
        { colKey: 'account', title: '账号 ID' },
        { colKey: 'error', title: '错误信息' },
      ]" size="small" hover>
        <template #raw="{ row }"><span :title="row.raw">{{ row.raw.substring(0, 40) }}...</span></template>
        <template #status="{ row }">
          <t-tag :theme="row.status === 'created' ? 'success' : 'danger'" size="small">{{ row.status }}</t-tag>
        </template>
        <template #account="{ row }">{{ row.account?.id || '-' }}</template>
        <template #error="{ row }">{{ row.error || '-' }}</template>
      </t-table>
      <t-space style="margin-top:12px" v-if="stats.failed > 0 && !executing">
        <t-button theme="primary" size="small" :disabled="executing" @click="retryFailed">
          重试失败项 ({{ stats.failed }})
        </t-button>
      </t-space>
    </t-card>
  </t-loading>
</template>

<style scoped>
.stat-num { font-size: 24px; font-weight: 700; line-height: 1.2; }
</style>
