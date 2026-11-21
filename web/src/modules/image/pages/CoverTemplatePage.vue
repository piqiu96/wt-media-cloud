<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from "vue";
import {
  useRoute,
  useRouter,
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
} from "vue-router";
import { opColumnWidth } from "../../../shared/ui/resource/opColumnWidth.js";
import ResourceId from "../../../shared/ui/resource/ResourceId.vue";
import { formatDateTime } from "../../../shared/utils/datetime.js";
import { useAutoRefresh } from "../../../shared/utils/useAutoRefresh.js";
import ResourceAdvancedToggle from "../../../shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceFieldHelp from "../../../shared/ui/resource/ResourceFieldHelp.vue";
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue";
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import MediaPreview from "../../../shared/ui/media/MediaPreview.vue";
import Overlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import Workspace from "../components/CoverTemplateEditor.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { createImageClient } from "../../../shared/api/image.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { createSessionClient } from "../../../shared/api/session.js";
import { createImageUpload } from "../uploadImage.js";
import {
  copy,
  sizes,
  isPublished,
  contentOf,
  samples,
  documentInput,
} from "../coverDocument.js";
const client = createImageClient(),
  files = createFilesClient(),
  session = createSessionClient(),
  route = useRoute(),
  router = useRouter();
const templates = ref([]),
  total = ref(0),
  loading = ref(false),
  images = ref([]),
  categoryRows = ref([]),
  teams = ref([]),
  user = ref(null),
  catalogError = ref("");
const query = ref(""),
  use = ref(""),
  content = ref(""),
  size = ref(""),
  platform = ref(""),
  scope = ref(""),
  advanced = ref(false),
  applied = ref({}),
  currentPage = ref(1),
  pageSize = ref(10);
const editorRow = ref(null),
  workspace = ref(null),
  initialLayer = ref(null),
  creating = ref(false),
  copySource = ref(null),
  form = ref({}),
  formError = ref(""),
  busy = ref(false),
  switchBusy = ref(null),
  error = ref(""),
  previewOpen = ref(false),
  previewSrc = ref(""),
  previewTitle = ref("");
const publishing = ref(null),
  checking = ref(false),
  publishResult = ref(null),
  publishError = ref("");
const categoryOptions = computed(() =>
  categoryRows.value.map((c) => ({ label: c.name, value: String(c.id) })),
);
const platformOptions = [
  { label: "百家号", value: "baidu" },
  { label: "B站", value: "bilibili" },
  { label: "小红书", value: "xiaohongshu" },
];
const row = computed(() => editorRow.value);
const pageRows = computed(() => templates.value);
const dimensions = (t) => `${t.canvas_width} × ${t.canvas_height}`;
function view(v) {
  return {
    ...v,
    document: v.published_config,
    draft: v.draft_config ? { document: v.draft_config } : null,
    version: v.published_revision,
    preview: v.thumbnail_resource_path
      ? files.resourceURL({ resource_path: v.thumbnail_resource_path })
      : "",
    categories: v.category_ids.map(
      (id) => categoryRows.value.find((c) => String(c.id) === id)?.name || id,
    ),
    platforms: v.platform_codes.map(
      (code) => platformOptions.find((p) => p.value === code)?.label || code,
    ),
    updatedAt: v.updated_at,
  };
}
const columns = computed(() => [
  { colKey: "id", title: "模板ID", width: 110 },
  { colKey: "preview", title: "封面", width: 118 },
  { colKey: "name", title: "模板名称", minWidth: 175 },
  { colKey: "spec", title: "规格／范围", minWidth: 150 },
  { colKey: "content", title: "内容状态", width: 112 },
  { colKey: "usage", title: "启用", width: 88 },
  { colKey: "updated", title: "更新时间", width: 145 },
  {
    colKey: "actions",
    title: "操作",
    width: Math.max(
      160,
      ...pageRows.value.map(
        (t) =>
          opColumnWidth(
            [
              "编辑",
              ...(t.draft ? [isPublished(t) ? "发布更新" : "发布"] : []),
              "复制",
              ...(!isPublished(t) ? ["删除"] : []),
            ],
            { variant: "text" },
          ) + 16,
      ),
    ),
    fixed: "right",
  },
]);
const imagePage = ref(0),
  imageTotal = ref(0),
  imageLoading = ref(false);
