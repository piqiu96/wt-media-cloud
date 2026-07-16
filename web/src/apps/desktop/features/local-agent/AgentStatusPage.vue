<script setup>
import { onMounted, ref } from "vue"
import { createDesktopStatusPreview } from "./init"
import BusinessStatus from "../../../../shared/ui/BusinessStatus.vue"

const page = ref(null)
const loading = ref(true)

onMounted(async () => {
  try {
    page.value = await createDesktopStatusPreview()
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true" size="large">
    <t-card title="Local Agent 控制台" :bordered="true" class="agent-card">
      <template #description>
        本地 Agent 状态和运行环境检测
      </template>

      <t-descriptions v-if="page" :column="2" bordered>
        <t-descriptions-item label="Agent ID">
          {{ page.agentId }}
        </t-descriptions-item>
        <t-descriptions-item label="运行状态">
          <BusinessStatus :status="page.primaryStatus.toLowerCase()" />
        </t-descriptions-item>
        <t-descriptions-item label="当前任务" :span="1">
          {{ page.currentTaskText }}
        </t-descriptions-item>
        <t-descriptions-item label="待回传结果" :span="1">
          {{ page.pendingResultText }}
        </t-descriptions-item>
      </t-descriptions>

      <t-empty v-else-if="!loading" description="Agent 不可达" />
    </t-card>
  </t-loading>
</template>

<style scoped>
.agent-card {
  margin: 16px;
}
</style>
