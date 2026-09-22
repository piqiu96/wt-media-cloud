<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  modelValue: { type: Array, default: () => [] },
  placeholder: { type: String, default: '输入后按回车添加' },
  max: { type: Number, default: 100 },
})
const emit = defineEmits(['update:modelValue'])

const input = ref('')
const batchVisible = ref(false)
const batchText = ref('')
const tags = computed(() => (Array.isArray(props.modelValue) ? props.modelValue : []))

function splitText(text) {
  return [...new Set(String(text || '').split(/[\n,，、;；]+/).map((item) => item.trim()).filter(Boolean))]
}

function addItems(items) {
  if (!items.length) return
  const merged = [...new Set([...tags.value, ...items])].slice(0, props.max)
  emit('update:modelValue', merged)
}

function addCurrent() {
  addItems(splitText(input.value))
  input.value = ''
}

function onPaste(event) {
  event.preventDefault()
  addItems(splitText((event.clipboardData || window.clipboardData).getData('text')))
}

function remove(index) {
  const next = [...tags.value]
  next.splice(index, 1)
  emit('update:modelValue', next)
}

function clearAll() {
  emit('update:modelValue', [])
}

function openBatch() {
  batchText.value = tags.value.join('\n')
  batchVisible.value = true
}

function confirmBatch() {
  emit('update:modelValue', splitText(batchText.value).slice(0, props.max))
  batchVisible.value = false
}
</script>

<template>
  <div class="keyword-tags">
    <div class="keyword-tags__input">
      <t-input v-model="input" :placeholder="placeholder" @enter="addCurrent" @paste="onPaste" />
      <t-button theme="primary" @click="addCurrent">添加</t-button>
      <t-button variant="outline" @click="openBatch">批量导入</t-button>
      <t-button v-if="tags.length" variant="text" theme="danger" @click="clearAll">清空全部</t-button>
    </div>
    <div class="keyword-tags__list">
      <t-tag v-for="(tag, index) in tags" :key="tag" closable theme="primary" variant="light" @close="remove(index)">{{ tag }}</t-tag>
      <span v-if="!tags.length" class="keyword-tags__empty">暂无关键词</span>
    </div>
    <div class="keyword-tags__footer">
      <span class="keyword-tags__count">{{ tags.length }}/{{ max }}</span>
      <span class="keyword-tags__hint">支持批量粘贴，多行文本自动拆分并去重</span>
    </div>

    <t-dialog v-model:visible="batchVisible" header="批量导入" width="480px" :confirm-btn="{ theme: 'primary', content: '导入' }" @confirm="confirmBatch">
      <t-textarea v-model="batchText" :rows="8" placeholder="每行一个关键词，支持批量粘贴，多行文本自动拆分" />
    </t-dialog>
  </div>
</template>

<style scoped>
.keyword-tags { display: flex; flex-direction: column; gap: 8px; }
.keyword-tags__input { display: flex; gap: 8px; align-items: center; }
.keyword-tags__input .t-input { flex: 1; }
.keyword-tags__list { display: flex; flex-wrap: wrap; gap: 8px; min-height: 30px; padding: 6px 0; }
.keyword-tags__empty { color: var(--wt-text-tertiary); font-size: 12px; }
.keyword-tags__footer { display: flex; justify-content: space-between; align-items: center; color: var(--wt-text-tertiary); font-size: 12px; }
.keyword-tags__count { color: var(--wt-primary); font-weight: 500; }
</style>