const imageHasMore = computed(() => imagePage.value * 100 < imageTotal.value);
let closingEditor = false;
let listSequence = 0,
  editorSequence = 0,
  checkId = 0,
  checkAbort = null;
async function load({ silent = false } = {}) {
  const n = ++listSequence;
  if (!silent) {
    loading.value = true;
    error.value = "";
  }
  const q = applied.value,
    [width, height] = (q.size || "").split("x");
  try {
    const data = await client.listCoverTemplate({
      search: q.query,
      enabled: q.use,
      content_status: q.content,
      width,
      height,
      platform: q.platform,
      category_id: q.scope === "general" ? "" : q.scope,
      scope: q.scope === "general" ? "general" : "",
      page: currentPage.value,
      page_size: pageSize.value,
    });
    if (n === listSequence) {
      templates.value = data.items.map(view);
      total.value = data.total;
    }
  } catch (e) {
    if (n === listSequence && !silent) error.value = e.message;
  } finally {
    if (n === listSequence && !silent) loading.value = false;
  }
}
function apply() {
  applied.value = {
    query: query.value.trim(),
    use: use.value,
    content: content.value,
    size: size.value,
    platform: platform.value,
    scope: scope.value,
  };
  currentPage.value = 1;
  load();
}
function reset() {
  query.value =
    use.value =
    content.value =
    size.value =
    platform.value =
    scope.value =
      "";
  apply();
}
async function loadCatalog() {
  catalogError.value = "";
  try {
    const [categories, me] = await Promise.all([
      client.listCategory(),
      session.me(),
    ]);
    categoryRows.value = categories;
    user.value = me;
    if (me.role === "admin") teams.value = await client.listTeam();
  } catch (e) {
    catalogError.value = e.message;
    throw e;
  }
}
async function retryCatalog() {
  try {
    await loadCatalog();
    await load();
    await readEditor();
  } catch {
    /* loadCatalog exposes the actual failure. */
  }
}
async function loadImagePage(reset = false) {
  if (imageLoading.value) return;
  imageLoading.value = true;
  try {
    const page = reset ? 1 : imagePage.value + 1;
    const data = await client.listImageAsset({
      usage: "background",
      enabled: true,
      page,
      page_size: 100,
    });
    const next = data.items.map((v) => ({
      ...v,
      src: files.resourceURL(v),
      uses: ["底图"],
    }));
    images.value = reset
      ? next
      : [
          ...images.value.filter((v) => !next.some((n) => n.id === v.id)),
          ...next,
        ];
    imagePage.value = page;
    imageTotal.value = data.total;
  } finally {
    imageLoading.value = false;
  }
}
async function loadMoreImage() {
  try {
    await loadImagePage();
  } catch (e) {
    feedback.toast.error(e.message);
  }
}
async function loadImages() {
  await loadImagePage(true);
  const id = contentOf(editorRow.value)?.imageAssetId;
  if (id && !images.value.some((i) => i.id === id)) {
    const v = await client.getImageAsset(id);
    images.value.push({ ...v, src: files.resourceURL(v), uses: ["底图"] });
  }
}
function openForm(source = null) {
  copySource.value = source;
  form.value = {
    request_key: crypto.randomUUID(),
    name: source ? source.name + " 副本" : "",
    size: source
      ? `${source.canvas_width}x${source.canvas_height}`
      : "1080x1440",
    categories: copy(source?.category_ids || []),
    platforms: copy(source?.platform_codes || []),
    team_id: source?.team_id || String(user.value?.team_id || ""),
  };
  formError.value = "";
  creating.value = true;
}
async function openEditor(t, layer = null) {
  try {
    initialLayer.value = layer;
    await router.push({ query: { ...route.query, template_id: t.id } });
  } catch (e) {
    feedback.toast.error(e.message);
  }
}
async function readEditor() {
  const id = route.query.template_id,
    n = ++editorSequence;
  if (!id) {
    editorRow.value = null;
    closingEditor = false;
    return;
  }
  try {
    const v = await client.getCoverTemplate(String(id));
    if (n !== editorSequence) return;
    editorRow.value = view(v);
    await loadImages();
  } catch (e) {
    if (n === editorSequence) {
      error.value = e.message;
      editorRow.value = null;
    }
  }
}
function closeEditor() {
  closingEditor = true;
  initialLayer.value = null;
  const q = { ...route.query };
  delete q.template_id;
  router.push({ query: q });
  load();
}
async function create() {
  if (busy.value) return;
  formError.value = "";
  if (!form.value.name.trim()) {
    formError.value = "请填写模板名称";
    return;
  }
  if (user.value?.role === "admin" && !form.value.team_id) {
    formError.value = "请选择运营团队";
    return;
  }
  if (catalogError.value) {
    formError.value = catalogError.value;
    return;
  }
  busy.value = true;
  try {
    const [width, height] = form.value.size.split("x").map(Number),
      input = {
        request_key: form.value.request_key,
        name: form.value.name,
        canvas_width: width,
        canvas_height: height,
        category_ids: form.value.categories,
        platform_codes: form.value.platforms,
      };
    if (user.value?.role === "admin") input.team_id = form.value.team_id;
    const v = copySource.value
      ? await client.copyCoverTemplate(copySource.value.id, input)
      : await client.createCoverTemplate(input);
    creating.value = false;
    await openEditor(v);
    if (copySource.value)
      feedback.toast.info("已创建新规格草稿，请确认排版并重新校验");
  } catch (e) {
    formError.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function save(document) {
  const source = editorRow.value;
  const v = await client.saveCoverTemplateDraft(source.id, {
    expected_revision: source.revision,
    document: documentInput(document),
  });
  editorRow.value = view(v);
}
async function toggle(id, enabled) {
  const t = templates.value.find((t) => t.id === id);
  if (
    switchBusy.value !== null ||
    !t ||
    !isPublished(t) ||
    enabled === t.enabled
  )
    return;
  if (
    !(await feedback.confirmWarning({
      title: `${enabled ? "启用" : "停用"}「${t.name}」？`,
      message: enabled
        ? "后续新生成将使用当前已发布配置，忽略编辑草稿。"
        : "停用后不能新选此模板或重新生成，已确认封面与历史任务保留。",
      confirmText: enabled ? "启用" : "停用",
    }))
  )
    return;
  switchBusy.value = id;
  try {
    const v = await client.setCoverTemplateUsage(id, {
      expected_revision: t.revision,
      enabled,
    });
    templates.value = templates.value.map((t) => (t.id === id ? view(v) : t));
    feedback.toast.success(enabled ? "模板已启用" : "模板已停用");
  } catch (e) {
    feedback.toast.error(e.message);
  } finally {
    switchBusy.value = null;
  }
}
async function remove(t) {
  if (
    !(await feedback.confirmWarning({
      title: `删除草稿「${t.name}」？`,
      message: "删除后不可恢复。",
      confirmText: "删除草稿",
      danger: true,
    }))
  )
    return;
  try {
    await client.deleteCoverTemplate(t.id, t.revision);
    feedback.toast.success("草稿已删除");
    await load();
  } catch (e) {
    feedback.toast.error(e.message);
  }
}
const publishRow = computed(() =>
  templates.value.find((t) => t.id === publishing.value),
);
async function openPublish(t) {
  publishing.value = t.id;
  publishResult.value = null;
  publishError.value = "";
  checking.value = true;
  const n = ++checkId;
  checkAbort?.abort();
  checkAbort = new AbortController();
  try {
    await loadImagesForPublish();
    const document=t.draft?.document || t.document;
    const background = document?.backgroundMode === "business" ? images.value.find((i) => i.enabled)?.id || "0" : "0";
    let result,
      issues = [];
    for (const title of [samples.short, samples.long]) {
      const data = await client.validateCoverTemplate(
        t.id,
        {
          expected_revision: t.revision,
          title,
          background_image_asset_id: background,
        },
        { signal: checkAbort.signal },
      );
      result = data;
      issues.push(...data.issues);
    }
    if (n === checkId)
      publishResult.value = {
        image: files.resourceURL(result.artwork.file),
        issues,
        background,
      };
  } catch (e) {
    if (n === checkId) {
      publishError.value = e.name === "AbortError" ? "" : e.message;
      publishResult.value = {
        image: "",
        issues: e.details?.issues || [
          {
            level: "error",
            layerId: "background",
            layerName: "画布",
            message: e.message,
          },
        ],
      };
    }
  } finally {
    if (n === checkId) checking.value = false;
  }
}
async function loadImagesForPublish() {
  await loadImagePage(true);
}
function closePublish() {
  if (busy.value) return;
  publishing.value = null;
  ++checkId;
  checkAbort?.abort();
  checking.value = false;
}
async function publish() {
  if (
    busy.value ||
    checking.value ||
    !publishResult.value ||
    publishResult.value.issues.some((i) => i.level === "error")
  )
    return;
  busy.value = true;
  publishError.value = "";
  try {
    const t = publishRow.value;
    const data = await client.publishCoverTemplate(t.id, {
      expected_revision: t.revision,
      background_image_asset_id: publishResult.value.background,
    });
    templates.value = templates.value.map((t) =>
      t.id === data.item.id ? view(data.item) : t,
    );
    publishing.value = null;
    feedback.toast.success(
      isPublished(t)
        ? "更新已发布，启停状态保持不变"
        : "已发布，当前停用，可通过开关启用",
    );
  } catch (e) {
    publishError.value = e.message;
    if (e.details) publishResult.value.issues = e.details;
  } finally {
    busy.value = false;
  }
}
function locate(issue) {
  const t = publishRow.value;
  closePublish();
  openEditor(t, issue);
}
const thumb = (t) => t.preview || "";
function showPreview(t) {
  previewSrc.value = thumb(t);
  previewTitle.value = t.name;
  previewOpen.value = true;
}
const uploader = createImageUpload(files, client);
let uploadFileKey = "";
async function uploadBackground(file) {
  const key = `${file.name}/${file.size}/${file.lastModified}`;
  if (key !== uploadFileKey) {
    uploader.reset(file);
    uploadFileKey = key;
  }
  const v = await uploader.save({
    name: file.name.replace(/\.[^.]+$/, ""),
    allow_as_cover: false,
    allow_as_background: true,
    category_ids: editorRow.value.category_ids,
    team_id: editorRow.value.team_id,
  });
  const image = { ...v, src: files.resourceURL(v), uses: ["底图"] };
  images.value = [...images.value.filter((i) => i.id !== v.id), image];
  uploadFileKey = "";
  return image;
}
watch(() => route.query.template_id, readEditor);
onBeforeRouteLeave(async () => {
  if (!workspace.value) return true;
  return await workspace.value.requestClose(false);
});
onBeforeRouteUpdate(async (to, from) => {
  if (
    closingEditor ||
    !workspace.value ||
    to.query.template_id === from.query.template_id
  )
    return true;
  return await workspace.value.requestClose(false);
});
useAutoRefresh(() => load({ silent: true }), {
  shouldSkip: () =>
    loading.value ||
    !!editorRow.value ||
    creating.value ||
    publishing.value !== null ||
    previewOpen.value ||
    busy.value ||
    switchBusy.value !== null,
});
onMounted(async () => {
  try {
    await loadCatalog();
    await load();
    await readEditor();
  } catch {}
});
onBeforeUnmount(() => {
  ++checkId;
  ++editorSequence;
  checkAbort?.abort();
});
</script>
<template>
  <div v-show="!row" class="cover-template-manager resource-list-page">
    <ResourcePageHeader title="封面模板"
      ><template #actions
        ><t-button @click="openForm()">新建模板</t-button></template
      ></ResourcePageHeader
    >
    <FeedbackAlert
      v-if="error || catalogError"
      theme="error"
      :message="error || catalogError"
      ><t-button variant="text" @click="retryCatalog()"
        >重试</t-button
      ></FeedbackAlert
    >
    <ResourceCard class="resource-list-panel">
      <ResourceFilterBar class="operations-tool-filters">
        <label class="wt-filter-field wt-filter-field--wide"
          ><span class="wt-filter-field__label">模板名称</span
          ><t-input
            v-model="query"
            placeholder="搜索模板名称"
            clearable
            @enter="apply"
        /></label>
        <label class="wt-filter-field"
          ><span class="wt-filter-field__label">内容状态</span
          ><t-select
            placeholder="全部"
            v-model="content"
            :options="[
              { label: '全部', value: '' },
              { label: '草稿', value: 'draft' },
              { label: '已发布', value: 'published' },
            ]"
        /></label>
        <label class="wt-filter-field"
          ><span class="wt-filter-field__label">使用状态</span
          ><t-select
            placeholder="全部"
            v-model="use"
            :options="[
              { label: '全部', value: '' },
              { label: '启用', value: 'true' },
              { label: '停用', value: 'false' },
            ]"
        /></label>
        <div class="wt-filter-actions">
          <t-button @click="apply">查询</t-button
          ><t-button theme="default" variant="outline" @click="reset"
            >重置</t-button
          >
        </div>
        <ResourceAdvancedToggle
          v-model="advanced"
          :count="
            Number(!!applied.size) +
            Number(!!applied.scope) +
            Number(!!applied.platform)
          "
          controls="cover-advanced-filters"
        />
        <div
          v-if="advanced"
          id="cover-advanced-filters"
          class="wt-advanced-filters"
        >
          <label class="wt-filter-field"
            ><span class="wt-filter-field__label">画布规格</span
            ><t-select
              placeholder="全部"
              v-model="size"
              :options="[{ label: '全部', value: '' }, ...sizes]"
          /></label>
          <label class="wt-filter-field"
            ><span class="wt-filter-field__label">适用范围</span
            ><t-select
              placeholder="全部"
              v-model="scope"
              :options="[
                { label: '全部', value: '' },
                { label: '通用', value: 'general' },
                ...categoryOptions,
              ]"
          /></label>
          <label class="wt-filter-field"
            ><span class="wt-filter-field__label">平台标签</span
            ><t-select
              placeholder="全部"
              v-model="platform"
              :options="[{ label: '全部', value: '' }, ...platformOptions]"
          /></label>
        </div>
      </ResourceFilterBar>
      <t-table
        class="resource-data-table"
        row-key="id"
        :data="pageRows"
        :loading="loading"
        :scroll="{ x: '1320px' }"
        :columns="columns"
        :hover="true"
        :pagination="{
          current: currentPage,
          pageSize,
          total,
          pageSizeOptions: [10, 20, 50],
          showJumper: false,
        }"
        @page-change="
          (p) => {
            currentPage = p.current;
            pageSize = p.pageSize;
            load();
          }
        "
      >
        <template #id="{ row: t }"
          ><ResourceId :value="t.id" label="模板ID"
        /></template>
        <template #preview="{ row: t }"
          ><button
            class="cover-thumbnail"
            :aria-label="`预览${t.name}`"
            :disabled="!thumb(t)"
            @click="showPreview(t)"
          >
            <img v-if="thumb(t)" :src="thumb(t)" :alt="t.name" /><span v-else
              >待预览</span
            >
          </button></template
        >
        <template #name="{ row: t }"
          ><strong class="cover-template-name">{{ t.name }}</strong
          ><small v-if="t.platforms?.length">{{
            t.platforms.join("、")
          }}</small></template
        >
        <template #spec="{ row: t }"
          ><span>{{ dimensions(t) }} px</span
          ><small>{{ t.categories?.join("、") || "通用" }}</small></template
        >
        <template #content="{ row: t }"
          ><t-tag
            size="small"
            :theme="isPublished(t) ? 'success' : 'default'"
            variant="light"
            >{{ isPublished(t) ? "已发布" : "草稿" }}</t-tag
          ><small v-if="isPublished(t) && t.draft">有草稿</small></template
        >
        <template #usage="{ row: t }"
          ><t-tooltip
            :content="
              isPublished(t)
                ? t.enabled
                  ? '已启用'
                  : '已停用'
                : '请先发布内容'
            "
            ><t-switch
              :value="!!t.enabled"
              :disabled="!isPublished(t) || switchBusy !== null"
              :loading="switchBusy === t.id"
              :aria-label="`启用${t.name}`"
              role="switch"
              :aria-checked="!!t.enabled"
              :aria-disabled="!isPublished(t) || switchBusy !== null"
              :tabindex="isPublished(t) ? 0 : -1"
              @keydown.enter.prevent="toggle(t.id, !t.enabled)"
              @keydown.space.prevent="toggle(t.id, !t.enabled)"
              @change="(v) => toggle(t.id, v)" /></t-tooltip
        ></template>
        <template #updated="{ row: t }"
          ><span>{{ formatDateTime(t.updatedAt) }}</span></template
        >
        <template #actions="{ row: t }"
          ><ResourceRowActions
            ><t-button variant="text" @click="openEditor(t)">编辑</t-button
            ><t-button v-if="t.draft" variant="text" @click="openPublish(t)">{{
              isPublished(t) ? "发布更新" : "发布"
            }}</t-button
            ><t-button variant="text" @click="openForm(t)">复制</t-button
            ><t-button
              v-if="!isPublished(t)"
              variant="text"
              theme="danger"
              @click="remove(t)"
              >删除</t-button
            ></ResourceRowActions
          ></template
        >
        <template #empty
          ><ResourceEmptyState
            :title="
              Object.values(applied).some(Boolean)
                ? '没有符合条件的模板'
                : '还没有封面模板'
            "
            :description="
              Object.values(applied).some(Boolean)
                ? '调整筛选条件后重试。'
                : '新建模板，保存设计后从列表发布。'
            "
            ><t-button
              v-if="!Object.values(applied).some(Boolean)"
              @click="openForm()"
              >新建模板</t-button
            ><t-button v-else theme="default" variant="outline" @click="reset"
              >重置筛选</t-button
            ></ResourceEmptyState
          ></template
        >
      </t-table>
    </ResourceCard>
  </div>
  <Overlay
    :visible="creating"
    :title="copySource ? '复制模板' : '新建模板'"
    :width="560"
    :busy="busy"
    @update:visible="(v) => !busy && (creating = v)"
  >
    <t-form label-align="top" @submit.prevent="create">
      <t-form-item :required-mark="true"
        ><template #label
          >模板名称
          <ResourceFieldHelp
            label="模板名称"
            content="创建后基础信息固定；需要调整时复制为新模板。" /></template
        ><t-input
          v-model="form.name"
          placeholder="请输入模板名称"
          :maxlength="128"
      /></t-form-item>
      <t-form-item
        v-if="user?.role === 'admin'"
        label="运营团队"
        :required-mark="true"
        ><t-select
          v-model="form.team_id"
          :options="teams.map((t) => ({ label: t.name, value: String(t.id) }))"
      /></t-form-item>
      <t-form-item label="画布规格" :required-mark="true"
        ><t-select v-model="form.size" :options="sizes"
      /></t-form-item>
      <t-form-item label="适用范围"
        ><t-select
          v-model="form.categories"
          multiple
          clearable
          placeholder="通用"
          :options="categoryOptions"
      /></t-form-item>
      <t-form-item label="平台标签"
        ><t-select
          v-model="form.platforms"
          multiple
          clearable
          placeholder="不限平台，可跨平台复用"
          :options="platformOptions"
      /></t-form-item>
    </t-form>
    <ResourceNotice v-if="formError" tone="warning">{{
      formError
    }}</ResourceNotice>
    <template #footer
      ><t-button
        theme="default"
        variant="outline"
        :disabled="busy"
        @click="creating = false"
        >取消</t-button
      ><t-button :loading="busy" @click="create">{{
        copySource ? "复制并编辑" : "创建并编辑"
      }}</t-button></template
    >
  </Overlay>
  <Workspace
    v-if="row"
    :key="row.id"
    ref="workspace"
    :row="row"
    :images="images"
    :load-more-image="loadMoreImage"
    :image-has-more="imageHasMore"
    :image-loading="imageLoading"
    :upload-background="uploadBackground"
    :persist="save"
    :initial-layer="initialLayer"
    @close="closeEditor"
  />
  <Overlay
    :visible="publishing !== null"
    :title="isPublished(publishRow || {}) ? '发布更新' : '发布模板'"
    :width="880"
    :busy="busy || checking"
    @update:visible="closePublish"
  >
    <p v-if="publishRow" class="cover-form-hint">
      {{ publishRow.name }} · 已保存草稿。{{
        isPublished(publishRow)
          ? "发布后保留当前启停状态。"
          : "首次发布后保持停用。"
      }}
    </p>
    <div class="cover-check-heading">
      发布前校验
      <ResourceFieldHelp
        label="发布前校验"
        content="使用精简、较长样例校验同一个动态标题及当前设计。预览和正式出图使用相同服务端引擎。"
      />
    </div>
    <t-loading :loading="checking"
      ><div class="cover-publish-grid">
        <div class="cover-publish-image">
          <img
            v-if="publishResult?.image"
            :src="publishResult.image"
            alt="待发布草稿预览"
          /><span v-else>{{
            checking ? "正在校验…" : "修复问题后可生成预览"
          }}</span>
        </div>
        <div>
          <p v-if="publishResult && !publishResult.issues.length">
            校验通过，可以发布。
          </p>
          <div
            v-for="(issue, i) in publishResult?.issues || []"
            :key="i"
            class="cover-publish-issue"
          >
            <strong
              >{{ issue.level === "error" ? "需修复" : "提醒" }} ·
              {{ issue.layerName }}</strong
            >
            <p>{{ issue.message }}</p>
            <t-button variant="text" @click="locate(issue)">定位修改</t-button>
          </div>
        </div>
      </div></t-loading
    >
    <ResourceNotice v-if="publishError" tone="warning"
      >{{ publishError }}；原已发布内容及启停状态保持不变。</ResourceNotice
    >
    <template #footer
      ><t-button
        theme="default"
        variant="outline"
        :disabled="busy || checking"
        @click="closePublish"
        >取消</t-button
      ><t-button
        :loading="busy || checking"
        :disabled="
          !publishResult ||
          publishResult.issues.some((i) => i.level === 'error')
        "
        @click="publish"
        >确认发布</t-button
      ></template
    >
  </Overlay>
  <MediaPreview
    v-model:visible="previewOpen"
    :title="previewTitle"
    :images="[{ src: previewSrc, label: previewTitle }]"
  />
