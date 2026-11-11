<script setup>
import { computed } from "vue";
import { displayStatus } from "./taskExperience.js";
import { publicWork } from "./taskActions.js";
const props = defineProps({ task: Object, db: Object });
const emit = defineEmits(["batch", "preview"]);
const account = computed(() =>
  props.db.accounts.find((a) => a.id === props.task.accountId),
);
const batch = computed(() =>
  props.db.batches.find((b) => b.id === props.task.batchId),
);
const latest = computed(() => props.task.attemptHistory?.at(-1));
const content = computed(() =>
  props.task.status === "待执行"
    ? props.task
    : latest.value?.snapshot || props.task.snapshot || props.task,
);
const work = computed(() => {
  try {
    return publicWork(
      props.task.platform,
      props.task.workUrl || props.task.evidence || "",
    ).url;
  } catch {
    return "";
  }
});
const date = (v) =>
  v ? new Date(v).toLocaleString("zh-CN", { hour12: false }) : "尚无记录";
</script>
<template>
  <article class="v4-detail">
    <nav>
      发布任务 /
      <t-button v-if="batch" variant="text" @click="emit('batch', batch)">{{
        batch.name
      }}</t-button
      ><span v-else>独立任务</span>
    </nav>
    <header>
      <div>
        <h2>{{ task.title || task.description }}</h2>
        <p>任务 #{{ task.id }}</p>
      </div>
      <t-tag
        variant="light"
        :theme="
          task.status === '已发布'
            ? 'success'
            : ['执行失败', '结果待确认', '等待人工'].includes(task.status)
              ? 'warning'
              : 'primary'
        "
        >{{ displayStatus(task) }}</t-tag
      >
    </header>
    <section class="v4-state">
      <strong>{{ displayStatus(task) }}</strong>
      <p>
        {{
          task.status === "平台审核中"
            ? "平台已接收，尚未公开；不计入已发布目标。"
            : task.status === "已取消"
              ? "任务已终止，资源占用仍保留；原任务不能恢复。"
              : task.status === "已放弃"
                ? "原任务已终止，资源已释放。"
                : task.evidence || "任务内容已保存，等待下一步操作。"
        }}
      </p>
      <small v-if="task.status === '执行中'"
        >当前阶段：{{ task.log?.at(-1)?.text || "等待执行器反馈" }}</small
      >
    </section>
    <section>
      <h3>任务摘要</h3>
      <dl class="v4-summary">
        <div>
          <dt>发布平台 / 类目</dt>
          <dd>{{ task.platform }} · {{ task.category }}</dd>
        </div>
        <div>
          <dt>发布账号</dt>
          <dd>{{ account?.name || "账号记录缺失" }}</dd>
        </div>
        <div>
          <dt>来源成片</dt>
          <dd>#{{ task.assetId }}</dd>
        </div>
        <div>
          <dt>创建时间</dt>
          <dd>{{ date(task.createdAt || task.created) }}</dd>
        </div>
        <div>
          <dt>最近更新</dt>
          <dd>
            {{
              date(task.updatedAt || task.verifiedAt || task.log?.at(-1)?.time)
            }}
          </dd>
        </div>
      </dl>
    </section>
    <section>
      <div class="v4-section-title">
        <h3>发布内容</h3>
        <small
          >内容版本
          {{ content.version || latest?.version || task.version || 1 }}</small
        >
      </div>
      <div class="v4-content">
        <button
          class="v4-cover"
          @click="emit('preview', { src: content.cover, title: '发布封面' })"
        >
          <img :src="content.cover" alt="发布封面" />
        </button>
        <dl>
          <div v-if="task.platform !== '百家号'">
            <dt>标题</dt>
            <dd>{{ content.title }}</dd>
          </div>
          <div>
            <dt>正文 / 简介</dt>
            <dd class="v4-body">
              {{ content.description || "未填写（选填）" }}
            </dd>
          </div>
          <div>
            <dt>封面方式</dt>
            <dd>
              {{ content.coverTreatment || content.coverMode || "素材原封面" }}
            </dd>
          </div>
          <div v-if="content.shortTitle">
            <dt>封面短语</dt>
            <dd>{{ content.shortTitle }}</dd>
          </div>
          <div>
            <dt>{{ task.platform === "B站" ? "标签" : "话题" }}</dt>
            <dd>{{ content.tags || content.topics || "未填写（选填）" }}</dd>
          </div>
          <div>
            <dt>创作声明</dt>
            <dd>{{ content.declaration || "未填写" }}</dd>
          </div>
          <div v-if="task.platform === 'B站'">
            <dt>分区</dt>
            <dd>{{ content.partition || "未填写" }}</dd>
          </div>
          <div>
            <dt>可见范围</dt>
            <dd>{{ content.visibility || "公开" }}</dd>
          </div>
        </dl>
      </div>
    </section>
    <section
      v-if="
        [
          '已发布',
          '平台审核中',
          '结果待确认',
          '执行失败',
          '已取消',
          '已放弃',
        ].includes(task.status)
      "
    >
      <h3>平台记录与关联作品</h3>
      <p v-if="work">
        <a :href="work" target="_blank" rel="noopener noreferrer">{{ work }}</a>
      </p>
      <p v-else>
        {{
          task.status === "已发布"
            ? "历史记录缺少有效作品链接，无法打开作品。"
            : "尚无公开作品链接。"
        }}
      </p>
      <p v-if="task.workId">已关联作品 #{{ task.workId }}</p>
      <p v-if="task.publicAt">公开时间：{{ date(task.publicAt) }}</p>
      <p v-if="task.cancelReason">取消原因：{{ task.cancelReason }}</p>
      <p v-if="task.evidence">平台反馈：{{ task.evidence }}</p>
      <details v-if="task.verifications?.length">
        <summary>核实记录</summary>
        <p v-for="(v, i) in task.verifications" :key="i">
          {{ v.actor }} · {{ date(v.at) }} · 尝试 {{ v.attempt }} ·
          {{ displayStatus({ status: v.status }) }} · {{ v.evidence }}
        </p>
      </details>
    </section>
    <section>
      <h3>执行历史</h3>
      <p v-if="!task.attemptHistory?.length">任务尚未执行</p>
      <details v-for="a in task.attemptHistory || []" :key="a.number">
        <summary>
          第 {{ a.number }} 次 · 内容版本 {{ a.version || 1 }} ·
          {{ displayStatus(a) }}
        </summary>
        <p>{{ date(a.started) }} — {{ date(a.ended) }}</p>
        <p>{{ a.receipt || "尚无平台反馈" }}</p>
        <p>执行快照：{{ a.snapshot?.title || a.snapshot?.description }}</p>
        <p v-for="(s, i) in a.steps || []" :key="i">
          {{ date(s.time) }} · {{ displayStatus(s) }} · {{ s.evidence }}
        </p>
      </details>
      <div v-if="task.log?.length" class="v4-history">
        <p v-for="(l, i) in task.log" :key="i">
          <time>{{ date(l.time) }}</time
          >{{ l.text.replaceAll("执行失败", "发布失败") }}
        </p>
      </div>
    </section>
  </article>
