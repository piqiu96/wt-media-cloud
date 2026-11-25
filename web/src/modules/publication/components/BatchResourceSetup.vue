<script setup>
import { computed, ref, watch } from "vue";
import { createPublicationBatchClient, createPublicationClient } from "../../../shared/api/publication.js";
import BatchCandidateWorkspace from "./BatchCandidateWorkspace.vue";
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue";
import { MoveIcon, RefreshIcon } from "tdesign-icons-vue-next";
import BatchSetupSection from "./BatchSetupSection.vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceReorderActions from "../../../shared/ui/resource/ResourceReorderActions.vue";
import { formatPublicationTime } from "../dateTime.js";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import PublishingResourcePicker from "./PublishingResourcePicker.vue";
import { batchAccountName, batchOutputID, batchOutputName, batchOutputProblem } from "../useBatchResourceCatalog.js";
import { batchInput, candidateShortfalls, displayAvailability, moveEntry, reconcileAllocation, snapshot } from "../batchPreparation.js";

const props = defineProps({
  form: { type: Object, required: true },
  catalog: { type: Object, required: true },
  categories: { type: Array, required: true },
  actor: { type: Object, required: true },
  disabled: Boolean,
  inheritedTargets: Boolean,
  recipes: Object,
  coverFiles: Object,
  validationContext: Object,
});
const emit = defineEmits(["allocated", "work"]);
const batch = createPublicationBatchClient();
const publication = createPublicationClient();
const picker = ref("");
const outputCategory = ref("");
const inventory = ref({});
const error = ref("");
const loading = ref(false);
const resourceLoading = ref(false);
const outcome = ref("");
const poolOpen=ref(false);
let inventorySequence = 0;
const automaticResourceLimit = 1000;

const shortfalls = computed(() => candidateShortfalls(props.form));
const nameOf = id => props.categories.find(item => item.value === id)?.label || "原类目暂不可用";
const accounts = computed(() => props.form.config.accounts);
const outputs = computed(() => props.form.config.output_ids);
const fixed = computed(() => props.disabled || loading.value);

function showResources(kind, categoryId) {
  outputCategory.value = categoryId;
  picker.value = kind;
}
function dropTarget(event, index) {
  if (fixed.value || props.inheritedTargets) return;
  try {
    const data = JSON.parse(event.dataTransfer.getData('application/x-wt-resource-reorder'));
    if (data.scope !== 'batch-targets') return;
    const from = props.form.targets.findIndex(row => row.category_id === data.key);
    if (from >= 0 && from !== index) moveEntry(props.form.targets, from, index-from);
  } catch { /* Ignore foreign drag payloads. */ }
}
const selectedCategories = computed(() => props.form.targets.map(row=>row.category_id));
async function changeCategories(values) {
  if (fixed.value || props.inheritedTargets) return;
  for (let i=props.form.targets.length-1;i>=0;i--) {
    if (!values.includes(props.form.targets[i].category_id)) removeTarget(i);
  }
  for (const value of values) if (!props.form.targets.some(row=>row.category_id===value)) props.form.targets.push({category_id:value,target_count:1});
}
function dragTarget(event, id) {
  event.dataTransfer.effectAllowed='move';
  event.dataTransfer.setData('application/x-wt-resource-reorder',JSON.stringify({scope:'batch-targets',key:id}));
}
async function checkResources() {
  await perform(async()=>{
    const result=await batch.validate({...batchInput(props.form),...props.validationContext});
    const issues=(result.issues||[]).filter(issue=>['category_id','media_account_id','composite_output_id'].includes(issue.field));
    if(issues.length) error.value=[...new Set(issues.map(issue=>issue.message))].join('；');
    else outcome.value='资源检查完成。文案、封面与执行设置仍需在后续步骤检查。';
  });
}
async function rematch() {
  await loadAvailable();
  if (!error.value) await allocate();
}
async function inspect() {
  const sequence = ++inventorySequence;
  resourceLoading.value = true;
  const entries = await Promise.all(props.form.targets.map(async ({ category_id }) => {
    try {
      return [category_id, await batch.resources({ platform: props.form.platform, category_id })];
    } catch {
      return [category_id, { error: "盘点读取失败" }];
    }
  }));
  if (sequence === inventorySequence) {
    inventory.value = Object.fromEntries(entries);
    resourceLoading.value = false;
  }
}
watch(() => props.form.targets.map(row => row.category_id).join(","), inspect, { immediate: true });

