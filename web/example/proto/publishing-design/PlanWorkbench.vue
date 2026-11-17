<script setup>
import { computed } from "vue";
import {
  batchTasks,
  itemRecord,
  planTasks,
  targetTotal,
  decision,
  batchSummary,
} from "./relationships.js";
const props = defineProps({ db: Object, plan: Object });
const emit = defineEmits(["batch", "edit"]);
const groups = computed(() =>
  (props.plan.platformOrder || [])
    .map((platform) => ({
      platform,
      batches: props.plan.items
        .map((i) => itemRecord(props.db, i))
        .filter((b) => b?.platform === platform),
    }))
    .filter((g) => g.batches.length),
);
const current = computed(() =>
  itemRecord(props.db, props.plan.items[props.plan.position]),
);
</script>
<template>
  <section>
    <t-alert
      :theme="plan.lockedAt ? 'warning' : 'info'"
      :message="
        plan.lockedAt
          ? '首次合法启动已锁定结构 · ' + plan.lockedAt
          : '未首次启动；编辑前检查各批候选与正式任务影响'
      "
    />
    <p>
      已创建 {{ planTasks(db, plan).length }} · 已处理批次 {{ plan.position }} /
      {{ plan.items.length }} · 当前 {{ current?.name || "无" }} · 下一批
      {{ itemRecord(db, plan.items[plan.position + 1])?.name || "无" }}
    </p>
    <h3>按平台分组的批次顺序</h3>
    <section class="group" v-for="g in groups" :key="g.platform">
      <h3>{{ g.platform }}</h3>
      <div class="batch-row" v-for="b in g.batches" :key="b.id">
        <div>
          <strong>{{ b.name }}</strong>
          <p>
            {{ b.targets.map((t) => t.category + " " + t.count).join(" / ") }} ·
            目标 {{ targetTotal(b) }} · 正式 {{ batchTasks(db, b).length }} ·
            已公开 {{ batchSummary(db, b).published }}
          </p>
          <small
            >{{ b.phase }} / {{ b.control }} · 创建缺口
            {{ batchSummary(db, b).creationGap }}</small
          >
        </div>
        <t-button variant="outline" @click="emit('batch', b)">{{
          b.phase === "待资源配置"
            ? "配置资源"
            : b.phase === "待内容配置"
              ? "配置内容"
              : b.control === "执行已结束"
                ? "查看结果"
                : "进入工作台"
        }}</t-button>
      </div>
    </section>
    <t-alert
      v-if="current"
      theme="info"
      :message="
        '当前批次衔接：' +
        (current.phase !== '内容预检通过'
          ? '等待当前批次配置'
          : decision(db, plan)) +
        '；下一批配置就绪不自动取得调度权。'
      "
    />
    <p>
      计划只管理批次，不绑定独立任务。审核中不算公开目标；迟到审核事实不会回滚游标。补量请另建计划。
    </p>
    <t-button
      variant="outline"
      :disabled="
        !!plan.lockedAt || ['已取消', '执行已结束'].includes(plan.control)
      "
      @click="emit('edit', plan)"
      >编辑未启动计划（影响检查）</t-button
    >
    <details>
      <summary>调度权与操作记录</summary>
      <p v-for="(l, i) in plan.log" :key="i">
        {{ l.time }} · {{ l.type }} {{ l.batchId || "" }}
      </p>
    </details>
  </section>
</template>
<style scoped>
.group {
  margin: 20px 0;
  border: 1px solid #dfe7f1;
  border-radius: 8px;
  overflow: hidden;
}
.group h3 {
  padding: 14px;
  background: #f3f6fb;
  margin: 0;
}
.batch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border-top: 1px solid #e7ecf2;
}
.batch-row p,
.batch-row small {
  color: #7a879b;
  font-size: 13px;
}
@media (max-width: 700px) {
  .batch-row {
    flex-wrap: wrap;
  }
}
</style>
