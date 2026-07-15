<script setup>
import { onMounted, ref } from 'vue'

const agentStatus = ref(null)
const loading = ref(true)

onMounted(async () => {
  try {
    const resp = await fetch('http://127.0.0.1:8765/api/v1/status')
    if (resp.ok) {
      const body = await resp.json()
      agentStatus.value = body.data || body
    }
  } catch {
    agentStatus.value = { status: 'unreachable' }
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="small">
    <t-card title="Agent 状态" :bordered="true">
      <t-descriptions v-if="agentStatus" :column="1" bordered>
        <t-descriptions-item label="运行状态">
          <t-tag :theme="agentStatus.status === 'idle' || agentStatus.status === 'running' ? 'success' : 'danger'">
            {{ agentStatus.status }}
          </t-tag>
        </t-descriptions-item>
        <t-descriptions-item label="Agent ID">{{ agentStatus.agent_id }}</t-descriptions-item>
        <t-descriptions-item label="当前任务">{{ agentStatus.current_task_id || '无' }}</t-descriptions-item>
        <t-descriptions-item label="待回传结果">{{ agentStatus.pending_result_count || 0 }}</t-descriptions-item>
      </t-descriptions>
      <t-empty v-else-if="!loading" description="Agent 不可达" />
    </t-card>
  </t-loading>
</template>