function removeTarget(index) {
  const id = props.form.targets[index].category_id;
  if (hasCandidates(id)) return;
  const config = props.form.config;
  config.manual_pairs = config.manual_pairs.filter(pair => pair.category_id !== id);
  config.output_ids = config.output_ids.filter(output => String(props.catalog.outputs.get(output)?.category_id) !== id);
  props.form.targets.splice(index, 1);
}
function hasCandidates(id) {
  return props.form.config.candidates.some(row => row.category_id === id);
}
function chooseOutput(id) {
  outputCategory.value = id;
  picker.value = "output";
}
function picked(row) {
  props.catalog.remember(picker.value, row);
  if (picker.value === "account") accounts.value.push({ media_account_id: String(row.id), limit: 1 });
  else outputs.value.push(batchOutputID(row));
}

async function pageResources(list, params, add, canContinue) {
  for (let page = 1; canContinue(); page++) {
    const result = await list({ ...params, page, page_size: 100 });
    for (const item of result.items || []) {
      if (!canContinue()) return;
      add(item);
    }
    if (page * 100 >= result.total || !result.items?.length) return;
  }
}

async function perform(work) {
  if (fixed.value) return;
  loading.value = true;
  emit("work", true);
  error.value = "";
  outcome.value = "";
  try {
    await work();
  } catch (failure) {
    error.value = failure.message || "资源操作未完成，请重试";
  } finally {
    loading.value = false;
    emit("work", false);
  }
}

function loadAvailable() {
  return perform(async () => {
    const targetIds = new Set(props.form.targets.map(row => row.category_id));
    const foundAccounts = new Map(accounts.value.map(row => [row.media_account_id, row]));
    const foundOutputs = new Set(outputs.value);
    const platform = props.form.platform;
    await pageResources(batch.accounts, { platform }, account => {
      props.catalog.remember("account", account);
      if (account.eligible === true && account.category_ids?.some(id => targetIds.has(id)) && !foundAccounts.has(String(account.id))) {
        foundAccounts.set(String(account.id), { media_account_id: String(account.id), limit: 1 });
      }
    }, () => foundAccounts.size < automaticResourceLimit);
    for (const target of props.form.targets) {
      const targetCount = Number(target.target_count) || 0;
      let count = [...foundOutputs].filter(id => String(props.catalog.outputs.get(id)?.category_id) === target.category_id).length;
      await pageResources(publication.outputs, { platform, category_id: target.category_id }, output => {
        props.catalog.remember("output", output);
        const id = batchOutputID(output);
        if (!foundOutputs.has(id) && !batchOutputProblem(output, target.category_id, props.actor.team_id)) {
          foundOutputs.add(id);
          count++;
        }
      }, () => foundOutputs.size < automaticResourceLimit && count < targetCount);
    }
    props.form.config.accounts = [...foundAccounts.values()];
    props.form.config.output_ids = [...foundOutputs];
    outcome.value = "已补入可用资源。可调整账号顺序、每账号上限和成片顺序，再进行配对。";
  });
}

function allocate() {
  return perform(async () => {
    const { platform, targets, config } = props.form;
    const { accounts, output_ids, manual_pairs } = config;
    const data = await batch.allocate(snapshot({ platform, targets, accounts, output_ids, manual_pairs }));
    props.form.config.candidates = reconcileAllocation(config.candidates, data.config.candidates || []);
    outcome.value = `已配对 ${props.form.config.candidates.length} 条候选`;
    emit("allocated");
  });
}
</script>

