<script setup>
import { ref, onMounted, onBeforeUnmount, useSlots } from "vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue";
import ResourceStatusBadge from "../../../shared/ui/resource/ResourceStatusBadge.vue";
import { batchStatus } from "../batchPresentation.js";
import { configurationLabels } from "../batchPreparation.js";
import BatchExecutionDetail from "./BatchExecutionDetail.vue";
import BatchFormalTasks from "./BatchFormalTasks.vue";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import {
  useBatchResourceCatalog,
  batchAccountName,
  batchOutputName,
} from "../useBatchResourceCatalog.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { platformLabel } from "../../../shared/utils/platforms.js";
import { formatPublicationTime } from "../dateTime.js";
const props = defineProps({
  batch: Object,
  categoryName: Function,
  taskLink: Function,
  resourceCatalog: Object,
  coverURL: Function,
});
const emit = defineEmits(["tasks"]);
const slots = useSlots();
const catalog = props.resourceCatalog || useBatchResourceCatalog(),
  files = createFilesClient();
function cover(row, key) {
  if (props.coverURL) return props.coverURL(row, key);
  const r = props.batch.readiness?.cover_resources?.find(
    (r) => r.row_key === row.row_key,
  );
  try {
    return r?.[key] ? files.resourceURL({ resource_path: r[key] }) : null;
  } catch {
    return null;
  }
}
const scope = ref({ page: 1, page_size: 10 }),
  recordPage = ref(1),
  recordTotal = ref(0),
  records = ref([]),
  error = ref("");
