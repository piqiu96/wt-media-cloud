<script setup>
import { nextTick, onMounted, onUnmounted, ref } from 'vue';
import { EditIcon, RefreshIcon } from 'tdesign-icons-vue-next';
const props=defineProps({label:String,value:String,multiline:Boolean,disabled:Boolean,error:String,source:String});
const emit=defineEmits(['change','regenerate']);
const editing=ref(false),draft=ref(''),input=ref(null),root=ref(null);
async function start(){if(props.disabled)return;draft.value=props.value || '';editing.value=true;await nextTick();input.value?.focus?.();}
function finish(){if(!editing.value)return;editing.value=false;emit('change',draft.value);}
function keyboard(event){
  if(event.isComposing)return;
  if(event.key==='Escape'){event.stopImmediatePropagation();event.preventDefault();editing.value=false;}
  else if(event.key==='Enter'&&!props.multiline){event.preventDefault();finish();}
}
// The shared overlay handles Escape at document capture; intercept local edits first.
function localEscape(event){if(editing.value && event.key==='Escape' && root.value?.contains(event.target))keyboard(event);}
onMounted(()=>window.addEventListener('keydown',localEscape,true));
onUnmounted(()=>window.removeEventListener('keydown',localEscape,true));
</script>
<template>
  <div ref="root" class="bulk-review-field" :class="{'is-wide':multiline,'has-error':error}" @keydown.capture="keyboard">
    <div class="bulk-review-field__heading"><span>{{label}}</span><small v-if="source">{{source}}</small><t-button size="small" theme="default" :disabled="disabled || editing" :aria-label="`重新生成${label}`" @click="emit('regenerate')"><template #icon><RefreshIcon /></template>重新生成</t-button></div>
    <component :is="multiline?'t-textarea':'t-input'" v-if="editing" ref="input" v-model="draft" :aria-label="`修改${label}`" :status="error?'error':undefined" :autosize="multiline?{minRows:3,maxRows:8}:undefined" @blur="finish" />
    <button v-else type="button" class="bulk-review-field__value" :aria-label="`修改${label}`" :disabled="disabled" @click="start"><span :class="{'is-empty':!value}">{{value || '未填写'}}</span><EditIcon aria-hidden="true" /></button>
    <small v-if="error" class="bulk-review-field__error" role="alert">{{error}}</small>
  </div>
</template>
<style scoped>
.bulk-review-field{min-width:0;display:flex;flex-direction:column;gap:6px}.bulk-review-field.is-wide{grid-column:1 / -1}.bulk-review-field__heading{display:flex;align-items:center;gap:8px;flex-wrap:wrap;font-size:13px;color:var(--wt-color-text-secondary)}.bulk-review-field__heading small{font-size:12px}.bulk-review-field__heading .t-button{margin-left:auto;min-height:28px!important;padding-inline:8px!important;font-size:12px!important}.bulk-review-field__value{border:1px solid transparent;background:transparent;border-radius:4px;padding:7px 8px;text-align:left;display:flex;align-items:flex-start;gap:8px;min-height:36px;width:100%;font:inherit;color:var(--wt-color-text-primary);cursor:text}.bulk-review-field__value span{flex:1;min-width:0;white-space:pre-wrap;overflow-wrap:anywhere;line-height:22px}.bulk-review-field__value .t-icon{flex-shrink:0;color:var(--wt-color-text-secondary);margin-top:3px}.bulk-review-field__value:hover:not(:disabled){background:var(--wt-color-bg-subtle)}.bulk-review-field__value:focus-visible{outline:2px solid var(--wt-color-primary);outline-offset:2px}.bulk-review-field__value:hover:not(:disabled) .t-icon{color:var(--wt-color-primary)}.bulk-review-field__value:disabled{cursor:default}.bulk-review-field__value:disabled .t-icon{display:none}.is-empty{color:var(--wt-color-text-secondary)}.bulk-review-field__error{color:var(--td-error-color)!important}.has-error .bulk-review-field__value{border-color:var(--td-error-color)}
</style>
