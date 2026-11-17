<script setup>
import { computed, nextTick, ref, watch } from "vue";
import {
  StarIcon as SparklesIcon,
} from "tdesign-icons-vue-next";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../src/shared/ui/resource/ResourceFilterBar.vue";
import "../../../src/shared/styles/resource-data-table.css";
import "../../../src/shared/styles/resource-list.css";
import ResourceFieldHelp from "../../../src/shared/ui/resource/ResourceFieldHelp.vue";
import ResourceOverlay from "./PrototypeOverlay.vue";
import BatchContentTable from "./BatchContentTable.vue";
import BatchConfiguration from "./BatchConfiguration.vue";
import { contentState, fillSuggestedBlanks, confirmContent, suggestionFields } from "./batchContentActions.js";
import {
  initCandidate,
  editField,
  requestSuggestion,
  receiveSuggestion,
  adopt,
  demoValues,
  invalidateCover,
} from "./content.js";
import { ensureCoverResults, coverSpecs, coverSignature, coverTextFieldsForRow, isTemplateCover, inspectCoverImage, coverReadinessIssues } from "./covers.js";
import { validate, declarationOptions } from "./model.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
const props = defineProps({
  rows: { type: Array, required: true },
  db: { type: Object, required: true },
});
const emit = defineEmits(["preview", "working"]);
const clone = (v) => JSON.parse(JSON.stringify(v));
const labels = {
  title: "发布标题",
  shortTitle: "封面短标题",
  description: "简介 / 正文",
  topics: "话题标签",
  tags: "平台标签",
  subTitle: "封面短标语",
};
const fieldKeys = Object.keys(labels);
const selected = ref([]),
  category = ref("全部"),
  status = ref("全部"),
  search = ref(""),
  applied = ref({ search: "", category: "全部", status: "全部" }),
  page = ref(1),
  pageSize = ref(10),
  error = ref(""),
  notice = ref("");
const configurationOpen=ref(false),approving=ref(false);
const aiIds = ref([]), aiGenerated = ref(false);
const aiOpen = ref(false),
  aiScope = ref("筛选结果"),
  aiFields = ref([...fieldKeys]),
  style = ref("实战攻略"),
  busy = ref(false),
  editOpen = ref(false),
  editing = ref(null),
  editOriginal = ref(""),
  saving = ref(false),
  editError = ref("");
const editAlert = ref(null);
watch(editError, async message => {
  if (!message) return;
  await nextTick();
  editAlert.value?.$el?.scrollIntoView({ block: "nearest" });
});
const categories = computed(() => [
  ...new Set(props.rows.map((r) => r.category)),
]);
const account = (id) => props.db.accounts.find((a) => a.id === id);
const asset = (id) => props.db.assets.find((v) => v.id === id);
const hasSuggestion = (r) =>
  Object.values(r.ai?.suggestions || {}).some(
    (s) => s.status === "待采用" && !s.error && !s.expired,
  );
