<script setup>
import { computed, ref, watch } from "vue";
import { createPublicationBatchClient, createPublicationClient } from "../../../shared/api/publication.js";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceReorderActions from "../../../shared/ui/resource/ResourceReorderActions.vue";
import { formatPublicationTime } from "../dateTime.js";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import PublishingResourcePicker from "./PublishingResourcePicker.vue";
import { batchAccountName, batchOutputID, batchOutputName, batchOutputProblem } from "../useBatchResourceCatalog.js";
import { candidateShortfalls, displayAvailability, moveEntry, reconcileAllocation, snapshot } from "../batchPreparation.js";

const props = defineProps({
  form: { type: Object, required: true },
  catalog: { type: Object, required: true },
  categories: { type: Array, required: true },
  actor: { type: Object, required: true },
  disabled: Boolean,
  inheritedTargets: Boolean,
});
const emit = defineEmits(["allocated", "work"]);
const batch = createPublicationBatchClient();
const publication = createPublicationClient();
const category = ref("");
const picker = ref("");
const outputCategory = ref("");
const inventory = ref({});
const error = ref("");
const loading = ref(false);
const resourceLoading = ref(false);
const outcome = ref("");
let inventorySequence = 0;
const automaticResourceLimit = 1000;

const availableCategories = computed(() => props.categories.filter(item => !props.form.targets.some(row => item.value === row.category_id)));
const shortfalls = computed(() => candidateShortfalls(props.form));
const nameOf = id => props.categories.find(item => item.value === id)?.label || "原类目暂不可用";
const accounts = computed(() => props.form.config.accounts);
const outputs = computed(() => props.form.config.output_ids);
const fixed = computed(() => props.disabled || loading.value);

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

function addTarget() {
  if (!category.value) return;
  props.form.targets.push({ category_id: category.value, target_count: 1 });
  category.value = "";
}
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
    <ResourceCard>
      <ResourceSection title="批次信息">
        <div class="batch-form-grid">
          <label>批次名称<t-input v-model="form.name" aria-label="批次名称" :maxlength="100" :disabled="fixed" placeholder="例如：本周百度视频发布" /></label>
          <label>发布平台<t-input aria-label="发布平台" value="百度" disabled /></label>
        </div>
      </ResourceSection>
    </ResourceCard>

    <ResourceCard>
      <ResourceSection title="目标与资源盘点">
        <div class="batch-inline">
          <t-select v-model="category" :options="availableCategories" filterable aria-label="目标类目" placeholder="选择目标类目" :disabled="fixed || inheritedTargets" />
          <t-button theme="default" variant="outline" :disabled="fixed || inheritedTargets || !category || form.targets.length >= 100" @click="addTarget">添加类目</t-button>
          <t-button v-if="form.targets.length" variant="text" :loading="resourceLoading" :disabled="fixed" @click="inspect">刷新盘点</t-button>
        </div>
        <p v-if="!form.targets.length" class="batch-muted">添加类目并填写发布目标，按列表顺序分配资源。</p>
        <div v-for="(target, index) in form.targets" :key="target.category_id" class="batch-resource-row">
          <ResourceReorderActions :item-key="target.category_id" scope="targets" :index="index" :count="form.targets.length"
            :draggable="false" :remove-disabled="fixed || inheritedTargets || hasCandidates(target.category_id)" @move="!fixed && !inheritedTargets && moveEntry(form.targets, index, $event)"
            @remove="removeTarget(index)">
            {{ nameOf(target.category_id) }}
          </ResourceReorderActions>
          <div class="batch-inline">
            <label>发布目标<t-input-number v-model="target.target_count" :aria-label="`${nameOf(target.category_id)}发布目标`" :min="0" :max="Number.MAX_SAFE_INTEGER" :decimal-places="0" :disabled="fixed || inheritedTargets" /></label>
            <div class="batch-inventory">
              <strong>{{ resourceLoading && !inventory[target.category_id] ? "盘点中" : displayAvailability(inventory[target.category_id]) }}</strong>
              <span v-if="inventory[target.category_id]?.as_of">
                可发布账号 {{ inventory[target.category_id].eligible_account_count }} / {{ inventory[target.category_id].account_count }}
                <template v-if="inventory[target.category_id].unknown_output_count"> · {{ inventory[target.category_id].unknown_output_count }} 个成片状态未知</template>
                · {{ formatPublicationTime(inventory[target.category_id].as_of) }}
              </span>
              <span>已配对 {{ shortfalls[index].candidate_count }} 条 · 缺口 {{ shortfalls[index].shortfall }} 条</span>
              <span v-if="hasCandidates(target.category_id)">移除类目前需先移除该类目的候选配置</span>
            </div>
          </div>
        </div>
      </ResourceSection>
    </ResourceCard>

    <ResourceCard v-if="form.targets.length">
      <ResourceSection title="配对资源">
        <FeedbackAlert v-if="error" theme="error" :message="error" />
        <p class="batch-muted" v-if="outcome" role="status">{{ outcome }}</p>
        <div class="batch-inline batch-section-actions">
          <t-button theme="default" variant="outline" :loading="loading" :disabled="fixed" @click="loadAvailable">载入可用资源</t-button>
          <t-button theme="primary" :loading="loading" :disabled="fixed || !accounts.length || !outputs.length" @click="allocate">按顺序自动配对</t-button>
        </div>
        <div class="batch-form-grid">
          <div>
            <div class="batch-subhead"><h4>本人发布账号</h4><t-button variant="text" :disabled="fixed" @click="picker = 'account'">选择账号</t-button></div>
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
      </ResourceSection>
    </ResourceCard>

    <PublishingResourcePicker v-if="picker" batch :visible="true" :kind="picker" :platform="form.platform"
      :category="picker === 'output' ? outputCategory : undefined" :team-id="String(actor.team_id || '')"
      :excluded-ids="picker === 'output' ? outputs : accounts.map(row => row.media_account_id)"
      @selected="picked" @update:visible="picker = ''" />
  </div>
</template>
