<script setup>
import { Select } from 'tdesign-vue-next';
defineProps({platforms:Array,categories:Array,platformOptions:Array,categoryOptions:Array,locked:Boolean});
const emit=defineEmits(['change']);
</script>
<template><div class="plan-goal-scope">
  <label><span><i>*</i>发布平台</span><Select :value="platforms" :options="platformOptions.map(value=>({value,label:value}))" multiple filterable :min-collapsed-num="2" :disabled="locked" placeholder="请选择发布平台" aria-label="发布平台" @change="values=>emit('change','platforms',values)" /></label>
  <label><span><i>*</i>发布类目</span><Select :value="categories" :options="categoryOptions.map(value=>({value,label:value}))" multiple filterable :min-collapsed-num="2" :disabled="locked" placeholder="请选择发布类目" aria-label="发布类目" @change="values=>emit('change','categories',values)" /></label>
</div></template>
<style scoped>.plan-goal-scope{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:24px;margin-bottom:18px}.plan-goal-scope label{display:flex;flex-direction:column;gap:8px;min-width:0}.plan-goal-scope label>span{font-size:14px;display:flex;gap:4px}.plan-goal-scope i{color:var(--td-error-color);font-style:normal}@media(max-width:680px){.plan-goal-scope{grid-template-columns:1fr;gap:16px}}</style>