const rowStatus = row=>contentState(row,props.db);
const filtered = computed(() => props.rows.filter((row) => {
  const filter = applied.value;
  const text = `${row.title || ""} ${row.description || ""} ${row.category} ${account(row.accountId)?.name || ""} ${asset(row.assetId)?.title || ""} V-${row.assetId}`.toLowerCase();
  return (filter.category === "全部" || filter.category === row.category) &&
    (filter.status === "全部" || filter.status === rowStatus(row)) &&
    text.includes(filter.search.trim().toLowerCase());
}));
function query() {
  applied.value = { search: search.value, category: category.value, status: status.value };
  page.value = 1;
}
function resetFilters() {
  search.value = ""; category.value = "全部"; status.value = "全部"; query();
}
watch(() => filtered.value.length, (length) => {
  page.value = Math.min(page.value, Math.max(1, Math.ceil(length / pageSize.value)));
});
const contextSummary = computed(() => `${props.rows[0]?.platform || ""} · ${categories.value.map(category => `${category} ${props.rows.filter(row => row.category === category).length} 条`).join("、")} · 共 ${props.rows.length} 条候选`);
const selectedRows = computed(() => props.rows.filter(row => selected.value.includes(row.candidateId)));
watch(
  () => props.rows,
  (rows) => {
    rows.forEach(initCandidate);
    selected.value = selected.value.filter((id) =>
      rows.some((r) => r.candidateId === id),
    );
  },
  { immediate: true, deep: false },
);
function scopeRows() {
  return aiScope.value === "当前任务" ? [editing.value].filter(Boolean) : props.rows.filter(row=>aiIds.value.includes(row.candidateId));
}
function openAI(single = false) {
  if (!single && !selectedRows.value.length) return;
  aiScope.value = single ? "当前任务" : "勾选任务";
  aiIds.value=selectedRows.value.map(row=>row.candidateId);
  aiGenerated.value=false;
  aiFields.value=availableAIFields.value;
  aiOpen.value = true;
  error.value = "";
}
const availableAIFields=computed(()=>[...new Set(scopeRows().flatMap(suggestionFields))]);
async function generate() {
  const rows = scopeRows();
  if (!rows.length || !aiFields.value.length) {
    error.value = "请选择至少一条任务与一个生成字段。";
    return;
  }
  busy.value = true;
  error.value = "";
  await nextTick();
  // Local suggestions are labelled as demonstrations and never call a model service.
  for (const row of rows) {
    const req = requestSuggestion(row, aiFields.value.filter(field=>suggestionFields(row).includes(field)));
    const values = {
      ...demoValues(row, style.value),
      subTitle: "实战记录 · 进阶思路",
    };
    receiveSuggestion(row, req, values);
    row.ai.progress = "演示建议已生成";
  }
  busy.value = false;
  aiGenerated.value = true;
}
function adoptField(row, field) {
  if (!suggestionFields(row).includes(field)) return;
  if (adopt(row, field)) {
    row.contentVersion = (row.contentVersion || 1) + 1;
    feedback.toast.success(`已采用${labels[field]}`);
  } else if(aiOpen.value) error.value = "建议已过期或已采用，请重新生成当前字段。";
  else editError.value = "建议已过期或已采用，请重新生成当前字段。";
}
const emptySuggestions = computed(()=>scopeRows().reduce((n,row)=>n+aiFields.value.filter(f=>suggestionFields(row).includes(f) && !String(row[f]||'').trim() && row.ai?.suggestions?.[f]?.status==='待采用' && !row.ai.suggestions[f].expired && !row.ai.suggestions[f].error).length,0));
function adoptBatch() {
  const n=fillSuggestedBlanks(scopeRows(),aiFields.value);
  aiOpen.value=false;feedback.toast.success(`已补齐 ${n} 个空白字段`);
}
function edit(row) {
  if(approving.value) return;
  editing.value = clone(row);
  editOriginal.value = JSON.stringify(editing.value);
  editError.value = "";
  editOpen.value = true;
}
async function closeEdit() {
  if (saving.value) return;
  if (
    JSON.stringify(editing.value) !== editOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃本条未保存修改？",
      message: "当前采用版本和其他任务保持不变。",
      confirmText: "放弃修改",
    }))
  )
    return;
  editOpen.value = false;
}
function setCoverMode(value) {
  editField(editing.value, "coverMode", value);
  if (value !== "模板合成") {setImage(0);return;}
  if (!editing.value.imageId) {
    const background = props.db.images.find(
      (i) => i.enabled && i.pureBackground && i.uses.includes("底图"),
    );
    if (background) setImage(background.id);
  }
  if (!editing.value.templateId) {
    const spec = coverSpecs(editing.value.platform)[0];
    const template = props.db.templates.find(
      (t) =>
        t.enabled &&
        (!spec ||
          Math.abs(
            t.document.width / t.document.height - spec.width / spec.height,
          ) < 0.01),
    );
    if (template) setTemplate(template.id);
  }
}
function setTemplate(id) {
  const row = editing.value,
    t = props.db.templates.find((t) => t.id === id);
  editField(row, "templateId", t?.id || null);
  row.templateVersion = t?.version || 1;
  row.templateDocument = t ? clone(t.document) : null;
  row.templateDocuments = {};
  row.templateSnapshots = {};
  for (const spec of coverSpecs(row.platform)) {
    const candidate = [t, ...props.db.templates].find(
      (t) =>
        t?.enabled &&
        Math.abs(
          t.document.width / t.document.height - spec.width / spec.height,
        ) < 0.01,
    );
    if (candidate) {
      row.templateDocuments[spec.key] = clone(candidate.document);
      row.templateSnapshots[spec.key] = {
        id: candidate.id,
        version: candidate.version || 1,
      };
    }
  }
  invalidateCover(row);
}
function setImage(id) {
  const image = props.db.images.find((i) => i.id === id);
  if (image) {
    editField(editing.value, "coverSource", image.src);
    editing.value.imageId = image.id;
    editing.value.imageVersion = image.version || 1;
  } else {
    editField(
      editing.value,
      "coverSource",
      asset(editing.value.assetId)?.cover,
    );
    editing.value.imageId = null;
  }
  if (!isTemplateCover(editing.value)) {
    editing.value.cover=editing.value.coverSource;editing.value.coverVariants={};editing.value.coverSignature=null;
    editing.value.coverError='';editing.value.coverRender='已选用';editing.value.coverApproved=false;
  }
}
const usableImages = computed(() =>
  props.db.images.filter(
    (i) =>
      i.enabled &&
      i.uses.includes(
        editing.value?.coverMode === "模板合成" ? "底图" : "现成封面",
      ),
  ),
);
const templateFields = computed(() => editing.value ? coverTextFieldsForRow(editing.value) : []);
const editorSuggestions = computed(() => Object.entries(editing.value?.ai?.suggestions || {}).filter(([field, suggestion]) => suggestionFields(editing.value).includes(field) && suggestion.status === "待采用"));
const usableTemplates = computed(() =>
  props.db.templates.filter((t) => t.enabled),
);
async function renderEdit() {
  saving.value = true;
  editError.value = "";
  try {
    await ensureCoverResults(editing.value);
  } catch (e) {
    editError.value = e.message;
  } finally {
    saving.value = false;
  }
}
async function saveEdit() {
  saving.value = true;
  editError.value = "";
  try {
    const row = editing.value, requested=JSON.stringify(row);
    if (!isTemplateCover(row)) await inspectCoverImage(row.cover);
    if(JSON.stringify(row)!==requested) {editError.value='内容已发生变化，请检查后重新保存';return;}
    const source=props.rows.find(item=>item.candidateId===row.candidateId);
    if(!source || JSON.stringify(source)!==editOriginal.value) {editError.value='原内容已变化，请关闭后重新编辑';return;}
    if(!isTemplateCover(row))row.coverError='';
    const { errors } = confirmContent(row, props.db);
    if (errors.length) {
      editError.value = errors.join("；");
      return;
    }
    row.contentApproved = true;
    row.coverApproved = true;
    const current = props.rows.find((r) => r.candidateId === row.candidateId);
    if (!current) {
      editError.value = "该候选任务已移出，请关闭后重新选择。";
      return;
    }
    row.contentVersion = (current.contentVersion || 1) + 1;
    row.overrides = [
      ...new Set([
        ...(current.overrides || []),
        ...[
          ...fieldKeys,
          "declaration",
          "visibility",
          "schedule",
          "partition",
        ].filter((f) => current[f] !== row[f]),
      ]),
    ];
    Object.assign(current, clone(row));
    editOpen.value = false;
    feedback.toast.success("已保存");
  } catch (e) {
    editError.value = e.message || "保存失败，当前输入已保留，可修复后重试。";
  } finally {
    saving.value = false;
  }
}
async function approveBatch() {
  if(approving.value)return;
  const rows=selectedRows.value.filter(row=>contentState(row,props.db)!=='已就绪');
  if(!rows.length)return;
  const snapshots=new Map(rows.map(row=>[row.candidateId,JSON.stringify(row)]));
  approving.value=true;emit('working',true);
  try {
    if(!(await feedback.confirmWarning({title:`确认 ${rows.length} 条内容`,message:'确认已检查所选文案及实际使用的封面。通过校验后标记已就绪，后续修改需重新确认。',confirmText:'确认内容'})))return;
    const failures=[];let n=0;
    for(const row of rows) {
      let imageError='';
      if(!isTemplateCover(row)){try{await inspectCoverImage(row.cover);}catch(e){imageError=e.message;}}
      if(!props.rows.includes(row) || JSON.stringify(row)!==snapshots.get(row.candidateId)) {failures.push('内容已发生变化，请检查后重新确认');continue;}
      if(!isTemplateCover(row))row.coverError=imageError;
      const result=imageError?{ok:false,errors:[imageError]}:confirmContent(row,props.db);
      if(result.ok)n++;else failures.push(result.errors[0]);
    }
    if(!failures.length)feedback.toast.success(`${n} 条内容已确认`);
    else error.value=`${n} 条已确认，${failures.length} 条未确认：${[...new Set(failures)].join('；')}`;
  }finally{approving.value=false;emit('working',false);}
}

