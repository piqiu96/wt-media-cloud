<script setup>
import { ref } from "vue"
import { createLocalAgentService } from "../local-agent/service.js"
// Static import, as in `AccountsPage.vue`: a dynamic import() of a bare specifier
// does not resolve in the packaged Tauri WebView ("Module name ... does not
// resolve to a valid file"). It is inert in Cloud Web, which never reaches the
// code that calls it.
import { invoke } from "@tauri-apps/api/core"

const logs = ref([])

// The health check goes through the Local Agent service rather than a bare
// `fetch` to the loopback port. The window's CSP does not allow the page to
// reach that port, so the direct request could only ever fail -- "Agent 不可达"
// was the one message this button could produce, with or without an Agent. The
// service reaches the same `/healthz` through the Rust bridge, which is not
// subject to the page's CSP.
async function refreshLogs() {
  try {
    await createLocalAgentService({ invoke }).health()
    logs.value = ['Agent 日志查看功能待实现']
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
