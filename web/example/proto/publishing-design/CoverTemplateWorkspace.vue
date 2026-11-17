<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from "vue";
import ImageEditor from "../../../src/shared/ui/media/ImageEditor.vue";
import ResourceFieldHelp from "../../../src/shared/ui/resource/ResourceFieldHelp.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import Overlay from "./PrototypeOverlay.vue";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import { copy, contentOf, checkDocument, samples, isPublished } from "./coverTemplates.js";
const props = defineProps({ row: Object, images: Array, persist: Function, initialLayer: [String, Object] });
const emit = defineEmits(["close"]);
const doc = ref(contentOf(props.row)), baseline = ref(JSON.stringify(doc.value)), engine = ref(null), mode = ref("edit"), status = ref("已保存"), busy = ref(false), error = ref(""), leave = ref(false), sampleKind = ref("short"), custom = ref(""), preview = ref(""), issues = ref([]), rendering = ref(false);
const previewPanel=ref(null),previewHeight=ref(null),backgroundReading=ref(false);
const businessImage = ref(props.images.find((i) => i.enabled && i.uses?.includes("底图"))?.src || "");
const backgroundMode=computed(()=>doc.value.backgroundMode || (doc.value.image?'fixed':'solid'));
const backgroundOptions=computed(()=>props.images.filter(i=>i.enabled&&i.uses?.includes('底图')).map(i=>({label:i.name,value:i.src})));
const fixedBackgroundOptions=computed(()=>{
 const options=[...backgroundOptions.value];
 if(doc.value.image&&!options.some(i=>i.value===doc.value.image)){
  const current=props.images.find(i=>i.src===doc.value.image);
  options.unshift({label:current?`${current.name}${current.enabled?'':'（已停用）'}`:'当前背景图片',value:doc.value.image,disabled:current?.enabled===false});
 }
 return options;
});
const sample = computed(() => sampleKind.value === "custom" ? custom.value : samples[sampleKind.value]);
const dirty = computed(() => JSON.stringify(doc.value) !== baseline.value);
const contentState = computed(() => isPublished(props.row) ? "已发布" : "草稿");
watch(doc, () => {
  preview.value = "";
  issues.value = [];
  ++renderingId;
  rendering.value = false;
  if (!busy.value) status.value = dirty.value ? "未保存" : "已保存";
}, { deep: true });
let renderingId = 0;
async function render() {
  const request = ++renderingId;
  rendering.value = true;
  preview.value = "";
  try {
    const result = await checkDocument(copy(doc.value), { sample: sample.value, businessImage: businessImage.value });
    if (request === renderingId) {
      preview.value = result.image;
      issues.value = result.issues;
    }
  } catch (e) {
    if (request === renderingId) issues.value = [{ level: "error", layerName: "画布", layerId: "background", message: e.message }];
  } finally {
    if (request === renderingId) rendering.value = false;
  }
}
watch([sampleKind, custom, businessImage], () => {
  if (mode.value === "preview") render();
});
async function showPreview() {
  mode.value = "preview";
  await render();
}
function locate(issue) {
  mode.value = "edit";
  nextTick(() => engine.value?.selectLayer(issue.layerId, issue.field));
}
async function save(close = false) {
  if (busy.value || backgroundReading.value) return false;
  busy.value = true;
  status.value = "保存中";
  error.value = "";
  const sent = copy(doc.value), sentPreview = preview.value;
  try {
    await props.persist(sent, sentPreview);
    baseline.value = JSON.stringify(sent);
    status.value = dirty.value ? "未保存" : "已保存";
    feedback.toast.success(dirty.value ? "已保存提交内容，后续修改尚未保存" : "草稿已保存，可返回列表发布");
    if (close && !dirty.value) finishClose();
    return !dirty.value;
  } catch (e) {
    status.value = "保存失败";
    error.value = e.message || "保存失败，请重试";
    return false;
  } finally {
    busy.value = false;
  }
}
let resolveClose = null, closePromise = null;
function finishClose() {
  emit("close");
  resolveClose?.(true);
  resolveClose = null;
  closePromise = null;
}
function stay() {
  leave.value = false;
  resolveClose?.(false);
  resolveClose = null;
  closePromise = null;
}
function requestClose() {
  if (busy.value || backgroundReading.value) return;
  if (!dirty.value) {
    finishClose();
    return;
  }
  leave.value = true;
  if (!closePromise) closePromise = new Promise((resolve) => resolveClose = resolve);
  return closePromise;
}
function fitPreview(){
 if(mode.value!=='preview'||!previewPanel.value)return;
 const footer=previewPanel.value.closest('.resource-detail-page')?.querySelector('.resource-detail-page__footer');
 previewHeight.value=Math.max(320,window.innerHeight-previewPanel.value.getBoundingClientRect().top-(footer?.getBoundingClientRect().height||69)-20);
}
watch([mode,error],()=>nextTick(fitPreview));
onMounted(()=>window.addEventListener('resize',fitPreview));
function beforeUnload(e) {
  if (dirty.value) {
    e.preventDefault();
    e.returnValue = "";
  }
}
window.addEventListener("beforeunload", beforeUnload);
onBeforeUnmount(() => {
  window.removeEventListener("beforeunload", beforeUnload);
  window.removeEventListener("resize",fitPreview);
  ++renderingId;
});
function patch(key, value) {
  doc.value = { ...doc.value, [key]: value };
}
function chooseMode(value) {
  doc.value = { ...doc.value, backgroundMode: value, image: value === "solid" ? "" : doc.value.image };
}
async function upload(files) {
  const file = files[0];
  if (!file) return;
  if (!["image/png", "image/jpeg", "image/webp"].includes(file.type)) {
    feedback.toast.error("请选择 PNG、JPEG 或 WebP 图片");
    return;
  }
  backgroundReading.value = true;
  try {
    const src = await new Promise((resolve, reject) => {
      const r = new FileReader();
      r.onload = () => resolve(r.result);
      r.onerror = reject;
      r.readAsDataURL(file);
    });
    const img = new Image();
    img.src = src;
    await img.decode();
    doc.value = { ...doc.value, image: src, backgroundMode: "fixed" };
  } catch {
    feedback.toast.error("图片读取失败，已保留原有背景");
  } finally {
    backgroundReading.value = false;
  }
}
function download() {
  const a = document.createElement("a");
  a.href = preview.value;
  a.download = `预览样例-${props.row.name}-${sampleKind.value}.png`;
  a.click();
}
function ready() {
  if (props.initialLayer) {
    const issue = props.initialLayer;
    engine.value?.selectLayer(typeof issue === "string" ? issue : issue.layerId, issue.field);
  }
}
defineExpose({ requestClose });
</script>
<template>
 <Overlay :visible="true" page hide-title :title="'编辑封面模板'" back-label="返回封面模板列表" :busy="busy||backgroundReading" class="cover-template-page" @update:visible="requestClose">
  <div class="cover-editor-heading"><h1>{{row.name}}</h1><t-tag size="small" :theme="isPublished(row)?'success':'default'" variant="light">{{contentState}}</t-tag><span v-if="isPublished(row)&&row.draft" class="cover-draft-note">有草稿</span></div>
  <ResourceNotice v-if="error" tone="warning">{{error}}；当前修改仍保留，可重试保存。</ResourceNotice>
  <ImageEditor v-show="mode==='edit'" ref="engine" v-model="doc" fixed-size template-mode :background-preview="backgroundMode==='business'?businessImage:undefined" @ready="ready">
   <template #settings="{selected,showSafe,setShowSafe}">
    <template v-if="selected==='background'">
     <h3>背景与画布</h3>
     <label class="cover-field">背景类型<t-select :value="backgroundMode" :options="[{label:'纯色',value:'solid'},{label:'固定图片',value:'fixed'},{label:'业务底图',value:'business'}]" @change="chooseMode" /></label>
     <label class="cover-field">背景颜色<input aria-label="背景颜色" type="color" :value="doc.background" @change="patch('background',$event.target.value)" /></label>
     <template v-if="backgroundMode!=='solid'">

      <label class="cover-field"><span>{{backgroundMode==='business'?'底图预览样例':'固定背景图片'}} <ResourceFieldHelp v-if="backgroundMode==='business'" label="底图预览样例" content="生成时使用实际业务底图，此处选择的图片仅用于预览。" /></span><t-select :value="backgroundMode==='business'?businessImage:doc.image" clearable :options="backgroundMode==='business'?backgroundOptions:fixedBackgroundOptions" @change="v=>backgroundMode==='business'?businessImage=v:patch('image',v)" /></label>
      <div v-if="backgroundMode==='fixed'" class="cover-field"><span>上传图片</span><t-upload theme="file" accept="image/png,image/jpeg,image/webp" :auto-upload="false" :files="[]" :file-list-display="null" :upload-button="null" :disabled="busy||backgroundReading" :trigger-button-props="{content:'上传图片',theme:'default',variant:'outline',loading:backgroundReading}" :input-attributes="{'aria-label':'上传固定背景'}" tips="支持 PNG、JPEG、WebP" @select-change="upload" /></div>
      <label class="cover-field">填充方式<t-select :value="doc.imageFit||'slice'" :options="[{label:'填满并裁切',value:'slice'},{label:'完整显示',value:'meet'}]" @change="v=>patch('imageFit',v)" /></label>
      <label class="cover-field">图片定位<t-select :value="doc.imageAnchor||'xMidYMid'" :options="[{label:'居中',value:'xMidYMid'},{label:'靠左',value:'xMinYMid'},{label:'靠右',value:'xMaxYMid'},{label:'靠上',value:'xMidYMin'},{label:'靠下',value:'xMidYMax'}]" @change="v=>patch('imageAnchor',v)" /></label>
     </template>
     <details class="cover-canvas-settings"><summary>画布设置</summary>
      <label class="cover-field"><span>安全边距（px） <ResourceFieldHelp label="安全边距" content="文字须位于画布边缘向内的安全范围。隐藏辅助线不影响校验。" /></span><t-input-number :value="doc.safe" :min="0" :max="Math.floor(Math.min(doc.width,doc.height)/2)-1" @change="v=>patch('safe',v)" /></label>
      <t-checkbox :checked="showSafe" @change="setShowSafe">显示安全区辅助线</t-checkbox>
     </details>
    </template>
    <div v-else class="cover-hint">选择图层以调整属性，或选择背景设置画布。</div>
   </template>
  </ImageEditor>
  <section v-if="mode==='preview'" ref="previewPanel" :style="previewHeight?{height:`${previewHeight}px`}:undefined" class="cover-preview-workspace" aria-label="预览与校验">
   <div class="cover-preview-stage" :aria-busy="rendering"><t-loading v-if="rendering"/><img v-if="preview" :src="preview" alt="封面模板预览"/><div v-else-if="!rendering" class="cover-preview-empty">{{rendering?'正在生成预览…':'请修复右侧问题后重新预览'}}</div></div>
   <aside><h3>预览与校验 <ResourceFieldHelp label="预览与校验" content="使用当前样例检查同一个动态标题的排版及安全区；发布时检查精简、较长样例。当前为本地原型预览。" /></h3>
    <div class="cover-field"><span>标题样例</span><t-radio-group v-model="sampleKind" variant="outline"><t-radio-button value="short">精简</t-radio-button><t-radio-button value="long">较长</t-radio-button><t-radio-button value="custom">自定义</t-radio-button></t-radio-group></div>
    <label v-if="sampleKind==='custom'" class="cover-field">自定义样例<t-textarea v-model="custom" placeholder="输入预览标题" /></label><p v-else class="cover-hint">{{sample}}</p>
    <label v-if="backgroundMode==='business'" class="cover-field">业务底图样例<t-select v-model="businessImage" :options="backgroundOptions" /></label>
    <div class="cover-preview-actions"><t-button theme="default" variant="outline" :loading="rendering" @click="render()">重新校验</t-button><t-button variant="text" :disabled="!preview||rendering" @click="download">导出预览 PNG</t-button></div>
    <p class="cover-hint">本地原型预览</p><t-tag v-if="!rendering&&!issues.length" theme="success" variant="light">当前样例校验通过</t-tag>
    <div v-for="(issue,i) in issues" :key="i" class="cover-issue" :class="issue.level"><strong>{{issue.layerName}}</strong><p>{{issue.message}}</p><t-button variant="text" @click="locate(issue)">定位修改</t-button></div>
   </aside>
  </section>
  <template #footer><span class="cover-save-status">{{status}}</span><t-button theme="default" variant="outline" :disabled="busy||backgroundReading" @click="mode==='edit'?showPreview():mode='edit'">{{mode==='edit'?'预览与校验':'返回编辑'}}</t-button><t-button :loading="busy" :disabled="backgroundReading||(!dirty&&status!=='保存失败')" @click="save()">保存草稿</t-button></template>
 </Overlay>
 <Teleport to="body"><t-dialog :visible="leave" header="保存修改后离开？" :footer="false" :close-on-overlay-click="false" @close="!busy&&stay()"><p>当前修改尚未保存。</p><ResourceNotice v-if="error" tone="warning">{{error}}；修改仍在，请重试或继续编辑。</ResourceNotice><div class="cover-dialog-actions"><t-button theme="default" variant="text" :disabled="busy" @click="stay">继续编辑</t-button><t-button theme="default" variant="outline" :disabled="busy" @click="finishClose">放弃修改</t-button><t-button :loading="busy" @click="save(true)">保存并离开</t-button></div></t-dialog></Teleport>
