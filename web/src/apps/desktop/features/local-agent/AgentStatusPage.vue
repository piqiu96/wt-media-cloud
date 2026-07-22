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
          <BusinessStatus :status="page.localStatusKey" :label="page.primaryStatus" />
        </t-descriptions-item>
        <t-descriptions-item label="Cloud登录">
          {{ page.cloudStatusText }}
        </t-descriptions-item>
        <t-descriptions-item label="BitBrowser">
          {{ page.bitbrowserStatusText }}
        </t-descriptions-item>
        <t-descriptions-item label="主账号ID">
          {{ page.mainUserText }}
        </t-descriptions-item>
        <t-descriptions-item label="运行环境">
          {{ page.runtimeText }}
        </t-descriptions-item>
        <t-descriptions-item label="可执行结论">
          <BusinessStatus :status="page.trustStatus" :label="page.trustText" />
        </t-descriptions-item>
      </t-descriptions>
      <t-alert v-if="page" :message="page.trustReason" theme="info" style="margin-top:16px" />

      <t-empty v-else-if="!loading" description="Agent 不可达" />
    </t-card>
  </t-loading>
</template>

<style scoped>
.agent-card {
  margin: 16px;
}
</style>
