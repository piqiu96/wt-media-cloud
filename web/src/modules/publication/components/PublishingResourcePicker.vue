<script setup>
import { coverMatchesRatio } from "../coverPreparation.js";
import { OutputVideoStatus } from "../../compose/outputStatus.js";
import { computed, onMounted, ref, watch } from "vue";
import ResourceRowActions from "../../../shared/ui/resource/ResourceRowActions.vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceFilterBar from "../../../shared/ui/resource/ResourceFilterBar.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { createMediaAccountClient } from "../../../shared/api/mediaAccounts.js";
import { createPublicationClient, createPublicationBatchClient } from "../../../shared/api/publication.js";
import { createImageClient } from "../../../shared/api/image.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { batchOutputID, batchOutputProblem } from "../useBatchResourceCatalog.js";
import { accountUnavailable, outputUnavailable } from "../eligibility.js";
const props = defineProps({
  visible: Boolean,
  kind: String,
  usage: {type:String, default:"cover"},
  aspect: Array,
  category: String,
  account: Object,
  userId: String,
  batch: Boolean,
  platform: { type: String, default: "baidu" },
  teamId: String,
  excludedIds: { type: Array, default: () => [] },
});
const emit = defineEmits(["update:visible", "selected", "view-account"]);
const accounts = createMediaAccountClient(),
  publication = createPublicationClient(),
  files = createFilesClient(), images = createImageClient();
const rows = ref([]),
  total = ref(0),
  page = ref(1),
  search = ref(""),
  busy = ref(false),
  error = ref("");
