<script setup>
import { computed, onMounted, ref } from "vue";
import { createImageClient } from "../../../shared/api/image.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { createSessionClient } from "../../../shared/api/session.js";
import { useAutoRefresh } from "../../../shared/utils/useAutoRefresh.js";
import { formatDateTime } from "../../../shared/utils/datetime.js";
import { feedback } from "../../../shared/ui/feedback/index.js";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceAdvancedToggle from "../../../shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceSelectionToolbar from "../../../shared/ui/resource/ResourceSelectionToolbar.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceNameLink from "../../../shared/ui/resource/ResourceNameLink.vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue";
import ImageAssetForm from "../components/ImageAssetForm.vue";
import ImageAssetDetail from "../components/ImageAssetDetail.vue";
const client = createImageClient(),
  files = createFilesClient(),
  session = createSessionClient();
const rows = ref([]),
  total = ref(0),
  page = ref(1),
  pageSize = ref(10),
  loading = ref(false),
  error = ref("");
const filters = ref({
    search: "",
    usage: "",
    category_id: "",
    enabled: "",
    aspect: "",
    source: "",
  }),
  applied = ref({});
const advanced = ref(false);
const selected = ref([]),
  categories = ref([]),
  teams = ref([]),
  user = ref(null),
  catalogError = ref(""),
  maxBytes = ref(0);
const formVisible = ref(false),
  editing = ref(null),
  busy = ref(false),
  rowBusy = ref("");
const detail = ref(null),
  references = ref([]),
  detailError = ref(""),
  detailBusy = ref(false);
const resultVisible = ref(false),
  results = ref([]);
const categoryOptions = computed(() =>
  categories.value.map((c) => ({ label: c.name, value: String(c.id) })),
);
const teamOptions = computed(() =>
  teams.value.map((t) => ({ label: t.name, value: String(t.id) })),
);
const categoryLabel = (ids) =>
  ids?.length
    ? ids
        .map(
          (id) => categories.value.find((c) => String(c.id) === id)?.name || id,
        )
        .join("、")
    : "通用";
const usageLabel = (row) =>
  [row.allow_as_cover && "现成封面", row.allow_as_background && "底图"].filter(
    Boolean,
  );
const detailImage = computed(
  () =>
    detail.value && {
      ...detail.value,
      src: files.resourceURL(detail.value),
      category: categoryLabel(detail.value.category_ids),
      uses: usageLabel(detail.value),
      size: detail.value.size_bytes,
      source: "人工上传",
      createdAt: detail.value.created_at,
      updatedAt: detail.value.updated_at,
    },
);
const columns = [
  { colKey: "row-select", type: "multiple", width: 48 },
  { colKey: "id", title: "ID", width: 90 },
  { colKey: "name", title: "图片", width: 260 },
  { colKey: "category", title: "内容类目", width: 140 },
  { colKey: "usage", title: "用途", width: 150 },
  { colKey: "size", title: "尺寸 / 格式", width: 160 },
  { colKey: "enabled", title: "使用状态", width: 100 },
  { colKey: "created_at", title: "创建时间", width: 170 },
  { colKey: "op", title: "操作", width: 160, fixed: "right" },
];
let sequence = 0,
  detailSequence = 0;
