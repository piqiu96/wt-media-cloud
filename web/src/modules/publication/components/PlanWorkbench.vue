<script setup>
// Structure adapted from PlanBasicForm, PlanGoalScope and plan/app.js; all data use the formal clients.
import { computed, ref } from "vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDateRange from "../../../shared/ui/resource/ResourceDateRange.vue";
import ResourceReorderActions from "../../../shared/ui/resource/ResourceReorderActions.vue";
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import PlanResourceInventory from "./PlanResourceInventory.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { splitBatch, mergeBatch, reorder, clone } from "../planPreparation.js";
const props = defineProps({
  form: Object,
  categories: Array,
  disabled: Boolean,
  batchFacts: { type: Object, default: () => ({}) },
});
const emit = defineEmits(["recommend"]);
const instruction = ref(""),
  error = ref(""),
  confirming = ref(false);
const fixed = computed(() => props.disabled || confirming.value);
const categoryName = (id) =>
  props.categories.find((c) => String(c.id) === id)?.name || "类目 ID：" + id;
const selected = computed(() => props.form.goals.map((g) => g.category_id));
const total = computed(() =>
  props.form.goals.reduce((n, g) => n + g.target_count, 0),
);
const allocated = (g) =>
  props.form.batches
    .filter((b) => b.platform === g.platform)
    .reduce(
      (n, b) =>
        n +
        (b.targets.find((t) => t.category_id === g.category_id)?.target_count ||
          0),
      0,
    );
