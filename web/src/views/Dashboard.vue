<script setup>
import { onMounted, ref } from 'vue'
import { createSessionClient } from '../session.js'

const sessionClient = createSessionClient()
const user = ref(null)

onMounted(async () => {
  try {
    user.value = await sessionClient.me()
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
        <t-statistic title="等待执行任务" :value="0" />
      </t-card>
      <t-card :bordered="true" theme="top" style="cursor:default">
        <t-statistic title="失败任务" :value="0" :trend="'negative'" />
      </t-card>
      <t-card :bordered="true" theme="top" style="cursor:default">
        <t-statistic title="社媒账号" :value="0" />
      </t-card>
    </div>

    <div style="margin-top:24px; display:grid; grid-template-columns: 1fr 1fr; gap:24px">
      <t-card title="快捷入口" :bordered="true">
        <div style="display:flex; flex-wrap:wrap; gap:8px">
          <t-button theme="default" size="small" @click="$router.push('/accounts')">社媒账号</t-button>
          <t-button theme="default" size="small" @click="$router.push('/tasks')">任务管理</t-button>
        </div>
      </t-card>
      <t-card title="系统信息" :bordered="true">
        <t-descriptions :column="1" size="small">
          <t-descriptions-item label="当前用户">{{ user?.username || '-' }}</t-descriptions-item>
          <t-descriptions-item label="角色">{{ user?.role || '-' }}</t-descriptions-item>
        </t-descriptions>
      </t-card>
    </div>
  </t-card>
</template>
