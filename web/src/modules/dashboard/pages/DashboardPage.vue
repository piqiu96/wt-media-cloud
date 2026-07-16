<script setup>
import { onMounted, ref } from 'vue'
import { createSessionClient } from '../../../shared/api/session.js'
import { createApiClient } from '../../../shared/api/http.js'

const sessionClient = createSessionClient()
const api = createApiClient()
const user = ref(null)
const taskStats = ref({ pending: 0, failed: 0, succeeded: 0 })
const accountCount = ref(0)

onMounted(async () => {
  try {
    user.value = await sessionClient.me()
    const [stats, accounts] = await Promise.all([
      api.get('/tasks/stats').catch(() => ({})),
      api.get('/media-accounts').catch(() => []),
    ])
    taskStats.value = { pending: 0, failed: 0, succeeded: 0, ...stats }
    accountCount.value = Array.isArray(accounts) ? accounts.length : 0
  } catch {
    // Will redirect to login
  }
})
</script>

<template>
  <t-card title="工作台" :bordered="true">
    <div style="display:grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap:16px">
      <t-card :bordered="true" theme="top" style="cursor:default">
        <t-statistic title="云端服务" :value="'正常'" :trend="'positive'" />
      </t-card>
      <t-card :bordered="true" theme="top" style="cursor:default">
        <t-statistic title="等待执行任务" :value="taskStats.pending" />
      </t-card>
      <t-card :bordered="true" theme="top" style="cursor:default">
        <t-statistic title="失败任务" :value="taskStats.failed" :trend="taskStats.failed > 0 ? 'negative' : undefined" />
      </t-card>
      <t-card :bordered="true" theme="top" style="cursor:default">
        <t-statistic title="社媒账号" :value="accountCount" />
      </t-card>
    </div>

    <div style="margin-top:24px; display:grid; grid-template-columns: 1fr 1fr; gap:24px">
      <t-card title="快捷入口" :bordered="true">
        <div style="display:flex; flex-wrap:wrap; gap:8px">
          <t-button theme="default" size="small" @click="$router.push('/accounts')">社媒账号</t-button>
          <t-button theme="default" size="small" @click="$router.push('/execute-tasks')">任务管理</t-button>
        </div>
      </t-card>
      <t-card title="系统信息" :bordered="true">
        <t-descriptions :column="1" size="small">
          <t-descriptions-item label="当前用户">{{ user?.username || '-' }}</t-descriptions-item>
          <t-descriptions-item label="角色">{{ user?.role || '-' }}</t-descriptions-item>
          <t-descriptions-item label="已完成任务">{{ taskStats.succeeded }}</t-descriptions-item>
        </t-descriptions>
      </t-card>
    </div>
  </t-card>
</template>
