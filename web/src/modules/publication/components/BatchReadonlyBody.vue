<script setup>
import { ref, onMounted } from "vue";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceDescriptions from "../../../shared/ui/resource/ResourceDescriptions.vue";
import BatchExecutionDetail from "./BatchExecutionDetail.vue";
import BatchFormalTasks from "./BatchFormalTasks.vue";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import {
  useBatchResourceCatalog,
  batchAccountName,
  batchOutputName,
} from "../useBatchResourceCatalog.js";
import { createFilesClient } from "../../../shared/api/files.js";
import { formatPublicationTime } from "../dateTime.js";
const props = defineProps({
  batch: Object,
  categoryName: Function,
  taskLink: Function,
});
const catalog = useBatchResourceCatalog(),
  files = createFilesClient();
function cover(row, key) {
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
  records = ref([]),
  error = ref("");
onMounted(async () => {
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
  try {
    records.value = (
      await createPublicationBatchClient().records(props.batch.id, {
        page: 1,
        page_size: 10,
      })
    ).items;
  } catch (e) {
    error.value = e.message;
  }
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
            { label: '发布平台', value: '百家号' },
            { label: '所属计划', value: batch.plan_name || '—' },
          ]"
          :key="item.label"
          :label="item.label"
          >{{ item.value }}</t-descriptions-item
        ></ResourceDescriptions
      ></ResourceSection
    ></ResourceCard
  ><BatchExecutionDetail
    v-if="batch.execution?.first_started_at"
    :batch="batch"
  /><ResourceCard v-else
    ><ResourceSection title="候选配置"
      ><div class="plan-table">
        <table>
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
              <td>{{ r.content.description || "待完善" }}</td>
              <td>
                {{
                  catalog.accounts.has(r.media_account_id)
                    ? batchAccountName(catalog.accounts.get(r.media_account_id))
                    : "账号ID：" + r.media_account_id
                }}
              </td>
              <td>
                {{
                  catalog.outputs.has(r.composite_output_id)
                    ? batchOutputName(
                        catalog.outputs.get(r.composite_output_id),
                      )
                    : "成片ID：" + r.composite_output_id
                }}
              </td>
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
      <p>正式发布任务 0 条</p>
      <ResourceDescriptions
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
        @scope="scope = $event" /></ResourceSection></ResourceCard
  ><ResourceCard
    ><ResourceSection title="操作记录"
      ><p v-if="error" role="alert">{{ error }}</p>
      <div v-for="r in records" :key="r.id" class="plan-record">
        {{ formatPublicationTime(r.created_at) }} · {{ r.event }}
      </div></ResourceSection
    ></ResourceCard
  >
</template>