</template>
<style scoped>
.cover-template-manager {
  min-width: 0;
}
.cover-template-manager :deep(.resource-page-header) {
  margin-bottom: 20px;
}
.cover-thumbnail {
  width: 96px;
  height: 104px;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--wt-bg-page);
  border: 1px solid var(--wt-border);
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  color: var(--wt-text-tertiary);
}
.cover-thumbnail:disabled {
  cursor: default;
}
.cover-thumbnail img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
.cover-template-name {
  font-weight: 400;
  overflow-wrap: anywhere;
}
.cover-template-manager small {
  display: block;
  margin-top: 6px;
  font-size: 12px;
  color: var(--wt-text-tertiary);
}
.cover-form-hint {
  font-size: 13px;
  line-height: 1.6;
  color: var(--wt-text-tertiary);
  margin: 12px 0 0;
}
.cover-publish-grid {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 24px;
  min-height: 300px;
  max-height: 55vh;
  overflow: auto;
  margin-top: 20px;
}
.cover-publish-image {
  background: var(--wt-bg-page);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  font-size: 13px;
  color: var(--wt-text-tertiary);
}
.cover-publish-image img {
  max-width: 100%;
  max-height: 380px;
  object-fit: contain;
}
.cover-publish-issue {
  border-bottom: 1px solid var(--wt-border);
  padding-bottom: 12px;
  margin-bottom: 12px;
  font-size: 13px;
}
.cover-publish-issue p {
  margin: 8px 0;
}
.cover-check-heading {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
  font-size: 13px;
  font-weight: 500;
}
@media (max-width: 680px) {
  .cover-publish-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  .cover-publish-image {
    max-height: 300px;
  }
}
</style>