<template>
  <div class="batch-stack">
    <BatchSetupSection number="01" title="发布范围" help="选择单个平台与参与分配的类目。">
      <div class="batch-form-grid batch-scope-grid">
        <label><span><em>*</em> 批次名称</span><t-input v-model="form.name" aria-label="批次名称" :maxlength="100" :disabled="fixed" placeholder="请输入批次名称"/></label>
        <label><span><em>*</em> 发布平台</span><t-select :value="form.platform" :options="[{label:'百家号',value:'baidu'}]" aria-label="批次发布平台" disabled/></label>
      </div>
      <div class="batch-category-field"><label><span><em>*</em> 发布类目</span></label><t-checkbox-group :value="selectedCategories" :disabled="fixed||inheritedTargets" @change="changeCategories"><t-checkbox v-for="c in categories" :key="c.value" :value="c.value" class="batch-category-chip" :disabled="hasCandidates(c.value)">{{c.label}}</t-checkbox></t-checkbox-group><small v-if="form.targets.some(t=>hasCandidates(t.category_id))" class="batch-muted">已有候选的类目需先移除对应候选再取消选择。</small></div>
    </BatchSetupSection>
    <BatchSetupSection v-if="form.targets.length" number="02" title="类目数量与优先级" help="上方类目优先匹配；数量不足时保留原目标并显示缺口。">
      <template #actions><t-button variant="outline" theme="default" :loading="resourceLoading" :disabled="fixed" @click="inspect"><template #icon><RefreshIcon/></template>刷新盘点</t-button></template>
      <div class="batch-total-line"><div><small>类目数量合计</small><strong>{{form.targets.reduce((n,t)=>n+t.target_count,0)}} <span>条候选目标</span></strong></div><p v-if="inheritedTargets">目标来自计划</p></div>
        <div v-if="form.targets.length" class="resource-data-table resource-embedded-table is-editable batch-target-container" role="region" aria-label="类目目标分配，可横向滚动" tabindex="0"><table><thead><tr><th>优先级</th><th>内容类目</th><th class="resource-table-number">可用账号</th><th class="resource-table-number">可用成片</th><th class="resource-table-number">本批次目标</th><th class="resource-table-number">已匹配</th><th class="resource-table-number">缺口</th></tr></thead><tbody><tr v-for="(target,index) in form.targets" :key="target.category_id" @dragover.prevent @drop.prevent="dropTarget($event,index)"><td><button class="batch-drag-handle" type="button" :draggable="!fixed&&!inheritedTargets" :disabled="fixed||inheritedTargets" :aria-label="'调整'+nameOf(target.category_id)+'优先级'" title="拖拽排序，或使用上下方向键" @dragstart="dragTarget($event,target.category_id)" @keydown.up.prevent="moveEntry(form.targets,index,-1)" @keydown.down.prevent="moveEntry(form.targets,index,1)"><MoveIcon/></button><span class="batch-priority">P{{index+1}}</span></td><td><strong>{{nameOf(target.category_id)}}</strong><small v-if="inventory[target.category_id]?.as_of" class="batch-muted batch-task-block">{{formatPublicationTime(inventory[target.category_id].as_of)}}</small></td><td class="resource-table-number"><t-button variant="text" size="small" :disabled="fixed" :aria-label="'查看'+nameOf(target.category_id)+'可用账号'" @click="showResources('account',target.category_id)">{{inventory[target.category_id]?.as_of?inventory[target.category_id].eligible_account_count:'—'}}</t-button></td><td class="resource-table-number"><t-button variant="text" size="small" :disabled="fixed" :aria-label="'查看'+nameOf(target.category_id)+'可用成片'" @click="showResources('output',target.category_id)">{{inventory[target.category_id]?.as_of?inventory[target.category_id].available_output_count:'—'}}</t-button><small class="batch-muted batch-task-block" v-if="inventory[target.category_id]?.error||inventory[target.category_id]?.unknown_output_count">{{displayAvailability(inventory[target.category_id])}}</small></td><td class="resource-table-number"><t-input-number v-model="target.target_count" :aria-label="nameOf(target.category_id)+'发布目标'" :min="0" :max="Number.MAX_SAFE_INTEGER" :decimal-places="0" :disabled="fixed||inheritedTargets"/></td><td class="resource-table-number">{{shortfalls[index].candidate_count}}</td><td class="resource-table-number">{{shortfalls[index].shortfall}}</td></tr></tbody></table></div>
    </BatchSetupSection>
    <BatchSetupSection v-if="form.targets.length" number="03" title="账号与成片匹配" help="系统按顺序匹配，可人工调整；保存配置不创建正式任务。">
      <template #actions><t-button variant="outline" theme="default" :loading="loading" :disabled="fixed" @click="rematch"><template #icon><RefreshIcon/></template>重新自动匹配</t-button><t-button variant="outline" theme="default" :loading="resourceLoading" :disabled="fixed" @click="checkResources">检查资源冲突</t-button><t-button variant="text" :disabled="fixed" @click="poolOpen=true">账号与本批上限</t-button></template>
      <FeedbackAlert v-if="error" theme="error" :message="error"/>
      <p v-if="outcome" role="status" class="batch-muted">{{outcome}}</p>
      <BatchCandidateWorkspace embedded resource-only :form="form" :catalog="catalog" :categories="categories" :actor="actor" :recipes="recipes||{}" :cover-files="coverFiles||{}" :validation-context="validationContext" :disabled="fixed" @work="emit('work',$event)"/>
    </BatchSetupSection>
    <ResourceOverlay :visible="poolOpen" title="发布账号与资源设置" :width="960" @update:visible="poolOpen=$event">
      <div class="batch-inline batch-section-actions"><t-button theme="default" variant="outline" :loading="loading" :disabled="fixed" @click="loadAvailable">载入可用资源</t-button></div>
        <div class="batch-form-grid">
          <div>
            <div class="batch-subhead"><h4>本人发布账号</h4><t-button variant="text" :disabled="fixed" @click="showResources('account','')">选择账号</t-button></div>
            <p class="batch-muted">每账号上限由本批所有类目共用。</p>
            <div v-for="(item, index) in accounts" :key="item.media_account_id" class="batch-resource-row">
              <ResourceReorderActions :item-key="item.media_account_id" scope="accounts" :index="index" :count="accounts.length"
                :draggable="false" :remove-disabled="fixed || form.config.candidates.some(row => row.media_account_id === item.media_account_id)" @move="!fixed && moveEntry(accounts, index, $event)" @remove="accounts.splice(index, 1)">
                {{ batchAccountName(catalog.accounts.get(item.media_account_id)) }}
              </ResourceReorderActions>
              <label class="batch-inline">本批上限<t-input-number v-model="item.limit" aria-label="本批上限" :min="1" :max="Number.MAX_SAFE_INTEGER" :decimal-places="0" :disabled="fixed" /></label>
              <small v-if="catalog.accounts.get(item.media_account_id)?.reason" class="batch-muted">{{ catalog.accounts.get(item.media_account_id).reason }}</small>
            </div>
          </div>
          <div>
            <div class="batch-subhead"><h4>候选成片</h4></div>
            <div class="batch-inline">
              <t-button v-for="target in form.targets" :key="target.category_id" variant="text" :disabled="fixed"
                @click="chooseOutput(target.category_id)">选片 · {{ nameOf(target.category_id) }}</t-button>
            </div>
            <div v-for="(id, index) in outputs" :key="id" class="batch-resource-row">
              <ResourceReorderActions :item-key="id" scope="outputs" :index="index" :count="outputs.length"
                :draggable="false" :remove-disabled="fixed || form.config.manual_pairs.some(row => row.composite_output_id === id)" @move="!fixed && moveEntry(outputs, index, $event)" @remove="outputs.splice(index, 1)">
                {{ batchOutputName(catalog.outputs.get(id)) }}
              </ResourceReorderActions>
              <small v-if="catalog.outputs.has(id)" class="batch-muted">{{ batchOutputProblem(catalog.outputs.get(id), undefined, actor.team_id) }}</small>
            </div>
          </div>
        </div>

      <template #footer><t-button @click="poolOpen=false">完成</t-button></template>
    </ResourceOverlay>

    <PublishingResourcePicker v-if="picker" batch :visible="true" :kind="picker" :platform="form.platform"
      :category="outputCategory || undefined" :team-id="String(actor.team_id || '')"
      :excluded-ids="picker === 'output' ? outputs : accounts.map(row => row.media_account_id)"
      @selected="picked" @update:visible="picker = ''" />
  </div>
</template>
