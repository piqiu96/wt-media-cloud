<script setup>
import { computed, ref, watch, nextTick } from "vue";
import Overlay from "./PrototypeOverlay.vue";
import {
  taskPresentation,
  displayStatus,
  canEditTask,
  canCancelTask,
  canReleaseTask,
} from "./taskExperience.js";
const props = defineProps({ task: Object, db: Object });
const emit = defineEmits([
  "start",
  "edit",
  "cancel",
  "release",
  "copy",
  "preview",
  "batch",
  "verify",
  "resume",
  "transition",
]);
const presentation = computed(() => taskPresentation(props.task, props.db));
const account = computed(() =>
  props.db.accounts.find((a) => a.id === props.task.accountId),
);
const batch = computed(() =>
  props.db.batches.find((b) => b.id === props.task.batchId),
);
const plan = computed(() =>
  props.db.plans.find((p) =>
    (p.items || []).some((i) => i.kind === "batch" && i.id === batch.value?.id),
  ),
);
const latest = computed(() => props.task.attemptHistory?.at(-1));
const content = computed(() =>
  ["待执行", "排队中", "已取消", "已放弃", "执行失败"].includes(
    props.task.status,
  )
    ? props.task
    : latest.value?.snapshot || props.task.snapshot || props.task,
);
const workLink = computed(() =>
  /^https:\/\//.test(props.task.evidence || "") ? props.task.evidence : "",
);
const extras = computed(() =>
  [
    {
      content: "编辑发布内容",
      value: "edit",
      disabled: !canEditTask(props.task),
    },
    {
      content: "取消任务",
      value: "cancel",
      disabled: !canCancelTask(props.task),
    },
    {
      content: "放弃并释放",
      value: "release",
      disabled: !canReleaseTask(props.task),
    },
    {
      content: "复制配置创建新任务",
      value: "copy",
      disabled: !["已取消", "已放弃"].includes(props.task.status),
    },
  ].filter((o) => !o.disabled),
);
const history = ref(null),
  checkArea = ref(null),
  resultArea = ref(null),
  humanOpen = ref(false),
  verifyOpen = ref(false),
  verifyEvidence = ref(""),
  verifyResult = ref(""),
  verifyError = ref("");
