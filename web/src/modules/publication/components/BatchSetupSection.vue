<script setup>
// Structure and geometry ported from example/proto/publishing-design/PublishComposer.vue.
// Keep production services in the caller; the preview is never a runtime dependency.
import ResourceFieldHelp from '../../../shared/ui/resource/ResourceFieldHelp.vue';
defineProps({number:String,title:String,help:String});
</script>
<template><section class="batch-section"><header class="batch-section-head"><div><span class="batch-number">{{number}}</span><h3>{{title}}<ResourceFieldHelp v-if="help" :label="title" :content="help"/></h3></div><div class="batch-head-actions"><slot name="actions"/></div></header><div class="batch-section-body"><slot/></div></section></template>
<style scoped>
.batch-section{border:1px solid var(--wt-border);border-radius:8px;background:var(--wt-bg-card);overflow:hidden;box-shadow:none}
.batch-section-head{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:24px 24px 0;margin-bottom:16px}.batch-section-head>div:first-child{display:flex;align-items:center;gap:12px;min-width:0;flex-wrap:wrap}.batch-section-head h3{display:flex;align-items:center;gap:8px;font-size:var(--wt-font-size-section,16px);font-weight:500;line-height:var(--wt-line-height-section,24px);margin:0}.batch-number{display:grid;place-items:center;min-width:28px;height:28px;padding:0 4px;box-sizing:border-box;border-radius:7px;background:var(--wt-color-primary-light);color:var(--wt-primary);font-size:12px;font-weight:500}.batch-section-body{padding:0 24px 24px}.batch-head-actions{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end}
@media(max-width:680px){.batch-section-head{padding:20px 16px 0;flex-wrap:wrap}.batch-section-body{padding:0 16px 20px}}
</style>
