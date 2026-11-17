<script setup>
defineProps({ platform: String, category: String });
</script>
<template>
  <div class="publishing-scope-filters">
    <label>发布平台<t-select :value="platform" :options="[{label:platform,value:platform}]" disabled aria-label="锁定的发布平台" /></label>
    <label>发布类目<t-select :value="category" :options="[{label:category,value:category}]" disabled aria-label="锁定的发布类目" /></label>
  </div>
</template>
<style scoped>
.publishing-scope-filters{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px 16px;margin:0 0 16px;font-size:13px}.publishing-scope-filters label{display:grid;gap:8px}
</style>