watch(
  () => props.task.id,
  () => {
    humanOpen.value = false;
    verifyOpen.value = false;
    verifyError.value = "";
  },
);
function focusArea(el) {
  el?.scrollIntoView({ behavior: "smooth", block: "start" });
  nextTick(() => el?.focus({ preventScroll: true }));
}
function primary() {
  const action = presentation.value.action;
  if (action === "start") emit("start", props.task);
  else if (action === "human") humanOpen.value = true;
  else if (action === "verify") {
    verifyEvidence.value = props.task.evidence || "";
    verifyResult.value = "";
    verifyError.value = "";
    verifyOpen.value = true;
  } else if (action === "copy") emit("copy", props.task);
  else if (action === "work" && workLink.value)
    window.open(workLink.value, "_blank", "noopener,noreferrer");
  else
    focusArea(
      ["已发布", "已取消", "已放弃"].includes(props.task.status)
        ? resultArea.value
        : history.value,
    );
}
function confirmResult() {
  if (!verifyEvidence.value.trim()) {
    verifyError.value = "请填写核验依据，不能凭状态选择确认结果。";
    return;
  }
  if (
    verifyResult.value &&
    verifyResult.value !== props.task.status &&
    verifyEvidence.value.trim() === (props.task.evidence || "").trim()
  ) {
    verifyError.value = "请补充本次查询得到的新证据，再确认结果变化。";
    return;
  }
  if (!verifyResult.value) {
    verifyError.value = "请选择实际核实到的结果；仍不明确时保持待确认。";
    return;
  }
  if (
    verifyResult.value === "已发布" &&
    !/^https:\/\//.test(verifyEvidence.value.trim())
  ) {
    verifyError.value = "公开发布需要完整的 HTTPS 作品链接。";
    return;
  }
  emit("verify", {
    task: props.task,
    evidence: verifyEvidence.value.trim(),
    status: verifyResult.value,
  });
  verifyOpen.value = false;
}
defineExpose({ primary });
</script>
<template>
  <article class="task-detail">
    <nav class="task-breadcrumb" aria-label="任务关联">
      <span>发布任务</span
      ><template v-if="batch"
        ><span> / </span
        ><t-button variant="text" @click="emit('batch', batch)">{{
          batch.name
        }}</t-button
        ><small v-if="plan"
          >所属计划：{{ plan.name }} · 经批次关联</small
        ></template
      ><span v-else> / 独立任务</span>
    </nav>
    <header class="task-heading">
      <div>
        <p>任务 #{{ task.id }} · {{ task.platform }} · {{ task.category }}</p>
        <h2>
          {{
            task.platform === "百家号"
              ? task.description || task.title
              : task.title
          }}
        </h2>
        <small
          >{{ account?.name }} · 窗口 {{ account?.window }} · 成片 #{{
            task.assetId
          }}
          · {{ task.attempts || 0 }} 次尝试</small
        >
      </div>
      <t-tag
        :theme="
          task.status === '已发布'
            ? 'success'
            : ['结果待确认', '执行失败', '等待人工'].includes(task.status)
              ? 'warning'
              : 'primary'
        "
        variant="light"
        >{{ displayStatus(task) }}</t-tag
      >
    </header>
    <section
      class="task-next"
      :class="{
        attention: ['结果待确认', '等待人工', '执行失败'].includes(task.status),
        done: task.status === '已发布',
      }"
    >
      <div>
        <span class="task-eyebrow">当前进展与下一步</span>
        <h3>{{ presentation.title }}</h3>
        <p>{{ presentation.description }}</p>
        <small v-if="task.status === '执行失败'"
          >失败步骤：{{
            task.failedStep ||
            latest?.failedStep ||
            "平台未返回具体步骤，请查看执行记录"
          }}<br />{{
            task.evidence || latest?.receipt || "历史记录未保留错误原因"
          }}</small
        ><small v-if="task.status === '执行中'"
          >当前步骤：{{
            task.log?.at(-1)?.text ||
            latest?.steps?.at(-1)?.status ||
            "已领取，等待执行端步骤反馈"
          }}</small
        >
      </div>
      <div class="task-next-actions">
        <t-button :disabled="presentation.disabled" @click="primary">{{
          presentation.label
        }}</t-button
        ><t-dropdown
          v-if="extras.length"
          :options="extras"
          @click="({ value }) => emit(value, task)"
          ><t-button variant="outline" theme="default"
            >更多</t-button
          ></t-dropdown
        >
      </div>
    </section>
    <section
      v-if="task.status === '待执行' || task.status === '执行失败'"
      ref="checkArea"
      class="task-section task-checks"
      tabindex="-1"
    >
      <div class="task-section-title">
        <h3>发布前检查</h3>
        <t-tag
          :theme="presentation.checks.length ? 'warning' : 'success'"
          variant="light"
          >{{
            presentation.checks.length
              ? `${presentation.checks.length} 项待处理`
              : "检查通过"
          }}</t-tag
        >
      </div>
      <ul v-if="presentation.checks.length">
        <li v-for="issue in presentation.checks" :key="issue">
          <span>{{ issue }}</span
          ><t-button
            variant="text"
            @click="canEditTask(task) ? emit('edit', task) : focusArea(history)"
            >去修复</t-button
          >
        </li>
      </ul>
      <p v-else>
        账号、成片占用、必填内容与封面版本已通过检查。启动前会再次校验。
      </p>
    </section>
    <section
      v-if="
        [
          '结果待确认',
          '平台审核中',
          '已发布',
          '执行失败',
          '已取消',
          '已放弃',
        ].includes(task.status)
      "
      ref="resultArea"
      class="task-section"
      tabindex="-1"
    >
      <h3>
        {{
          ["已取消", "已放弃"].includes(task.status)
            ? "终止原因与资源占用"
            : "平台结果与证据"
        }}
      </h3>
      <dl class="task-facts">
        <div>
          <dt>当前结果</dt>
          <dd>{{ task.result }}</dd>
        </div>
        <div>
          <dt>成片占用</dt>
          <dd>
            {{ task.status === "已放弃" ? "已释放" : "保留当前平台占用" }}
          </dd>
        </div>
        <div v-if="task.publicAt">
          <dt>公开发布时间</dt>
          <dd>
            {{
              new Date(task.publicAt).toLocaleString("zh-CN", {
                timeZone: "Asia/Shanghai",
              })
            }}
          </dd>
        </div>
        <div v-if="task.submittedAt">
          <dt>提交时间</dt>
          <dd>
            {{
              new Date(task.submittedAt).toLocaleString("zh-CN", {
                timeZone: "Asia/Shanghai",
              })
            }}
          </dd>
        </div>
        <div v-if="task.platformId">
          <dt>平台稿件</dt>
          <dd>{{ task.platformId }}</dd>
        </div>
      </dl>
      <a
        v-if="workLink"
        :href="workLink"
        target="_blank"
        rel="noopener noreferrer"
        >{{ workLink }}</a
      >
      <p v-else>
        {{
          task.evidence ||
          (task.status === "已发布"
            ? "此历史样例未保留作品链接，不能据此验证真实平台公开结果。"
            : ["已取消", "已放弃"].includes(task.status)
              ? "历史样例未记录原因。"
              : "尚无平台证据，请先查询原账号稿件。")
        }}
      </p>
      <details v-if="latest?.receipt">
        <summary>查看原始平台反馈</summary>
        <pre>{{ latest.receipt }}</pre>
      </details>
    </section>
    <section class="task-section">
      <div class="task-section-title">
        <h3>
          {{
            task.status === "已发布"
              ? "最终发布内容"
              : task.status === "执行失败"
                ? "下一次发布内容"
                : ["执行中", "等待人工", "结果待确认", "平台审核中"].includes(
                      task.status,
                    )
                  ? "本次执行使用的内容"
                  : "发布内容与封面"
          }}
        </h3>
        <span
          >内容版本
          {{
            latest && !["待执行", "排队中", "执行失败"].includes(task.status)
              ? latest.version
              : task.version || 1
          }}</span
        >
      </div>
      <div class="task-content">
        <button class="task-cover" @click="emit('preview', { row: content })">
          <img :src="content.cover" alt="发布封面预览" /><span
            >查看封面与各比例效果</span
          >
        </button>
        <div>
          <h4>{{ content.platform === "百家号" ? "作品描述" : "发布标题" }}</h4>
          <p>
            {{
              content.platform === "百家号"
                ? content.description
                : content.title
            }}
          </p>
          <template v-if="content.platform !== '百家号' && content.description"
            ><h4>{{ content.platform === "B站" ? "简介" : "正文" }}</h4>
            <p>{{ content.description }}</p></template
          >
          <dl class="task-facts">
            <div v-if="content.platform === 'B站'">
              <dt>分区 / 标签</dt>
              <dd>{{ content.partition }} · {{ content.tags }}</dd>
            </div>
            <div v-if="content.topics">
              <dt>话题</dt>
              <dd>{{ content.topics }}</dd>
            </div>
            <div>
              <dt>创作声明</dt>
              <dd>{{ content.declaration || "待确认" }}</dd>
            </div>
            <div v-if="content.platform !== '百家号'">
              <dt>可见范围</dt>
              <dd>{{ content.visibility || "公开" }}</dd>
            </div>
            <div>
              <dt>发布时间</dt>
              <dd>{{ content.schedule || "启动后尽快提交" }}</dd>
            </div>
          </dl>
        </div>
      </div>
    </section>
    <section
      v-if="task.attempts > 0 || task.log?.length"
      ref="history"
      class="task-section"
      tabindex="-1"
    >
      <h3>{{ task.status === "排队中" ? "排队与执行记录" : "执行进展" }}</h3>
      <p v-if="task.status === '排队中'">
        等待账号 {{ account?.name }} · 窗口
        {{ account?.window }}；同账号串行执行。
      </p>
      <ol v-if="task.log?.length" class="task-timeline">
        <li v-for="(log, i) in task.log.slice(-4)" :key="i">
          <time>{{
            new Date(log.time).toLocaleTimeString("zh-CN", {
              timeZone: "Asia/Shanghai",
            })
          }}</time
          ><span>{{ log.text }}</span>
        </li>
      </ol>
      <p v-else>历史样例记录 {{ task.attempts }} 次尝试，未保留步骤明细。</p>
      <details v-if="task.attemptHistory?.length">
        <summary>全部执行尝试 · {{ task.attemptHistory.length }} 次</summary>
        <div
          v-for="attempt in task.attemptHistory"
          :key="attempt.number"
          class="task-attempt"
        >
          <strong
            >第 {{ attempt.number }} 次 · 内容 v{{ attempt.version }} ·
            {{ attempt.status }}</strong
          >
          <p>{{ attempt.started }} → {{ attempt.ended || "尚未结束" }}</p>
          <p>{{ attempt.errorCode }} {{ attempt.receipt }}</p>
          <p>当次标题：{{ attempt.snapshot?.title }}</p>
          <p v-for="(step, i) in attempt.steps" :key="i">
            {{ step.time }} · {{ step.status }} · {{ step.evidence }}
          </p>
        </div>
      </details>
    </section>
    <details
      v-if="['排队中', '执行中', '等待人工'].includes(task.status)"
      class="demo-controls"
    >
      <summary>本地演示 · 推进执行场景</summary>
      <p>仅切换原型记录，不执行真实平台操作。</p>
      <div class="task-demo-actions">
        <t-button
          v-for="status in task.status === '排队中'
            ? ['执行中']
            : task.status === '等待人工'
              ? ['结果待确认']
              : ['等待人工', '结果待确认', '平台审核中', '执行失败']"
          :key="status"
          variant="outline"
          @click="emit('transition', task, status)"
          >{{ status === "执行失败" ? "确认失败" : status }}</t-button
        >
      </div>
    </details>
  </article>
  <Overlay v-model:visible="humanOpen" title="处理人工验证" :width="600"
    ><h3>{{ account?.name }} · 窗口 {{ account?.window }}</h3>
    <p>
      {{
        task.evidence || "请在原账号会话完成登录或验证，保留上传与提交现场。"
      }}
    </p>
    <p>完成后从安全节点继续；若已发生提交，先核实稿件结果。</p>
    <p class="muted">本地原型仅演示处理后的状态，不会打开真实账号窗口。</p>
    <template #footer
      ><t-button variant="outline" @click="humanOpen = false">稍后处理</t-button
      ><t-button
        @click="
          emit('resume', task);
          humanOpen = false;
        "
        >已完成处理，继续任务</t-button
      ></template
    ></Overlay
  >
  <Overlay v-model:visible="verifyOpen" title="核实平台结果" :width="640"
    ><p>{{ account?.name }} · {{ task.platform }} · 成片 #{{ task.assetId }}</p>
    <p>
      先查看同账号已有稿件，再记录查到的结果。结果仍不明确时保留当前状态与占用。
    </p>
    <label class="verify-field"
      >核验依据<t-textarea
        v-model="verifyEvidence"
        placeholder="作品链接、稿件标识或明确未发布的核实依据" /></label
    ><label class="verify-field"
      >核实结果<t-radio-group v-model="verifyResult"
        ><t-radio value="结果待确认">仍不明确</t-radio
        ><t-radio value="平台审核中">已接收，审核中</t-radio
        ><t-radio value="已发布">已公开发布</t-radio
        ><t-radio value="执行失败"
          >确认未发布且执行已结束</t-radio
        ></t-radio-group
      ></label
    ><t-alert v-if="verifyError" theme="warning" :message="verifyError" />
    <p class="muted">
      本地演示不自动查询平台；保存此处信息不构成真实平台验收。
    </p>
    <template #footer
      ><t-button variant="outline" @click="verifyOpen = false">取消</t-button
      ><t-button @click="confirmResult">保存核实结果</t-button></template
    ></Overlay
  >
