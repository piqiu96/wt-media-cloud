<script setup>
import {
  computed,
  onMounted,
  onBeforeUnmount,
  ref,
  watch,
  nextTick,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { createWorkClient } from "../../../shared/api/work.js";
import { createComposeClient } from "../../../shared/api/compose.js";
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { platformLabel } from "../../../shared/utils/platforms.js";
import {
  publicationCreatedRangeParams,
  publicationScheduleInput,
  formatPublicationTime as date,
} from "../../publication/dateTime.js";
import ResourcePageHeader from "../../../shared/ui/resource/ResourcePageHeader.vue";
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue";
import { opColumnWidth } from "../../../shared/ui/resource/opColumnWidth.js";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import ResourceAdvancedToggle from "../../../shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceDateRange from "../../../shared/ui/resource/ResourceDateRange.vue";
import ResourceEmptyState from "../../../shared/ui/resource/ResourceEmptyState.vue";
import ResourceNameLink from "../../../shared/ui/resource/ResourceNameLink.vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceDetailPage from "../../../shared/ui/resource/ResourceDetailPage.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import MediaPreview from "../../../shared/ui/media/MediaPreview.vue";
import AccountSummaryDialog from "../../accounts/components/AccountSummaryDialog.vue";
const client = createWorkClient(),
  compose = createComposeClient(),
  accounts = createMediaAccountClient(),
  files = createFilesClient(),
  route = useRoute(),
  router = useRouter();
const blank = () => ({
  search: "",
  platform: "",
  category_id: "",
  media_account_id: "",
  published_range: [],
});
const filters = ref(blank()),
  applied = ref({}),
  advanced = ref(false),
  rows = ref([]),
  page = ref(1),
  pageSize = ref(20),
  total = ref(0),
  loading = ref(false),
  error = ref(""),
  detail = ref(null),
  detailLoading = ref(false),
  categories = ref([]),
  catalogError = ref("");
const pageRoot = ref(null),
  detailError = ref(""),
  requestedDetailId = ref("");
let listScroll = 0;
watch(detail, async (next, previous) => {
  if (next && previous && String(next.id) === String(previous.id)) return;
  const container = pageRoot.value?.closest(".content-area");
  if (!previous) listScroll = container?.scrollTop || 0;
  await nextTick();
  if (container && !disposed) container.scrollTop = next ? 0 : listScroll;
});
const summary = ref(null);
const today = () => publicationScheduleInput(Date.now()).slice(0, 10);
const todayActive = computed(
  () =>
    applied.value.published_range?.[0] === today() &&
    applied.value.published_range?.[1] === today(),
);
function showToday() {
  filters.value.published_range = [today(), today()];
  advanced.value = true;
  query();
}
const accountOptions = ref([]),
  accountLoading = ref(false),
  accountError = ref(""),
  accountSummaryId = ref(""),
  accountSummaryVisible = ref(false);
const preview = ref(false),
  previewImages = ref([]);
let listSequence = 0,
  detailSequence = 0,
  accountSequence = 0,
  disposed = false;
onBeforeUnmount(() => {
  disposed = true;
  listSequence++;
  detailSequence++;
  accountSequence++;
  clearInterval(metricsTimer);
});
const categoryOptions = computed(() =>
  categories.value.map((c) => ({ label: c.name, value: String(c.id) })),
);
const categoryName = (id) =>
  categories.value.find((c) => String(c.id) === String(id))?.name || "—";
const metricColumns = [
  { colKey: "view", title: "播放量", width: 90 },
  { colKey: "like", title: "点赞", width: 80 },
  { colKey: "favorite", title: "收藏", width: 80 },
  { colKey: "coin", title: "投币", width: 80 },
  { colKey: "reply", title: "评论", width: 80 },
  { colKey: "danmaku", title: "弹幕", width: 80 },
];
const refreshing = ref([]);
let metricsTimer;
function metricValue(row, key) {
  const value = row.platform === "bilibili" ? row.metrics?.[key] : null;
  return Number.isSafeInteger(value) && value >= 0
    ? value.toLocaleString("zh-CN")
    : "—";
}
function metricStatus(row) {
  if (row.platform !== "bilibili") return "暂不支持";
  const m = row.metrics;
  if (!m || m.status === "queued") return "等待更新（需负责人本机服务在线）";
  if (m.status === "refreshing")
    return Date.now() - new Date(m.last_attempt_at).getTime() > 120000
      ? "等待重试（需负责人本机服务在线）"
      : "更新中";
  if (m.status === "failed")
    return (
      {
        rate_limited: "平台请求受限，稍后自动重试",
        identity_mismatch: "作品或账号不匹配，更新失败",
        invalid_response: "平台数据不完整，更新失败",
      }[m.last_error] || "更新失败，稍后自动重试"
    );
  return "";
}
async function refreshMetrics(row) {
  const id = String(row.id);
  if (refreshing.value.includes(id)) return;
  refreshing.value = [...refreshing.value, id];
  error.value = "";
  try {
    const metrics = await client.refreshMetrics(id);
    if (disposed) return;
    feedback.toast.success(
      metrics.status === "ready"
        ? "刚刚已更新，30 秒内复用本次结果"
        : "已请求更新，负责人本机服务在线时自动获取",
    );
    rows.value = rows.value.map((r) =>
      String(r.id) === id ? { ...r, metrics } : r,
    );
    if (String(detail.value?.id) === id)
      detail.value = { ...detail.value, metrics };
  } catch (e) {
    if (!disposed) error.value = e.message;
  } finally {
    refreshing.value = refreshing.value.filter((key) => key !== id);
  }
}
async function pollMetrics() {
  if (
    disposed ||
    loading.value ||
    detailLoading.value ||
    (typeof document !== "undefined" && document.hidden)
  )
    return;
  if (detail.value) {
    if (detail.value.platform !== "bilibili") return;
    const id = String(detail.value.id),
      sequence = detailSequence;
    try {
      const item = await client.get(id);
      if (
        !disposed &&
        sequence === detailSequence &&
        String(detail.value?.id) === id
      )
        detail.value = item;
    } catch {
      /* Keep the last successful page; explicit refresh still reports errors. */
    }
  } else if (rows.value.some((r) => r.platform === "bilibili")) {
    const sequence = ++listSequence;
    try {
      const data = await client.list(params());
      if (!disposed && sequence === listSequence && !detail.value) {
        rows.value = data.items;
        total.value = data.total;
      }
    } catch {
      /* Preserve visible counters during a transient Cloud outage. */
    }
  }
}
const columns = [
  { colKey: "id", title: "作品 ID", width: 90 },
  { colKey: "title", title: "作品", width: 320 },
  { colKey: "platform", title: "平台", width: 100 },
  { colKey: "account", title: "发布账号", width: 180 },
  { colKey: "category", title: "内容类目", width: 130 },
  { colKey: "metrics", title: "作品指标", width: 230 },
  { colKey: "metrics_updated", title: "指标更新时间", width: 230 },
  { colKey: "published_at", title: "平台发布时间", width: 180 },
  {
    colKey: "op",
    title: "操作",
    width: opColumnWidth(["详情", "刷新指标", "打开作品", "原任务"], {
      variant: "text",
    }),
    fixed: "right",
  },
];
function params() {
  const range = publicationCreatedRangeParams(applied.value.published_range);
  return {
    ...applied.value,
    published_range: undefined,
    published_from: range.created_from,
    published_before: range.created_before,
    page: page.value,
    page_size: pageSize.value,
  };
}
async function load() {
  const n = ++listSequence;
  loading.value = true;
  error.value = "";
  try {
    const input = params();
    const [data, counts] = await Promise.all([
      client.list(input),
      client.summary(input),
    ]);
    if (!disposed && n === listSequence) {
      rows.value = data.items;
      total.value = data.total;
      summary.value = counts;
    }
  } catch (e) {
    if (!disposed && n === listSequence) error.value = e.message;
  } finally {
    if (!disposed && n === listSequence) loading.value = false;
  }
}
function query() {
  applied.value = {
    ...filters.value,
    published_range: [...filters.value.published_range],
  };
  page.value = 1;
  load();
}
function reset() {
  filters.value = blank();
  query();
}
async function loadCatalog() {
  catalogError.value = "";
  try {
    const data = await compose.listCategories({ scope: "all" });
    if (!disposed) categories.value = data;
  } catch (e) {
    if (!disposed) catalogError.value = e.message;
  }
}
async function searchAccount(search = "") {
  const n = ++accountSequence;
  accountLoading.value = true;
  accountError.value = "";
  try {
    const data = await accounts.listPage({ search, page: 1, page_size: 20 });
    if (!disposed && n === accountSequence)
      accountOptions.value = data.items.map((a) => ({
        label: `${a.platform_name || a.platform_account_id} · ${platformLabel(a.platform)}`,
        value: String(a.id),
      }));
  } catch (e) {
    if (!disposed && n === accountSequence) accountError.value = e.message;
  } finally {
    if (!disposed && n === accountSequence) accountLoading.value = false;
  }
}
async function openDetail(row) {
  const n = ++detailSequence;
  detailLoading.value = true;
  requestedDetailId.value = String(row.id);
  detailError.value = "";
  try {
    const item = await client.get(String(row.id));
    if (!disposed && n === detailSequence) detail.value = item;
  } catch (e) {
    if (!disposed && n === detailSequence) detailError.value = e.message;
  } finally {
    if (!disposed && n === detailSequence) detailLoading.value = false;
  }
}
function retryDetail() {
  if (requestedDetailId.value)
    return openDetail({ id: requestedDetailId.value });
}
function back() {
  detailSequence++;
  requestedDetailId.value = "";
  detailError.value = "";
  detail.value = null;
  detailLoading.value = false;
  error.value = "";
  if (route.query.work_id) {
    const { work_id, ...query } = route.query;
    router.replace({ path: "/work", query });
  }
}
function viewAccount(id) {
  accountSummaryId.value = String(id);
  accountSummaryVisible.value = true;
}
function sourceTask(work) {
  router.push({
    path: "/publish",
    query: { publication_id: work.publication_id },
  });
}
function coverURL(row) {
  try {
    return files.resourceURL({ resource_path: row.cover_resource_path });
  } catch {
    return "";
  }
}
function previewCover(row) {
  const src = coverURL(row);
  if (!src) return;
  previewImages.value = [{ src, label: row.title }];
  preview.value = true;
}
function publicURL(row) {
  try {
    const u = new URL(row.work_url);
    if (u.protocol !== "https:" || u.username || u.password) return "";
    return u.href;
  } catch {
    return "";
  }
}
watch(
  () => route.query.work_id,
  (id) => {
    if (/^[1-9][0-9]*$/.test(String(id || ""))) openDetail({ id });
    else {
      detailSequence++;
      requestedDetailId.value = "";
      detailError.value = "";
      detail.value = null;
      detailLoading.value = false;
    }
  },
);
onMounted(() => {
  metricsTimer = setInterval(pollMetrics, 15000);
  load();
  loadCatalog();
  searchAccount();
  const id = String(route.query.work_id || "");
  if (/^[1-9][0-9]*$/.test(id)) openDetail({ id });
});
</script>
<template>
  <div
    ref="pageRoot"
    class="wt-resource-page resource-visual-approved work-page"
    :class="{ 'resource-list-page': !detail }"
  >
    <template v-if="!detail">
      <ResourcePageHeader title="作品列表" />
      <ResourceStatGrid
        :items="[
          {
            key: 'today',
            label: '今日已发布',
            value: summary?.today ?? '—',
            tone: 'success',
          },
        ]"
        interactive
        :active-key="todayActive ? 'today' : ''"
        @select="showToday"
      />
      <FeedbackAlert v-if="detailError" theme="error" :message="detailError"
        ><t-button variant="text" @click="retryDetail">重试打开作品</t-button
        ><t-button variant="text" @click="back"
          >返回作品列表</t-button
        ></FeedbackAlert
      >
      <FeedbackAlert v-if="error" theme="error" :message="error"
        ><t-button variant="text" @click="load">重试</t-button></FeedbackAlert
      >
      <FeedbackAlert v-if="catalogError" theme="error" :message="catalogError"
        ><t-button variant="text" @click="loadCatalog"
          >重试加载类目</t-button
        ></FeedbackAlert
      >
      <ResourceCard class="resource-list-panel">
        <ResourceFilterBar>
          <label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">作品</span
            ><t-input
              v-model="filters.search"
              placeholder="搜索标题或作品 ID"
              clearable
              @enter="query"
          /></label>
          <label class="wt-filter-field"
            ><span class="wt-filter-field__label">平台</span
            ><t-select
              v-model="filters.platform"
              clearable
              placeholder="全部"
              :options="[
                { label: '百度', value: 'baidu' },
                { label: 'B站', value: 'bilibili' },
                { label: '小红书', value: 'xiaohongshu' },
              ]"
          /></label>
          <label class="wt-filter-field"
            ><span class="wt-filter-field__label">发布账号</span
            ><t-select
              v-model="filters.media_account_id"
              filterable
              remote
              clearable
              placeholder="全部"
              :loading="accountLoading"
              :options="accountOptions"
              @search="searchAccount"
            /><span v-if="accountError" class="work-field-error"
              >{{ accountError
              }}<t-button variant="text" size="small" @click="searchAccount()"
                >重试</t-button
              ></span
            ></label
          >
          <div class="wt-filter-actions">
            <t-button @click="query">查询</t-button
            ><t-button theme="default" variant="outline" @click="reset"
              >重置</t-button
            >
          </div>
          <ResourceAdvancedToggle v-model="advanced" />
          <div v-if="advanced" class="wt-advanced-filters">
            <label class="wt-filter-field"
              ><span class="wt-filter-field__label">内容类目</span
              ><t-select
                v-model="filters.category_id"
                :options="categoryOptions"
                clearable
                placeholder="全部"
            /></label>
            <label class="wt-filter-field"
              ><span class="wt-filter-field__label"
                >平台发布时间（北京时间）</span
              ><ResourceDateRange v-model:value="filters.published_range"
            /></label>
          </div>
        </ResourceFilterBar>
        <t-table
          class="resource-data-table"
          row-key="id"
          :data="rows"
          :columns="columns"
          :loading="loading || detailLoading"
          :pagination="{
            current: page,
            pageSize,
            total,
            showJumper: true,
            showPageSize: true,
          }"
          @page-change="
            (p) => {
              page = p.current;
              pageSize = p.pageSize;
              load();
            }
          "
        >
          <template #title="{ row }"
            ><div class="work-title">
              <button
                v-if="coverURL(row)"
                class="work-cover"
                type="button"
                :aria-label="`预览作品封面 ${row.title}`"
                @click="previewCover(row)"
              >
                <img :src="coverURL(row)" alt="作品封面" /></button
              ><ResourceNameLink @click="openDetail(row)">{{
                row.title
              }}</ResourceNameLink>
            </div></template
          >
          <template #platform="{ row }">{{
            platformLabel(row.platform)
          }}</template>
          <template #account="{ row }"
            ><t-button
              variant="text"
              size="small"
              @click="viewAccount(row.media_account_id)"
              >{{
                row.account_name || `账号 ${row.media_account_id}`
              }}</t-button
            ></template
          >
          <template #category="{ row }">{{
            categoryName(row.category_id)
          }}</template>
          <template #metrics="{ row }">
            <div v-if="row.platform === 'bilibili'" class="work-metrics-grid">
              <span v-for="col in metricColumns" :key="col.colKey"
                ><span class="work-metrics-label">{{ col.title }}</span>
                {{ metricValue(row, col.colKey) }}</span
              >
            </div>
            <span v-else>—</span>
          </template>
          <template #metrics_updated="{ row }">
            <template v-if="row.platform === 'bilibili'">
              <div>
                {{
                  row.metrics?.collected_at
                    ? date(row.metrics.collected_at)
                    : "尚未获取"
                }}
              </div>
              <div
                v-if="metricStatus(row)"
                class="work-metrics-status"
                :class="{
                  'work-field-error': row.metrics?.status === 'failed',
                }"
              >
                {{ metricStatus(row) }}
              </div>
            </template>
            <span v-else>—</span>
          </template>
          <template #published_at="{ row }">{{
            date(row.published_at)
          }}</template>
          <template #op="{ row }"
            ><ResourceRowActions
              ><t-button variant="text" size="small" @click="openDetail(row)"
                >详情</t-button
              ><t-button
                v-if="row.platform === 'bilibili'"
                variant="text"
                size="small"
                :loading="refreshing.includes(String(row.id))"
                @click="refreshMetrics(row)"
                >刷新指标</t-button
              ><t-button
                v-if="publicURL(row)"
                variant="text"
                size="small"
                :href="publicURL(row)"
                target="_blank"
                rel="noopener noreferrer"
                >打开作品</t-button
              ><t-button variant="text" size="small" @click="sourceTask(row)"
                >原任务</t-button
              ></ResourceRowActions
            ></template
          >
          <template #empty
            ><ResourceEmptyState
              title="暂无已公开作品"
              description="指定稿件核实公开后会进入作品列表，审核中稿件请在发布管理中处理。"
          /></template>
        </t-table>
      </ResourceCard>
    </template>
    <ResourceDetailPage
      v-else
      title="作品详情"
      back-label="返回作品列表"
      :busy="detailLoading"
      @back="back"
    >
      <FeedbackAlert v-if="detailError" theme="error" :message="detailError"
        ><t-button variant="text" @click="retryDetail">重试打开作品</t-button
        ><t-button variant="text" @click="back"
          >返回作品列表</t-button
        ></FeedbackAlert
      >
      <FeedbackAlert v-if="error" theme="error" :message="error" />
      <ResourceCard class="detail-card"
        ><ResourceSection title="作品信息"
          ><ResourceDescriptions>
            <t-descriptions-item label="作品 ID">{{
              detail.id
            }}</t-descriptions-item>
            <t-descriptions-item label="平台">{{
              platformLabel(detail.platform)
            }}</t-descriptions-item>
            <t-descriptions-item label="内容类目">{{
              categoryName(detail.category_id)
            }}</t-descriptions-item>
            <t-descriptions-item label="发布账号"
              ><t-button
                variant="text"
                @click="viewAccount(detail.media_account_id)"
                >{{
                  detail.account_name || `账号 ${detail.media_account_id}`
                }}</t-button
              ></t-descriptions-item
            >
            <t-descriptions-item label="平台作品 ID">{{
              detail.platform_work_id
            }}</t-descriptions-item>
            <t-descriptions-item label="真实 UID">{{
              detail.platform_account_uid
            }}</t-descriptions-item>
            <t-descriptions-item label="标题" full>{{
              detail.title
            }}</t-descriptions-item>
            <t-descriptions-item label="封面" full
              ><button
                v-if="coverURL(detail)"
                class="work-detail-cover"
                type="button"
                aria-label="预览作品封面"
                @click="previewCover(detail)"
              >
                <img :src="coverURL(detail)" alt="作品封面" /></button
            ></t-descriptions-item>
            <t-descriptions-item label="公开链接" full
              ><a
                v-if="publicURL(detail)"
                :href="publicURL(detail)"
                target="_blank"
                rel="noopener noreferrer"
                >{{ detail.work_url }}</a
              ></t-descriptions-item
            >
            <t-descriptions-item label="平台实际发布时间">{{
              date(detail.published_at)
            }}</t-descriptions-item>
            <t-descriptions-item label="首次核实公开时间">{{
              date(detail.first_observed_at)
            }}</t-descriptions-item>
          </ResourceDescriptions></ResourceSection
        ></ResourceCard
      >
      <ResourceCard v-if="detail.platform === 'bilibili'" class="detail-card">
        <ResourceSection title="作品指标">
          <ResourceStatGrid
            :items="
              metricColumns.map((col) => ({
                key: col.colKey,
                label: col.title,
                value: metricValue(detail, col.colKey),
              }))
            "
          />
          <ResourceDescriptions>
            <t-descriptions-item label="最后更新">{{
              detail.metrics?.collected_at
                ? date(detail.metrics.collected_at)
                : "尚未获取"
            }}</t-descriptions-item>
            <t-descriptions-item label="更新状态">{{
              metricStatus(detail) || "已更新"
            }}</t-descriptions-item>
          </ResourceDescriptions>
          <p class="work-metrics-status">
            B站公开累计数据，可能有延迟；播放量不等于观看人数。
          </p>
          <t-button
            theme="default"
            variant="outline"
            :loading="refreshing.includes(String(detail.id))"
            @click="refreshMetrics(detail)"
            >刷新指标</t-button
          >
        </ResourceSection>
      </ResourceCard>
      <ResourceCard class="detail-card"
        ><ResourceSection title="来源信息"
          ><ResourceDescriptions>
            <t-descriptions-item label="发布任务"
              ><t-button variant="text" @click="sourceTask(detail)"
                >任务 {{ detail.publication_id }}</t-button
              ></t-descriptions-item
            >
            <t-descriptions-item label="原尝试 ID">{{
              detail.attempt_id
            }}</t-descriptions-item>
            <t-descriptions-item label="成片 ID">{{
              detail.composite_output_id
            }}</t-descriptions-item>
          </ResourceDescriptions></ResourceSection
        ></ResourceCard
      >
    </ResourceDetailPage>
    <MediaPreview
      v-model:visible="preview"
      :images="previewImages"
      title="作品封面"
    />
    <AccountSummaryDialog
      v-model:visible="accountSummaryVisible"
      :account-id="accountSummaryId"
    />
  </div>
</template>
<style scoped>
.work-page {
  min-width: 0;
}
.work-title {
  display: flex;
  align-items: center;
  gap: 12px;
}
.work-cover,
.work-detail-cover {
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
  flex-shrink: 0;
}
.work-cover img {
  display: block;
  width: 56px;
  height: 56px;
  object-fit: cover;
  border-radius: var(--wt-radius, 6px);
}
.work-detail-cover img {
  display: block;
  max-width: 240px;
  max-height: 280px;
  object-fit: contain;
  border-radius: var(--wt-radius, 6px);
}
.work-metrics-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 12px;
  font-variant-numeric: tabular-nums;
}
.work-metrics-label {
  color: var(--wt-color-text-secondary, #64748b);
}
.work-metrics-status {
  font-size: 12px;
  color: var(--wt-color-text-secondary, #64748b);
}
.work-field-error {
  font-size: 12px;
  color: var(--wt-color-error, #d54941);
}
</style>