const balanced = computed(
  () =>
    props.form.goals.length &&
    props.form.goals.every((g) => allocated(g) === g.target_count) &&
    props.form.batches.every((b) => b.targets.length),
);
const sum = (b) => b.targets.reduce((n, t) => n + t.target_count, 0);
const fact = (b) => props.batchFacts[b.batch_id] || {};
const immutable = (b) => !!fact(b).fixed;
const hasFixedFrom = (i) => props.form.batches.slice(i).some(immutable);
async function mutate(fn, rows = [], confirmation = null) {
  if (fixed.value) return;
  const original = JSON.stringify(props.form);
  confirming.value = true;
  error.value = "";
  try {
    if (confirmation || rows.some((b) => fact(b).configured)) {
      const accepted = await feedback.confirmWarning(
        confirmation || {
          title: "调整已配置批次？",
          message:
            "目标或批次安排改变后，原候选配置需要重新检查和确认。系统不会自动覆盖账号、成片与文案，请保存后逐批复核。",
          confirmText: "确认调整",
        },
      );
      if (!accepted) return;
    }
    if (original !== JSON.stringify(props.form))
      throw Error("配置已变化，请检查后重新操作");
    fn();
  } catch (e) {
    error.value = e.message;
  } finally {
    confirming.value = false;
  }
}
async function scope(values) {
  const removed = props.form.goals.filter(
    (g) => !values.includes(g.category_id),
  );
  const impacted = props.form.batches.filter((b) =>
    b.targets.some((t) => removed.some((g) => g.category_id === t.category_id)),
  );
  if (impacted.some(immutable)) {
    error.value = "已终止批次的目标范围不可移除";
    return;
  }
  mutate(
    () => {
      props.form.goals = props.form.goals.filter((g) =>
        values.includes(g.category_id),
      );
      for (const id of values)
        if (!props.form.goals.some((g) => g.category_id === id))
          props.form.goals.push({
            platform: "baidu",
            category_id: id,
            target_count: 1,
          });
      for (const b of props.form.batches)
        b.targets = b.targets.filter((t) => values.includes(t.category_id));
    },
    impacted,
    impacted.length
      ? {
          title: "移除目标范围？",
          message:
            "移除范围内的目标与批次分配将被清除，其他配置保留。已有资源配置须在保存后重新检查，空批次需删除或重新分配。",
          confirmText: "确认移除",
        }
      : null,
  );
}
function distribute() {
  if (hasFixedFrom(0)) {
    error.value = "存在已终止固定批次，不能整体重新生成";
    return;
  }
  mutate(() => {
    props.form.batches = [
      {
        row_key: crypto.randomUUID(),
        name: "批次 01",
        platform: "baidu",
        group_order: 1,
        batch_order: 1,
        targets: clone(
          props.form.goals.map(({ category_id, target_count }) => ({
            category_id,
            target_count,
          })),
        ),
      },
    ];
  }, props.form.batches);
}
function add() {
  if (fixed.value) return;
  props.form.batches.push({
    row_key: crypto.randomUUID(),
    name: "批次 " + String(props.form.batches.length + 1).padStart(2, "0"),
    platform: "baidu",
    targets: [],
  });
  reorder(props.form);
}
function cell(b, id) {
  return b.targets.find((t) => t.category_id === id)?.target_count || 0;
}
function setCell(b, id, value) {
  if (immutable(b)) return;
  mutate(() => {
    b.targets = b.targets.filter((t) => t.category_id !== id);
    if (value > 0) b.targets.push({ category_id: id, target_count: value });
  }, [b]);
}
function move(i, d) {
  const j = i + d;
  if (j < 0 || j >= props.form.batches.length) return;
  const from = Math.min(i, j),
    to = Math.max(i, j);
  if (props.form.batches.slice(from, to + 1).some(immutable)) return;
  mutate(() => {
    const [b] = props.form.batches.splice(i, 1);
    props.form.batches.splice(j, 0, b);
    reorder(props.form);
  });
}
function drop(event, i) {
  if (fixed.value) return;
  try {
    const data = JSON.parse(
      event.dataTransfer.getData("application/x-wt-resource-reorder"),
    );
    if (data.scope !== "plan-batches") return;
    const from = props.form.batches.findIndex((b) => b.row_key === data.key);
    if (
      from < 0 ||
      props.form.batches
        .slice(Math.min(from, i), Math.max(from, i) + 1)
        .some(immutable)
    )
      return;
    mutate(() => {
      const [b] = props.form.batches.splice(from, 1);
      props.form.batches.splice(i, 0, b);
      reorder(props.form);
    });
  } catch {}
}
function split(i) {
  if (hasFixedFrom(i)) return;
  mutate(() => splitBatch(props.form, i), [props.form.batches[i]]);
}
function merge(i) {
  if (hasFixedFrom(i)) return;
  mutate(() => mergeBatch(props.form, i), props.form.batches.slice(i, i + 2));
}
function remove(i) {
  if (hasFixedFrom(i) || fixed.value) return;
  return mutate(
    () => {
      props.form.batches.splice(i, 1);
      reorder(props.form);
    },
    [props.form.batches[i]],
    {
      title: "删除批次规划",
      message:
        "删除后，该批次目标回到待分配区。已有配置不会自动转移，保存前需重新分配对应目标数量。",
      confirmText: "确认删除",
    },
  );
}
</script>
<template>
  <div class="plan-stack plan-workbench-stack">
  <ResourceCard class="plan-editor-section"
    ><ResourceSection title="基础信息" variant="form"
      ><template #title-extra><span class="plan-section-number" aria-hidden="true">01</span></template><div class="plan-basic-grid">
        <label
          ><span><i>*</i>计划名称</span
          ><t-input
            v-model="form.name"
            :disabled="fixed"
            :maxlength="100"
            placeholder="例如：新赛季内容矩阵 · 周度发布"
            aria-label="计划名称"
        /></label>
        <label
          ><span><i>*</i>执行周期</span
          ><ResourceDateRange
            allow-input
            :value="[form.date_start, form.date_end]"
            :disabled="fixed"
            @change="
              (v) => {
                form.date_start = v?.[0] || '';
                form.date_end = v?.[1] || '';
              }
            "
        /></label>
        <label class="plan-basic-note"
          ><span>计划备注</span
          ><t-textarea
            v-model="form.note"
            :disabled="fixed"
            :maxlength="2000"
            :rows="3"
            placeholder="记录发布目标、活动背景、投放重点等"
            aria-label="计划备注"
        /></label></div></ResourceSection
  ></ResourceCard>
  <PlanResourceInventory
    :goals="form.goals"
    :category-name="categoryName"
    :disabled="fixed"
  />
  <ResourceCard class="plan-editor-section"
    ><ResourceSection
      title="目标设定"
      variant="form"
      description="按平台与类目设定目标；资源盘点仅供准备参考，当前库存不会限制或降低计划目标。"
    >
      <template #title-extra><span class="plan-section-number" aria-hidden="true">03</span></template>
      <div class="plan-goal-scope">
        <label
          ><span><i>*</i>发布平台</span
          ><t-select
            :value="['baidu']"
            multiple
            disabled
            :options="[{ value: 'baidu', label: '百家号' }]"
            aria-label="发布平台" /></label
        ><label
          ><span><i>*</i>发布类目</span
          ><t-select
            :value="selected"
            multiple
            filterable
            :min-collapsed-num="2"
            :options="
              categories.map((c) => ({ value: String(c.id), label: c.name }))
            "
            :disabled="fixed"
            aria-label="发布类目"
            @change="scope"
        /></label>
      </div>
      <div class="plan-section-split">
        <strong>平台 × 类目目标分配矩阵</strong
        ><span class="plan-muted">单元格仅展示对应组合目标数量</span>
      </div>
      <div
        v-if="form.goals.length"
        class="plan-detail-table resource-data-table resource-embedded-table is-editable"
        tabindex="0"
        role="region"
        aria-label="目标分配矩阵，可横向滚动"
      >
        <table>
          <thead>
            <tr>
              <th>平台 / 类目</th>
              <th
                v-for="g in form.goals"
                :key="g.category_id"
                class="resource-table-number"
              >
                {{ categoryName(g.category_id) }}
              </th>
              <th class="resource-table-number">平台合计</th>
              <th>可安排量</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>百家号</td>
              <td
                v-for="g in form.goals"
                :key="g.category_id"
                class="resource-table-number"
              >
                <t-input-number
                  v-model="g.target_count"
                  :disabled="fixed"
                  :min="1"
                  :decimal-places="0"
                  :aria-label="categoryName(g.category_id) + '目标数'"
                />
              </td>
              <td class="resource-table-number">{{ total }}</td>
              <td>待逐批配置核实</td>
            </tr>
            <tr class="plan-detail-total">
              <td>类目合计</td>
              <td
                v-for="g in form.goals"
                :key="g.category_id"
                class="resource-table-number"
              >
                {{ g.target_count }}
              </td>
              <td class="resource-table-number">{{ total }}</td>
              <td>计划目标 {{ total }} 条</td>
            </tr>
          </tbody>
        </table>
      </div>
      <ResourceNotice v-else>请先选择至少一个发布类目</ResourceNotice>
    </ResourceSection></ResourceCard
  >
  <ResourceCard class="plan-editor-section"
    ><ResourceSection
      title="批次拆解"
      variant="form"
      description="每批只发布一个平台，批次严格顺序执行。创建计划只生成待配置骨架，不创建发布任务。"
    >
      <template #title-extra><span class="plan-section-number" aria-hidden="true">04</span></template>
      <div class="plan-section-split">
        <strong
          >批次规划表
          <span class="plan-muted"
            >共 {{ form.batches.length }} 批 · 严格串行</span
          ></strong
        ><ResourceRowActions
          ><t-button
            variant="outline"
            :disabled="fixed || !form.goals.length || hasFixedFrom(0)"
            @click="distribute"
            >按目标生成一批</t-button
          ><t-button
            variant="outline"
            :disabled="fixed || !form.goals.length"
            @click="add"
            >新增批次</t-button
          ></ResourceRowActions
        >
      </div>
      <div class="plan-inline">
        <t-input
          v-model="instruction"
          :disabled="fixed"
          placeholder="本次拆批要求"
          aria-label="AI拆批要求"
        /><t-button
          :disabled="fixed || !form.goals.length || hasFixedFrom(0)"
          variant="outline"
          @click="emit('recommend', instruction)"
          >AI 推荐拆批</t-button
        >
      </div>
      <p v-if="error" role="alert">{{ error }}</p>
      <div
        class="plan-detail-table resource-data-table resource-embedded-table is-editable"
        tabindex="0"
        role="region"
        aria-label="批次规划表，可横向滚动"
      >
        <table>
          <thead>
            <tr>
              <th>顺序</th>
              <th class="plan-batch-name">批次名称</th>
              <th>平台</th>
              <th
                v-for="g in form.goals"
                :key="g.category_id"
                class="resource-table-number"
              >
                {{ categoryName(g.category_id) }}
              </th>
              <th class="resource-table-number">批次合计</th>
              <th>资源状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(b, i) in form.batches"
              :key="b.row_key"
              @dragover.prevent
              @drop.prevent="drop($event, i)"
            >
              <td>
                <div class="plan-inline">
                  <span>{{
                    String(i + 1).padStart(2, "0")
                  }}</span><ResourceReorderActions
                    class="plan-order-actions"
                    :index="i"
                    :count="form.batches.length"
                    :item-key="b.row_key"
                    scope="plan-batches"
                    :draggable="!fixed && !immutable(b)"
                    :move-up-disabled="
                      fixed ||
                      immutable(b) ||
                      immutable(form.batches[i - 1] || {})
                    "
                    :move-down-disabled="
                      fixed ||
                      immutable(b) ||
                      immutable(form.batches[i + 1] || {})
                    "
                    :remove-disabled="true"
                    @move="move(i, $event)"
                  />
                </div>
              </td>
              <td>
                <t-input
                  v-model="b.name"
                  :disabled="fixed || immutable(b)"
                  :maxlength="100"
                  :aria-label="'批次' + (i + 1) + '名称'"
                />
              </td>
              <td>百家号</td>
              <td
                v-for="g in form.goals"
                :key="g.category_id"
                class="resource-table-number"
              >
                <t-input-number
                  :value="cell(b, g.category_id)"
                  :disabled="fixed || immutable(b)"
                  :min="0"
                  :decimal-places="0"
                  :aria-label="b.name + categoryName(g.category_id) + '数量'"
                  @change="(v) => setCell(b, g.category_id, v)"
                />
              </td>
              <td class="resource-table-number">{{ sum(b) }} 条</td>
              <td>
                {{
                  immutable(b)
                    ? "已终止 · 固定"
                    : fact(b).configured
                      ? "已配置 · 调整后需复核"
                      : "待配置"
                }}
              </td>
              <td>
                <ResourceRowActions
                  ><t-button
                    variant="text"
                    :disabled="fixed || hasFixedFrom(i)"
                    @click="split(i)"
                    >拆分</t-button
                  ><t-button
                    variant="text"
                    :disabled="
                      fixed || hasFixedFrom(i) || i === form.batches.length - 1
                    "
                    @click="merge(i)"
                    >合并下一批</t-button
                  ><t-button
                    theme="danger"
                    variant="text"
                    :disabled="fixed || hasFixedFrom(i)"
                    @click="remove(i)"
                    >删除</t-button
                  ></ResourceRowActions
                >
              </td>
            </tr>
          </tbody>
        </table>
        <div class="plan-batch-summary">
          <span :class="balanced ? 'plan-valid' : 'plan-invalid'">{{
            balanced
              ? "目标与批次分配已匹配"
              : "请调整批次分目标，与每个目标单元格一致"
          }}</span><span class="plan-muted"
            >原始批次合计 {{ form.batches.reduce((n, b) => n + sum(b), 0) }} /
            目标 {{ total }} 条</span
          >
        </div>
      </div>
      <ResourceNotice
        v-if="form.goals.some((g) => allocated(g) !== g.target_count)"
        tone="warning"
        >{{
          form.goals
            .filter((g) => allocated(g) !== g.target_count)
            .map(
              (g) =>
                categoryName(g.category_id) +
                "：已分配 " +
                allocated(g) +
                " / 目标 " +
                g.target_count,
            )
            .join("；")
        }}。调整任意单元格不会自动修改其他格数量。</ResourceNotice
      >
    </ResourceSection></ResourceCard
  >
  </div>