const titles = {
  account: "选择发布账号",
  output: "选择成片",
  image: "选择发布封面",
};
const columns = computed(() => [
  { colKey: "id", title: "ID", width: 80 },
  {
    colKey: "name",
    title: props.kind === "account" ? "发布账号" : "名称",
    width: 240,
  },
  {
    colKey: "scope",
    title: props.kind === "account" ? "账号所属用户 / 真实 UID" : "资源信息",
    width: 200,
  },
  { colKey: "op", title: "选择", width: 240 },
]);
let sequence = 0;
async function load() {
  const n = ++sequence;
  busy.value = true;
  error.value = "";
  try {
    const params = { page: page.value, page_size: 10, search: search.value };
    let data;
    if (props.batch && props.kind === "account")
      data = await createPublicationBatchClient().accounts({ ...params, platform: props.platform, category_id: props.category });
    else if (props.kind === "account")
      data = await accounts.listPage({ ...params, platform: "baidu", user_id: props.userId });
    else if (props.kind === "output")
      data = await publication.outputs({ ...params, platform: props.platform || "baidu", ...(props.batch && { category_id: props.category }) });
    else data = await images.listImageAsset({ ...params, usage: props.usage, enabled: true, category_id: props.category });
    if (n !== sequence) return;
    if (!props.batch && props.kind === "output" && (data.items || []).some(row => typeof row.occupied !== "boolean"))
      throw new Error("暂时无法确认成片占用，请重新查询");
    rows.value = data.items || [];
    total.value = data.total || 0;
  } catch (e) {
    if (n === sequence) {
      rows.value = [];
      total.value = 0;
      error.value = e.message;
    }
  } finally {
    if (n === sequence) busy.value = false;
  }
}
function reason(row) {
  if (props.batch && props.kind === "account") {
    if (props.excludedIds.includes(String(row.id))) return "已选择该账号";
    return row.eligible === true ? "" : row.reason || "账号资格未知";
  }
  if (props.batch && props.kind === "output") {
    if (props.excludedIds.includes(batchOutputID(row))) return "已选择该成片";
    return batchOutputProblem(row, props.category, props.teamId);
  }
  return props.kind === "account"
    ? accountUnavailable(row, props.category, props.userId)
    : props.kind === "output"
      ? outputUnavailable(row, props.category, props.account)
      : row.deleted_at
        ? "文件正在删除"
        : props.usage === "cover" && props.aspect && !coverMatchesRatio(row,...props.aspect) ? `需要${props.aspect.join(":")}且不超过5MB的图片` : "";
}
function select(row) {
  if (busy.value || reason(row)) return;
  emit("selected", props.kind === "image" ? { ...row, image_asset_id: row.id, id: row.file_id } : row);
  emit("update:visible", false);
}
function resetPicker() {
  if (!props.visible) {
    sequence++;
    return;
  }
  page.value = 1;
  search.value = "";
  rows.value = [];
  total.value = 0;
  load();
}
watch(() => [props.visible, props.kind, props.category], resetPicker);
onMounted(() => { if (props.visible) resetPicker(); });
</script>
<template>
  <ResourceOverlay
    form-actions
    :visible="visible"
    :title="titles[kind] || '选择资源'"
    :width="960"
    :footer="false"
    @update:visible="emit('update:visible', $event)"
  >
    <ResourceFilterBar
      ><label class="wt-filter-field wt-filter-field--wide"
        ><span class="wt-filter-field__label">搜索名称或 ID</span><t-input
          v-model="search"
          aria-label="搜索资源"
          @enter="
            page = 1;
            load();
          "
      /></label>
      <div class="wt-filter-actions">
        <t-button
          theme="primary"
          @click="
            page = 1;
            load();
          "
          >查询</t-button
        ><t-button
          variant="outline"
          @click="
            search = '';
            page = 1;
            load();
          "
          >重置</t-button
        >
      </div></ResourceFilterBar
    >
    <FeedbackAlert v-if="error" theme="error" :message="error" />
    <div class="table-scroll-wrap resource-data-table resource-embedded-table is-flush">
      <t-table
        class="wt-resource-table"
        :data="rows"
        :columns="columns"
        :row-key="batch && kind === 'output' ? 'composite_output_id' : 'id'"
        :loading="busy"
        empty="没有符合当前范围的资源"
        :scroll="{ x: '800px' }"
      >
        <template #name="{ row }">
          <span v-if="batch && kind === 'account'">{{ row.name }}</span>
          <t-button v-else-if="kind === 'account'" theme="primary" variant="text" size="small" @click.stop="emit('view-account', String(row.id))">{{ row.platform_name || "未识别账号" }}</t-button>
          <template v-else>{{ row.name || row.file_name || "未命名" }}</template>
        </template>
        <template #scope="{ row }"
          ><span v-if="batch && kind === 'account'">本人账号</span><span v-else-if="kind === 'account'"
            >{{ row.user_name || "—" }} ·
            {{ row.platform_account_id || "未识别" }}</span><span v-else-if="kind === 'output'"
            ><template v-if="!batch">类目 {{ row.category_id }} · </template>
            {{
              row.video?.status === OutputVideoStatus.Ready ? "视频可读" : "视频不可用"
            }}</span><img
            v-else-if="!row.deleted_at"
            :src="files.resourceURL(row)"
            alt="封面预览"
            class="picker-cover"
        /></template>
        <template #op="{ row }"
          ><span v-if="reason(row)" class="picker-reason">{{
            reason(row)
          }}</span><ResourceRowActions v-else><t-button
            :disabled="busy"
            size="small"
            variant="text"
            @click="select(row)"
            >选择</t-button
          ></ResourceRowActions></template
        >
      </t-table>
    </div>
    <div class="pagination-bar">
      <t-pagination
        :current="page"
        :page-size="10"
        :total="total"
        :show-jumper="false"
        :show-page-size="false"
        @current-change="
          page = $event;
          load();
        "
      />
    </div>
  </ResourceOverlay>
</template>
<style scoped>
.picker-cover {
  width: 72px;
  height: 45px;
  object-fit: contain;
}
.picker-reason {
  color: var(--wt-text-secondary);
  font-size: 13px;
}
</style>