let recordSequence = 0;
const recordLabels = {
  "publication_batch.create": "保存批次",
  "publication_batch.configure": "保存配置修改",
  "publication_batch.start": "开始发布",
  "publication_batch.pause": "暂停发布",
  "publication_batch.resume": "继续发布",
  "publication_batch.stop": "终止发布",
  "publication_batch.finish": "结束批次",
  "publication_batch.plan_skeleton": "创建计划批次",
  created: "保存批次",
  configuration_updated: "保存配置修改",
  publication_batch_created: "保存批次",
  publication_batch_configuration_updated: "保存配置修改",
};
async function loadRecords(page = 1) {
  const run = ++recordSequence,
    id = props.batch.id;
  recordPage.value = page;
  try {
    const result = await createPublicationBatchClient().records(id, {
      page,
      page_size: 10,
    });
    if (run === recordSequence && id === props.batch.id) {
      records.value = result.items;
      recordTotal.value = result.total;
    }
  } catch (e) {
    if (run === recordSequence) error.value = e.message;
  }
}
onBeforeUnmount(() => recordSequence++);
onMounted(async () => {
  if (props.resourceCatalog && slots.records) return;
  try {
    await catalog.hydrate(
      {
        accounts: props.batch.config?.accounts || [],
        output_ids: props.batch.config?.output_ids || [],
        candidates: props.batch.config?.candidates || [],
      },
      {
        owner: props.batch.operator_id,
        platform: props.batch.platform,
        readOnly: true,
      },
    );
  } catch (e) {
    error.value = "资源名称暂不可用：" + e.message;
  }
  if (!slots.records) await loadRecords();
});
</script>
<template>
  <ResourceCard
    ><ResourceSection title="批次信息"
      ><ResourceDescriptions
        ><t-descriptions-item
          v-for="item in [
            { label: '批次名称', value: batch.name },
            { label: '批次ID', value: batch.id },
            {
              label: '配置状态',
              value:
                configurationLabels[batch.configuration_status] || '待配置',
            },
            {
              label: '执行状态',
              value: batchStatus(batch),
            },
            { label: '发布平台', value: batch.platform === 'baidu' ? '百家号' : platformLabel(batch.platform) },
            { label: '所属计划', value: batch.plan_name || '独立批次' },
            { label: '创建人 ID', value: batch.created_by || '历史记录未提供' },
            {
              label: '创建时间',
              value: formatPublicationTime(batch.created_at),
            },
            {
              label: '最近更新',
              value: formatPublicationTime(batch.updated_at),
            },
          ]"
          :key="item.label"
          :label="item.label"
          ><ResourceStatusBadge v-if="['配置状态','执行状态'].includes(item.label)" :label="item.value"/><template v-else>{{ item.value }}</template></t-descriptions-item
        ></ResourceDescriptions
      ></ResourceSection
    ></ResourceCard
  ><BatchExecutionDetail
    v-if="batch.execution?.first_started_at || batch.execution?.stopped_at"
    :batch="batch"
    @tasks="emit('tasks', $event)"
  /><ResourceCard v-if="!batch.execution?.first_started_at"
    ><ResourceSection title="候选配置"
      ><p v-if="batch.execution_status === 'not_started'" class="batch-muted">
        批次尚未开始，正式发布任务 0 条。
      </p>
      <ResourceDescriptions
        ><t-descriptions-item label="发布目标"
          >{{
            batch.targets.reduce((n, t) => n + t.target_count, 0)
          }}
          条</t-descriptions-item
        ><t-descriptions-item label="候选"
          >{{ batch.config?.candidates?.length || 0 }} 条</t-descriptions-item
        ><t-descriptions-item label="目标缺口"
          >{{
            Math.max(
              0,
              batch.targets.reduce((n, t) => n + t.target_count, 0) -
                (batch.config?.candidates?.length || 0),
            )
          }}
          条</t-descriptions-item
        ><t-descriptions-item label="正式发布任务"
          >0 条</t-descriptions-item
        ></ResourceDescriptions
      >
      <slot name="candidates"
        ><div
          class="plan-detail-table resource-data-table resource-embedded-table is-summary batch-readonly-candidates"
          role="region" aria-label="批次候选配置，可横向滚动" tabindex="0"
        >
          <table>
            <colgroup><col style="width:76px"/><col style="width:22%"/><col style="width:100px"/><col style="width:26%"/><col style="width:168px"/><col style="width:76px"/></colgroup>
            <thead>
              <tr>
                <th>类目</th>
                <th>发布文案</th>
                <th>发布账号</th>
                <th>成片</th>
                <th>封面</th>
                <th>确认</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in batch.config?.candidates || []" :key="r.row_key">
                <td>{{ categoryName(r.category_id) }}</td>
                <td><span class="batch-readonly-text" :title="r.content.description">{{ r.content.description || "待完善" }}</span></td>
                <td>
                  {{
                    catalog.accounts.has(r.media_account_id)
                      ? batchAccountName(
                          catalog.accounts.get(r.media_account_id),
                        )
                      : "账号ID：" + r.media_account_id
                  }}
                </td>
                <td><span class="batch-readonly-text" :title="batchOutputName(catalog.outputs.get(r.composite_output_id))">
                  {{
                    catalog.outputs.has(r.composite_output_id)
                      ? batchOutputName(
                          catalog.outputs.get(r.composite_output_id),
                        )
                      : "成片ID：" + r.composite_output_id
                  }}
                </span></td>
                <td>
                  <img
                    v-if="cover(r, 'cover_resource_path')"
                    :src="cover(r, 'cover_resource_path')"
                    alt="横版封面"
                    width="96"
                  /><img
                    v-if="cover(r, 'portrait_cover_resource_path')"
                    :src="cover(r, 'portrait_cover_resource_path')"
                    alt="竖版封面"
                    width="48"
                  />
                </td>
                <td>{{ r.content.confirmed ? "已确认" : "待确认" }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </slot>
    </ResourceSection>
    <ResourceSection
      v-if="batch.readiness?.shortfalls?.length"
      title="目标与缺口"
      ><ul class="batch-categories">
        <li
          v-for="entry in batch.readiness.shortfalls"
          :key="entry.category_id"
        >
          <strong>{{ categoryName(entry.category_id) }}</strong><span
            >目标 {{ entry.target_count }} · 候选 {{ entry.candidate_count }} ·
            缺口 {{ entry.shortfall }}</span
          >
        </li>
      </ul></ResourceSection
    >
    <ResourceSection v-if="batch.readiness?.issues?.length" title="配置问题"
      ><ul class="batch-issues">
        <li v-for="(issue, i) in batch.readiness.issues" :key="i">
          {{ issue.message }}
        </li>
      </ul></ResourceSection
    >
    <ResourceSection title="执行设置"
      ><ResourceDescriptions
        ><t-descriptions-item
          v-for="item in [
            {
              label: '并发数',
              value: batch.execution_setting.concurrency_limit,
            },
            {
              label: '组间发布间隔',
              value: batch.execution_setting.group_interval_seconds + '秒',
            },
            {
              label: '组间处理',
              value:
                batch.execution_setting.between_group_policy === 'auto_continue'
                  ? '自动继续'
                  : '异常暂停',
            },
            {
              label: '异常任务上限',
              value: batch.execution_setting.exception_task_limit,
            },
          ]"
          :key="item.label"
          :label="item.label"
          >{{ item.value }}</t-descriptions-item
        ></ResourceDescriptions
      ></ResourceSection
    ></ResourceCard
  ><ResourceCard v-if="batch.execution?.first_started_at"
    ><ResourceSection title="批次任务"
      ><slot name="tasks"
        ><BatchFormalTasks
          :batch="batch"
          :account-name="
            (r) => batchAccountName(catalog.accounts.get(r.media_account_id))
          "
          :output-name="
            (r) => batchOutputName(catalog.outputs.get(r.composite_output_id))
          "
          :scope="scope"
          active
          :task-link="taskLink"
          @scope="scope = $event" /></slot></ResourceSection></ResourceCard
  ><ResourceCard
    ><ResourceSection title="操作记录"
      ><slot name="records"
        ><p v-if="error" role="alert">{{ error }}</p>
        <div v-for="r in records" :key="r.id" class="plan-record">
          {{ formatPublicationTime(r.created_at) }} ·
          {{ recordLabels[r.event] || "批次业务记录" }}
        </div>
        <ResourceNotice v-if="!records.length && !error"
          >暂无操作记录</ResourceNotice
        >
        <t-pagination
          v-if="recordTotal > 10"
          :current="recordPage"
          :page-size="10"
          :total="recordTotal"
          :show-page-size="false"
          @current-change="loadRecords"
        /> </slot></ResourceSection
  ></ResourceCard>
</template>

<style scoped>
.batch-readonly-candidates table { min-width: 880px; table-layout: fixed; }
.batch-readonly-text { display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; white-space: normal; overflow-wrap: anywhere; }
.batch-readonly-candidates img { max-height: 72px; object-fit: contain; vertical-align: middle; }
.batch-readonly-candidates img + img { margin-left: 8px; }
</style>
