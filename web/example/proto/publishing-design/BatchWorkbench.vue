<script setup>
import { ref, computed } from "vue";
import {
  batchTasks,
  claimTask,
  setResult,
  unresolved,
} from "./relationships.js";
const props = defineProps({ db: Object, batch: Object });
const emit = defineEmits(["task"]);
const page = ref(1);
const running = ref(false),
  message = ref(""),
  filter = ref("全部"),
  q = ref("");
const rows = computed(() => batchTasks(props.db, props.batch));
const filtered = computed(() =>
  rows.value.filter(
    (t) =>
      (filter.value === "全部" || t.status === filter.value) &&
      (t.title + String(t.id) + String(t.accountId)).includes(q.value),
  ),
);
async function tick(all = false) {
  running.value = true;
  message.value = "";
  let processed = 0;
  try {
    do {
      const claimed = rows.value.filter((t) => claimTask(props.db, t));
      if (!claimed.length) break;
      await new Promise((r) => setTimeout(r, 500));
      for (const t of claimed) {
        if (t.status !== "执行中" || !t.claimed) continue;
        setResult(t, t.demoOutcome || "已发布", {
          evidence: "https://demo.invalid/evidence/" + t.id,
        });
        processed++;
      }
      if (!all) break;
    } while (
      rows.value.some((t) => t.status === "排队中") &&
      props.batch.control === "执行中"
    );
    message.value = `本轮演示处理 ${processed} 条；${rows.value.filter(unresolved).length} 条仍未收口。不会调用真实平台。`;
  } finally {
    running.value = false;
  }
}
</script>
<template>
  <section class="batch-workbench">
    <h3>账号队列与异常隔离</h3>
    <p>
      同账号串行，跨账号最多 3
      个在途；账号登录或未知结果只阻断该账号。未来定时项保持排队，不提前领取。
    </p>
    <div class="queue-grid">
      <div v-for="id in [...new Set(rows.map((t) => t.accountId))]" :key="id">
        <strong>{{ db.accounts.find((a) => a.id === id)?.name }}</strong>
        <p>
          {{ rows.filter((t) => t.accountId === id && unresolved(t)).length }}
          未决 / {{ rows.filter((t) => t.accountId === id).length }} 总数
        </p>
        <small>{{
          rows.some(
            (t) =>
              t.accountId === id &&
              ["等待人工", "结果待确认"].includes(t.status),
          )
            ? "账号队列阻断"
            : "可正常推进"
        }}</small>
      </div>
    </div>
    <div class="demo-buttons">
      <label
        >状态<select v-model="filter">
          <option>全部</option>
          <option
            v-for="s in [
              '待执行',
              '排队中',
              '执行中',
              '等待人工',
              '结果待确认',
              '平台审核中',
              '已发布',
              '执行失败',
              '已取消',
            ]"
            :key="s"
          >
            {{ s }}
          </option>
        </select></label
      ><t-input v-model="q" placeholder="搜索任务 / 标题 / 账号" />
    </div>
    <t-table
      row-key="id"
      :data="filtered.slice((page - 1) * 8, page * 8)"
      :columns="[
        { colKey: 'id', title: '任务', width: 90 },
        { colKey: 'title', title: '发布内容', minWidth: 180 },
        { colKey: 'status', title: '当前状态', width: 120 },
        { colKey: 'action', title: '操作', width: 100 },
      ]"
      :pagination="{
        current: page,
        pageSize: 8,
        total: filtered.length,
        showJumper: false,
        showPageSize: false,
      }"
      @page-change="(p) => (page = p.current)"
      ><template #action="{ row }"
        ><t-button variant="text" @click="emit('task', row)"
          >详情 / 处理</t-button
        ></template
      ></t-table
    >
  </section>
</template>
<style scoped>
.batch-workbench {
  margin: 24px 0;
}
.queue-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}
.queue-grid > div {
  border: 1px solid #e1e7f0;
  border-radius: 8px;
  padding: 14px;
}
.queue-grid small {
  color: #9c6c1a;
}
.demo-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  margin: 16px 0;
}
.demo-buttons > .t-input__wrap {
  max-width: 300px;
}
.demo-buttons select {
  padding: 8px;
  border: 1px solid #d8e1ed;
}
</style>
