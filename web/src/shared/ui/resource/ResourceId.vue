<script setup>
import { ref } from 'vue'
import { CopyIcon, CheckIcon } from 'tdesign-icons-vue-next'
const props=defineProps({ value:[String,Number], label:{type:String,default:'ID'} })
const copied=ref(false),failed=ref(false)
async function copy(){
 try{await navigator.clipboard.writeText(String(props.value));copied.value=true;failed.value=false;}
 catch{failed.value=true;}
}
</script>
<template><span class="resource-id"><span class="resource-id-value" :title="String(value ?? '-')">{{value ?? '-'}}</span><button v-if="value != null" type="button" :aria-label="`复制${label} ${value}`" :title="copied?'已复制':`复制${label}`" @click="copy"><CheckIcon v-if="copied"/><CopyIcon v-else/></button><span class="resource-id-feedback" role="status">{{failed?'复制失败，请选中 ID 复制':copied?'已复制':''}}</span></span></template>
<style scoped>
.resource-id{display:inline-flex;align-items:center;gap:6px;max-width:100%;font-variant-numeric:tabular-nums}.resource-id-value{overflow-wrap:anywhere;user-select:text}.resource-id button{flex:none;display:inline-flex;padding:3px;border:0;background:transparent;color:var(--wt-text-tertiary);cursor:pointer;border-radius:3px}.resource-id button:hover{color:var(--wt-primary);background:var(--td-bg-color-container-hover)}.resource-id button:focus-visible{outline:2px solid var(--wt-primary);outline-offset:1px}.resource-id-feedback{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}
</style>
