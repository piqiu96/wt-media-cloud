<script setup>
import {computed} from 'vue'
import {ChevronDownIcon} from 'tdesign-icons-vue-next'
import SystemAvatar from '../../../src/shared/ui/SystemAvatar.vue'
import {currentPrototypeProfile} from './prototypeSession.js'
import {feedback} from '../../../src/shared/ui/feedback/index.js'
const emit=defineEmits(['navigate'])
const profile=computed(currentPrototypeProfile)
function select(item){if(item.value==='personal')emit('navigate','personal');else feedback.toast.info('原型演示：不会退出真实账号')}
</script>
<template><div class="operations-account"><t-dropdown trigger="click" :options="[{content:'我的账号',value:'personal'},{content:'退出',value:'logout'}]" @click="select"><t-button theme="default" variant="text" aria-label="我的账号"><SystemAvatar :avatar-id="profile.avatar" :size="28"/><span class="operations-account-name">{{profile.nickname}}</span><ChevronDownIcon/></t-button></t-dropdown></div></template>
<style scoped>.operations-account{position:fixed;right:28px;top:18px;z-index:110}.operations-account :deep(.t-button__text){display:flex;align-items:center;gap:8px}.operations-account-name{max-width:120px;overflow:hidden;white-space:nowrap;text-overflow:ellipsis}@media(max-width:900px){.operations-account{right:20px}}@media(max-width:680px){.operations-account{top:10px;right:16px}.operations-account-name{max-width:80px}}</style>
