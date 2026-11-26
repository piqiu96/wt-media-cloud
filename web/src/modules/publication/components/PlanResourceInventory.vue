<script setup>
import { computed, ref, watch, onBeforeUnmount } from "vue";
import { platformLabel } from "../../../shared/utils/platforms.js";
import ResourceCard from "../../../shared/ui/resource/ResourceCard.vue";
import ResourceSection from "../../../shared/ui/resource/ResourceSection.vue";
import ResourceNotice from "../../../shared/ui/resource/ResourceNotice.vue";
import ResourceStatGrid from "../../../shared/ui/resource/ResourceStatGrid.vue";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import { formatPublicationTime } from "../dateTime.js";
const props = defineProps({
  goals: Array,
  categoryName: Function,
  disabled: Boolean,
});
const api = createPublicationBatchClient(),
  rows = ref({}),
  loading = ref(false);
let generation = 0;
const key = (g) => g.platform + ":" + g.category_id;
const all = computed(() =>
  props.goals.map((g) => ({ ...g, resource: rows.value[key(g)] })),
);
// The existing endpoint supplies resource counts, not platform quotas or per-account limits.
const known = (r) => r && !r.error && !!r.as_of;
const total = (field) =>
  all.value.length && all.value.every((g) => known(g.resource))
    ? all.value.reduce((n, g) => n + Number(g.resource[field] || 0), 0)
    : "—";
const stats = computed(() => [
  {
    key: "accounts",
    label: "本人可用账号",
    value: "按类目查看",
    description: "同一账号可覆盖多个类目，不重复汇总",
  },
  {
    key: "outputs",
    label: "可用成片",
    value: total("available_output_count"),
    unit: "条",
  },
  {
    key: "unknown",
    label: "状态未知成片",
    value: total("unknown_output_count"),
    unit: "条",
  },
  {
    key: "capacity",
    label: "可安排量",
    value: "容量未知",
    description: "待逐批配置并核实真实额度",
  },
]);
async function inspect() {
  const run = ++generation;
  loading.value = true;
  rows.value = {};
  const values = await Promise.all(
    props.goals.map(async (g) => {
      try {
        return [
          key(g),
          await api.resources({
            platform: g.platform,
            category_id: g.category_id,
          }),
        ];
      } catch {
        return [key(g), { error: true }];
      }
    }),
  );
  if (run === generation) {
    rows.value = Object.fromEntries(values);
    loading.value = false;
  }
}
watch(() => props.goals.map(key).sort().join(","), inspect, {
  immediate: true,
});
onBeforeUnmount(() => generation++);
</script>
<template>
  <ResourceCard class="plan-editor-section"
    ><ResourceSection
      title="资源盘点" variant="form"
      description="盘点仅查看本人资源，不锁定账号或成片。账号可覆盖多个类目并顺序发布，类目账号数不可直接相加。"
    >
      <template #title-extra><span class="plan-section-number" aria-hidden="true">02</span></template>
      <div class="plan-section-tools">
        <t-button
          variant="outline"
          :loading="loading"
          :disabled="disabled"
          @click="inspect"
          >重新盘点</t-button
        >
      </div>
      <ResourceStatGrid :items="stats" />
      <div
        v-if="all.length"
        class="plan-detail-table resource-data-table resource-embedded-table is-summary"
        role="region"
        aria-label="本人资源盘点，可横向滚动"
        tabindex="0"
      >
        <table>
          <thead>
            <tr>
              <th>平台 / 类目</th>
              <th class="resource-table-number">可用账号</th>
              <th class="resource-table-number">可用成片</th>
              <th class="resource-table-number">状态未知</th>
              <th>可安排量</th>
              <th>盘点时间 / 提示</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="g in all" :key="key(g)">
              <td>
                {{ platformLabel(g.platform) }}<small>{{ categoryName(g.category_id) }}</small>
              </td>
              <template v-if="known(g.resource)"
                ><td class="resource-table-number">
                  {{ g.resource.eligible_account_count }} /
                  {{ g.resource.account_count }}
                </td>
                <td class="resource-table-number">
                  {{ g.resource.available_output_count }}
                </td>
                <td class="resource-table-number">
                  {{ g.resource.unknown_output_count }}
                </td>
                <td>待逐批配置核实</td>
                <td>
                  {{ formatPublicationTime(g.resource.as_of)
                  }}<small v-if="!g.resource.eligible_account_count"
                    >暂无可用账号，当前无法开始发布</small
                  ><small v-if="g.resource.unknown_output_count"
                    >部分成片状态未知，需核实后安排</small
                  ><small
                    v-if="g.target_count > g.resource.available_output_count"
                    >素材缺口
                    {{ g.target_count - g.resource.available_output_count }}
                    条（目标保持 {{ g.target_count }} 条）</small
                  ><small v-if="g.resource.gaps?.length">{{
                    g.resource.gaps
                      .map(
                        (v) =>
                          ({
                            eligible_account_missing: "缺少可用账号",
                            available_output_missing: "缺少可用成片",
                            output_availability_unknown: "成片可用状态未知",
                          })[v] || "资源需核实",
                      )
                      .join("；")
                  }}</small>
                </td></template
              >
              <td v-else colspan="5">
                {{
                  g.resource?.error
                    ? "盘点读取失败，请重新盘点"
                    : loading
                      ? "正在盘点"
                      : "可用量未知，请重新盘点"
                }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <ResourceNotice v-else
        >选择目标类目后显示对应本人资源。目标允许超过当前库存，计划目标不会随盘点自动减少。</ResourceNotice
      >
    </ResourceSection></ResourceCard
  >
</template>
