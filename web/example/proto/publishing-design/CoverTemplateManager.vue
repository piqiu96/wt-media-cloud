<script setup>
import {opColumnWidth} from "../../../src/shared/ui/resource/opColumnWidth.js";
import ResourceId from '../../../src/shared/ui/resource/ResourceId.vue';
import {formatDateTime} from '../../../src/shared/utils/datetime.js';
import {readOperationRoute,updateOperationURL} from "./routes.js";
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from "vue";
import ResourceAdvancedToggle from "../../../src/shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceFieldHelp from "../../../src/shared/ui/resource/ResourceFieldHelp.vue";
import ResourcePageHeader from "../../../src/shared/ui/resource/ResourcePageHeader.vue";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../src/shared/ui/resource/ResourceFilterBar.vue";
import ResourceRowActions from "../../../src/shared/ui/resource/ResourceRowActions.vue";
import ResourceEmptyState from "../../../src/shared/ui/resource/ResourceEmptyState.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import MediaPreview from "../../../src/shared/ui/media/MediaPreview.vue";
import Overlay from "./PrototypeOverlay.vue";
import Workspace from "./CoverTemplateWorkspace.vue";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import { templateReferences } from "./library.js";
import { copy, sizes, isPublished, contentOf, dimensions, createTemplate, saveDraft, publishDraft, setUsage, cloneTemplate, checkDocument } from "./coverTemplates.js";
const props = defineProps({ db: Object, persist: Function });
const thumbnails = ref({});
const query = ref(""), use = ref(""), content = ref(""), size = ref(""), platform = ref(""), scope = ref(""), advanced = ref(false), applied = ref({}), currentPage = ref(1), pageSize = ref(10);
const editorId = ref(null), workspace = ref(null), initialLayer = ref(null), creating = ref(false), copySource = ref(null), form = ref({}), formError = ref(""), busy = ref(false), switchBusy = ref(null), error = ref(""), previewOpen = ref(false), previewSrc = ref(""), previewTitle = ref("");
const publishing = ref(null), checking = ref(false), publishResult = ref(null), publishError = ref("");
const categories = computed(() => [...new Set(props.db.images.map((i) => i.category).filter(Boolean))].filter((x) => x !== "通用"));
const platforms = ["百家号", "B站", "小红书"];
const row = computed(() => props.db.templates.find((t) => t.id === editorId.value));
const rows = computed(() => {
  const q = applied.value;
  return props.db.templates.filter((t) => {
    const doc = contentOf(t);
    return (!q.query || t.name.toLowerCase().includes(q.query.toLowerCase())) && (!q.use || String(!!t.enabled) === q.use) && (!q.content || (q.content === "draft" ? !!t.draft || !isPublished(t) : isPublished(t))) && (!q.size || `${doc.width}x${doc.height}` === q.size) && (!q.platform || t.platforms?.includes(q.platform)) && (!q.scope || (q.scope === "通用" ? !t.categories?.length : t.categories?.includes(q.scope)));
  }).sort((a, b) => String(b.updatedAt || "").localeCompare(String(a.updatedAt || "")));
});
const pageRows = computed(() => rows.value.slice((currentPage.value - 1) * pageSize.value, currentPage.value * pageSize.value));
const columns = computed(()=>[{ colKey: "id", title: "模板ID", width: 110 }, { colKey: "preview", title: "封面", width: 118 }, { colKey: "name", title: "模板名称", minWidth: 175 }, { colKey: "spec", title: "规格／范围", minWidth: 150 }, { colKey: "content", title: "内容状态", width: 112 }, { colKey: "usage", title: "启用", width: 88 }, { colKey: "updated", title: "更新时间", width: 145 }, { colKey: "actions", title: "操作", width: Math.max(112,...pageRows.value.map(t=>opColumnWidth(['编辑',...(t.draft?[isPublished(t)?'发布更新':'发布']:[]),'复制',...(!isPublished(t)?['删除']:[])],{variant:'text'})+16)), fixed: "right" }]);
function apply() {
  applied.value = { query: query.value.trim(), use: use.value, content: content.value, size: size.value, platform: platform.value, scope: scope.value };
  currentPage.value = 1;
}
function reset() {
  query.value = use.value = content.value = size.value = platform.value = scope.value = "";
  apply();
}
function newId() {
  return Math.max(0, ...props.db.templates.map((t) => Number(t.id) || 0)) + 1;
}
async function commit(next) {
  await props.persist(next);
  error.value = "";
}
async function update(next) {
  await commit(props.db.templates.map((t) => t.id === next.id ? next : t));
}
function openForm(source = null) {
  copySource.value = source;
  form.value = { name: source ? source.name + " 副本" : "", size: source ? `${contentOf(source).width}x${contentOf(source).height}` : "1080x1440", categories: copy(source?.categories || []), platforms: copy(source?.platforms || []) };
  formError.value = "";
  creating.value = true;
}
function openEditor(t, layer = null) {
  initialLayer.value = layer;
  editorId.value = t.id;
  updateOperationURL("templates",{templateId:t.id},true);
}
function closeEditor() {
  editorId.value = null;
  initialLayer.value = null;
  updateOperationURL("templates",{},true);
}
async function create() {
  if (busy.value) return;
  busy.value = true;
  formError.value = "";
  try {
    const fields = { ...copy(form.value), id: newId() };
    const t = copySource.value ? cloneTemplate(copySource.value, fields) : createTemplate(fields);
    await commit([...props.db.templates, t]);
    creating.value = false;
    openEditor(t);
    if (copySource.value) feedback.toast.info("已创建新规格草稿，请确认排版并重新校验");
  } catch (e) {
    formError.value = e.message || "创建失败，请重试";
  } finally {
    busy.value = false;
  }
}
async function save(document, preview) {
  const source = props.db.templates.find((t) => t.id === editorId.value);
  await update(saveDraft(source, document, preview));
}
async function toggle(templateId, value) {
  const t = props.db.templates.find((t2) => t2.id === templateId);
  if (switchBusy.value !== null || !t || !isPublished(t)) return;
  const enabled = !!value;
  if (enabled === !!t.enabled) return;
  switchBusy.value = t.id;
  try {
    if (!await feedback.confirmWarning({ title: `${enabled ? "启用" : "停用"}“${t.name}”？`, message: enabled ? "后续新生成将使用当前已发布配置，忽略编辑草稿。" : `停用后不能新选此模板或重新生成，历史任务快照保留。已经确认的 PNG 成品可继续使用，不受模板停用影响。`, confirmText: enabled ? "启用" : "停用" })) return;
    await update(setUsage(t, enabled));
    feedback.toast.success(enabled ? "模板已启用" : "模板已停用");
  } catch (e) {
    error.value = e.message || "操作失败，请重试";
    feedback.toast.error("启停失败，原状态保持不变");
  } finally {
    switchBusy.value = null;
  }
}
async function remove(t) {
  if (!await feedback.confirmWarning({ title: `删除草稿“${t.name}”？`, message: "删除后不可恢复。", confirmText: "删除草稿", danger: true })) return;
  try {
    await commit(props.db.templates.filter((x) => x.id !== t.id));
    feedback.toast.success("草稿已删除");
  } catch (e) {
    error.value = e.message;
  }
}
let checkId = 0;
async function openPublish(t) {
  publishing.value = t.id;
  publishError.value = "";
  publishResult.value = null;
  checking.value = true;
  const n = ++checkId;
  try {
    const result = await checkDocument(copy(t.draft.document), { allSamples: true, businessImage: props.db.images.find((i) => i.enabled && i.uses?.includes("底图"))?.src || "" });
    if (n === checkId) publishResult.value = result;
  } catch (e) {
    if (n === checkId) publishError.value = e.message;
  } finally {
    if (n === checkId) checking.value = false;
  }
}
function closePublish() {
  if (busy.value) return;
  publishing.value = null;
  ++checkId;
  checking.value = false;
}
const publishRow = computed(() => props.db.templates.find((t) => t.id === publishing.value));
async function publish() {
  if (busy.value || checking.value || !publishResult.value || publishResult.value.issues.some((i) => i.level === "error")) return;
  busy.value = true;
  publishError.value = "";
  try {
    const t = publishRow.value;
    await update(publishDraft(t, publishResult.value.image));
    publishing.value = null;
    feedback.toast.success(isPublished(t) ? "更新已发布，启停状态保持不变" : "已发布，当前停用，可通过开关启用");
  } catch (e) {
    publishError.value = e.message || "发布失败，原已发布内容和启停状态不变";
  } finally {
    busy.value = false;
  }
}
function locate(issue) {
  const t = publishRow.value;
  closePublish();
  openEditor(t, issue);
}
function thumb(t) {
  const cached = thumbnails.value[t.id];
  return (cached?.version === t.version ? cached.image : null) || t.preview || (!isPublished(t) ? t.draft?.preview : "");
}
function showPreview(t) {
  previewSrc.value = thumb(t);
  previewTitle.value = `${t.name} · ${isPublished(t) ? "已发布配置" : "草稿样例"}`;
  previewOpen.value = true;
}
function editableHash() {
  const id = readOperationRoute(new URL(location.href)).templateId;
  const t = props.db.templates.find((t2) => t2.id === id);
  if (t && !editorId.value) openEditor(t);
}
onMounted(async () => {
  editableHash();
  for (const t of props.db.templates.filter((t2) => !t2.contentModel && isPublished(t2))) {
    try {
      const result = await checkDocument(contentOf(t));
      if (result.image) thumbnails.value[t.id] = { version: t.version, image: result.image };
    } catch {
    }
  }
});
onBeforeUnmount(() => ++checkId);
</script>
<template>
 <div class="cover-template-manager resource-list-page">
  <div class="breadcrumbs"><span>运营工具 / 封面模板</span></div>
  <ResourcePageHeader title="封面模板"><template #actions><t-button @click="openForm()">新建模板</t-button></template></ResourcePageHeader>
  <ResourceNotice v-if="error" tone="warning">{{error}}；操作未生效，请重试。<t-button variant="text" @click="error=''">关闭提示</t-button></ResourceNotice>
  <ResourceCard class="resource-list-panel">
   <ResourceFilterBar class="operations-tool-filters">
    <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">模板名称</span><t-input v-model="query" placeholder="搜索模板名称" clearable @enter="apply" /></label>
    <label class="wt-filter-field"><span class="wt-filter-field__label">内容状态</span><t-select v-model="content" :options="[{label:'全部',value:''},{label:'草稿',value:'draft'},{label:'已发布',value:'published'}]" /></label>
    <label class="wt-filter-field"><span class="wt-filter-field__label">使用状态</span><t-select v-model="use" :options="[{label:'全部',value:''},{label:'启用',value:'true'},{label:'停用',value:'false'}]" /></label>
    <div class="wt-filter-actions"><t-button @click="apply">查询</t-button><t-button theme="default" variant="outline" @click="reset">重置</t-button></div>
    <ResourceAdvancedToggle v-model="advanced" :count="Number(!!applied.size) + Number(!!applied.scope) + Number(!!applied.platform)" controls="cover-advanced-filters" />
    <div v-if="advanced" id="cover-advanced-filters" class="wt-advanced-filters">
     <label class="wt-filter-field"><span class="wt-filter-field__label">画布规格</span><t-select v-model="size" :options="[{label:'全部',value:''},...sizes]" /></label>
     <label class="wt-filter-field"><span class="wt-filter-field__label">适用范围</span><t-select v-model="scope" :options="[{label:'全部',value:''},...['通用',...categories].map(value=>({label:value,value}))]" /></label>
     <label class="wt-filter-field"><span class="wt-filter-field__label">平台标签</span><t-select v-model="platform" :options="[{label:'全部',value:''},...platforms.map(value=>({label:value,value}))]" /></label>
    </div>
   </ResourceFilterBar>
   <t-table class="resource-data-table" row-key="id" :data="pageRows" :columns="columns" :hover="true" :pagination="{current:currentPage,pageSize,total:rows.length,pageSizeOptions:[10,20,50],showJumper:false}" @page-change="p=>{currentPage=p.current;pageSize=p.pageSize}">
    <template #id="{row:t}"><ResourceId :value="t.id" label="模板ID" /></template>
    <template #preview="{row:t}"><button class="cover-thumbnail" :aria-label="`预览${t.name}`" :disabled="!thumb(t)" @click="showPreview(t)"><img v-if="thumb(t)" :src="thumb(t)" :alt="t.name"/><span v-else>待预览</span></button></template>
    <template #name="{row:t}"><strong class="cover-template-name">{{t.name}}</strong><small v-if="t.platforms?.length">{{t.platforms.join('、')}}</small></template>
    <template #spec="{row:t}"><span>{{dimensions(t)}} px</span><small>{{t.categories?.join('、')||'通用'}}</small></template>
    <template #content="{row:t}"><t-tag size="small" :theme="isPublished(t)?'success':'default'" variant="light">{{isPublished(t)?'已发布':'草稿'}}</t-tag><small v-if="isPublished(t)&&t.draft">有草稿</small></template>
    <template #usage="{row:t}"><t-tooltip :content="isPublished(t)?(t.enabled?'已启用':'已停用'):'请先发布内容'"><t-switch :value="!!t.enabled" :disabled="!isPublished(t)||switchBusy!==null" :loading="switchBusy===t.id" :aria-label="`启用${t.name}`" role="switch" :aria-checked="!!t.enabled" :aria-disabled="!isPublished(t)||switchBusy!==null" :tabindex="isPublished(t)?0:-1" @keydown.enter.prevent="toggle(t.id,!t.enabled)" @keydown.space.prevent="toggle(t.id,!t.enabled)" @change="v=>toggle(t.id,v)" /></t-tooltip></template>
    <template #updated="{row:t}"><span>{{formatDateTime(t.updatedAt)}}</span></template>
    <template #actions="{row:t}"><ResourceRowActions><t-button variant="text" @click="openEditor(t)">编辑</t-button><t-button v-if="t.draft" variant="text" @click="openPublish(t)">{{isPublished(t)?'发布更新':'发布'}}</t-button><t-button variant="text" @click="openForm(t)">复制</t-button><t-button v-if="!isPublished(t)" variant="text" theme="danger" :disabled="!!templateReferences(db,t).length" @click="remove(t)">删除</t-button></ResourceRowActions></template>
    <template #empty><ResourceEmptyState :title="db.templates.length?'没有符合条件的模板':'还没有封面模板'" :description="db.templates.length?'调整筛选条件后重试。':'新建模板，保存设计后从列表发布。'"><t-button v-if="!db.templates.length" @click="openForm()">新建模板</t-button><t-button v-else theme="default" variant="outline" @click="reset">重置筛选</t-button></ResourceEmptyState></template>
   </t-table>
  </ResourceCard>
 </div>
 <Overlay :visible="creating" :title="copySource?'复制模板':'新建模板'" :width="560" :busy="busy" @update:visible="v=>!busy&&(creating=v)">
  <t-form label-align="top" @submit="create">
   <t-form-item :required-mark="true"><template #label>模板名称 <ResourceFieldHelp label="模板名称" content="创建后基础信息固定；需要调整时复制为新模板。" /></template><t-input v-model="form.name" placeholder="请输入模板名称" :maxlength="60" /></t-form-item>
   <t-form-item label="画布规格" :required-mark="true"><t-select v-model="form.size" :options="sizes" /></t-form-item>
   <t-form-item label="适用范围"><t-select v-model="form.categories" multiple clearable placeholder="通用" :options="categories.map(value=>({label:value,value}))" /></t-form-item>
   <t-form-item label="平台标签"><t-select v-model="form.platforms" multiple clearable placeholder="不限平台，可跨平台复用" :options="platforms.map(value=>({label:value,value}))" /></t-form-item>
  </t-form>
  <ResourceNotice v-if="formError" tone="warning">{{formError}}</ResourceNotice>
  <template #footer><t-button theme="default" variant="outline" :disabled="busy" @click="creating=false">取消</t-button><t-button :loading="busy" @click="create">{{copySource?'复制并编辑':'创建并编辑'}}</t-button></template>
 </Overlay>
 <Workspace v-if="row" :key="row.id" ref="workspace" :row="row" :images="db.images" :persist="save" :initial-layer="initialLayer" @close="closeEditor" />
 <Overlay :visible="publishing!==null" :title="isPublished(publishRow||{})?'发布更新':'发布模板'" :width="880" :busy="busy||checking" @update:visible="closePublish">
  <p v-if="publishRow" class="cover-form-hint">{{publishRow.name}} · 已保存草稿。{{isPublished(publishRow)?'发布后保留当前启停状态。':'首次发布后保持停用。'}}</p>
  <div class="cover-check-heading">发布前校验 <ResourceFieldHelp label="发布前校验" content="使用精简、较长样例校验同一个动态标题及当前设计。当前校验在本机完成，正式服务校验结果以接入后为准。" /></div>
  <t-loading :loading="checking"><div class="cover-publish-grid"><div class="cover-publish-image"><img v-if="publishResult?.image" :src="publishResult.image" alt="待发布草稿预览"/><span v-else>{{checking?'正在校验…':'修复问题后可生成预览'}}</span></div><div><p v-if="publishResult&&!publishResult.issues.length">校验通过，可以发布。</p><div v-for="(issue,i) in publishResult?.issues||[]" :key="i" class="cover-publish-issue"><strong>{{issue.level==='error'?'需修复':'提醒'}} · {{issue.layerName}}</strong><p>{{issue.message}}</p><t-button variant="text" @click="locate(issue)">定位修改</t-button></div></div></div></t-loading>
  <ResourceNotice v-if="publishError" tone="warning">{{publishError}}；原已发布内容及启停状态保持不变。</ResourceNotice>
  <template #footer><t-button theme="default" variant="outline" :disabled="busy||checking" @click="closePublish">取消</t-button><t-button :loading="busy||checking" :disabled="!publishResult||publishResult.issues.some(i=>i.level==='error')" @click="publish">确认发布</t-button></template>
 </Overlay>
 <MediaPreview v-model:visible="previewOpen" :title="previewTitle" :images="[{src:previewSrc,label:previewTitle}]" />
