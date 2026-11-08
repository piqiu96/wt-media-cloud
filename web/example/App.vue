<script setup>
import { onMounted, ref } from 'vue'
import ComponentDemo from './common/ComponentDemo.vue'
const localWorkspace = ref(false)
onMounted(async () => { try { localWorkspace.value = (await (await fetch('/__example/context')).json()).localWorkspace } catch {} })
const prototypes = [
  ['浏览器窗口与代理', 'browser-proxy/'], ['社媒账号', 'social-accounts/'],
  ['内容运营工作台', 'content-operations/'], ['内容挖掘', 'content-mining/'],
  ['发布管理逐页评审', 'content-operations/publishing-review.html'],
  ['内容生产评审', 'content-operations/review.html?scope=production#page-review'],
]
</script>
<template>
  <main class="example-home wt-resource-page">
    <h1>原型与公共组件</h1>
    <p>已有原型保留确认结果；新原型直接使用正式组件。所有业务操作均为演示。</p>
    <nav class="prototype-links" aria-label="原型入口"><a v-for="[name, url] in prototypes" :key="url" :href="`/example/proto/${url}`">{{ name }}</a></nav>
    <p><a href="/__docs/workspace/docs/design/README.md">当前设计工作稿</a> · <a href="/__docs/workspace/docs/standards/frontend/README.md">正式前端规范</a> · <a href="/example/common/index.html">已确认 HTML 公共参考</a></p>
    <p class="context">{{ localWorkspace ? '文档来源：本地 Workspace，保存后刷新可查看未提交修改。' : '本地 Workspace 文档未挂载；文档入口将提供 GitHub 已推送版本链接。' }}</p>
    <ComponentDemo />
  </main>
</template>
<style scoped>
.example-home{max-width:1280px;margin:0 auto;padding:32px 24px}.example-home h1{font-size:24px}.example-home p{line-height:1.7}.prototype-links{display:flex;flex-wrap:wrap;gap:12px 24px;margin:24px 0}.example-home a{color:var(--wt-primary)}.context{color:var(--td-text-color-secondary)}
</style>
