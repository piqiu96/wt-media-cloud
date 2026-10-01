<script setup>
import { onMounted, ref } from "vue"
import { bindTrustedLocalAgent, createDesktopStatusPreview } from "./init"
import BusinessStatus from "../../../../shared/ui/BusinessStatus.vue"

const page = ref(null)
const loading = ref(true)
const actionLoading = ref(false)
const actionMessage = ref("")
const actionError = ref("")

async function refreshStatus() {
  actionError.value = ""
  actionMessage.value = ""
  try {
    page.value = await createDesktopStatusPreview()
  } catch (error) {
    actionError.value = error?.message || "重新检测本机环境失败"
  }
}

async function reloadPage() {
  loading.value = true
  try {
    await refreshStatus()
  } finally {
    loading.value = false
  }
}

async function bindTrustedNode() {
  actionLoading.value = true
  actionError.value = ""
  actionMessage.value = ""
  try {
    page.value = await bindTrustedLocalAgent()
    actionMessage.value = "本机可信状态已刷新，可以继续执行本机浏览器相关操作。"
  } catch (error) {
    actionError.value = error?.message || "刷新本机可信状态失败"
  } finally {
    actionLoading.value = false
  }
}

onMounted(reloadPage)
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
        <t-descriptions-item label="比特浏览器账号">
          {{ page.mainUserText }}
        </t-descriptions-item>
        <t-descriptions-item label="当前会话节点">
          {{ page.nodeText }}
        </t-descriptions-item>
        <t-descriptions-item label="运行环境">
          {{ page.runtimeText }}
        </t-descriptions-item>
        <t-descriptions-item label="当前电脑状态">
          <BusinessStatus :status="page.trustStatus" :label="page.trustText" />
        </t-descriptions-item>
      </t-descriptions>
      <div v-if="page" class="agent-actions">
        <t-button :loading="loading" @click="reloadPage">重新检测本机环境</t-button>
        <t-button variant="outline" @click="$router.push('/personal-info')">查看设备绑定</t-button>
        <t-button
          v-if="page.canBindTrustedNode"
          theme="primary"
          :loading="actionLoading"
          @click="bindTrustedNode"
        >
          {{ page.bindActionText }}
        </t-button>
      </div>
      <t-alert v-if="actionMessage" :message="actionMessage" theme="success" style="margin-top:16px" />
      <t-alert v-if="actionError" :message="actionError" theme="error" style="margin-top:16px" />
      <t-alert v-if="page" :message="page.trustReason" theme="info" style="margin-top:16px" />

      <t-empty v-else-if="!loading" description="Agent 不可达" />
    </t-card>
  </t-loading>
</template>

<style scoped>
.agent-card {
  margin: 16px;
}

.agent-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}
</style>