</template>
<style scoped>
.cover-template-manager {min-width:0;}.cover-template-manager :deep(.resource-page-header){margin-bottom:20px;}.cover-thumbnail{width:96px;height:104px;padding:4px;display:flex;align-items:center;justify-content:center;background:var(--wt-bg-page);border:1px solid var(--wt-border);border-radius:4px;cursor:pointer;font-size:12px;color:var(--wt-text-tertiary);}.cover-thumbnail:disabled{cursor:default;}.cover-thumbnail img{max-width:100%;max-height:100%;object-fit:contain;}.cover-template-name{font-weight:400;overflow-wrap:anywhere;}.cover-template-manager small{display:block;margin-top:6px;font-size:12px;color:var(--wt-text-tertiary);}.cover-form-hint{font-size:13px;line-height:1.6;color:var(--wt-text-tertiary);margin:12px 0 0;}.cover-publish-grid{display:grid;grid-template-columns:280px 1fr;gap:24px;min-height:300px;max-height:55vh;overflow:auto;margin-top:20px;}.cover-publish-image{background:var(--wt-bg-page);display:flex;align-items:center;justify-content:center;padding:16px;font-size:13px;color:var(--wt-text-tertiary);}.cover-publish-image img{max-width:100%;max-height:380px;object-fit:contain;}.cover-publish-issue{border-bottom:1px solid var(--wt-border);padding-bottom:12px;margin-bottom:12px;font-size:13px;}.cover-publish-issue p{margin:8px 0;}
.cover-check-heading{display:flex;align-items:center;gap:8px;margin-top:16px;font-size:13px;font-weight:500;}
@media(max-width:680px){.cover-publish-grid{grid-template-columns:minmax(0,1fr);gap:16px;}.cover-publish-image{max-height:300px;}}
</style>