async function load({ silent = false } = {}) {
  const n = ++sequence;
  if (!silent) {
    loading.value = true;
    error.value = "";
  }
  try {
    const data = await client.listImageAsset({
      ...applied.value,
      page: page.value,
      page_size: pageSize.value,
    });
    if (n === sequence) {
      rows.value = data.items;
      total.value = data.total;
    }
  } catch (e) {
    if (n === sequence && !silent) error.value = e.message;
  } finally {
    if (n === sequence && !silent) loading.value = false;
  }
}
function query() {
  page.value = 1;
  selected.value = [];
  applied.value = { ...filters.value };
  load();
}
function reset() {
  filters.value = {
    search: "",
    usage: "",
    category_id: "",
    enabled: "",
    aspect: "",
    source: "",
  };
  query();
}
async function loadCatalog() {
  catalogError.value = "";
  try {
    const [categoryRows, me, filePage] = await Promise.all([
      client.listCategory(),
      session.me(),
      files.list({ page: 1, page_size: 1 }),
    ]);
    categories.value = categoryRows;
    user.value = me;
    maxBytes.value = filePage.upload_max_bytes;
    if (me.role === "admin") teams.value = await client.listTeam();
  } catch (e) {
    catalogError.value = e.message;
  }
}
function openForm(row = null) {
  editing.value = row;
  formVisible.value = true;
}
async function setUsage(row, enabled) {
  if (rowBusy.value) return;
  const action = enabled ? "启用" : "停用";
  if (
    !(await feedback.confirmWarning({
      title: `${action}图片`,
      message: `${action}「${row.name}」？${enabled ? "启用后可被新内容选用。" : "停用后不再供新内容选用，已确认封面和历史任务保留。"}`,
      confirmText: action,
    }))
  )
    return;
  rowBusy.value = row.id;
  try {
    const next = await client.setImageAssetUsage(row.id, {
      expected_revision: row.revision,
      enabled,
    });
    rows.value = rows.value.map((r) => (r.id === row.id ? next : r));
    feedback.toast.success(`图片已${action}`);
  } catch (e) {
    feedback.toast.error(e.message);
  } finally {
    rowBusy.value = "";
  }
}
async function remove(row) {
  if (
    !(await feedback.confirmWarning({
      title: "删除图片",
      message: `从图片库删除「${row.name}」？被模板或正式任务引用时无法删除。原始文件保留在文件管理中。`,
      confirmText: "删除",
      danger: true,
    }))
  )
    return;
  rowBusy.value = row.id;
  try {
    await client.deleteImageAsset(row.id, row.revision);
    selected.value = selected.value.filter((id) => id !== row.id);
    feedback.toast.success("图片已删除");
    await load();
  } catch (e) {
    feedback.toast.error(e.message);
  } finally {
    rowBusy.value = "";
  }
}
async function openDetail(row) {
  const n = ++detailSequence;
  detail.value = row;
  references.value = [];
  detailError.value = "";
  detailBusy.value = true;
  try {
    const [item, data] = await Promise.all([
      client.getImageAsset(row.id),
      client.listImageAssetReference(row.id),
    ]);
    if (n === detailSequence) {
      detail.value = item;
      references.value = data.items;
    }
  } catch (e) {
    if (n === detailSequence) detailError.value = e.message;
  } finally {
    if (n === detailSequence) detailBusy.value = false;
  }
}
async function batch(operation) {
  if (busy.value || !selected.value.length) return;
  const label = { enable: "启用", disable: "停用", dual_usage: "标记双用途" }[
    operation
  ];
  if (
    !(await feedback.confirmWarning({
      title: `批量${label}`,
      message: `对选中的 ${selected.value.length} 张图片${label}？已确认封面与历史任务保留。`,
      confirmText: label,
    }))
  )
    return;
  busy.value = true;
  try {
    const data = await client.batchImageAsset({
      operation,
      items: rows.value
        .filter((r) => selected.value.includes(r.id))
        .map((r) => ({ id: r.id, expected_revision: r.revision })),
    });
    results.value = data.items;
    resultVisible.value = true;
    selected.value = data.items.filter((r) => !r.success).map((r) => r.id);
    await load();
  } catch (e) {
    feedback.toast.error(e.message);
  } finally {
    busy.value = false;
  }
}
useAutoRefresh(() => load({ silent: true }), {
  shouldSkip: () =>
    loading.value ||
    formVisible.value ||
    detail.value ||
    detailBusy.value ||
    resultVisible.value ||
    busy.value ||
    rowBusy.value ||
    selected.value.length,
});
onMounted(() => {
  load();
  loadCatalog();
});
</script>
<template>
  <div class="image-library-page wt-resource-page resource-visual-approved resource-list-page">
    <ResourcePageHeader title="图片库"
      ><template #actions
        ><t-button @click="openForm()">上传图片</t-button></template
      ></ResourcePageHeader
    >
    <FeedbackAlert v-if="catalogError" theme="error" :message="catalogError"
      ><t-button variant="text" @click="loadCatalog"
        >重试加载类目</t-button
      ></FeedbackAlert
    >
    <ResourceCard class="resource-list-panel image-library-panel">
      <ResourceFilterBar class="image-library-filters">
        <label class="wt-filter-field wt-filter-field--wide"
          ><span class="wt-filter-field__label">图片名称</span
          ><t-input v-model="filters.search" @enter="query"
        /></label>
        <label class="wt-filter-field"
          ><span class="wt-filter-field__label">内容类目</span
          ><t-select
            placeholder="全部"
            v-model="filters.category_id"
            :options="[{ label: '全部', value: '' }, ...categoryOptions]"
        /></label>
        <label class="wt-filter-field"
          ><span class="wt-filter-field__label">用途</span
          ><t-select
            placeholder="全部"
            v-model="filters.usage"
            :options="[
              { label: '全部', value: '' },
              { label: '现成封面', value: 'cover' },
              { label: '底图', value: 'background' },
            ]"
        /></label>
        <label class="wt-filter-field"
          ><span class="wt-filter-field__label">使用状态</span
          ><t-select
            placeholder="全部"
            v-model="filters.enabled"
            :options="[
              { label: '全部', value: '' },
              { label: '启用', value: 'true' },
              { label: '停用', value: 'false' },
            ]"
        /></label>
        <label class="wt-filter-field"
          ><span class="wt-filter-field__label">图片比例</span
          ><t-select
            placeholder="全部"
            v-model="filters.aspect"
            :options="[
              { label: '全部', value: '' },
              { label: '横图', value: 'landscape' },
              { label: '竖图', value: 'portrait' },
              { label: '正方形', value: 'square' },
            ]"
        /></label>
        <div class="wt-filter-actions">
          <t-button @click="query">查询</t-button>
          <t-button theme="default" variant="outline" @click="reset">重置</t-button>
        </div>
        <div class="image-advanced-section">
          <ResourceAdvancedToggle
            v-model="advanced"
            :count="Number(!!applied.source)"
            controls="image-library-advanced-filters"
          />
          <div
            v-if="advanced"
            id="image-library-advanced-filters"
            class="wt-advanced-filters"
          >
            <label class="wt-filter-field"
              ><span class="wt-filter-field__label">来源</span
              ><t-select
                placeholder="全部"
                v-model="filters.source"
                :options="[
                  { label: '全部', value: '' },
                  { label: '上传图片', value: 'upload' },
                  { label: '模板生成', value: 'render' },
                ]"
            /></label>
          </div>
        </div>
      </ResourceFilterBar>
      <ResourceSelectionToolbar
        :count="selected.length"
        unit="张图片"
        @clear="selected = []"
        ><t-button variant="text" :disabled="busy" @click="batch('dual_usage')"
          >批量标记双用途</t-button
        ><t-button variant="text" :disabled="busy" @click="batch('enable')"
          >批量启用</t-button
        ><t-button variant="text" :disabled="busy" @click="batch('disable')"
          >批量停用</t-button
        ></ResourceSelectionToolbar
      >
      <FeedbackAlert v-if="error" theme="error" :message="error"
        ><t-button variant="text" @click="load()">重试</t-button></FeedbackAlert
      >
      <template v-else>
        <div class="resource-data-table">
          <t-table
            class="wt-resource-table"
            row-key="id"
            :data="rows"
            :columns="columns"
            :loading="loading"
            :selected-row-keys="selected"
            :scroll="{ x: '1378px' }"
            @select-change="selected = $event"
          >
            <template #name="{ row }"
              ><div class="image-cell">
                <button
                  class="image-thumbnail"
                  :aria-label="`查看图片 ${row.name}`"
                  @click="openDetail(row)"
                >
                  <img :src="files.resourceURL(row)" alt="" /></button
                ><ResourceNameLink @click="openDetail(row)">{{
                  row.name
                }}</ResourceNameLink>
              </div></template
            >
            <template #category="{ row }">{{
              categoryLabel(row.category_ids)
            }}</template><template #usage="{ row }">{{
              usageLabel(row).join("、")
            }}</template>
            <template #size="{ row }"
              >{{ row.width }} × {{ row.height
              }}<small class="image-secondary">{{
                row.format.toUpperCase()
              }}</small></template
            >
            <template #enabled="{ row }"
              ><t-switch
                :value="row.enabled"
                :loading="rowBusy === row.id"
                :disabled="!!rowBusy || busy"
                :label="['启用', '停用']"
                @change="setUsage(row, $event)"
            /></template>
            <template #created_at="{ row }">{{
              formatDateTime(row.created_at)
            }}</template>
            <template #op="{ row }"
              ><ResourceRowActions
                ><t-button
                  variant="text"
                  :disabled="busy || !!rowBusy"
                  @click="openForm(row)"
                  >编辑</t-button
                ><t-button variant="text" @click="openDetail(row)"
                  >详情</t-button
                ><t-button
                  variant="text"
                  theme="danger"
                  :disabled="busy || !!rowBusy"
                  @click="remove(row)"
                  >删除</t-button
                ></ResourceRowActions
              ></template
            >
            <template #empty
              ><ResourceEmptyState
                :title="
                  Object.values(applied).some(Boolean)
                    ? '没有符合条件的图片'
                    : '还没有图片'
                "
                :description="
                  Object.values(applied).some(Boolean)
                    ? '调整筛选条件后查询。'
                    : '上传图片后可用作现成封面或模板底图。'
                "
            /></template>
          </t-table>
        </div>
        <div class="pagination-bar">
          <t-pagination
            v-model:current="page"
            v-model:page-size="pageSize"
            :total="total"
            :show-jumper="false"
            :page-size-options="[10, 20, 50]"
            @current-change="
              selected = [];
              load();
            "
            @page-size-change="
              page = 1;
              selected = [];
              load();
            "
          />
        </div>
      </template>
    </ResourceCard>
    <ImageAssetForm
      v-model:visible="formVisible"
      :item="editing"
      :categories="categoryOptions"
      :teams="teamOptions"
      :admin="user?.role === 'admin'"
      :max-bytes="maxBytes"
      :catalog-error="catalogError"
      @saved="load()"
    />
    <ImageAssetDetail
      :image="detailImage"
      :references="references"
      :loading="detailBusy"
      :error="detailError"
      @retry="openDetail(detail)"
      @close="
        detail = null;
        detailSequence++;
      "
    />
    <ResourceOverlay
      v-model:visible="resultVisible"
      title="批量处理结果"
      :width="960"
      ><div class="image-batch-summary">
        成功 {{ results.filter((r) => r.success).length }} 张 · 失败
        {{ results.filter((r) => !r.success).length }} 张
      </div>
      <t-table
        class="resource-data-table resource-embedded-table"
        row-key="id"
        :data="results"
        :columns="[
          { colKey: 'id', title: '图片 ID' },
          { colKey: 'result', title: '处理结果' },
        ]"
        ><template #result="{ row }">{{
          row.success ? "已完成" : row.error
        }}</template></t-table
      ><template #footer
        ><t-button
          theme="default"
          variant="outline"
          @click="resultVisible = false"
          >关闭</t-button
        ></template
      ></ResourceOverlay
    >
  </div>