</template>
<style scoped>
/* plan/style.css section spacing; avoid inherited card margins adding a second gap. */
.plan-workbench-stack { gap:18px; padding:0; min-width:0; }
.plan-workbench-stack > :deep(.resource-card) { margin:0; }

.plan-order-actions {
  margin: 0;
}
.plan-order-actions :deep(.resource-reorder__heading) {
  display: none;
}
.plan-order-actions :deep(.resource-reorder__handle > span) {
  display: none;
}
.plan-order-actions :deep(.resource-reorder__actions) {
  gap: 4px;
  flex-wrap: nowrap;
}
.plan-order-actions :deep(.resource-reorder__actions > .t-button:last-child) {
  display: none;
}

.plan-basic-grid,
.plan-goal-scope {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}
.plan-basic-grid label,
.plan-goal-scope label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.plan-basic-grid label > span,
.plan-goal-scope label > span {
  display: flex;
  gap: 4px;
  font-size: 14px;
}
.plan-basic-grid i,
.plan-goal-scope i {
  font-style: normal;
  color: var(--td-error-color);
}
.plan-basic-note {
  grid-column: 1/-1;
}
.plan-goal-scope {
  margin-bottom: 18px;
}
.plan-batch-name {
  min-width: 210px;
}
.plan-detail-table .t-input-number {
  width: 108px;
}
.plan-section-split {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin: 20px 0 12px;
  flex-wrap: wrap;
}
.plan-section-split strong {
  font-size: 14px;
  font-weight: 500;
}
.plan-batch-summary {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  background: var(--wt-color-bg-subtle);
  font-size: 13px;
}
.plan-valid {
  color: var(--wt-color-success-text);
}
.plan-invalid {
  color: var(--wt-color-warning-text);
}
@media (max-width: 800px) {
  .plan-basic-grid,
  .plan-goal-scope {
    grid-template-columns: 1fr;
  }
}
</style>