defineExpose({ edit });
</script>
<template>
  <section class="batch-content-workspace resource-list-page">
    <ResourceCard class="resource-list-panel batch-content-panel">
      <header class="batch-content-panel-header">
        <div class="batch-content-title">
          <h3>内容配置 <ResourceFieldHelp label="内容配置" content="检查文案准确性、实际封面效果及创作声明。确认内容会校验必填项、平台限制和资源可用性，并标记当前版本已就绪；创建任务及执行发布仍需后续操作。" /></h3>
          <span>{{ contextSummary }}</span>
        </div>

      </header>
      <t-alert v-if="error" theme="warning" :message="error" close-btn @close="error = ''" class="batch-content-message" />
      <t-alert v-if="notice" theme="info" :message="notice" close-btn @close="notice = ''" class="batch-content-message" />
      <ResourceFilterBar>
        <label class="wt-filter-field wt-filter-field--wide batch-content-search">
          <span class="wt-filter-field__label">发布内容 / 账号 / 成片</span>
          <t-input v-model="search" aria-label="搜索发布内容" placeholder="输入标题、账号或成片编号" clearable @enter="query" />
        </label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">类目</span>
          <t-select v-model="category" aria-label="筛选类目" :options="['全部', ...categories].map(value => ({ label: value, value }))" />
        </label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">配置状态</span>
          <t-select v-model="status" aria-label="筛选配置状态" :options="['全部', '待完善', '待确认', '已就绪'].map(value => ({ label: value, value }))" />
        </label>
        <div class="wt-filter-actions"><t-button @click="query">查询</t-button><t-button variant="outline" theme="default" @click="resetFilters">重置</t-button></div>
      </ResourceFilterBar>
      <div v-if="selected.length" class="batch-content-toolbar resource-form-actions">
        <span class="batch-content-selection">已选 <b>{{ selected.length }}</b> 条</span>
        <t-button variant="outline" theme="default" :disabled="approving" @click="configurationOpen=true">批量配置</t-button>
        <t-button variant="outline" theme="default" :loading="approving" :disabled="approving || selectedRows.every(row=>rowStatus(row)==='已就绪')" @click="approveBatch">确认内容</t-button>
        <t-button v-if="selected.length" variant="text" @click="selected = []">清空选择</t-button>
        <span class="batch-content-total">共 {{ filtered.length }} 条</span>
      </div>
      <BatchContentTable :rows="filtered" :db="db" selectable :selected="selected"
        :pagination="{current:page,pageSize,pageSizeOptions:[10,20,50],total:filtered.length,showJumper:false}"
        @select-change="keys=>selected=keys" @page-change="value=>{page=value.current;pageSize=value.pageSize}"
        @edit="edit" @preview="emit('preview',$event)">
        <template #empty>没有符合条件的内容<t-button variant="text" @click="resetFilters">重置筛选</t-button></template>
      </BatchContentTable>
    </ResourceCard>
  </section>
  <BatchConfiguration v-model:visible="configurationOpen" :rows="rows" :ids="selected" :db="db" @preview="emit('preview',$event)" @working="value=>emit('working',value)" />
  <ResourceOverlay
    v-model:visible="aiOpen"
    title="AI 生成"
    :width="800"
    :busy="busy"
    ><div class="batch-ai-dialog">
      <t-tag theme="primary" variant="light">本地演示</t-tag>
      <p class="batch-ai-scope">{{ aiScope === '当前任务' ? '当前编辑内容 · 1 条' : `已选 ${scopeRows().length} 条` }}</p>
      <label>生成字段</label
      ><t-checkbox-group v-model="aiFields" class="batch-ai-fields"
        ><t-checkbox v-for="f in availableAIFields" :key="f" :value="f">{{
          labels[f]
        }}</t-checkbox></t-checkbox-group
      ><label
        >文案风格<t-select
          :popup-props="{ attach: 'body', zIndex: 5500 }"
          v-model="style"
          :options="
            ['实战攻略', '简洁'].map((s) => ({ label: s, value: s }))
          " /></label
      ><t-alert v-if="error" theme="warning" :message="error" />
      <div v-if="aiGenerated" class="batch-ai-results">
        <details v-for="(row,index) in scopeRows()" :key="row.candidateId" :open="index===0"><summary>{{asset(row.assetId)?.title}}</summary>
          <div v-for="field in aiFields.filter(field=>suggestionFields(row).includes(field))" :key="field" class="batch-ai-result-field"><strong>{{labels[field]}}</strong><p>{{row.ai?.suggestions?.[field]?.value || '暂无建议'}}</p><small v-if="row[field]">当前：{{row[field]}}</small><t-button variant="text" theme="primary" size="small" :disabled="row.ai?.suggestions?.[field]?.status!=='待采用' || row.ai?.suggestions?.[field]?.expired" @click="adoptField(row,field)">{{row[field]?'替换此字段':'采用此字段'}}</t-button></div>
        </details>
      </div>
    </div>
    <template #footer
      ><t-button variant="outline" @click="aiOpen = false">取消</t-button
      ><t-button
        :variant="aiGenerated ? 'outline' : 'base'" :theme="aiGenerated ? 'default' : 'primary'"
        :loading="busy"
        :disabled="!scopeRows().length || !aiFields.length"
        @click="generate"
        >{{aiGenerated ? '重新生成' : '生成建议'}}</t-button>
      <t-button v-if="aiGenerated" :disabled="!emptySuggestions" @click="adoptBatch">补齐空白</t-button>
      </template
    ></ResourceOverlay
  >
  <ResourceOverlay :visible="editOpen" @update:visible="value => { if (!value) closeEdit(); }" title="编辑发布内容" :width="960" :busy="saving">
    <div v-if="editing" class="batch-content-editor">
      <div class="batch-content-source"><div><small>发布账号</small><strong>{{account(editing.accountId)?.name}}</strong><span>{{editing.platform}}</span></div><div><small>发布成片</small><strong>{{asset(editing.assetId)?.title}}</strong><span>V-{{editing.assetId}} · {{editing.category}}</span></div></div>
      <t-alert ref="editAlert" role="alert" v-if="editError" theme="warning" :message="editError" class="batch-content-message" />
      <div class="batch-content-form">
        <div class="batch-content-form-heading"><h4>发布文案</h4><t-button variant="text" size="small" @click="openAI(true)"><template #icon><SparklesIcon /></template>AI推荐</t-button></div>
        <label v-if="editing.platform === '百家号'" class="batch-content-field"><span><em>*</em> 作品描述</span><t-textarea :value="editing.description || ''" :maxlength="50" :autosize="{ minRows: 2, maxRows: 4 }" aria-label="作品描述" @change="value => editField(editing, 'description', value)" /></label>
        <label v-else class="batch-content-field"><span><em>*</em> 发布标题</span><t-input :value="editing.title || ''" :maxlength="editing.platform === 'B站' ? 80 : 20" show-limit-number aria-label="发布标题" @change="value => editField(editing, 'title', value)" /></label>
        <label v-if="editing.platform !== '百家号'" class="batch-content-field"><span>{{ editing.platform === 'B站' ? '简介' : '正文' }}</span><t-textarea :value="editing.description || ''" :maxlength="editing.platform === 'B站' ? 2000 : 1000" :autosize="{ minRows: 3, maxRows: 6 }" aria-label="简介 / 正文" @change="value => editField(editing, 'description', value)" /></label>
        <label v-if="editing.platform === 'B站'" class="batch-content-field"><span><em>*</em> 平台标签 <ResourceFieldHelp label="平台标签" content="使用与内容相关的标签，以顿号分隔。" /></span><t-input :value="editing.tags || ''" aria-label="平台标签" @change="value => editField(editing, 'tags', value)" /></label>
        <label v-else class="batch-content-field"><span>话题</span><t-input :value="editing.platform === '百家号' ? editing.tags : editing.topics" aria-label="话题" @change="value => editField(editing, editing.platform === '百家号' ? 'tags' : 'topics', value)" /></label>
        <section class="batch-content-cover-section">
          <h4><em>*</em> 发布封面 <ResourceFieldHelp label="发布封面" :content="editing.coverMode === '模板合成' ? '使用模板文字槽位与平台比例生成实际图片。文字或底图修改后，需要重新生成并确认。' : '替换图片保留原图文字，发布标题不会自动写入图片。'" /></h4>
          <t-radio-group :value="editing.coverMode" variant="default-filled" aria-label="封面方式" @change="setCoverMode"><t-radio-button value="现成封面">替换图片</t-radio-button><t-radio-button value="模板合成">使用模板</t-radio-button></t-radio-group>
          <label v-if="editing.coverMode === '模板合成'" class="batch-content-field"><span><em>*</em> 封面模板</span><t-select :popup-props="{ attach: 'body', zIndex: 5500 }" :value="editing.templateId" :options="usableTemplates.map(template => ({ label: template.name, value: template.id }))" placeholder="请选择模板" aria-label="封面模板" @change="setTemplate" /></label>
          <label class="batch-content-field"><span>{{ editing.coverMode === '模板合成' ? '封面底图' : '封面图片' }}</span><t-select :popup-props="{ attach: 'body', zIndex: 5500 }" :value="editing.imageId || 0" :options="[{ label: '成片自带封面', value: 0 }, ...usableImages.map(image => ({ label: image.name, value: image.id }))]" aria-label="封面图片" @change="setImage" /></label>
          <label v-for="field in editing.coverMode === '模板合成' ? templateFields : []" :key="field.key" class="batch-content-field"><span><em>*</em> {{ field.label }}</span><t-input :value="editing[field.key] || ''" :aria-label="field.label" @change="value => editField(editing, field.key, value)" /></label>
          <div v-if="isTemplateCover(editing)" class="batch-content-render"><t-button variant="outline" theme="default" :loading="saving" @click="renderEdit">生成封面</t-button></div>
          <div class="batch-content-preview">
            <span class="batch-content-preview-label">实际封面</span>
            <button class="batch-content-cover-preview" aria-label="放大封面" type="button" @click="emit('preview', { kind: 'cover', row: editing })"><img :src="editing.cover" alt="当前实际封面" /></button>
            <div class="batch-content-cover-status"><t-tag size="small" :theme="editing.coverApproved && !coverReadinessIssues(editing).length ? 'success':'default'" variant="light">{{coverReadinessIssues(editing).length ? (isTemplateCover(editing)?'需生成封面':'图片待完善') : editing.coverApproved ? '已确认' : isTemplateCover(editing)?'已生成，待确认':'已选用，待确认'}}</t-tag></div>
          </div>
        </section>
        <section v-if="editorSuggestions.length" class="batch-content-suggestions"><h4>AI 演示建议</h4><div v-for="[field, suggestion] in editorSuggestions" :key="field" class="batch-content-suggestion"><span>{{ labels[field] }}</span><p>{{ suggestion.value || suggestion.error || '暂无可用建议' }}</p><t-button variant="text" size="small" :disabled="!!suggestion.expired || !!suggestion.error" @click="adoptField(editing, field)">{{ suggestion.expired ? '建议已过期' : '采用此字段' }}</t-button></div></section>
        <section class="batch-content-platform-section"><h4>平台设置</h4>
          <div class="batch-content-platform-grid">
            <label class="batch-content-field"><span><em>*</em> 创作声明</span><t-select :popup-props="{ attach: 'body', zIndex: 5500 }" :value="editing.declaration" :options="declarationOptions(editing.platform)" placeholder="按内容事实选择" aria-label="创作声明" @change="value => editField(editing, 'declaration', value)" /></label>
            <label v-if="editing.platform === 'B站'" class="batch-content-field"><span><em>*</em> 分区</span><t-select :popup-props="{ attach: 'body', zIndex: 5500 }" :value="editing.partition" :options="[{ label: '游戏', value: '游戏' }]" aria-label="分区" @change="value => editField(editing, 'partition', value)" /></label>
            <label v-if="editing.platform !== '百家号'" class="batch-content-field"><span>可见范围</span><t-select :popup-props="{ attach: 'body', zIndex: 5500 }" :value="editing.visibility" :options="['公开', '仅自己可见'].map(value => ({ label: value, value }))" aria-label="可见范围" @change="value => editField(editing, 'visibility', value)" /></label>
          </div>
        </section>
      </div>
    </div>
    <template #footer><t-button variant="outline" theme="default" :disabled="saving" @click="closeEdit">取消</t-button><t-button :loading="saving" @click="saveEdit">保存</t-button></template>
  </ResourceOverlay>

