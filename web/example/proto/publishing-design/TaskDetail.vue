<script setup>
import { computed } from "vue";
import { displayStatus, taskPresentation } from "./taskExperience.js";
import { publicWork } from "./taskActions.js";
const props = defineProps({ task: Object, db: Object });
const emit = defineEmits(["batch", "preview"]);
const account = computed(() =>
  props.db.accounts.find((a) => a.id === props.task.accountId),
);
const batch = computed(() =>
  props.db.batches.find((b) => b.id === props.task.batchId),
);
const presentation = computed(() => taskPresentation(props.task, props.db));
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
    <section class="v4-state" :data-state="task.status">
      <strong>{{ displayStatus(task) }}</strong>
      <p>
        {{
          task.status === "平台审核中"
            ? "平台已接收，尚未公开；不计入已发布目标。"
            : task.status === "已取消"
              ? "任务已终止，资源占用仍保留；原任务不能恢复。"
              : task.status === "已放弃"
                ? "原任务已终止，资源已释放。"
                : task.evidence || presentation.description
        }}
      </p>
      <p v-if="task.status === '待执行'" class="v4-checks">
        {{
          presentation.checks.length
            ? "执行前需完善：" + presentation.checks.join("；")
            : "执行前检查已通过，可启动任务。"
        }}
      </p>
      <small v-if="task.failedStep">失败步骤：{{ task.failedStep }}</small>
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
            <dd>
              {{
                (content.declaration || "未填写").replace("（原型设置）", "")
              }}
            </dd>
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
      <p v-if="!task.attemptHistory?.length">
        {{
          task.status === "待执行" && !task.attempts
            ? "任务尚未执行"
            : "暂无已保存的执行记录"
        }}
      </p>
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
  color: var(--wt-color-text-primary);
  font-size: 14px;
  line-height: 1.7;
}
.v4-detail nav {
  font-size: 12px;
  color: var(--wt-color-text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
}
.v4-detail nav :deep(.t-button) {
  height: 28px;
  padding: 0 6px;
  font-size: 12px;
  color: var(--wt-color-primary);
}
.v4-detail header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 12px 0 20px;
}
.v4-detail h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  overflow-wrap: anywhere;
}
.v4-detail header p {
  color: var(--wt-color-text-secondary);
  font-size: 12px;
  margin: 4px 0 0;
}
.v4-detail section {
  padding: 24px 0 0;
  margin: 24px 0 0;
  border-top: 1px solid var(--wt-color-border);
}
.v4-detail h3 {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 18px;
}
.v4-detail h3::before {
  content: "";
  width: 3px;
  height: 14px;
  background: var(--wt-color-primary);
  border-radius: 2px;
  flex: none;
}
.v4-detail .v4-state {
  padding: 12px 16px;
  margin: 0;
  background: var(--wt-color-bg-subtle);
  border: 1px solid var(--wt-color-border);
  border-radius: 4px;
}
.v4-state strong {
  font-size: 14px;
  font-weight: 500;
}
.v4-state p {
  margin: 4px 0 0;
  color: var(--wt-color-text-secondary);
  font-size: 13px;
}
.v4-state[data-state="结果待确认"],
.v4-state[data-state="等待人工"] {
  background: var(--wt-color-warning-bg);
  border-color: var(--wt-color-warning-border);
}
.v4-state[data-state="执行失败"] {
  background: var(--wt-color-danger-bg);
  border-color: var(--wt-color-danger-border);
}
.v4-state[data-state="已发布"] {
  background: var(--wt-color-success-bg);
  border-color: var(--wt-color-success-border);
}
.v4-state small {
  display: block;
  margin-top: 6px;
  color: var(--wt-color-text-secondary);
}
.v4-summary {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 20px;
  margin: 0;
}
dt {
  font-size: 13px;
  color: var(--wt-color-text-secondary);
}
dd {
  margin: 6px 0 0;
  overflow-wrap: anywhere;
  font-size: 13px;
}
.v4-section-title {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 16px;
}
.v4-section-title small {
  color: var(--wt-color-text-assist);
  font-size: 12px;
}
.v4-content {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 28px;
}
.v4-cover {
  border: 1px solid var(--wt-color-border);
  border-radius: 4px;
  padding: 0;
  background: var(--wt-color-bg-subtle);
  cursor: zoom-in;
  align-self: start;
  overflow: hidden;
}
.v4-cover img {
  display: block;
  width: 100%;
  max-height: 260px;
  object-fit: contain;
}
.v4-cover:focus-visible {
  outline: 2px solid var(--wt-color-primary);
  outline-offset: 3px;
}
.v4-content dl {
  margin: 0;
}
.v4-content dl > div {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 12px;
  padding: 10px 0;
}
.v4-content dl > div:first-child {
  padding-top: 0;
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
  border-bottom: 1px solid var(--wt-color-border);
  padding: 10px 0;
  font-size: 13px;
}
.v4-history time {
  color: var(--wt-color-text-secondary);
  flex-shrink: 0;
}
details {
  padding: 12px 0;
  font-size: 13px;
  border-bottom: 1px solid var(--wt-color-border);
}
summary {
  cursor: pointer;
  color: var(--wt-color-primary);
}
a {
  color: var(--wt-color-primary);
  overflow-wrap: anywhere;
}
@media (max-width: 1100px) {
  .v4-summary {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 650px) {
  .v4-content {
    grid-template-columns: 1fr;
  }
  .v4-cover {
    width: 220px;
    max-width: 100%;
  }
  .v4-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .v4-detail header {
    align-items: flex-start;
  }
}
</style>
