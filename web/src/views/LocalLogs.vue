<script setup>
import { ref } from 'vue'

const logs = ref([])

async function refreshLogs() {
  try {
    const resp = await fetch('http://127.0.0.1:8765/healthz')
    if (resp.ok) {
      // Placeholder: log viewing will be implemented with Agent log API
      logs.value = ['Agent 日志查看功能待实现']
    } else {
      logs.value = ['Agent 不可达']
    }
  } catch {
    logs.value = ['Agent 不可达']
  }
}
</script>

<template>
  <t-card title="本地日志" :bordered="true">
    <template #actions>
      <t-button theme="default" size="small" @click="refreshLogs">刷新</t-button>
    </template>
    <t-empty v-if="!logs.length" description="暂无日志" />
    <t-list v-else>
      <t-list-item v-for="(log, idx) in logs" :key="idx">
        {{ log }}
      </t-list-item>
    </t-list>
  </t-card>
</template>
