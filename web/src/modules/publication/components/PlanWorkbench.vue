<script setup>
import { ref } from "vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceReorderActions from "../../../shared/ui/resource/ResourceReorderActions.vue";
import { splitBatch, mergeBatch, reorder, clone } from "../planPreparation.js";
const props = defineProps({
  form: Object,
  categories: Array,
  disabled: Boolean,
});
const emit = defineEmits(["recommend"]);
const selected = ref([]);
const instruction = ref("");
const error = ref("");
const categoryName = (id) =>
  props.categories.find((c) => String(c.id) === id)?.name || id;
function goals() {
  for (const id of selected.value)
    if (!props.form.goals.some((g) => g.category_id === id))
      props.form.goals.push({
        platform: "baidu",
        category_id: id,
        target_count: 1,
      });
  props.form.goals = props.form.goals.filter((g) =>
    selected.value.includes(g.category_id),
  );
}
function distribute() {
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
}
function act(fn) {
  try {
    error.value = "";
    fn();
  } catch (e) {
    error.value = e.message;
  }
}
function drop(event, i) {
  if (props.disabled) return;
  try {
    const data = JSON.parse(
      event.dataTransfer.getData("application/x-wt-resource-reorder"),
    );
    if (data.scope !== "plan-batches") return;
    const from = props.form.batches.findIndex((b) => b.row_key === data.key);
    if (from < 0) return;
    const [b] = props.form.batches.splice(from, 1);
    props.form.batches.splice(i, 0, b);
    reorder(props.form);
  } catch {}
}
function move(i, d) {
  if (props.disabled) return;
  const j = i + d;
  if (j < 0 || j >= props.form.batches.length) return;
  const [b] = props.form.batches.splice(i, 1);
  props.form.batches.splice(j, 0, b);
  reorder(props.form);
}
selected.value = props.form.goals.map((g) => g.category_id);
</script>
<template>
  <ResourceCard
    ><ResourceSection title="基础信息"
      ><div class="plan-fields">
        <label
          >计划名称<t-input
            v-model="form.name"
            :disabled="disabled"
            :maxlength="100"
            aria-label="计划名称" /></label
        ><label
          >执行周期
          <div class="plan-fields">
            <input
              v-model="form.date_start"
              type="date"
              aria-label="开始日期"
              :disabled="disabled"
            /><input
              v-model="form.date_end"
              type="date"
              aria-label="结束日期"
              :disabled="disabled"
            /></div></label
        ><label class="plan-wide"
          >备注<t-textarea
            v-model="form.note"
            :disabled="disabled"
            :maxlength="2000"
            aria-label="计划备注"
        /></label></div></ResourceSection
  ></ResourceCard>
  <ResourceCard
    ><ResourceSection title="目标设定"
      ><div class="plan-fields">
        <label
          >发布平台<t-select
            value="baidu"
            :options="[{ value: 'baidu', label: '百家号' }]"
            disabled
            aria-label="发布平台" /></label
        ><label
          >发布类目<t-select
            v-model="selected"
            multiple
            filterable
            :min-collapsed-num="2"
            :options="
              categories.map((c) => ({ value: String(c.id), label: c.name }))
            "
            :disabled="disabled"
            aria-label="发布类目"
            @change="goals"
        /></label>
      </div>
      <div class="plan-table">
        <table>
          <thead>
            <tr>
              <th>平台</th>
              <th>类目</th>
              <th>目标数</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="g in form.goals" :key="g.category_id">
              <td>百家号</td>
              <td>{{ categoryName(g.category_id) }}</td>
              <td>
                <t-input-number
                  v-model="g.target_count"
                  :disabled="disabled"
                  :min="1"
                  :decimal-places="0"
                  :aria-label="categoryName(g.category_id) + '目标数'"
                />
              </td>
            </tr>
          </tbody>
        </table></div></ResourceSection
  ></ResourceCard>
  <ResourceCard
    ><ResourceSection title="批次安排"
      ><p class="plan-muted">
        每个平台、类目的批次分目标须与原目标一致。创建计划只生成待配置骨架，不创建发布任务。
      </p>
      <div class="plan-inline">
        <t-button
          variant="outline"
          :disabled="disabled || !form.goals.length"
          @click="distribute"
          >按目标生成一批</t-button
        ><t-input
          v-model="instruction"
          :disabled="disabled"
          placeholder="本次拆批要求"
          aria-label="AI拆批要求"
        /><t-button
          :disabled="disabled || !form.goals.length"
          variant="outline"
          @click="emit('recommend', instruction)"
          >AI 推荐拆批</t-button
        >
      </div>
      <p v-if="error" role="alert">{{ error }}</p>
      <div
        v-for="(b, i) in form.batches"
        :key="b.row_key"
        class="plan-batch-editor"
        @dragover.prevent
        @drop.prevent="drop($event, i)"
      >
        <div class="plan-inline">
          <t-input
            v-model="b.name"
            :disabled="disabled"
            :maxlength="100"
            :aria-label="'批次' + (i + 1) + '名称'"
          /><ResourceReorderActions
            :index="i"
            :count="form.batches.length"
            :item-key="b.row_key"
            scope="plan-batches"
            :draggable="!disabled"
            :remove-disabled="true"
            @move="move(i, $event)"
          /><t-button
            variant="text"
            :disabled="disabled"
            @click="act(() => splitBatch(form, i))"
            >拆分</t-button
          ><t-button
            variant="text"
            :disabled="disabled || i === form.batches.length - 1"
            @click="act(() => mergeBatch(form, i))"
            >合并下一批</t-button
          ><t-button
            theme="danger"
            variant="text"
            :disabled="disabled"
            @click="
              form.batches.splice(i, 1);
              reorder(form);
            "
            >删除</t-button
          >
        </div>
        <div class="plan-fields">
          <label v-for="t in b.targets" :key="t.category_id"
            >{{ categoryName(t.category_id)
            }}<t-input-number
              v-model="t.target_count"
              :disabled="disabled"
              :min="1"
              :decimal-places="0"
              :aria-label="b.name + categoryName(t.category_id) + '数量'"
          /></label>
        </div></div></ResourceSection
  ></ResourceCard>
</template>
