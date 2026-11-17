<script setup>
import { Input, Textarea } from 'tdesign-vue-next'
import ResourceDateRange from '../../../../src/shared/ui/resource/ResourceDateRange.vue'
import ResourceFieldHelp from '../../../../src/shared/ui/resource/ResourceFieldHelp.vue'
const props=defineProps({editor:Object,original:Object,actor:Object,members:Array,locked:Boolean})
const emit=defineEmits(['change'])
function dates(value){emit('change',{dateStart:value?.[0]||'',dateEnd:value?.[1]||''})}
</script>
<template><div class="plan-basic-grid">
 <label class="plan-basic-name"><span><i>*</i>计划名称</span><Input :value="editor.name" :maxlength="60" placeholder="例如：新赛季内容矩阵 · 周度发布" @update:value="emit('change',{name:$event})" /></label>

 <label class="plan-basic-period"><span><i>*</i>执行周期<ResourceFieldHelp label="执行周期" content="账号容量按所选周期进行测算；周期精确到日期。资源锁定后不可修改。" /></span><ResourceDateRange :value="[editor.dateStart,editor.dateEnd]" :disabled="locked" :status="editor.dateStart && editor.dateEnd && editor.dateStart>editor.dateEnd?'error':undefined" @change="dates"/><small v-if="editor.dateStart && editor.dateEnd && editor.dateStart>editor.dateEnd" role="alert">结束日期不能早于开始日期</small></label>
 <label class="plan-basic-note"><span>计划备注</span><Textarea :value="editor.note" :rows="3" placeholder="记录发布目标、活动背景、投放重点等" @update:value="emit('change',{note:$event})" /></label>
</div></template>
<style scoped>
.plan-basic-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:24px}.plan-basic-grid label{display:flex;flex-direction:column;gap:8px;min-width:0}.plan-basic-grid label>span{display:flex;align-items:center;gap:6px;font-size:14px;font-weight:400}.plan-basic-grid i{font-style:normal;color:var(--td-error-color)}.plan-basic-grid small{color:var(--td-error-color)}.plan-basic-period{width:100%}.plan-basic-note{grid-column:1 / -1}@media(max-width:800px){.plan-basic-grid{grid-template-columns:1fr}}
</style>