</template>
<style scoped>
.image-library-panel {
  container: image-library / inline-size;
}
.image-library-page .image-library-panel .image-library-filters {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: end;
}
.image-library-page .image-library-panel .image-library-filters > .wt-filter-field {
  width: 100%;
  max-width: none;
}
.image-library-filters > .wt-filter-actions {
  justify-self: end;
  margin-left: 0;
  flex-wrap: nowrap;
}
@container image-library (min-width: 960px) {
  .image-library-page .image-library-panel .image-library-filters {
    grid-template-columns: minmax(180px, 1.5fr) repeat(4, minmax(110px, 1fr)) max-content;
  }
}
@container image-library (max-width: 640px) {
  .image-library-page .image-library-panel .image-library-filters {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@container image-library (max-width: 420px) {
  .image-library-page .image-library-panel .image-library-filters {
    grid-template-columns: minmax(0, 1fr);
  }
}

.image-advanced-section {
  display: grid;
  grid-column: 1 / -1;
  gap: 16px;
  min-width: 0;
}
.image-cell {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.image-thumbnail {
  width: 64px;
  height: 48px;
  flex-shrink: 0;
  padding: 0;
  border: 1px solid var(--wt-border);
  border-radius: 4px;
  overflow: hidden;
  background: var(--wt-bg-page);
  cursor: pointer;
}
.image-thumbnail img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.image-secondary {
  display: block;
  color: var(--wt-text-tertiary);
  font-size: 12px;
  margin-top: 4px;
}
.image-batch-summary {
  text-align: center;
  font-size: 18px;
  font-weight: 500;
  margin: 16px 0 24px;
}
</style>