</template>

<style scoped>
.batch-content-workspace { min-width: 0; margin-top: 16px; color: var(--wt-color-text-primary); }
.batch-content-panel-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px 16px; padding: 20px 24px 0; }
.batch-content-title { display: flex; align-items: baseline; flex-wrap: wrap; gap: 8px 16px; min-width: 0; }
.batch-content-title h3 { display: flex; align-items: center; gap: 6px; margin: 0; font-size: 16px; line-height: 24px; font-weight: 500; }
.batch-content-title > span { color: var(--wt-color-text-secondary); font-size: 12px; line-height: 20px; }
.batch-content-panel .batch-content-search { flex: 1 1 280px !important; max-width: none !important; }
.batch-content-message { margin: 16px 24px 0; }
.batch-content-editor > .batch-content-message { margin: 0; }
.batch-content-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 0 24px 16px; }
.batch-content-selection { font-size: 13px; color: var(--wt-color-text-secondary); margin-right: 8px; }
.batch-content-selection b { color: var(--wt-color-primary); font-weight: 500; }
.batch-content-total { margin-left: auto; color: var(--wt-color-text-secondary); font-size: 12px; }
.batch-content-table { isolation: isolate; }
.batch-content-identity { display: flex; align-items: center; gap: 12px; min-width: 0; }
.batch-content-identity > img { width: 48px; height: 60px; object-fit: cover; border-radius: 4px; flex: none; border: 1px solid var(--wt-color-border); }
.batch-content-identity > div { min-width: 0; }
.batch-content-cell-title, .batch-content-cell-summary { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; overflow-wrap: anywhere; }
.batch-content-cell-title { font-weight: 500; }
.batch-content-cell-ellipsis { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.batch-content-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; margin-top: 4px; }
.batch-content-meta small { margin: 0 !important; }
.batch-content-muted { color: var(--wt-color-text-tertiary); font-size: 12px; }
.batch-content-empty { display: grid; justify-items: center; padding: 24px; gap: 8px; }
.batch-content-source { display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px 24px;padding:16px;background:var(--wt-color-bg-subtle);border-radius:6px; }
.batch-content-source > div { display:flex;flex-direction:column;gap:4px;min-width:0; }
.batch-content-source strong {font-size:14px;font-weight:500;overflow-wrap:anywhere;}
.batch-content-source small,.batch-content-source span{font-size:12px;color:var(--wt-color-text-secondary);}
.batch-content-editor, .batch-content-form { display: flex; flex-direction: column; gap: 24px; min-width: 0; }
.batch-content-editor h4 { margin: 0; display: flex; align-items: center; gap: 6px; font-size: 14px; font-weight: 500; line-height: 22px; color: var(--wt-color-text-primary); }
.batch-content-form-heading { display: flex; justify-content: space-between; align-items: center; margin-bottom: -8px; }
.batch-content-field { display: flex; flex-direction: column; gap: 8px; min-width: 0; font-size: 13px; line-height: 20px; color: var(--wt-color-text-primary); }
.batch-content-field > span { font-weight: 500; }
.batch-content-editor em, .batch-ai-dialog em { color: var(--wt-color-danger); font-style: normal; }
.batch-content-field small, .batch-content-suggestions h4 small { color: var(--wt-color-text-secondary); font-size: 12px; font-weight: 400; }
.batch-content-field span small { margin-left: 6px; }
.batch-content-cover-section { display: flex; flex-direction: column; align-items: stretch; gap: 16px; }
.batch-content-cover-section :deep(.t-radio-group) { align-self: flex-start; }
.batch-content-render { display: flex; }
.batch-content-preview { display: flex; flex-direction: column; align-items: stretch; gap: 12px; }
.batch-content-preview-label { font-size: 13px; line-height: 20px; color: var(--wt-color-text-secondary); }
.batch-content-cover-preview { display:flex;align-items:center;justify-content:center;width:100%;height:240px;padding:12px;box-sizing:border-box;overflow:hidden;border:1px solid var(--wt-color-border);border-radius:6px;background:var(--wt-color-bg-subtle);cursor:zoom-in; }
.batch-content-cover-preview img {display:block;max-width:100%;max-height:100%;width:auto;height:auto;object-fit:contain;}
.batch-content-cover-status { display: flex; align-items: center; gap: 8px; color: var(--wt-color-text-secondary); font-size: 12px; }
.batch-content-platform-section { padding-top: 0; }
.batch-content-platform-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px 24px; margin-top: 16px; }
.batch-content-footer-note { margin-right: auto; font-size: 12px; line-height: 20px; color: var(--wt-color-text-secondary); }
.batch-content-suggestions { display: grid; gap: 12px; padding: 16px; border: 1px solid var(--wt-color-border); border-radius: 4px; background: var(--wt-color-bg-subtle); }
.batch-content-suggestion { display: grid; grid-template-columns: 90px minmax(0, 1fr) auto; gap: 8px; align-items: start; font-size: 13px; }
.batch-content-suggestion p { margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; }
.batch-ai-dialog { display: flex; flex-direction: column; gap: 16px; }
.batch-ai-dialog p { margin: 0; }
.batch-ai-dialog > label { margin: 0; display: flex; flex-direction: column; gap: 8px; color: var(--wt-color-text-primary); font-size: 13px; font-weight: 500; }
.batch-bulk-edit-dialog > :deep(.t-checkbox) { flex-direction: row; justify-content: flex-start; align-items: center; gap: 0; margin: 0; align-self: flex-start; font-weight: 400; }
.batch-ai-dialog > :deep(.t-tag) { align-self: flex-start; }
.batch-inline-note { font-size: 12px; line-height: 20px; color: var(--wt-color-text-secondary); }
@media (max-width: 680px) {
  .batch-content-panel-header { padding: 20px 16px 0; }
  .batch-content-toolbar { padding-inline: 16px; }
  .batch-content-message { margin-inline: 16px; }
  .batch-content-source { grid-template-columns:minmax(0,1fr);padding:12px; }

  .batch-content-source > .t-tag { margin-left: 0; }
  .batch-content-platform-grid { grid-template-columns: minmax(0, 1fr); }
  .batch-content-footer-note { flex-basis: 100%; }
  .batch-content-suggestion { grid-template-columns: minmax(0, 1fr); }
}

.batch-ai-results{max-height:300px;overflow:auto;border:1px solid var(--wt-color-border);border-radius:6px;padding:0 16px;}
.batch-ai-results details{padding:12px 0;}.batch-ai-results details+details{border-top:1px solid var(--wt-color-border)}.batch-ai-results summary{cursor:pointer;font-size:14px;font-weight:500;}.batch-ai-result-field{display:grid;grid-template-columns:100px minmax(0,1fr) auto;gap:6px 12px;padding-top:12px;font-size:13px;}.batch-ai-result-field strong{font-weight:500;}.batch-ai-result-field p{overflow-wrap:anywhere;}.batch-ai-result-field small{grid-column:2;color:var(--wt-color-text-secondary);font-size:12px;}.batch-ai-result-field .t-button{grid-column:3;grid-row:1 / span 2;}@media(max-width:680px){.batch-ai-result-field{grid-template-columns:minmax(0,1fr)}.batch-ai-result-field small,.batch-ai-result-field .t-button{grid-column:1;grid-row:auto}}
</style>