</template>
<style scoped>
.cover-editor-heading{display:flex;align-items:center;gap:12px;margin:0 0 16px;min-height:36px;}.cover-editor-heading h1{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;margin:0;font-size:24px;font-weight:500;line-height:34px;color:var(--wt-text-primary);}.cover-draft-note{font-size:13px;white-space:nowrap;color:var(--wt-text-tertiary);}.cover-canvas-settings{border-top:1px solid var(--wt-border);padding-top:8px;margin-top:20px;}.cover-canvas-settings summary{cursor:pointer;font-size:13px;font-weight:500;padding:8px 0;}.cover-field+h3{margin-top:20px;}:deep(.editor-properties-panel>h3){font-size:14px;font-weight:500;margin:0 0 16px;}
.cover-field {display:flex;flex-direction:column;gap:8px;margin:16px 0;font-size:13px;color:var(--wt-text-secondary);}
.cover-field input[type=color]{width:48px;height:32px;border:1px solid var(--wt-border);background:none;}
.cover-hint{font-size:13px;line-height:1.6;color:var(--wt-text-tertiary);margin:12px 0;}

.cover-preview-workspace{display:grid;grid-template-columns:minmax(280px,1fr) 300px;gap:24px;height:max(380px,calc(100dvh - 242px));}.cover-preview-stage{overflow:hidden;box-sizing:border-box;background:var(--wt-bg-page);display:flex;justify-content:center;align-items:center;min-height:0;padding:20px;}.cover-preview-stage :deep(.t-loading){height:100%;width:100%;display:flex;justify-content:center;align-items:center;}.cover-preview-stage img{max-height:100%;max-width:100%;object-fit:contain;box-shadow:var(--wt-shadow-popup);}.cover-preview-workspace aside{overflow:auto;padding-right:8px;}.cover-preview-workspace h3{display:flex;align-items:center;gap:8px;margin:0 0 16px;font-size:16px;line-height:24px;font-weight:500;}.cover-preview-empty{font-size:14px;color:var(--wt-text-tertiary);}.cover-preview-actions{display:flex;gap:8px;flex-wrap:wrap;margin:16px 0;}.cover-issue{padding:12px 0;border-top:1px solid var(--wt-border);font-size:13px;}.cover-issue p{margin:6px 0;}.cover-issue.error strong{color:var(--wt-danger);}.cover-issue.warning strong{color:var(--wt-warning);}.cover-save-status{margin-right:auto;font-size:13px;color:var(--wt-text-tertiary);}.cover-dialog-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:24px;}
:deep(.editor-stage svg){background:white;}:deep(.editor-layer .layer-name){overflow-wrap:anywhere;}
@media(max-width:900px){.cover-preview-workspace{grid-template-columns:minmax(0,1fr);height:auto;gap:16px;}.cover-preview-stage{height:360px;}.cover-preview-workspace aside{overflow:visible;padding:0;}}
</style>
