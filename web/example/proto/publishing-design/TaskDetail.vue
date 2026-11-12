<script setup>
import { computed } from "vue";
import { displayStatus } from "./taskExperience.js";
import { publicWork } from "./taskActions.js";
const props = defineProps({ task: Object, db: Object });
const emit = defineEmits(["batch", "preview"]);
const account = computed(() =>
  props.db.accounts.find((a) => a.id === props.task.accountId),
);
const sourceAsset = computed(() =>
  props.db.assets.find((asset) => asset.id === props.task.assetId),
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
const date = (value) => {
  if (!value) return "尚无记录";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime())
    ? "尚无记录"
    : parsed.toLocaleString("zh-CN", { hour12: false });
};
const progress = computed(() => {
  const task = props.task;
  const stopped = ["已取消", "已放弃"].includes(task.status);
  const failed = task.status === "执行失败";
  const current = stopped
    ? 3
    : failed
      ? task.submittedAt || task.platformId
        ? 2
        : 1
      : ({
          待执行: 0,
          排队中: 1,
          执行中: 1,
          等待人工: 1,
          结果待确认: 2,
          平台审核中: 2,
          已发布: 3,
        }[task.status] ?? 0);
  return ["准备内容", "执行任务", "确认结果", "发布结果"].map(
    (label, index) => ({
      label,
      current: index === current,
      complete: !stopped && (index < current || task.status === "已发布"),
      tone:
        index === current
          ? stopped
            ? "stopped"
            : failed
              ? "error"
              : ["等待人工", "结果待确认", "平台审核中"].includes(task.status)
                ? "waiting"
                : "active"
          : "",
      description: index === current ? displayStatus(task) : "",
    }),
  );
});
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
      <h2>{{ task.title || task.description }}</h2>
    </header>
    <ol class="v4-progress" aria-label="发布进度">
      <li
        v-for="(step, index) in progress"
        :key="step.label"
        :class="[
          { 'is-complete': step.complete, 'is-current': step.current },
          step.tone,
        ]"
        :aria-current="step.current ? 'step' : undefined"
      >
        <span class="v4-step-number" aria-hidden="true">{{
          step.complete ? "✓" : step.tone === "error" ? "!" : index + 1
        }}</span>
        <div>
          <strong>{{ step.label }}</strong
          ><span v-if="step.description" class="v4-step-description">{{
            step.description
          }}</span>
        </div>
      </li>
    </ol>
    <section>
      <h3>任务摘要</h3>
      <dl class="v4-summary">
        <div>
          <dt>任务 ID</dt>
          <dd>{{ task.id }}</dd>
        </div>
        <div>
          <dt>任务状态</dt>
          <dd>
            <t-tag
              variant="light"
              :theme="
                task.status === '已发布'
                  ? 'success'
                  : task.status === '执行失败'
                    ? 'danger'
                    : ['结果待确认', '等待人工', '平台审核中'].includes(
                          task.status,
                        )
                      ? 'warning'
                      : task.status === '执行中'
                        ? 'primary'
                        : 'default'
              "
              >{{ displayStatus(task) }}</t-tag
            >
          </dd>
        </div>
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
          <dd>
            <t-button
              v-if="sourceAsset"
              class="v4-asset-link"
              variant="text"
              :aria-label="`查看来源成片 ${sourceAsset.id}`"
              @click="emit('preview', { asset: sourceAsset })"
              >{{ sourceAsset.id }}</t-button
            ><span v-else>{{ task.assetId }} · 成片记录缺失</span>
          </dd>
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
        <dl class="v4-content-fields">
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
      <p v-if="task.failedStep">失败步骤：{{ task.failedStep }}</p>
      <p v-if="task.status === '等待人工' && task.evidence">
        处理原因：{{ task.evidence }}
      </p>
      <p v-if="task.status === '执行中'">
        当前阶段：{{ task.log?.at(-1)?.text || "等待执行器反馈" }}
      </p>
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
  margin: 12px 0 24px;
}
.v4-detail h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  overflow-wrap: anywhere;
}
.v4-progress {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  list-style: none;
  padding: 0;
  margin: 0 0 16px;
  gap: 24px;
}
.v4-progress li {
  position: relative;
  display: flex;
  gap: 10px;
  min-width: 0;
  color: var(--wt-color-text-assist);
}
.v4-progress li:not(:last-child)::after {
  content: "";
  position: absolute;
  top: 14px;
  left: 124px;
  right: -14px;
  height: 1px;
  background: var(--wt-color-border-control);
}
.v4-progress strong {
  display: block;
  font-size: 14px;
  line-height: 28px;
  font-weight: 500;
}
.v4-step-number {
  width: 28px;
  height: 28px;
  border: 1px solid var(--wt-color-border-control);
  border-radius: 50%;
  display: grid;
  place-items: center;
  flex: none;
  font-size: 13px;
  line-height: 1;
  box-sizing: border-box;
}
.v4-step-description {
  display: block;
  font-size: 12px;
  margin-top: 3px;
}
.v4-progress .is-complete,
.v4-progress .active {
  color: var(--wt-color-primary);
}
.v4-progress .is-complete .v4-step-number {
  border-color: var(--wt-color-primary);
  background: var(--wt-color-primary-light);
}
.v4-progress .active .v4-step-number {
  color: white;
  background: var(--wt-color-primary);
  border-color: var(--wt-color-primary);
}
.v4-progress .waiting {
  color: var(--wt-color-warning-text);
}
.v4-progress .waiting .v4-step-number {
  border-color: var(--wt-color-warning-border);
  background: var(--wt-color-warning-bg);
}
.v4-progress .error {
  color: var(--wt-color-danger-text);
}
.v4-progress .error .v4-step-number {
  border-color: var(--wt-color-danger-border);
  background: var(--wt-color-danger-bg);
}
.v4-progress .stopped {
  color: var(--wt-color-text-secondary);
}
.v4-detail section {
  margin: 32px 0 0;
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
.v4-summary,
.v4-content-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px 32px;
  margin: 0;
}
.v4-summary > div,
.v4-content-fields > div {
  display: grid;
  grid-template-columns: 104px minmax(0, 1fr);
  align-items: baseline;
  gap: 16px;
  min-width: 0;
}
dt {
  font-size: 13px;
  color: var(--wt-color-text-secondary);
}
dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-size: 14px;
}
.v4-asset-link {
  padding: 0 4px;
  margin: -4px;
  height: 28px;
  font-size: 14px;
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
@media (max-width: 900px) {
  .v4-content {
    grid-template-columns: 180px minmax(0, 1fr);
    gap: 24px;
  }
  .v4-content-fields > div {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
@media (max-width: 650px) {
  .v4-content {
    grid-template-columns: 1fr;
  }
  .v4-cover {
    width: 180px;
    max-width: 100%;
  }
  .v4-summary {
    grid-template-columns: 1fr;
  }
  .v4-content-fields > div {
    grid-template-columns: 1fr;
  }
  .v4-progress {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 16px;
  }
  .v4-progress li::after {
    display: none;
  }
}
</style>