</template>
<style scoped>
.v4-detail {
  color: #26364e;
}
.v4-detail nav {
  font-size: 13px;
  color: #79869a;
}
.v4-detail header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 16px 0 24px;
}
.v4-detail h2 {
  font-size: 24px;
  margin: 0;
}
.v4-detail header p {
  color: #7b889a;
  margin: 6px 0;
}
.v4-detail section {
  background: white;
  border: 1px solid #e4eaf3;
  border-radius: 9px;
  padding: 24px;
  margin: 0 0 18px;
  box-shadow: 0 3px 12px #25466c04;
}
.v4-detail h3 {
  font-size: 18px;
  margin: 0 0 18px;
  color: #1e3558;
}
.v4-state {
  background: #f0f5ff !important;
}
.v4-state p {
  margin: 8px 0;
  color: #61718b;
}
.v4-summary {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 20px;
  margin: 0;
}
.v4-summary div {
  border-right: 1px solid #edf0f5;
  padding-right: 12px;
}
dt {
  font-size: 12px;
  color: #76849b;
}
dd {
  margin: 6px 0 0;
  overflow-wrap: anywhere;
}
.v4-section-title {
  display: flex;
  justify-content: space-between;
}
.v4-section-title small {
  color: #8794a7;
}
.v4-content {
  display: grid;
  grid-template-columns: minmax(230px, 32%) 1fr;
  gap: 28px;
}
.v4-cover {
  border: 0;
  padding: 0;
  background: transparent;
  cursor: zoom-in;
  align-self: start;
}
.v4-cover img {
  width: 100%;
  border-radius: 8px;
}
.v4-content dl {
  margin: 0;
}
.v4-content dl > div {
  display: grid;
  grid-template-columns: 95px 1fr;
  padding: 11px 0;
  border-bottom: 1px solid #edf0f6;
}
.v4-content dd {
  margin: 0;
  font-size: 14px;
}
.v4-body {
  white-space: pre-wrap;
}
.v4-history p {
  display: flex;
  gap: 24px;
  border-bottom: 1px solid #edf1f6;
  padding: 10px 0;
  font-size: 13px;
}
.v4-history time {
  color: #7c899c;
  flex-shrink: 0;
}
details {
  padding: 10px 0;
  font-size: 13px;
}
summary {
  cursor: pointer;
}
a {
  color: #2c58cb;
}
@media (max-width: 1100px) {
  .v4-summary {
    grid-template-columns: repeat(3, 1fr);
  }
}
@media (max-width: 650px) {
  .v4-content {
    grid-template-columns: 1fr;
  }
  .v4-summary {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