</template>
<style scoped>
.task-detail {
  color: #263b57;
}
.task-breadcrumb {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  color: #6c7d94;
  font-size: 12px;
  margin-bottom: 18px;
}
.task-breadcrumb small {
  flex-basis: 100%;
  padding-top: 4px;
}
.task-heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 22px;
}
.task-heading h2 {
  font-size: 23px;
  line-height: 1.4;
  margin: 8px 0;
  overflow-wrap: anywhere;
}
.task-heading p,
.task-heading small {
  color: #708097;
  margin: 0;
  font-size: 12px;
}
.task-next {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
  background: #edf4ff;
  border: 1px solid #cfdef7;
  border-radius: 10px;
  padding: 22px;
  margin-bottom: 20px;
}
.task-next.attention {
  background: #fffaef;
  border-color: #efdfb8;
}
.task-next.done {
  background: #eff9f4;
  border-color: #cde8da;
}
.task-eyebrow {
  font-size: 11px;
  color: #647d9c;
}
.task-next h3 {
  font-size: 18px;
  margin: 5px 0 8px;
}
.task-next p {
  font-size: 13px;
  line-height: 1.7;
  margin: 0;
  color: #5c708b;
}
.task-next-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}
.task-section {
  padding: 22px 0;
  border-bottom: 1px solid #e7ecf2;
  scroll-margin-top: 20px;
}
.task-section h3 {
  font-size: 15px;
  margin: 0 0 15px;
}
.task-section-title {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
}
.task-section-title span,
.task-section p {
  font-size: 13px;
  line-height: 1.7;
}
.task-section-title span {
  color: #77879b;
  font-size: 12px;
}
.task-checks {
  border: 1px solid #e3e9f2;
  border-radius: 8px;
  padding: 18px;
}
.task-checks ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.task-checks li {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 15px;
  padding: 4px 0;
  font-size: 13px;
}
.task-checks p {
  margin: 0;
  color: #527c65;
}
.task-content {
  display: grid;
  grid-template-columns: 210px minmax(0, 1fr);
  gap: 24px;
}
.task-cover {
  border: 0;
  padding: 0;
  align-self: start;
  cursor: zoom-in;
  background: #f3f6fa;
  border-radius: 8px;
  overflow: hidden;
  color: #4870a5;
}
.task-cover img {
  width: 100%;
  display: block;
  aspect-ratio: 4/3;
  object-fit: contain;
}
.task-cover span {
  display: block;
  padding: 9px;
  font-size: 11px;
}
.task-content h4 {
  font-size: 12px;
  color: #74839a;
  font-weight: 500;
  margin: 0 0 6px;
}
.task-content p {
  margin: 0 0 14px;
  white-space: pre-wrap;
}
.task-facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 15px;
  margin: 12px 0;
}
.task-facts dt {
  font-size: 12px;
  color: #8190a3;
  margin-bottom: 5px;
}
.task-facts dd {
  margin: 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.task-timeline {
  list-style: none;
  margin: 0 0 15px;
  padding: 0;
}
.task-timeline li {
  display: flex;
  gap: 18px;
  padding: 9px 0;
  border-bottom: 1px solid #edf0f4;
  font-size: 12px;
}
.task-timeline time {
  color: #8b98aa;
  flex: none;
}
.task-section summary {
  font-size: 13px;
  color: #416c9f;
  cursor: pointer;
  padding: 10px 0;
}
.task-attempt {
  padding: 12px;
  background: #f7f9fc;
  margin: 8px 0;
  border-radius: 6px;
}
.task-attempt p {
  font-size: 12px;
  margin: 6px 0;
}
.task-demo-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.verify-field {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 18px 0;
}
.verify-field .t-radio-group {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}
pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
a {
  color: #286bd5;
  overflow-wrap: anywhere;
}
@media (max-width: 700px) {
  .task-next {
    align-items: stretch;
    flex-direction: column;
    padding: 16px;
  }
  .task-next-actions {
    align-self: flex-start;
  }
  .task-content {
    grid-template-columns: 1fr;
  }
  .task-cover {
    max-width: 240px;
  }
  .task-facts {
    grid-template-columns: 1fr;
  }
  .task-heading h2 {
    font-size: 20px;
  }
  .task-timeline li {
    flex-direction: column;
    gap: 5px;
  }
}
</style>
